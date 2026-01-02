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

type volumeDataSource struct {
	c *client.Client
}

func NewVolumeDataSource() fwds.DataSource { return &volumeDataSource{} }

func (d *volumeDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_volume"
}

func (d *volumeDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Retrieves information about an existing volume by UUID.",
		Attributes: map[string]dsschema.Attribute{
			"id": dsschema.StringAttribute{
				Required:    true,
				Description: "The UUID of the volume to retrieve.",
			},
			"display_name": dsschema.StringAttribute{
				Computed:    true,
				Description: "Display name of the volume.",
			},
			"size_gb": dsschema.Int64Attribute{
				Computed:    true,
				Description: "Size of the volume in GB.",
			},
			"storage_class_id": dsschema.Int64Attribute{
				Computed:    true,
				Description: "ID of the storage class.",
			},
			"attached_vm_id": dsschema.StringAttribute{
				Computed:    true,
				Description: "UUID of the VM the volume is attached to (null if not attached).",
			},
			"state": dsschema.StringAttribute{
				Computed:    true,
				Description: "State of the volume.",
			},
			"sds_pool_name": dsschema.StringAttribute{
				Computed:    true,
				Description: "Name of the SDS pool.",
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
				Description: "Read bandwidth limit (MB/s).",
			},
			"write_bandwidth_limit": dsschema.Int64Attribute{
				Computed:    true,
				Description: "Write bandwidth limit (MB/s).",
			},
			"created_at": dsschema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the volume was created.",
			},
		},
	}
}

func (d *volumeDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type volumeDataSourceModel struct {
	ID                  types.String `tfsdk:"id"`
	DisplayName         types.String `tfsdk:"display_name"`
	SizeGB              types.Int64  `tfsdk:"size_gb"`
	StorageClassID      types.Int64  `tfsdk:"storage_class_id"`
	AttachedVMID        types.String `tfsdk:"attached_vm_id"`
	State               types.String `tfsdk:"state"`
	SDSPoolName         types.String `tfsdk:"sds_pool_name"`
	ReadIOPSLimit       types.Int64  `tfsdk:"read_iops_limit"`
	WriteIOPSLimit      types.Int64  `tfsdk:"write_iops_limit"`
	ReadBandwidthLimit  types.Int64  `tfsdk:"read_bandwidth_limit"`
	WriteBandwidthLimit types.Int64  `tfsdk:"write_bandwidth_limit"`
	CreatedAt           types.String `tfsdk:"created_at"`
}

func (d *volumeDataSource) Read(ctx context.Context, req fwds.ReadRequest, resp *fwds.ReadResponse) {
	var config volumeDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := config.ID.ValueString()

	var vol models.Volume
	err := d.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.VolumesEP, uuid), nil, &vol)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read volume", err.Error())
		return
	}

	config.ID = types.StringValue(vol.DiskUUID)
	config.DisplayName = types.StringValue(vol.DisplayName)
	config.SizeGB = types.Int64Value(int64(vol.SizeGB))
	config.StorageClassID = types.Int64Value(int64(vol.StorageClassID))
	config.State = types.StringValue(vol.State)
	config.SDSPoolName = types.StringValue(vol.SDSPoolName)
	config.CreatedAt = types.StringValue(vol.CreatedAt)

	if vol.VMUUID != nil {
		config.AttachedVMID = types.StringValue(*vol.VMUUID)
	} else {
		config.AttachedVMID = types.StringNull()
	}

	if vol.Limits != nil {
		config.ReadIOPSLimit = types.Int64Value(int64(vol.Limits.ReadIOPSLimit))
		config.WriteIOPSLimit = types.Int64Value(int64(vol.Limits.WriteIOPSLimit))
		config.ReadBandwidthLimit = types.Int64Value(int64(vol.Limits.ReadBandwidthLimit))
		config.WriteBandwidthLimit = types.Int64Value(int64(vol.Limits.WriteBandwidthLimit))
	} else {
		config.ReadIOPSLimit = types.Int64Null()
		config.WriteIOPSLimit = types.Int64Null()
		config.ReadBandwidthLimit = types.Int64Null()
		config.WriteBandwidthLimit = types.Int64Null()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
