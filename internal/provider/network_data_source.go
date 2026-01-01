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

type networkDataSource struct {
	c *client.Client
}

func NewNetworkDataSource() fwds.DataSource { return &networkDataSource{} }

func (d *networkDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_network"
}

func (d *networkDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Retrieves information about an existing network by UUID.",
		Attributes: map[string]dsschema.Attribute{
			"id": dsschema.StringAttribute{
				Required:    true,
				Description: "The UUID of the network to retrieve.",
			},
			"name": dsschema.StringAttribute{
				Computed:    true,
				Description: "Name of the network.",
			},
			"cidr": dsschema.StringAttribute{
				Computed:    true,
				Description: "CIDR block of the network.",
			},
			"router_uuid": dsschema.StringAttribute{
				Computed:    true,
				Description: "UUID of the attached router, if any.",
			},
			"network_type": dsschema.StringAttribute{
				Computed:    true,
				Description: "Type of network: 'private' or 'public'.",
			},
			"status": dsschema.StringAttribute{
				Computed:    true,
				Description: "Current status of the network.",
			},
			"created_at": dsschema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the network was created.",
			},
		},
	}
}

func (d *networkDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type networkDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	CIDR        types.String `tfsdk:"cidr"`
	RouterUUID  types.String `tfsdk:"router_uuid"`
	NetworkType types.String `tfsdk:"network_type"`
	Status      types.String `tfsdk:"status"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

func (d *networkDataSource) Read(ctx context.Context, req fwds.ReadRequest, resp *fwds.ReadResponse) {
	var config networkDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := config.ID.ValueString()

	var network models.Network
	err := d.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.NetworksEP, uuid), nil, &network)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read network", err.Error())
		return
	}

	config.ID = types.StringValue(network.NetworkUUID)
	config.Name = types.StringValue(network.Name)
	config.CIDR = types.StringValue(network.CIDR)
	config.NetworkType = types.StringValue(network.NetworkType)
	config.Status = types.StringValue(network.Status)
	config.CreatedAt = types.StringValue(network.CreatedAt)

	if network.RouterUUID != nil && *network.RouterUUID != "" {
		config.RouterUUID = types.StringValue(*network.RouterUUID)
	} else {
		config.RouterUUID = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
