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

type routerDataSource struct {
	c *client.Client
}

func NewRouterDataSource() fwds.DataSource { return &routerDataSource{} }

func (d *routerDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_router"
}

func (d *routerDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Retrieves information about an existing router by UUID.",
		Attributes: map[string]dsschema.Attribute{
			"id": dsschema.StringAttribute{
				Required:    true,
				Description: "The UUID of the router to retrieve.",
			},
			"name": dsschema.StringAttribute{
				Computed:    true,
				Description: "Name of the router.",
			},
			"ipv4_address": dsschema.StringAttribute{
				Computed:    true,
				Description: "Public IPv4 address of the router.",
			},
			"ipv6_address": dsschema.StringAttribute{
				Computed:    true,
				Description: "Public IPv6 address of the router.",
			},
			"status": dsschema.StringAttribute{
				Computed:    true,
				Description: "Current status of the router.",
			},
			"created_at": dsschema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the router was created.",
			},
		},
	}
}

func (d *routerDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type routerDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	IPv4Address types.String `tfsdk:"ipv4_address"`
	IPv6Address types.String `tfsdk:"ipv6_address"`
	Status      types.String `tfsdk:"status"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

func (d *routerDataSource) Read(ctx context.Context, req fwds.ReadRequest, resp *fwds.ReadResponse) {
	var config routerDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := config.ID.ValueString()

	var router models.Router
	err := d.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.RoutersEP, uuid), nil, &router)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read router", err.Error())
		return
	}

	config.ID = types.StringValue(router.RouterUUID)
	config.Name = types.StringValue(router.Name)
	config.IPv4Address = types.StringValue(router.IPv4Address)
	config.IPv6Address = types.StringValue(router.IPv6Address)
	config.Status = types.StringValue(router.Status)
	config.CreatedAt = types.StringValue(router.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
