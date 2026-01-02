package provider

import (
	"context"
	"fmt"
	"time"

	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

type vmResource struct {
	c *client.Client
}

func NewVMResource() tfresource.Resource { return &vmResource{} }

func (r *vmResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_vm"
}

func (r *vmResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		Description: "Manages a virtual machine in SCAMP.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed:    true,
				Description: "The UUID of the VM.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": rschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Display name of the VM (max 100 characters).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vm_class_id": rschema.Int64Attribute{
				Required:    true,
				Description: "ID of the VM class (CPU, memory configuration).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"root_disk_class_id": rschema.Int64Attribute{
				Required:    true,
				Description: "ID of the storage class for root disk (IOPS, bandwidth).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"primary_network_class_id": rschema.Int64Attribute{
				Required:    true,
				Description: "ID of the network class for primary network (speed, traffic).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"vm_template_id": rschema.Int64Attribute{
				Required:    true,
				Description: "ID of the VM template (OS image).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"primary_network_id": rschema.StringAttribute{
				Required:    true,
				Description: "ID (UUID) of the primary network to attach the VM to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ssh_key_id": rschema.Int64Attribute{
				Optional:    true,
				Description: "ID of the SSH key to inject.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"root_disk_gb": rschema.Int64Attribute{
				Required:    true,
				Description: "Root disk size in GB (10-1000).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"os_password": rschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
				Description: "OS password (8-64 characters). Auto-generated if not provided.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"assign_public_ips": rschema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Assign public IPv4/IPv6 addresses (default: false).",
				PlanModifiers: []planmodifier.Bool{
					// No RequiresReplace - could be changed in future
				},
			},
			"description": rschema.StringAttribute{
				Optional:    true,
				Description: "Description of the VM (local only, not sent to API).",
			},
			"tags": rschema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Tags for the VM as key-value pairs (local only, not sent to API).",
			},
			// Computed fields
			"vm_name": rschema.StringAttribute{
				Computed:    true,
				Description: "System name of the VM.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cpu_cores": rschema.Int64Attribute{
				Computed:    true,
				Description: "Number of vCPU cores.",
			},
			"memory_mb": rschema.Int64Attribute{
				Computed:    true,
				Description: "Memory in MB.",
			},
			"os_user": rschema.StringAttribute{
				Computed:    true,
				Description: "OS username.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"status": rschema.StringAttribute{
				Computed:    true,
				Description: "Status of the VM (queued, active, etc.).",
			},
			"state": rschema.StringAttribute{
				Computed:    true,
				Description: "State of the VM (running, stopped, etc.).",
			},
			"ip_internal": rschema.StringAttribute{
				Computed:    true,
				Description: "Internal IP address.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ipv6_address": rschema.StringAttribute{
				Computed:    true,
				Description: "IPv6 address.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"public_ip_v4": rschema.StringAttribute{
				Computed:    true,
				Description: "Public IPv4 address.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"public_ip_v6": rschema.StringAttribute{
				Computed:    true,
				Description: "Public IPv6 address.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": rschema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the VM was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *vmResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

type vmModel struct {
	ID               types.String `tfsdk:"id"`
	DisplayName      types.String `tfsdk:"display_name"`
	VMClassID        types.Int64  `tfsdk:"vm_class_id"`
	RootDiskClassID        types.Int64  `tfsdk:"root_disk_class_id"`
	PrimaryNetworkClassID  types.Int64  `tfsdk:"primary_network_class_id"`
	VMTemplateID     types.Int64  `tfsdk:"vm_template_id"`
	PrimaryNetworkID types.String `tfsdk:"primary_network_id"`
	SSHKeyID         types.Int64  `tfsdk:"ssh_key_id"`
	RootDiskGB       types.Int64  `tfsdk:"root_disk_gb"`
	OSPassword       types.String `tfsdk:"os_password"`
	AssignPublicIPs  types.Bool   `tfsdk:"assign_public_ips"`
	Description      types.String `tfsdk:"description"`
	Tags             types.Map    `tfsdk:"tags"`
	// Computed
	VMName      types.String `tfsdk:"vm_name"`
	CPUCores    types.Int64  `tfsdk:"cpu_cores"`
	MemoryMB    types.Int64  `tfsdk:"memory_mb"`
	OSUser      types.String `tfsdk:"os_user"`
	Status      types.String `tfsdk:"status"`
	State       types.String `tfsdk:"state"`
	IPInternal  types.String `tfsdk:"ip_internal"`
	IPv6Address types.String `tfsdk:"ipv6_address"`
	PublicIPv4  types.String `tfsdk:"public_ip_v4"`
	PublicIPv6  types.String `tfsdk:"public_ip_v6"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

func (r *vmResource) setModelFromVM(m *vmModel, vm *models.VM) {
	m.ID = types.StringValue(vm.VMUUID)
	m.VMName = types.StringValue(vm.VMName)
	m.DisplayName = types.StringValue(vm.DisplayName)
	m.CPUCores = types.Int64Value(int64(vm.CPUCores))
	m.MemoryMB = types.Int64Value(int64(vm.MemoryMB))
	m.RootDiskGB = types.Int64Value(int64(vm.DiskGB))
	m.VMClassID = types.Int64Value(int64(vm.VMClassID))
	m.RootDiskClassID = types.Int64Value(int64(vm.StorageClassID))
	m.PrimaryNetworkClassID = types.Int64Value(int64(vm.NetworkClassID))
	m.VMTemplateID = types.Int64Value(int64(vm.VMTemplateID))
	m.PrimaryNetworkID = types.StringValue(vm.NetworkUUID)
	m.OSUser = types.StringValue(vm.OSUser)
	m.Status = types.StringValue(vm.Status)
	m.State = types.StringValue(vm.State)
	if vm.CreatedAt != "" {
		m.CreatedAt = types.StringValue(vm.CreatedAt)
	}

	if vm.SSHKeyID != nil {
		m.SSHKeyID = types.Int64Value(int64(*vm.SSHKeyID))
	} else {
		m.SSHKeyID = types.Int64Null()
	}

	if vm.Network != nil {
		m.IPInternal = types.StringValue(vm.Network.IPInternal)
		m.IPv6Address = types.StringValue(vm.Network.IPv6Address)
		m.PublicIPv4 = types.StringValue(vm.Network.PublicIPv4)
		m.PublicIPv6 = types.StringValue(vm.Network.PublicIPv6)
	}
}

func (r *vmResource) waitForVMRunning(ctx context.Context, uuid string, timeout time.Duration) (*models.VM, error) {
	deadline := time.Now().Add(timeout)
	for {
		var vm models.VM
		err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.VMsEP, uuid), nil, &vm)
		if err != nil {
			return nil, err
		}
		if vm.State == "running" {
			return &vm, nil
		}
		if time.Now().After(deadline) {
			return &vm, fmt.Errorf("timeout waiting for VM %s to start (state: %s)", uuid, vm.State)
		}
		tflog.Debug(ctx, "Waiting for VM to start", map[string]any{
			"uuid":  uuid,
			"state": vm.State,
		})
		time.Sleep(1 * time.Second)
	}
}

func (r *vmResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan vmModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := map[string]any{
		"vm_class_id":      plan.VMClassID.ValueInt64(),
		"storage_class_id": plan.RootDiskClassID.ValueInt64(),
		"network_class_id": plan.PrimaryNetworkClassID.ValueInt64(),
		"vm_template_id":   plan.VMTemplateID.ValueInt64(),
		"network_uuid":     plan.PrimaryNetworkID.ValueString(),
		"disk_gb":          plan.RootDiskGB.ValueInt64(),
	}

	if !plan.DisplayName.IsNull() && plan.DisplayName.ValueString() != "" {
		payload["display_name"] = plan.DisplayName.ValueString()
	}
	if !plan.SSHKeyID.IsNull() {
		payload["ssh_key_id"] = plan.SSHKeyID.ValueInt64()
	}
	if !plan.OSPassword.IsNull() && plan.OSPassword.ValueString() != "" {
		payload["os_password"] = plan.OSPassword.ValueString()
	}
	if !plan.AssignPublicIPs.IsNull() && plan.AssignPublicIPs.ValueBool() {
		payload["assign_public_ips"] = true
	}

	var createResp models.VMCreateResponse
	if err := r.c.PostJSON(ctx, client.VMsEP, payload, &createResp); err != nil {
		resp.Diagnostics.AddError("Failed to create VM", err.Error())
		return
	}

	// Save initial data from create response
	plan.ID = types.StringValue(createResp.VMUUID)
	plan.VMName = types.StringValue(createResp.VMName)
	plan.OSUser = types.StringValue(createResp.OSUser)
	plan.OSPassword = types.StringValue(createResp.OSPassword)
	plan.Status = types.StringValue(createResp.Status)

	// Wait for VM to start running
	activeVM, err := r.waitForVMRunning(ctx, createResp.VMUUID, 5*time.Minute)
	if err != nil {
		resp.Diagnostics.AddWarning("VM created but not yet active", err.Error())
	} else {
		// Preserve os_password from create response (it's not returned in GET)
		savedPassword := plan.OSPassword
		r.setModelFromVM(&plan, activeVM)
		plan.OSPassword = savedPassword
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vmResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state vmModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := state.ID.ValueString()
	if uuid == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	var vm models.VM
	err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.VMsEP, uuid), nil, &vm)
	if err != nil {
		// Assume 404 - resource deleted
		resp.State.RemoveResource(ctx)
		return
	}

	// Preserve fields not returned by API
	savedPassword := state.OSPassword
	savedAssignPublicIPs := state.AssignPublicIPs

	r.setModelFromVM(&state, &vm)

	// Restore preserved fields
	state.OSPassword = savedPassword
	state.AssignPublicIPs = savedAssignPublicIPs

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vmResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	// VM doesn't support updates - all changes require replace
	var plan vmModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vmResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state vmModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := state.ID.ValueString()
	if uuid == "" {
		return
	}

	if err := r.c.Delete(ctx, fmt.Sprintf("%s/%s", client.VMsEP, uuid)); err != nil {
		resp.Diagnostics.AddError("Failed to delete VM", err.Error())
		return
	}
}
