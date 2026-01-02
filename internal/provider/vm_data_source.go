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

type vmDataSource struct {
	c *client.Client
}

func NewVMDataSource() fwds.DataSource { return &vmDataSource{} }

func (d *vmDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_vm"
}

func (d *vmDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Retrieves information about an existing VM by UUID.",
		Attributes: map[string]dsschema.Attribute{
			"id": dsschema.StringAttribute{
				Required:    true,
				Description: "The UUID of the VM to retrieve.",
			},
			"display_name": dsschema.StringAttribute{
				Computed:    true,
				Description: "Display name of the VM.",
			},
			"vm_name": dsschema.StringAttribute{
				Computed:    true,
				Description: "System name of the VM.",
			},
			"vm_class_id": dsschema.Int64Attribute{
				Computed:    true,
				Description: "ID of the VM class.",
			},
			"root_disk_class_id": dsschema.Int64Attribute{
				Computed:    true,
				Description: "ID of the storage class for root disk.",
			},
			"primary_network_class_id": dsschema.Int64Attribute{
				Computed:    true,
				Description: "ID of the network class for primary network.",
			},
			"vm_template_id": dsschema.Int64Attribute{
				Computed:    true,
				Description: "ID of the VM template.",
			},
			"primary_network_id": dsschema.StringAttribute{
				Computed:    true,
				Description: "ID (UUID) of the primary network.",
			},
			"ssh_key_id": dsschema.Int64Attribute{
				Computed:    true,
				Description: "ID of the SSH key.",
			},
			"root_disk_gb": dsschema.Int64Attribute{
				Computed:    true,
				Description: "Root disk size in GB.",
			},
			"cpu_cores": dsschema.Int64Attribute{
				Computed:    true,
				Description: "Number of vCPU cores.",
			},
			"memory_mb": dsschema.Int64Attribute{
				Computed:    true,
				Description: "Memory in MB.",
			},
			"os_user": dsschema.StringAttribute{
				Computed:    true,
				Description: "OS username.",
			},
			"status": dsschema.StringAttribute{
				Computed:    true,
				Description: "Status of the VM.",
			},
			"state": dsschema.StringAttribute{
				Computed:    true,
				Description: "State of the VM (running, stopped, etc.).",
			},
			"ip_internal": dsschema.StringAttribute{
				Computed:    true,
				Description: "Internal IP address.",
			},
			"ipv6_address": dsschema.StringAttribute{
				Computed:    true,
				Description: "IPv6 address.",
			},
			"public_ip_v4": dsschema.StringAttribute{
				Computed:    true,
				Description: "Public IPv4 address.",
			},
			"public_ip_v6": dsschema.StringAttribute{
				Computed:    true,
				Description: "Public IPv6 address.",
			},
			"created_at": dsschema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the VM was created.",
			},
		},
	}
}

func (d *vmDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type vmDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	DisplayName      types.String `tfsdk:"display_name"`
	VMName           types.String `tfsdk:"vm_name"`
	VMClassID        types.Int64  `tfsdk:"vm_class_id"`
	RootDiskClassID       types.Int64  `tfsdk:"root_disk_class_id"`
	PrimaryNetworkClassID types.Int64  `tfsdk:"primary_network_class_id"`
	VMTemplateID     types.Int64  `tfsdk:"vm_template_id"`
	PrimaryNetworkID types.String `tfsdk:"primary_network_id"`
	SSHKeyID         types.Int64  `tfsdk:"ssh_key_id"`
	RootDiskGB       types.Int64  `tfsdk:"root_disk_gb"`
	CPUCores         types.Int64  `tfsdk:"cpu_cores"`
	MemoryMB         types.Int64  `tfsdk:"memory_mb"`
	OSUser           types.String `tfsdk:"os_user"`
	Status           types.String `tfsdk:"status"`
	State            types.String `tfsdk:"state"`
	IPInternal       types.String `tfsdk:"ip_internal"`
	IPv6Address      types.String `tfsdk:"ipv6_address"`
	PublicIPv4       types.String `tfsdk:"public_ip_v4"`
	PublicIPv6       types.String `tfsdk:"public_ip_v6"`
	CreatedAt        types.String `tfsdk:"created_at"`
}

func (d *vmDataSource) Read(ctx context.Context, req fwds.ReadRequest, resp *fwds.ReadResponse) {
	var config vmDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := config.ID.ValueString()

	var vm models.VM
	err := d.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.VMsEP, uuid), nil, &vm)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read VM", err.Error())
		return
	}

	config.ID = types.StringValue(vm.VMUUID)
	config.DisplayName = types.StringValue(vm.DisplayName)
	config.VMName = types.StringValue(vm.VMName)
	config.VMClassID = types.Int64Value(int64(vm.VMClassID))
	config.RootDiskClassID = types.Int64Value(int64(vm.StorageClassID))
	config.PrimaryNetworkClassID = types.Int64Value(int64(vm.NetworkClassID))
	config.VMTemplateID = types.Int64Value(int64(vm.VMTemplateID))
	config.PrimaryNetworkID = types.StringValue(vm.NetworkUUID)
	config.RootDiskGB = types.Int64Value(int64(vm.DiskGB))
	config.CPUCores = types.Int64Value(int64(vm.CPUCores))
	config.MemoryMB = types.Int64Value(int64(vm.MemoryMB))
	config.OSUser = types.StringValue(vm.OSUser)
	config.Status = types.StringValue(vm.Status)
	config.State = types.StringValue(vm.State)
	config.CreatedAt = types.StringValue(vm.CreatedAt)

	if vm.SSHKeyID != nil {
		config.SSHKeyID = types.Int64Value(int64(*vm.SSHKeyID))
	} else {
		config.SSHKeyID = types.Int64Null()
	}

	if vm.Network != nil {
		config.IPInternal = types.StringValue(vm.Network.IPInternal)
		config.IPv6Address = types.StringValue(vm.Network.IPv6Address)
		config.PublicIPv4 = types.StringValue(vm.Network.PublicIPv4)
		config.PublicIPv6 = types.StringValue(vm.Network.PublicIPv6)
	} else {
		config.IPInternal = types.StringNull()
		config.IPv6Address = types.StringNull()
		config.PublicIPv4 = types.StringNull()
		config.PublicIPv6 = types.StringNull()
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
