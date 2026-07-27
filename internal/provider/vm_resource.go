package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
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

// ValidateConfig keeps the readable name and the raw id mutually exclusive:
// exactly one of each pair must be set. Without it a config could name a class
// and pin an id pointing somewhere else, and the id would silently win.
//
// Written by hand rather than pulling in terraform-plugin-framework-validators:
// three checks are not worth a new dependency in a published provider.
func (r *vmResource) ValidateConfig(ctx context.Context, req tfresource.ValidateConfigRequest, resp *tfresource.ValidateConfigResponse) {
	var cfg vmModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A value that is still unknown at validate time (comes from a variable or
	// another resource) counts as set: we cannot see it yet, and it will be
	// there by apply.
	set := func(name types.String, id types.Int64) (bool, bool) {
		hasName := !name.IsNull() && (name.IsUnknown() || name.ValueString() != "")
		hasID := !id.IsNull()
		return hasName, hasID
	}

	for _, pair := range []struct {
		nameAttr, idAttr string
		name             types.String
		id               types.Int64
	}{
		{"vm_class", "vm_class_id", cfg.VMClass, cfg.VMClassID},
		{"root_disk_class", "root_disk_class_id", cfg.RootDiskClass, cfg.RootDiskClassID},
		{"image", "vm_template_id", cfg.Image, cfg.VMTemplateID},
	} {
		hasName, hasID := set(pair.name, pair.id)
		switch {
		case hasName && hasID:
			resp.Diagnostics.AddError(
				fmt.Sprintf("Both %s and %s are set", pair.nameAttr, pair.idAttr),
				fmt.Sprintf("Set one of them. %s is the readable form and is resolved against the catalogue; "+
					"%s pins a specific id.", pair.nameAttr, pair.idAttr),
			)
		case !hasName && !hasID:
			resp.Diagnostics.AddError(
				fmt.Sprintf("Neither %s nor %s is set", pair.nameAttr, pair.idAttr),
				fmt.Sprintf("Set %s (recommended, e.g. vm_class = \"bs-burst-xs\") or %s.",
					pair.nameAttr, pair.idAttr),
			)
		}
	}
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
				Optional:    true,
				Computed:    true,
				Description: "ID of the VM class (CPU, memory configuration). Resolved from vm_class when that is set instead.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"vm_class": rschema.StringAttribute{
				Optional:    true,
				Description: "Name of the VM class, e.g. bs-burst-xs or hf-m. Use this or vm_class_id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"root_disk_class_id": rschema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "ID of the storage class for root disk (IOPS, bandwidth). Resolved from root_disk_class when that is set instead.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"root_disk_class": rschema.StringAttribute{
				Optional:    true,
				Description: "Name of the storage class for the root disk, e.g. R1. Use this or root_disk_class_id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"vm_template_id": rschema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "ID of the VM template (OS image). Resolved from image when that is set instead.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"image": rschema.StringAttribute{
				Optional:    true,
				Description: "OS image, by slug (ubuntu-26.04, debian-13) or display name (Ubuntu 26.04 LTS). Case does not matter, and a unique substring works. Use this or vm_template_id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"primary_network_id": rschema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "UUID of the network the VM sits on. Leave it out and the VM joins the "  +
					"organisation's default-network, the one provisioned with the account that cannot "  +
					"be deleted from the panel.",
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
			"security_group_id": rschema.StringAttribute{
				Optional: true,
				Description: "UUID of the security group the VM joins. Omit it and the platform " +
					"picks the organisation's default group, which is what the panel does too. " +
					"Changing it re-creates the VM: moving a running VM between groups is done " +
					"through the security group itself, not from here. Not Computed on purpose - " +
					"the API takes this on create but never reports it back on a VM, so an omitted " +
					"value stays null instead of claiming a group we cannot verify.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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
	VMClass          types.String `tfsdk:"vm_class"`
	RootDiskClassID        types.Int64  `tfsdk:"root_disk_class_id"`
	RootDiskClass          types.String `tfsdk:"root_disk_class"`
	VMTemplateID     types.Int64  `tfsdk:"vm_template_id"`
	Image            types.String `tfsdk:"image"`
	PrimaryNetworkID types.String `tfsdk:"primary_network_id"`
	SSHKeyID         types.Int64  `tfsdk:"ssh_key_id"`
	RootDiskGB       types.Int64  `tfsdk:"root_disk_gb"`
	OSPassword       types.String `tfsdk:"os_password"`
	AssignPublicIPs  types.Bool   `tfsdk:"assign_public_ips"`
	SecurityGroupID  types.String `tfsdk:"security_group_id"`
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
		// The API returns addresses with their prefix ("194.110.174.110/24").
		// A netmask has no business in an output that feeds DNS records, ssh
		// commands or firewall rules, so it is stripped here.
		m.IPInternal = types.StringValue(stripPrefixLen(vm.Network.IPInternal))
		m.IPv6Address = types.StringValue(stripPrefixLen(vm.Network.IPv6Address))
		m.PublicIPv4 = types.StringValue(stripPrefixLen(vm.Network.PublicIPv4))
		m.PublicIPv6 = types.StringValue(stripPrefixLen(vm.Network.PublicIPv6))
	}
}

// waitForVMReady waits for a VM that is actually usable, not merely started.
//
// The domain reports "running" several seconds before the control plane has
// finished with it: vm_name arrives with the create reconcile, and public
// addresses are attached after that. Returning on "running" alone is why an
// apply used to hand back an empty name and an empty public IP for a VM that
// had both a minute later.
func (r *vmResource) waitForVMReady(ctx context.Context, uuid string, wantPublicIP bool, timeout time.Duration) (*models.VM, error) {
	deadline := time.Now().Add(timeout)
	var last models.VM
	for {
		var vm models.VM
		if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.VMsEP, uuid), nil, &vm); err != nil {
			return nil, err
		}
		last = vm

		running := vm.State == "running"
		named := vm.VMName != ""
		addressed := !wantPublicIP || (vm.Network != nil && vm.Network.PublicIPv4 != "")
		if running && named && addressed {
			return &vm, nil
		}

		if time.Now().After(deadline) {
			switch {
			case !running:
				return &last, fmt.Errorf("timeout waiting for VM %s to start (state: %s)", uuid, vm.State)
			case !named:
				return &last, fmt.Errorf("VM %s is running but has no name yet", uuid)
			default:
				return &last, fmt.Errorf("VM %s is running but no public IP was attached yet", uuid)
			}
		}
		tflog.Debug(ctx, "Waiting for VM to be ready", map[string]any{
			"uuid": uuid, "state": vm.State, "named": named, "addressed": addressed,
		})
		time.Sleep(2 * time.Second)
	}
}

func (r *vmResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan vmModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Names win when both are given? No - the schema forbids that pair, so at
	// this point exactly one of each is set. Resolve the readable one.
	vmClassID := plan.VMClassID.ValueInt64()
	if !plan.VMClass.IsNull() && plan.VMClass.ValueString() != "" {
		id, err := resolveVMClass(ctx, r.c, plan.VMClass.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Could not resolve vm_class", err.Error())
			return
		}
		vmClassID = int64(id)
	}

	diskClassID := plan.RootDiskClassID.ValueInt64()
	if !plan.RootDiskClass.IsNull() && plan.RootDiskClass.ValueString() != "" {
		id, err := resolveStorageClass(ctx, r.c, plan.RootDiskClass.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Could not resolve root_disk_class", err.Error())
			return
		}
		diskClassID = int64(id)
	}

	templateID := plan.VMTemplateID.ValueInt64()
	if !plan.Image.IsNull() && plan.Image.ValueString() != "" {
		id, err := resolveTemplate(ctx, r.c, plan.Image.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Could not resolve image", err.Error())
			return
		}
		templateID = int64(id)
	}

	networkUUID := plan.PrimaryNetworkID.ValueString()
	if plan.PrimaryNetworkID.IsNull() || plan.PrimaryNetworkID.IsUnknown() || networkUUID == "" {
		uuid, err := resolveDefaultNetwork(ctx, r.c)
		if err != nil {
			resp.Diagnostics.AddError("Could not pick a network", err.Error())
			return
		}
		networkUUID = uuid
	}

	payload := map[string]any{
		"vm_class_id":      vmClassID,
		"storage_class_id": diskClassID,
		"vm_template_id":   templateID,
		"network_uuid":     networkUUID,
		"disk_gb":          plan.RootDiskGB.ValueInt64(),
	}

	if !plan.DisplayName.IsNull() && plan.DisplayName.ValueString() != "" {
		payload["display_name"] = plan.DisplayName.ValueString()
	}
	if !plan.SecurityGroupID.IsNull() && !plan.SecurityGroupID.IsUnknown() {
		payload["sg_uuid"] = plan.SecurityGroupID.ValueString()
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

	// Save initial data from create response. The id goes into state right here,
	// before any waiting: a VM that exists but whose wait timed out must still be
	// something terraform destroy can remove. Without it a slow provision left a
	// running, billable VM that no state file referenced.
	plan.ID = types.StringValue(createResp.VMUUID)
	resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)
	plan.VMName = types.StringValue(createResp.VMName)
	plan.OSUser = types.StringValue(createResp.OSUser)
	plan.OSPassword = types.StringValue(createResp.OSPassword)
	plan.Status = types.StringValue(createResp.Status)

	// Wait for VM to start running
	wantPublicIP := !plan.AssignPublicIPs.IsNull() && plan.AssignPublicIPs.ValueBool()
	activeVM, err := r.waitForVMReady(ctx, createResp.VMUUID, wantPublicIP, 5*time.Minute)
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

	// The DELETE only queues the work: the platform tears the domain down and
	// releases its disks, IPs and NIC afterwards. Returning here would report
	// success for something not done yet - and if it then failed, the VM would
	// live on with nothing tracking it. Wait for it to actually disappear.
	if err := r.waitForVMGone(ctx, uuid, 10*time.Minute); err != nil {
		resp.Diagnostics.AddError("VM delete did not complete", err.Error())
	}
}

// waitForVMGone polls until the VM reads back as 404. Anything else - still
// there, or an error - keeps it in state, which is the safe direction: a
// resource Terraform still knows about can be retried, an orphan cannot.
func (r *vmResource) waitForVMGone(ctx context.Context, uuid string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		var vm models.VM
		err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.VMsEP, uuid), nil, &vm)
		if err != nil {
			if strings.Contains(err.Error(), "http 404") {
				return nil
			}
			return fmt.Errorf("could not confirm the VM is gone: %w", err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for VM %s to be deleted (status: %s)", uuid, vm.Status)
		}
		tflog.Debug(ctx, "Waiting for VM to disappear", map[string]any{
			"uuid": uuid, "status": vm.Status,
		})
		time.Sleep(2 * time.Second)
	}
}
