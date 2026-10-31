// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSourceWithConfigure = &poolDataSource{}

type poolDataSource struct {
	data *Data
}

type poolDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	CIDRs       types.Set    `tfsdk:"cidrs"`
	Family      types.Int64  `tfsdk:"family"`
	ParentPool  types.String `tfsdk:"parent_pool"`
	Description types.String `tfsdk:"description"`
	Labels      types.Map    `tfsdk:"labels"`
	Allocations types.Map    `tfsdk:"allocations"`
	Strategy    types.String `tfsdk:"strategy"`
	Cooldown    types.Object `tfsdk:"cooldown"`
}

// NewPoolDataSource returns the ipam_pool data source.
func NewPoolDataSource() datasource.DataSource {
	return &poolDataSource{}
}

func (d *poolDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pool"
}

func (d *poolDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads a pool and its current allocations.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Required:    true,
				Description: "Pool ID.",
				Validators:  []validator.String{poolIDValidators()[0]},
			},
			"cidrs": schema.SetAttribute{
				ElementType: types.StringType, Computed: true, Description: "The pool's CIDRs.",
			},
			"family":      schema.Int64Attribute{Computed: true, Description: "Address family: 4 or 6."},
			"parent_pool": schema.StringAttribute{Computed: true, Description: "Parent pool ID; null for a root pool."},
			"description": schema.StringAttribute{Computed: true, Description: "Description."},
			"labels": schema.MapAttribute{
				ElementType: types.StringType, Computed: true, Description: "Labels.",
			},
			"allocations": schema.MapAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Allocation ID to CIDR, including child-pool carve-outs (IDs `pool:<child>:<incarnation>`).",
			},
			"strategy": schema.StringAttribute{
				Computed: true, Description: "Allocation strategy: `lowest` or `best-fit`.",
			},
			"cooldown": schema.SingleNestedAttribute{
				Computed: true,
				Description: "Cooldown for released CIDRs, duration in Go canonical form such as `24h0m0s`; " +
					"null when none.",
				Attributes: map[string]schema.Attribute{
					"mode":     schema.StringAttribute{Computed: true, Description: "`strict` or `fit-aware`."},
					"duration": schema.StringAttribute{Computed: true, Description: "Go duration, such as `24h0m0s`."},
				},
			},
		},
	}
}

func (d *poolDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.data = providerData(req.ProviderData, &resp.Diagnostics)
}

func (d *poolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m poolDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)

	if resp.Diagnostics.HasError() {
		return
	}

	pool, err := d.data.Client.GetPool(ctx, m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.Append(errorDiagnostic("Read pool", err))

		return
	}

	allocations, err := d.data.Client.ListAllocations(ctx, pool.ID)
	if err != nil {
		resp.Diagnostics.Append(errorDiagnostic("Read pool allocations", err))

		return
	}

	var resourceModel poolResourceModel

	resp.Diagnostics.Append(resourceModel.setPool(ctx, pool)...)

	byID := make(map[string]string, len(allocations))
	for _, allocation := range allocations {
		byID[allocation.ID] = allocation.CIDR.String()
	}

	allocationMap, diags := types.MapValueFrom(ctx, types.StringType, byID)
	resp.Diagnostics.Append(diags...)

	labels, diags := types.MapValueFrom(ctx, types.StringType, pool.Labels)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	m.CIDRs = resourceModel.CIDRs
	m.Family = resourceModel.Family
	m.ParentPool = resourceModel.ParentPool
	m.Description = types.StringValue(pool.Description)
	m.Labels = labels
	m.Allocations = allocationMap
	m.Strategy = resourceModel.Strategy
	m.Cooldown = resourceModel.Cooldown

	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
