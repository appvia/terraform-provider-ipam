// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSourceWithConfigure = &allocationDataSource{}

type allocationDataSource struct {
	data *Data
}

type allocationDataSourceModel struct {
	Pool         types.String `tfsdk:"pool"`
	ID           types.String `tfsdk:"id"`
	CIDR         types.String `tfsdk:"cidr"`
	PrefixLength types.Int64  `tfsdk:"prefix_length"`
	Description  types.String `tfsdk:"description"`
	Labels       types.Map    `tfsdk:"labels"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

// NewAllocationDataSource returns the ipam_allocation data source.
func NewAllocationDataSource() datasource.DataSource {
	return &allocationDataSource{}
}

func (d *allocationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_allocation"
}

func (d *allocationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Reads one existing allocation by pool and ID.",
		Attributes: map[string]schema.Attribute{
			"pool": schema.StringAttribute{
				Required: true, Description: "Pool ID.", Validators: poolIDValidators(),
			},
			"id":            schema.StringAttribute{Required: true, Description: "Allocation ID."},
			"cidr":          schema.StringAttribute{Computed: true, Description: "The allocated CIDR."},
			"prefix_length": schema.Int64Attribute{Computed: true, Description: "Prefix length of `cidr`."},
			"description":   schema.StringAttribute{Computed: true, Description: "Description."},
			"labels": schema.MapAttribute{
				ElementType: types.StringType, Computed: true, Description: "Labels.",
			},
			"created_at": schema.StringAttribute{
				Computed: true, Description: "When the allocation was created (RFC 3339).",
			},
		},
	}
}

func (d *allocationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.data = providerData(req.ProviderData, &resp.Diagnostics)
}

func (d *allocationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m allocationDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)

	if resp.Diagnostics.HasError() {
		return
	}

	allocation, err := d.data.Client.GetAllocation(ctx, m.Pool.ValueString(), m.ID.ValueString())
	if err != nil {
		resp.Diagnostics.Append(errorDiagnostic("Read allocation", err))

		return
	}

	labels, diags := types.MapValueFrom(ctx, types.StringType, allocation.Labels)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	m.CIDR = types.StringValue(allocation.CIDR.String())
	m.PrefixLength = types.Int64Value(int64(allocation.CIDR.Bits()))
	m.Description = types.StringValue(allocation.Description)
	m.Labels = labels
	m.CreatedAt = types.StringValue(allocation.CreatedAt.UTC().Format(time.RFC3339Nano))

	resp.Diagnostics.Append(resp.State.Set(ctx, m)...)
}
