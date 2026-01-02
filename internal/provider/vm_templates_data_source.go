package provider

import (
	"context"

	fwds "github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

type vmTemplatesDataSource struct {
	c *client.Client
}

func NewVMTemplatesDataSource() fwds.DataSource { return &vmTemplatesDataSource{} }

func (d *vmTemplatesDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_vm_templates"
}

func (d *vmTemplatesDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Retrieves list of available VM templates (OS images).",
		Attributes: map[string]dsschema.Attribute{
			"items": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "List of VM templates.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id": dsschema.Int64Attribute{
							Computed:    true,
							Description: "ID of the VM template.",
						},
						"name": dsschema.StringAttribute{
							Computed:    true,
							Description: "Display name of the template.",
						},
						"api_name": dsschema.StringAttribute{
							Computed:    true,
							Description: "Internal API name of the template.",
						},
						"os_family": dsschema.StringAttribute{
							Computed:    true,
							Description: "OS family: linux, windows, bsd.",
						},
						"os_type": dsschema.StringAttribute{
							Computed:    true,
							Description: "OS type: ubuntu, debian, centos, windows-server, etc.",
						},
						"version": dsschema.StringAttribute{
							Computed:    true,
							Description: "OS version.",
						},
					},
				},
			},
		},
	}
}

func (d *vmTemplatesDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type vmTemplateModel struct {
	ID       types.Int64  `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	APIName  types.String `tfsdk:"api_name"`
	OSFamily types.String `tfsdk:"os_family"`
	OSType   types.String `tfsdk:"os_type"`
	Version  types.String `tfsdk:"version"`
}

type vmTemplatesDataSourceModel struct {
	Items []vmTemplateModel `tfsdk:"items"`
}

func (d *vmTemplatesDataSource) Read(ctx context.Context, req fwds.ReadRequest, resp *fwds.ReadResponse) {
	var listResp models.VMTemplatesListResponse
	err := d.c.GetJSON(ctx, client.VMTemplatesEP, nil, &listResp)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read VM templates", err.Error())
		return
	}

	var state vmTemplatesDataSourceModel
	for _, item := range listResp.Items {
		if !item.IsActive {
			continue
		}
		state.Items = append(state.Items, vmTemplateModel{
			ID:       types.Int64Value(int64(item.ID)),
			Name:     types.StringValue(item.Name),
			APIName:  types.StringValue(item.APIName),
			OSFamily: types.StringValue(item.OSFamily),
			OSType:   types.StringValue(item.OSType),
			Version:  types.StringValue(item.Version),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
