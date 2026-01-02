package provider

import (
	"context"

	fwds "github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

type networkClassesDataSource struct {
	c *client.Client
}

func NewNetworkClassesDataSource() fwds.DataSource { return &networkClassesDataSource{} }

func (d *networkClassesDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_network_classes"
}

func (d *networkClassesDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Retrieves list of available network classes (speed, traffic configurations).",
		Attributes: map[string]dsschema.Attribute{
			"items": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "List of network classes.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id": dsschema.Int64Attribute{
							Computed:    true,
							Description: "ID of the network class.",
						},
						"name": dsschema.StringAttribute{
							Computed:    true,
							Description: "Name of the network class.",
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
				},
			},
		},
	}
}

func (d *networkClassesDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type networkClassModel struct {
	ID                          types.Int64   `tfsdk:"id"`
	Name                        types.String  `tfsdk:"name"`
	Description                 types.String  `tfsdk:"description"`
	DownloadMbitLimit           types.Int64   `tfsdk:"download_mbit_limit"`
	UploadMbitLimit             types.Int64   `tfsdk:"upload_mbit_limit"`
	IncludedTrafficGB           types.Int64   `tfsdk:"included_traffic_gb"`
	PricePerHourMillicents      types.Float64 `tfsdk:"price_per_hour_millicents"`
	TrafficPricePerGBMillicents types.Float64 `tfsdk:"traffic_price_per_gb_millicents"`
}

type networkClassesDataSourceModel struct {
	Items []networkClassModel `tfsdk:"items"`
}

func (d *networkClassesDataSource) Read(ctx context.Context, req fwds.ReadRequest, resp *fwds.ReadResponse) {
	var listResp models.NetworkClassesListResponse
	err := d.c.GetJSON(ctx, client.NetworkClassesEP, nil, &listResp)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read network classes", err.Error())
		return
	}

	var state networkClassesDataSourceModel
	for _, item := range listResp.Items {
		if !item.IsActive {
			continue
		}
		state.Items = append(state.Items, networkClassModel{
			ID:                          types.Int64Value(int64(item.ID)),
			Name:                        types.StringValue(item.Name),
			Description:                 types.StringValue(item.Description),
			DownloadMbitLimit:           types.Int64Value(int64(item.DownloadMbitLimit)),
			UploadMbitLimit:             types.Int64Value(int64(item.UploadMbitLimit)),
			IncludedTrafficGB:           types.Int64Value(int64(item.IncludedTrafficGB)),
			PricePerHourMillicents:      types.Float64Value(item.PricePerHourMillicents),
			TrafficPricePerGBMillicents: types.Float64Value(item.TrafficPricePerGBMillicents),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
