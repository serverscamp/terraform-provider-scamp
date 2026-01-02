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

type vmTemplateDataSource struct {
	c *client.Client
}

func NewVMTemplateDataSource() fwds.DataSource { return &vmTemplateDataSource{} }

func (d *vmTemplateDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_vm_template"
}

func (d *vmTemplateDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Find a VM template by os_type (e.g., 'Ubuntu', 'Alpine').",
		Attributes: map[string]dsschema.Attribute{
			"os_type": dsschema.StringAttribute{
				Required:    true,
				Description: "OS type to search for (e.g., 'Ubuntu', 'Alpine').",
			},
			"id": dsschema.Int64Attribute{
				Computed:    true,
				Description: "ID of the template.",
			},
			"name": dsschema.StringAttribute{
				Computed:    true,
				Description: "Name of the template.",
			},
			"api_name": dsschema.StringAttribute{
				Computed:    true,
				Description: "API name of the template.",
			},
			"os_family": dsschema.StringAttribute{
				Computed:    true,
				Description: "OS family (Linux, Windows, etc.).",
			},
			"version": dsschema.StringAttribute{
				Computed:    true,
				Description: "OS version.",
			},
		},
	}
}

func (d *vmTemplateDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type vmTemplateDataSourceModel struct {
	OSType   types.String `tfsdk:"os_type"`
	ID       types.Int64  `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	APIName  types.String `tfsdk:"api_name"`
	OSFamily types.String `tfsdk:"os_family"`
	Version  types.String `tfsdk:"version"`
}

func (d *vmTemplateDataSource) Read(ctx context.Context, req fwds.ReadRequest, resp *fwds.ReadResponse) {
	var config vmTemplateDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	osType := config.OSType.ValueString()

	var listResp models.VMTemplatesListResponse
	err := d.c.GetJSON(ctx, client.VMTemplatesEP, nil, &listResp)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read VM templates", err.Error())
		return
	}

	for _, item := range listResp.Items {
		if !item.IsActive {
			continue
		}
		if item.OSType == osType {
			config.ID = types.Int64Value(int64(item.ID))
			config.Name = types.StringValue(item.Name)
			config.APIName = types.StringValue(item.APIName)
			config.OSFamily = types.StringValue(item.OSFamily)
			config.Version = types.StringValue(item.Version)
			resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
			return
		}
	}

	resp.Diagnostics.AddError("Template not found", fmt.Sprintf("No active template with os_type '%s'", osType))
}
