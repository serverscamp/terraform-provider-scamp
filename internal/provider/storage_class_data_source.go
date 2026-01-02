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

type storageClassDataSource struct {
	c *client.Client
}

func NewStorageClassDataSource() fwds.DataSource { return &storageClassDataSource{} }

func (d *storageClassDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_storage_class"
}

func (d *storageClassDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Find a storage class by name (e.g., 'SSD', 'HDD').",
		Attributes: map[string]dsschema.Attribute{
			"name": dsschema.StringAttribute{
				Required:    true,
				Description: "Name of the storage class to search for.",
			},
			"id": dsschema.Int64Attribute{
				Computed:    true,
				Description: "ID of the storage class.",
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
	}
}

func (d *storageClassDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type storageClassDataSourceModel struct {
	Name                     types.String  `tfsdk:"name"`
	ID                       types.Int64   `tfsdk:"id"`
	Description              types.String  `tfsdk:"description"`
	MaxSizeGB                types.Int64   `tfsdk:"max_size_gb"`
	ReadIOPSLimit            types.Int64   `tfsdk:"read_iops_limit"`
	WriteIOPSLimit           types.Int64   `tfsdk:"write_iops_limit"`
	ReadBandwidthLimit       types.Int64   `tfsdk:"read_bandwidth_limit"`
	WriteBandwidthLimit      types.Int64   `tfsdk:"write_bandwidth_limit"`
	ReplicaCount             types.Int64   `tfsdk:"replica_count"`
	PricePerGBHourMillicents types.Float64 `tfsdk:"price_per_gb_hour_millicents"`
}

func (d *storageClassDataSource) Read(ctx context.Context, req fwds.ReadRequest, resp *fwds.ReadResponse) {
	var config storageClassDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := config.Name.ValueString()

	var listResp models.StorageClassesListResponse
	err := d.c.GetJSON(ctx, client.StorageClassesEP, nil, &listResp)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read storage classes", err.Error())
		return
	}

	for _, item := range listResp.Items {
		if !item.IsActive {
			continue
		}
		if item.Name == name {
			config.ID = types.Int64Value(int64(item.ID))
			config.Description = types.StringValue(item.Description)
			config.MaxSizeGB = types.Int64Value(int64(item.MaxSizeGB))
			config.ReadIOPSLimit = types.Int64Value(int64(item.ReadIOPSLimit))
			config.WriteIOPSLimit = types.Int64Value(int64(item.WriteIOPSLimit))
			config.ReadBandwidthLimit = types.Int64Value(int64(item.ReadBandwidthLimit))
			config.WriteBandwidthLimit = types.Int64Value(int64(item.WriteBandwidthLimit))
			config.ReplicaCount = types.Int64Value(int64(item.ReplicaCount))
			config.PricePerGBHourMillicents = types.Float64Value(item.PricePerGBHourMillicents)
			resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
			return
		}
	}

	resp.Diagnostics.AddError("Storage class not found", fmt.Sprintf("No active storage class with name '%s'", name))
}
