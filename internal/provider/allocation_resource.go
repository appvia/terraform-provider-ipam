// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/appvia/ipam"
)

var (
	_ resource.ResourceWithConfigure      = &allocationResource{}
	_ resource.ResourceWithImportState    = &allocationResource{}
	_ resource.ResourceWithIdentity       = &allocationResource{}
	_ resource.ResourceWithValidateConfig = &allocationResource{}
)

type allocationResource struct {
	data *Data
}

type allocationResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Pool         types.String `tfsdk:"pool"`
	PrefixLength types.Int64  `tfsdk:"prefix_length"`
	CIDR         types.String `tfsdk:"cidr"`
	Description  types.String `tfsdk:"description"`
	Labels       types.Map    `tfsdk:"labels"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

type allocationIdentityModel struct {
	Pool types.String `tfsdk:"pool"`
	ID   types.String `tfsdk:"id"`
}

// NewAllocationResource returns the ipam_allocation resource.
func NewAllocationResource() resource.Resource {
	return &allocationResource{}
}

func (r *allocationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allocation"
}

func (r *allocationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "One CIDR allocated from a pool: the next free CIDR of `prefix_length`, or exactly " +
			"`cidr`. The allocation ID is generated when it is created. Only `description` and `labels` " +
			"update in place; every other change replaces the allocation. Destroy releases the CIDR.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Allocation ID: a UUIDv7 generated when the allocation is created.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"pool": schema.StringAttribute{
				Required:      true,
				Description:   "ID of the pool to allocate from.",
				Validators:    poolIDValidators(),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"prefix_length": schema.Int64Attribute{
				Optional:      true,
				Description:   "Allocate the next free CIDR of this prefix length. Conflicts with `cidr`.",
				Validators:    []validator.Int64{int64validator.Between(0, 128)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"cidr": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "The allocated CIDR. Set it to allocate exactly this canonical CIDR " +
					"(a reservation); otherwise it is computed. Conflicts with `prefix_length`.",
				Validators: []validator.String{canonicalCIDR{}},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Free-text description. Updated in place.",
			},
			"labels": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Free-form labels, for example the owning team. Updated in place.",
			},
			"created_at": schema.StringAttribute{
				Computed:      true,
				Description:   "When the allocation was created (RFC 3339).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *allocationResource) IdentitySchema(
	_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse,
) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"pool": identityschema.StringAttribute{RequiredForImport: true, Description: "Pool ID."},
			"id":   identityschema.StringAttribute{RequiredForImport: true, Description: "Allocation ID."},
		},
	}
}

func (r *allocationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.data = providerData(req.ProviderData, &resp.Diagnostics)
}

// ValidateConfig sees only the configuration, so a computed cidr never counts
// as requested.
func (r *allocationResource) ValidateConfig(
	ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse,
) {
	var m allocationResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)

	if resp.Diagnostics.HasError() || m.PrefixLength.IsUnknown() || m.CIDR.IsUnknown() {
		return
	}

	if m.PrefixLength.IsNull() == m.CIDR.IsNull() {
		resp.Diagnostics.AddAttributeError(path.Root("prefix_length"), "Invalid ipam_allocation configuration",
			"exactly one of prefix_length or cidr must be configured.")
	}
}

func (r *allocationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan allocationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	spec, diags := plan.spec(ctx)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Generated once: every retry below reuses it, so a retry after a lost
	// response finds the committed allocation instead of making a second one.
	spec.ID = ipam.NewAllocationID()
	plan.ID = types.StringValue(spec.ID)

	mctx, cancel := r.data.MutationContext(ctx)
	defer cancel()

	allocation, err := r.data.Client.Allocate(mctx, plan.Pool.ValueString(), spec)
	if err != nil {
		resp.Diagnostics.Append(errorDiagnostic("Create allocation", err))

		if mayHaveCommitted(err) {
			// Keep the id so Terraform records the (tainted) allocation and a
			// refresh or destroy reaches it, rather than leaking it.
			plan.unknownToNull()
			resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
			setAllocationIdentity(ctx, resp.Identity, plan, &resp.Diagnostics)
		}

		return
	}

	resp.Diagnostics.Append(plan.setAllocation(ctx, allocation)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	setAllocationIdentity(ctx, resp.Identity, plan, &resp.Diagnostics)
}

// mayHaveCommitted reports whether a failed Allocate can have written the
// allocation. Only failures decided before any write are definite.
func mayHaveCommitted(err error) bool {
	var opErr *ipam.OpError
	if errors.As(err, &opErr) && (opErr.Reason == "overlap" || opErr.Reason == "id_reused") {
		return false
	}

	for _, definite := range []error{
		ipam.ErrPoolExhausted, ipam.ErrNotFound, ipam.ErrInvalidArgument,
		ipam.ErrInvalidCIDR, ipam.ErrUnsupportedSchema,
	} {
		if errors.Is(err, definite) {
			return false
		}
	}

	return true
}

func (r *allocationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state allocationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	allocation, err := r.data.Client.GetAllocation(ctx, state.Pool.ValueString(), state.ID.ValueString())
	if errors.Is(err, ipam.ErrNotFound) {
		resp.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		resp.Diagnostics.Append(errorDiagnostic("Read allocation", err))

		return
	}

	resp.Diagnostics.Append(state.setAllocation(ctx, allocation)...)

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	setAllocationIdentity(ctx, resp.Identity, state, &resp.Diagnostics)
}

func (r *allocationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan allocationResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	labels, diags := modelLabels(ctx, plan.Labels)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	mctx, cancel := r.data.MutationContext(ctx)
	defer cancel()

	err := r.data.Client.UpdateAllocationMetadata(mctx, plan.Pool.ValueString(), plan.ID.ValueString(),
		plan.Description.ValueString(), labels)
	if err != nil {
		resp.Diagnostics.Append(errorDiagnostic("Update allocation", err))

		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	setAllocationIdentity(ctx, resp.Identity, plan, &resp.Diagnostics)
}

func (r *allocationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state allocationResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	mctx, cancel := r.data.MutationContext(ctx)
	defer cancel()

	err := r.data.Client.Release(mctx, state.Pool.ValueString(), state.ID.ValueString())
	if err != nil && !alreadyGone(err) {
		resp.Diagnostics.Append(errorDiagnostic("Delete allocation", err))
	}
}

const allocationImportFormat = "Expected an import ID of the form <pool>/<id>"

// parseAllocationImportID splits on the first "/": pool IDs cannot contain one.
func parseAllocationImportID(raw string) (pool, id string, err error) {
	pool, id, found := strings.Cut(raw, "/")
	if !found || pool == "" || id == "" {
		return "", "", fmt.Errorf("%s, for example prod/018f22e2-1865-7be8-9f45-35d90607d0f7; got %q",
			allocationImportFormat, raw)
	}

	return pool, id, nil
}

func (r *allocationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	identity := allocationIdentityModel{}

	if req.ID != "" {
		pool, id, err := parseAllocationImportID(req.ID)
		if err != nil {
			resp.Diagnostics.AddError("Invalid import ID", err.Error())

			return
		}

		identity = allocationIdentityModel{Pool: types.StringValue(pool), ID: types.StringValue(id)}
	} else {
		resp.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)

		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("pool"), identity.Pool)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), identity.ID)...)

	if resp.Identity != nil {
		resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
	}
}

func setAllocationIdentity(
	ctx context.Context, identity *tfsdk.ResourceIdentity, m allocationResourceModel, diags *diag.Diagnostics,
) {
	if identity == nil {
		return
	}

	diags.Append(identity.Set(ctx, allocationIdentityModel{Pool: m.Pool, ID: m.ID})...)
}

func modelLabels(ctx context.Context, value types.Map) (map[string]string, diag.Diagnostics) {
	if value.IsNull() {
		return nil, nil
	}

	labels := map[string]string{}
	diags := value.ElementsAs(ctx, &labels, false)

	return labels, diags
}

func (m allocationResourceModel) spec(ctx context.Context) (ipam.AllocationSpec, diag.Diagnostics) {
	labels, diags := modelLabels(ctx, m.Labels)

	spec := ipam.AllocationSpec{Description: m.Description.ValueString(), Labels: labels}

	if m.PrefixLength.IsNull() {
		prefix, err := netip.ParsePrefix(m.CIDR.ValueString())
		if err != nil {
			diags.AddAttributeError(path.Root("cidr"), "Invalid CIDR", err.Error())
		}

		spec.CIDR = prefix

		return spec, diags
	}

	spec.PrefixLength = int(m.PrefixLength.ValueInt64())

	return spec, diags
}

// unknownToNull clears values only the store can supply, so a partial state
// after an uncertain create is fully known.
func (m *allocationResourceModel) unknownToNull() {
	if m.CIDR.IsUnknown() {
		m.CIDR = types.StringNull()
	}

	if m.CreatedAt.IsUnknown() {
		m.CreatedAt = types.StringNull()
	}
}

// setAllocation copies the stored allocation into the model. The request kind
// decides between prefix_length and an explicit cidr, so an import matches the
// configuration that created it. Empty description and labels keep the
// model's null-or-empty form so a config that omits them stays stable.
func (m *allocationResourceModel) setAllocation(ctx context.Context, allocation ipam.Allocation) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(allocation.ID)
	m.Pool = types.StringValue(allocation.PoolID)
	m.CIDR = types.StringValue(allocation.CIDR.String())
	m.CreatedAt = types.StringValue(allocation.CreatedAt.UTC().Format(time.RFC3339Nano))

	m.PrefixLength = types.Int64Null()
	if allocation.Request.Kind == ipam.RequestNext {
		m.PrefixLength = types.Int64Value(int64(allocation.Request.PrefixLength))
	}

	if allocation.Description != "" || !m.Description.IsNull() {
		m.Description = types.StringValue(allocation.Description)
	}

	if len(allocation.Labels) > 0 || (!m.Labels.IsNull() && !m.Labels.IsUnknown()) {
		m.Labels, diags = types.MapValueFrom(ctx, types.StringType, allocation.Labels)
	} else {
		m.Labels = types.MapNull(types.StringType)
	}

	return diags
}
