package provider

import (
	"context"

	fwds "github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

type storageClassesDataSource struct {
	c *client.Client
}

func NewStorageClassesDataSource() fwds.DataSource { return &storageClassesDataSource{} }

func (d *storageClassesDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_storage_classes"
}

func (d *storageClassesDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Retrieves list of available storage classes (IOPS, bandwidth configurations).",
		Attributes: map[string]dsschema.Attribute{
			"items": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "List of storage classes.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id": dsschema.Int64Attribute{
							Computed:    true,
							Description: "ID of the storage class.",
						},
						"name": dsschema.StringAttribute{
							Computed:    true,
							Description: "Name of the storage class.",
						},
						"description": dsschema.StringAttribute{
							Computed:    true,
							Description: "Description of the storage class.",
						},
						"max_size_gb": dsschema.Int64Attribute{
							Computed:    true,
							Description: "Maximum disk size in GB.",
						},
						"read_iops_limit": dsschema.Int64Attribute{
							Computed:    true,
							Description: "Read IOPS limit.",
						},
						"write_iops_limit": dsschema.Int64Attribute{
							Computed:    true,
							Description: "Write IOPS limit.",
						},
						"read_bandwidth_limit": dsschema.Int64Attribute{
							Computed:    true,
							Description: "Read bandwidth limit in MB/s.",
						},
						"write_bandwidth_limit": dsschema.Int64Attribute{
							Computed:    true,
							Description: "Write bandwidth limit in MB/s.",
						},
						"replica_count": dsschema.Int64Attribute{
							Computed:    true,
							Description: "Number of data replicas.",
						},
						"price_per_gb_hour_millicents": dsschema.Float64Attribute{
							Computed:    true,
							Description: "Price per GB per hour in millicents.",
						},
					},
				},
			},
		},
	}
}

func (d *storageClassesDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type storageClassModel struct {
	ID                       types.Int64   `tfsdk:"id"`
	Name                     types.String  `tfsdk:"name"`
	Description              types.String  `tfsdk:"description"`
	MaxSizeGB                types.Int64   `tfsdk:"max_size_gb"`
	ReadIOPSLimit            types.Int64   `tfsdk:"read_iops_limit"`
	WriteIOPSLimit           types.Int64   `tfsdk:"write_iops_limit"`
	ReadBandwidthLimit       types.Int64   `tfsdk:"read_bandwidth_limit"`
	WriteBandwidthLimit      types.Int64   `tfsdk:"write_bandwidth_limit"`
	ReplicaCount             types.Int64   `tfsdk:"replica_count"`
	PricePerGBHourMillicents types.Float64 `tfsdk:"price_per_gb_hour_millicents"`
}

type storageClassesDataSourceModel struct {
	Items []storageClassModel `tfsdk:"items"`
}

func (d *storageClassesDataSource) Read(ctx context.Context, req fwds.ReadRequest, resp *fwds.ReadResponse) {
	var listResp models.StorageClassesListResponse
	err := d.c.GetJSON(ctx, client.StorageClassesEP, nil, &listResp)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read storage classes", err.Error())
		return
	}

	var state storageClassesDataSourceModel
	for _, item := range listResp.Items {
		if !item.IsActive {
			continue
		}
		state.Items = append(state.Items, storageClassModel{
			ID:                       types.Int64Value(int64(item.ID)),
			Name:                     types.StringValue(item.Name),
			Description:              types.StringValue(item.Description),
			MaxSizeGB:                types.Int64Value(int64(item.MaxSizeGB)),
			ReadIOPSLimit:            types.Int64Value(int64(item.ReadIOPSLimit)),
			WriteIOPSLimit:           types.Int64Value(int64(item.WriteIOPSLimit)),
			ReadBandwidthLimit:       types.Int64Value(int64(item.ReadBandwidthLimit)),
			WriteBandwidthLimit:      types.Int64Value(int64(item.WriteBandwidthLimit)),
			ReplicaCount:             types.Int64Value(int64(item.ReplicaCount)),
			PricePerGBHourMillicents: types.Float64Value(item.PricePerGBHourMillicents),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
