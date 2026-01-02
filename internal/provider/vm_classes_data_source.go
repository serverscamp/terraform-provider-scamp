package provider

import (
	"context"

	fwds "github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

type vmClassesDataSource struct {
	c *client.Client
}

func NewVMClassesDataSource() fwds.DataSource { return &vmClassesDataSource{} }

func (d *vmClassesDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_vm_classes"
}

func (d *vmClassesDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Retrieves list of available VM classes (CPU, memory configurations).",
		Attributes: map[string]dsschema.Attribute{
			"items": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "List of VM classes.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id": dsschema.Int64Attribute{
							Computed:    true,
							Description: "ID of the VM class.",
						},
						"name": dsschema.StringAttribute{
							Computed:    true,
							Description: "Name of the VM class.",
						},
						"description": dsschema.StringAttribute{
							Computed:    true,
							Description: "Description of the VM class.",
						},
						"cpu_cores": dsschema.Int64Attribute{
							Computed:    true,
							Description: "Number of vCPU cores.",
						},
						"cpu_min_usage": dsschema.Int64Attribute{
							Computed:    true,
							Description: "Minimum CPU usage percentage.",
						},
						"cpu_max_usage": dsschema.Int64Attribute{
							Computed:    true,
							Description: "Maximum CPU usage percentage.",
						},
						"memory_mb": dsschema.Int64Attribute{
							Computed:    true,
							Description: "Memory in MB.",
						},
						"price_per_hour_millicents": dsschema.Float64Attribute{
							Computed:    true,
							Description: "Price per hour in millicents (10000 = 1 EUR).",
						},
					},
				},
			},
		},
	}
}

func (d *vmClassesDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type vmClassModel struct {
	ID                     types.Int64   `tfsdk:"id"`
	Name                   types.String  `tfsdk:"name"`
	Description            types.String  `tfsdk:"description"`
	CPUCores               types.Int64   `tfsdk:"cpu_cores"`
	CPUMinUsage            types.Int64   `tfsdk:"cpu_min_usage"`
	CPUMaxUsage            types.Int64   `tfsdk:"cpu_max_usage"`
	MemoryMB               types.Int64   `tfsdk:"memory_mb"`
	PricePerHourMillicents types.Float64 `tfsdk:"price_per_hour_millicents"`
}

type vmClassesDataSourceModel struct {
	Items []vmClassModel `tfsdk:"items"`
}

func (d *vmClassesDataSource) Read(ctx context.Context, req fwds.ReadRequest, resp *fwds.ReadResponse) {
	var listResp models.VMClassesListResponse
	err := d.c.GetJSON(ctx, client.VMClassesEP, nil, &listResp)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read VM classes", err.Error())
		return
	}

	var state vmClassesDataSourceModel
	for _, item := range listResp.Items {
		if !item.IsActive {
			continue
		}
		state.Items = append(state.Items, vmClassModel{
			ID:                     types.Int64Value(int64(item.ID)),
			Name:                   types.StringValue(item.Name),
			Description:            types.StringValue(item.Description),
			CPUCores:               types.Int64Value(int64(item.CPUCores)),
			CPUMinUsage:            types.Int64Value(int64(item.CPUMinUsage)),
			CPUMaxUsage:            types.Int64Value(int64(item.CPUMaxUsage)),
			MemoryMB:               types.Int64Value(int64(item.MemoryMB)),
			PricePerHourMillicents: types.Float64Value(item.PricePerHourMillicents),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
