// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/appvia/ipam"
)

var (
	_ resource.ResourceWithConfigure      = &poolResource{}
	_ resource.ResourceWithImportState    = &poolResource{}
	_ resource.ResourceWithIdentity       = &poolResource{}
	_ resource.ResourceWithValidateConfig = &poolResource{}
)

var poolIDPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

type poolResource struct {
	data *Data
}

type poolResourceModel struct {
	ID           types.String `tfsdk:"id"`
	CIDRs        types.Set    `tfsdk:"cidrs"`
	ParentPool   types.String `tfsdk:"parent_pool"`
	PrefixLength types.Int64  `tfsdk:"prefix_length"`
	CIDR         types.String `tfsdk:"cidr"`
	Family       types.Int64  `tfsdk:"family"`
	Description  types.String `tfsdk:"description"`
	Labels       types.Map    `tfsdk:"labels"`
	Strategy     types.String `tfsdk:"strategy"`
	Cooldown     types.Object `tfsdk:"cooldown"`
}

type cooldownModel struct {
	Mode     types.String `tfsdk:"mode"`
	Duration types.String `tfsdk:"duration"`
}

var cooldownAttrTypes = map[string]attr.Type{"mode": types.StringType, "duration": types.StringType}

type poolIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

// NewPoolResource returns the ipam_pool resource.
func NewPoolResource() resource.Resource {
	return &poolResource{}
}

func (r *poolResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pool"
}

func poolIDValidators() []validator.String {
	return []validator.String{stringvalidator.RegexMatches(poolIDPattern,
		"must be 1-63 lowercase letters, digits and hyphens, starting and ending with a letter or digit")}
}

func (r *poolResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A pool of IP ranges. A root pool lists its `cidrs`; a child pool is carved from " +
			"`parent_pool` with either `prefix_length` or an explicit `cidr`. Only `description`, " +
			"`labels`, `strategy` and `cooldown` update in place; every other change replaces the pool. " +
			"A pool that still holds " +
			"allocations or child pools cannot be destroyed.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:      true,
				Description:   "Pool ID: 1-63 lowercase letters, digits and hyphens.",
				Validators:    poolIDValidators(),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"cidrs": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Description: "Canonical CIDRs of a root pool (one address family, no overlaps). " +
					"For a child pool this is computed: the single CIDR carved from the parent.",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
					setvalidator.ValueStringsAre(canonicalCIDR{}),
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
					setplanmodifier.RequiresReplace(),
				},
			},
			"parent_pool": schema.StringAttribute{
				Optional:      true,
				Description:   "ID of the pool to carve this child pool from.",
				Validators:    poolIDValidators(),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"prefix_length": schema.Int64Attribute{
				Optional:      true,
				Description:   "Carve the next free CIDR of this prefix length from `parent_pool`.",
				Validators:    []validator.Int64{int64validator.Between(0, 128)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"cidr": schema.StringAttribute{
				Optional:      true,
				Description:   "Carve exactly this canonical CIDR from `parent_pool`.",
				Validators:    []validator.String{canonicalCIDR{}},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"family": schema.Int64Attribute{
				Computed:      true,
				Description:   "Address family: 4 or 6.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Free-text description. Updated in place.",
			},
			"labels": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Free-form labels. Updated in place.",
			},
			"strategy": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Allocation strategy for `next` requests: `best-fit` (smallest free block that " +
					"fits) or `lowest` (lowest free address). Defaults to `best-fit` for new pools; an existing " +
					"or adopted pool keeps its stored strategy while `strategy` is omitted, and a pool re-created " +
					"by replacement gets `best-fit` unless `strategy` is set. Updated in place. A pool with " +
					"`best-fit` or a cooldown is stored as a v2 document that the ipam CLI and library before " +
					"v0.3.0 and terraform-provider-ipam v0.1.x cannot read.",
				Validators: []validator.String{
					stringvalidator.OneOf(string(ipam.StrategyLowest), string(ipam.StrategyBestFit)),
				},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cooldown": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Holds released CIDRs back from `next` requests. Omit for no cooldown. Updated in place.",
				Attributes: map[string]schema.Attribute{
					"mode": schema.StringAttribute{
						Required: true,
						Description: "`strict` refuses a `next` request while only cooling space fits it; " +
							"`fit-aware` avoids cooling space only when an equally tight free block exists. " +
							"Explicit `cidr` requests bypass cooldown.",
						Validators: []validator.String{
							stringvalidator.OneOf(string(ipam.CooldownStrict), string(ipam.CooldownFitAware)),
						},
					},
					"duration": schema.StringAttribute{
						Required: true,
						Description: "How long a released CIDR cools, as a Go duration in whole seconds " +
							"between 1s and 8760h, for example `24h`.",
						Validators: []validator.String{cooldownDuration{}},
					},
				},
			},
		},
	}
}

func (r *poolResource) IdentitySchema(
	_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse,
) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"id": identityschema.StringAttribute{RequiredForImport: true, Description: "Pool ID."},
		},
	}
}

func (r *poolResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.data = providerData(req.ProviderData, &resp.Diagnostics)
}

func providerData(raw any, diags *diag.Diagnostics) *Data {
	if raw == nil {
		return nil
	}

	data, ok := raw.(*Data)
	if !ok {
		diags.AddError("Unexpected provider data",
			fmt.Sprintf("Expected *provider.Data, got %T. This is a bug in the provider.", raw))

		return nil
	}

	return data
}

func (r *poolResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var m poolResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)

	if resp.Diagnostics.HasError() || m.ParentPool.IsUnknown() {
		return
	}

	invalid := func(attr, detail string) {
		resp.Diagnostics.AddAttributeError(path.Root(attr), "Invalid ipam_pool configuration", detail)
	}

	if m.ParentPool.IsNull() {
		if !m.PrefixLength.IsNull() {
			invalid("prefix_length", "prefix_length cannot be configured without parent_pool.")
		}

		if !m.CIDR.IsNull() {
			invalid("cidr", "cidr cannot be configured without parent_pool.")
		}

		if m.CIDRs.IsNull() {
			invalid("cidrs", "cidrs must be configured for a root pool (a pool without parent_pool).")
		}

		return
	}

	if !m.CIDRs.IsNull() {
		invalid("cidrs", "cidrs cannot be configured together with parent_pool: a child pool's CIDR is "+
			"carved from its parent with prefix_length or cidr.")
	}

	if m.PrefixLength.IsUnknown() || m.CIDR.IsUnknown() {
		return
	}

	if m.PrefixLength.IsNull() == m.CIDR.IsNull() {
		invalid("parent_pool", "exactly one of prefix_length or cidr must be configured with parent_pool.")
	}
}

func (r *poolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan poolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	spec, diags := plan.spec(ctx)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	mctx, cancel := r.data.MutationContext(ctx)
	defer cancel()

	pool, err := r.data.Client.CreatePool(mctx, spec)
	if err != nil {
		resp.Diagnostics.Append(errorDiagnostic("Create pool", err))

		return
	}

	// An omitted cooldown matches any stored one, so CreatePool adopted an
	// existing pool whose cooldown the configuration does not declare.
	if plan.Cooldown.IsNull() && pool.Policy.Cooldown.Mode != ipam.CooldownNone {
		resp.Diagnostics.Append(errorDiagnostic("Create pool", &ipam.OpError{
			Op: "create pool", Pool: pool.ID, Reason: "pool_exists", Err: ipam.ErrConflict,
		}))

		return
	}

	resp.Diagnostics.Append(plan.setPool(ctx, pool)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	setIdentity(ctx, resp.Identity, plan.ID, &resp.Diagnostics)
}

func (r *poolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state poolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	pool, err := r.data.Client.GetPool(ctx, state.ID.ValueString())
	if errors.Is(err, ipam.ErrNotFound) {
		resp.State.RemoveResource(ctx)

		return
	}

	if err != nil {
		resp.Diagnostics.Append(errorDiagnostic("Read pool", err))

		return
	}

	resp.Diagnostics.Append(state.setPool(ctx, pool)...)

	if pool.Parent != nil && state.PrefixLength.IsNull() && state.CIDR.IsNull() {
		resp.Diagnostics.Append(r.readRequest(ctx, pool.Parent, &state)...)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
	setIdentity(ctx, resp.Identity, state.ID, &resp.Diagnostics)
}

// readRequest restores prefix_length or cidr of an imported child pool from
// its carve-out in the parent.
func (r *poolResource) readRequest(ctx context.Context, parent *ipam.ParentRef, state *poolResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	carveOut, err := r.data.Client.GetAllocation(ctx, parent.PoolID, parent.AllocationID)
	if err != nil {
		diags.Append(errorDiagnostic("Read pool", err))

		return diags
	}

	switch carveOut.Request.Kind {
	case ipam.RequestExplicit:
		state.CIDR = types.StringValue(carveOut.Request.CIDR.String())
	case ipam.RequestNext:
		state.PrefixLength = types.Int64Value(int64(carveOut.Request.PrefixLength))
	}

	return diags
}

func (r *poolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state poolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	labels, diags := plan.labels(ctx)
	resp.Diagnostics.Append(diags...)

	planPolicy, planDiags := plan.policy(ctx)
	resp.Diagnostics.Append(planDiags...)

	statePolicy, stateDiags := state.policy(ctx)
	resp.Diagnostics.Append(stateDiags...)

	if resp.Diagnostics.HasError() {
		return
	}

	// An omitted strategy keeps the stored one.
	if planPolicy.Strategy == ipam.StrategyDefault {
		planPolicy.Strategy = statePolicy.Strategy
	}

	mctx, cancel := r.data.MutationContext(ctx)
	defer cancel()

	err := r.data.Client.UpdatePoolMetadata(mctx, plan.ID.ValueString(), plan.Description.ValueString(), labels)
	if err != nil {
		resp.Diagnostics.Append(errorDiagnostic("Update pool", err))

		return
	}

	// Rewriting an unchanged policy would turn a v1 pool document into v2.
	if planPolicy != statePolicy {
		err = r.updatePolicy(mctx, plan.ID.ValueString(), planPolicy)
		if err != nil {
			resp.Diagnostics.Append(errorDiagnostic("Update pool policy", err))

			return
		}
	}

	// plan.Strategy stays as planned: a planned null must remain null in state,
	// and the next refresh fills it in.
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
	setIdentity(ctx, resp.Identity, plan.ID, &resp.Diagnostics)
}

// updatePolicy writes policy, reading the stored strategy when neither plan nor
// state knows it (state written by v0.1.x and applied with -refresh=false).
func (r *poolResource) updatePolicy(ctx context.Context, id string, policy ipam.PoolPolicy) error {
	if policy.Strategy == ipam.StrategyDefault {
		pool, err := r.data.Client.GetPool(ctx, id)
		if err != nil {
			return err
		}

		policy.Strategy = pool.Policy.Strategy
	}

	return r.data.Client.UpdatePoolPolicy(ctx, id, policy)
}

func (r *poolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state poolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	mctx, cancel := r.data.MutationContext(ctx)
	defer cancel()

	err := r.data.Client.DeletePool(mctx, state.ID.ValueString())
	if err != nil && !alreadyGone(err) {
		resp.Diagnostics.Append(errorDiagnostic("Delete pool", err))
	}
}

// alreadyGone reports a plain not-found; residue reported as not-found with a
// reason (pool_incomplete) still needs operator attention.
func alreadyGone(err error) bool {
	var opErr *ipam.OpError

	return errors.Is(err, ipam.ErrNotFound) && (!errors.As(err, &opErr) || opErr.Reason == "")
}

func (r *poolResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("id"), req, resp)
}

func setIdentity(ctx context.Context, identity *tfsdk.ResourceIdentity, id types.String, diags *diag.Diagnostics) {
	if identity == nil {
		return
	}

	diags.Append(identity.Set(ctx, poolIdentityModel{ID: id})...)
}

func (m poolResourceModel) labels(ctx context.Context) (map[string]string, diag.Diagnostics) {
	if m.Labels.IsNull() {
		return nil, nil
	}

	labels := map[string]string{}
	diags := m.Labels.ElementsAs(ctx, &labels, false)

	return labels, diags
}

// policy returns the configured policy: a null or unknown strategy is
// StrategyDefault and a null cooldown is no cooldown.
func (m poolResourceModel) policy(ctx context.Context) (ipam.PoolPolicy, diag.Diagnostics) {
	var (
		policy ipam.PoolPolicy
		diags  diag.Diagnostics
	)

	if !m.Strategy.IsNull() && !m.Strategy.IsUnknown() {
		policy.Strategy = ipam.AllocationStrategy(m.Strategy.ValueString())
	}

	if m.Cooldown.IsNull() || m.Cooldown.IsUnknown() {
		return policy, diags
	}

	var cooldown cooldownModel

	diags.Append(m.Cooldown.As(ctx, &cooldown, basetypes.ObjectAsOptions{})...)

	duration, err := time.ParseDuration(cooldown.Duration.ValueString())
	if err != nil {
		diags.AddAttributeError(path.Root("cooldown").AtName("duration"), "Invalid cooldown duration", err.Error())

		return policy, diags
	}

	policy.Cooldown = ipam.Cooldown{Mode: ipam.CooldownMode(cooldown.Mode.ValueString()), Duration: duration}

	return policy, diags
}

func (m poolResourceModel) spec(ctx context.Context) (ipam.PoolSpec, diag.Diagnostics) {
	labels, diags := m.labels(ctx)
	policy, policyDiags := m.policy(ctx)
	diags.Append(policyDiags...)

	spec := ipam.PoolSpec{
		ID:          m.ID.ValueString(),
		ParentPool:  m.ParentPool.ValueString(),
		Description: m.Description.ValueString(),
		Labels:      labels,
		Policy:      policy,
	}

	if spec.ParentPool == "" {
		var raw []string

		diags.Append(m.CIDRs.ElementsAs(ctx, &raw, false)...)

		for _, s := range raw {
			prefix, err := netip.ParsePrefix(s)
			if err != nil {
				diags.AddAttributeError(path.Root("cidrs"), "Invalid CIDR", err.Error())

				continue
			}

			spec.CIDRs = append(spec.CIDRs, prefix)
		}

		return spec, diags
	}

	if m.CIDR.IsNull() {
		spec.Request = ipam.Request{Kind: ipam.RequestNext, PrefixLength: int(m.PrefixLength.ValueInt64())}

		return spec, diags
	}

	prefix, err := netip.ParsePrefix(m.CIDR.ValueString())
	if err != nil {
		diags.AddAttributeError(path.Root("cidr"), "Invalid CIDR", err.Error())
	}

	spec.Request = ipam.Request{Kind: ipam.RequestExplicit, CIDR: prefix}

	return spec, diags
}

// setPool copies the stored pool into the model. Empty description and labels
// keep the model's null-or-empty form so a config that omits them stays stable.
func (m *poolResourceModel) setPool(ctx context.Context, pool ipam.Pool) diag.Diagnostics {
	var diags diag.Diagnostics

	cidrs := make([]string, 0, len(pool.CIDRs))
	for _, prefix := range pool.CIDRs {
		cidrs = append(cidrs, prefix.String())
	}

	m.ID = types.StringValue(pool.ID)
	m.CIDRs, diags = types.SetValueFrom(ctx, types.StringType, cidrs)
	m.Family = types.Int64Value(int64(pool.Family))

	m.ParentPool = types.StringNull()
	if pool.Parent != nil {
		m.ParentPool = types.StringValue(pool.Parent.PoolID)
	}

	if pool.Description != "" || !m.Description.IsNull() {
		m.Description = types.StringValue(pool.Description)
	}

	if len(pool.Labels) > 0 || (!m.Labels.IsNull() && !m.Labels.IsUnknown()) {
		labels, labelDiags := types.MapValueFrom(ctx, types.StringType, pool.Labels)
		diags.Append(labelDiags...)

		m.Labels = labels
	} else {
		m.Labels = types.MapNull(types.StringType)
	}

	m.Strategy = types.StringValue(string(pool.Policy.Strategy))

	diags.Append(m.setCooldown(ctx, pool.Policy.Cooldown)...)

	return diags
}

// setCooldown copies the stored cooldown into the model, keeping the model's
// duration spelling ("24h" rather than "24h0m0s") while it means the same.
func (m *poolResourceModel) setCooldown(ctx context.Context, cooldown ipam.Cooldown) diag.Diagnostics {
	if cooldown.Mode == ipam.CooldownNone {
		m.Cooldown = types.ObjectNull(cooldownAttrTypes)

		return nil
	}

	var diags diag.Diagnostics

	duration := cooldown.Duration.String()

	if !m.Cooldown.IsNull() && !m.Cooldown.IsUnknown() {
		var prior cooldownModel

		diags.Append(m.Cooldown.As(ctx, &prior, basetypes.ObjectAsOptions{})...)

		parsed, err := time.ParseDuration(prior.Duration.ValueString())
		if err == nil && parsed == cooldown.Duration {
			duration = prior.Duration.ValueString()
		}
	}

	object, objectDiags := types.ObjectValue(cooldownAttrTypes, map[string]attr.Value{
		"mode":     types.StringValue(string(cooldown.Mode)),
		"duration": types.StringValue(duration),
	})
	diags.Append(objectDiags...)

	m.Cooldown = object

	return diags
}

// cooldownDuration requires a Go duration of whole seconds within the bounds
// the ipam library accepts.
type cooldownDuration struct{}

func (cooldownDuration) Description(context.Context) string {
	return "value must be a whole number of seconds between 1s and 8760h, for example 24h"
}

func (v cooldownDuration) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }

func (cooldownDuration) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	raw := req.ConfigValue.ValueString()

	d, err := time.ParseDuration(raw)
	if err != nil {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid cooldown duration",
			fmt.Sprintf("%q is not a Go duration such as 24h.", raw))

		return
	}

	if d%time.Second != 0 || d < time.Second || d > 8760*time.Hour {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid cooldown duration",
			fmt.Sprintf("%q must be a whole number of seconds between 1s and 8760h, for example 24h.", raw))
	}
}

// canonicalCIDR requires the form the ipam library stores (masked, lowercase),
// so the state always equals the configuration.
type canonicalCIDR struct{}

func (canonicalCIDR) Description(context.Context) string {
	return "value must be a canonical CIDR such as 10.0.0.0/16 or 2001:db8::/32"
}

func (v canonicalCIDR) MarkdownDescription(ctx context.Context) string { return v.Description(ctx) }

func (canonicalCIDR) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	raw := req.ConfigValue.ValueString()

	prefix, err := netip.ParsePrefix(raw)
	if err != nil || prefix.Addr().Is4In6() || prefix.Masked().String() != raw {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid CIDR",
			fmt.Sprintf("%q is not a canonical CIDR: use the network address and lowercase hex, for "+
				"example 10.0.0.0/16 or 2001:db8::/32.", raw))
	}
}
