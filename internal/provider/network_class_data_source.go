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

type networkClassDataSource struct {
	c *client.Client
}

func NewNetworkClassDataSource() fwds.DataSource { return &networkClassDataSource{} }

func (d *networkClassDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_network_class"
}

func (d *networkClassDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Find a network class by name (e.g., '100 Mbit', '1 Gbit').",
		Attributes: map[string]dsschema.Attribute{
			"name": dsschema.StringAttribute{
				Required:    true,
				Description: "Name of the network class to search for.",
			},
			"id": dsschema.Int64Attribute{
				Computed:    true,
				Description: "ID of the network class.",
			},
			"description": dsschema.StringAttribute{
				Computed:    true,
				Description: "Description of the network class.",
			},
			"download_mbit_limit": dsschema.Int64Attribute{
				Computed:    true,
				Description: "Download speed limit in Mbit/s.",
			},
			"upload_mbit_limit": dsschema.Int64Attribute{
				Computed:    true,
				Description: "Upload speed limit in Mbit/s.",
			},
			"included_traffic_gb": dsschema.Int64Attribute{
				Computed:    true,
				Description: "Included traffic in GB per month.",
			},
			"price_per_hour_millicents": dsschema.Float64Attribute{
				Computed:    true,
				Description: "Base price per hour in millicents.",
			},
			"traffic_price_per_gb_millicents": dsschema.Float64Attribute{
				Computed:    true,
				Description: "Price per GB over limit in millicents.",
			},
		},
	}
}

func (d *networkClassDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type networkClassDataSourceModel struct {
	Name                        types.String  `tfsdk:"name"`
	ID                          types.Int64   `tfsdk:"id"`
	Description                 types.String  `tfsdk:"description"`
	DownloadMbitLimit           types.Int64   `tfsdk:"download_mbit_limit"`
	UploadMbitLimit             types.Int64   `tfsdk:"upload_mbit_limit"`
	IncludedTrafficGB           types.Int64   `tfsdk:"included_traffic_gb"`
	PricePerHourMillicents      types.Float64 `tfsdk:"price_per_hour_millicents"`
	TrafficPricePerGBMillicents types.Float64 `tfsdk:"traffic_price_per_gb_millicents"`
}

func (d *networkClassDataSource) Read(ctx context.Context, req fwds.ReadRequest, resp *fwds.ReadResponse) {
	var config networkClassDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := config.Name.ValueString()

	var listResp models.NetworkClassesListResponse
	err := d.c.GetJSON(ctx, client.NetworkClassesEP, nil, &listResp)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read network classes", err.Error())
		return
	}

	for _, item := range listResp.Items {
		if !item.IsActive {
			continue
		}
		if item.Name == name {
			config.ID = types.Int64Value(int64(item.ID))
			config.Description = types.StringValue(item.Description)
			config.DownloadMbitLimit = types.Int64Value(int64(item.DownloadMbitLimit))
			config.UploadMbitLimit = types.Int64Value(int64(item.UploadMbitLimit))
			config.IncludedTrafficGB = types.Int64Value(int64(item.IncludedTrafficGB))
			config.PricePerHourMillicents = types.Float64Value(item.PricePerHourMillicents)
			config.TrafficPricePerGBMillicents = types.Float64Value(item.TrafficPricePerGBMillicents)
			resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
			return
		}
	}

	resp.Diagnostics.AddError("Network class not found", fmt.Sprintf("No active network class with name '%s'", name))
}
