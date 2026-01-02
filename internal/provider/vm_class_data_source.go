package provider

import (
	"context"
	"fmt"

	fwds "github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

type vmClassDataSource struct {
	c *client.Client
}

func NewVMClassDataSource() fwds.DataSource { return &vmClassDataSource{} }

func (d *vmClassDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_vm_class"
}

func (d *vmClassDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Find a VM class by name (e.g., 'small', 'medium', 'large').",
		Attributes: map[string]dsschema.Attribute{
			"name": dsschema.StringAttribute{
				Required:    true,
				Description: "Name of the VM class to search for.",
			},
			"id": dsschema.Int64Attribute{
				Computed:    true,
				Description: "ID of the VM class.",
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
	}
}

func (d *vmClassDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type vmClassDataSourceModel struct {
	Name                   types.String  `tfsdk:"name"`
	ID                     types.Int64   `tfsdk:"id"`
	Description            types.String  `tfsdk:"description"`
	CPUCores               types.Int64   `tfsdk:"cpu_cores"`
	CPUMinUsage            types.Int64   `tfsdk:"cpu_min_usage"`
	CPUMaxUsage            types.Int64   `tfsdk:"cpu_max_usage"`
	MemoryMB               types.Int64   `tfsdk:"memory_mb"`
	PricePerHourMillicents types.Float64 `tfsdk:"price_per_hour_millicents"`
}

func (d *vmClassDataSource) Read(ctx context.Context, req fwds.ReadRequest, resp *fwds.ReadResponse) {
	var config vmClassDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := config.Name.ValueString()

	var listResp models.VMClassesListResponse
	err := d.c.GetJSON(ctx, client.VMClassesEP, nil, &listResp)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read VM classes", err.Error())
		return
	}

	for _, item := range listResp.Items {
		if !item.IsActive {
			continue
		}
		if item.Name == name {
			config.ID = types.Int64Value(int64(item.ID))
			config.Description = types.StringValue(item.Description)
			config.CPUCores = types.Int64Value(int64(item.CPUCores))
			config.CPUMinUsage = types.Int64Value(int64(item.CPUMinUsage))
			config.CPUMaxUsage = types.Int64Value(int64(item.CPUMaxUsage))
			config.MemoryMB = types.Int64Value(int64(item.MemoryMB))
			config.PricePerHourMillicents = types.Float64Value(item.PricePerHourMillicents)
			resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
			return
		}
	}

	resp.Diagnostics.AddError("VM class not found", fmt.Sprintf("No active VM class with name '%s'", name))
}
