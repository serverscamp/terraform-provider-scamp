package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"

	"github.com/hashicorp/terraform-plugin-framework/path"
	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

type vmNetworkAttachmentResource struct {
	c *client.Client
}

func NewVMNetworkAttachmentResource() tfresource.Resource { return &vmNetworkAttachmentResource{} }

func (r *vmNetworkAttachmentResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_vm_network_attachment"
}

func (r *vmNetworkAttachmentResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		Description: "An extra network interface on a VM. The VM's first network is set on the VM " +
			"itself (primary_network_id); use this resource for every additional network, so one VM " +
			"can sit in two or more networks at once. Every field forces replacement: the platform " +
			"attaches and detaches interfaces, it does not move them between networks.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed:    true,
				Description: "Interface ID assigned by the platform.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vm_id": rschema.StringAttribute{
				Required:    true,
				Description: "UUID of the VM to attach the network to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"network_id": rschema.StringAttribute{
				Required:    true,
				Description: "UUID of the network to attach.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"is_primary": rschema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "Make this the VM's primary interface. Leave false unless you are " +
					"deliberately moving the default route.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"ip_internal": rschema.StringAttribute{
				Computed:    true,
				Description: "Internal IP the platform assigned on this network.",
			},
			"mac_address": rschema.StringAttribute{
				Computed:    true,
				Description: "MAC address of the interface.",
			},
		},
	}
}

func (r *vmNetworkAttachmentResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

type vmNetworkAttachmentModel struct {
	ID         types.String `tfsdk:"id"`
	VMID       types.String `tfsdk:"vm_id"`
	NetworkID  types.String `tfsdk:"network_id"`
	IsPrimary  types.Bool   `tfsdk:"is_primary"`
	IPInternal types.String `tfsdk:"ip_internal"`
	MACAddress types.String `tfsdk:"mac_address"`
}

// findNic looks an interface up in the VM's current NIC list, by interface id
// when we already know it, otherwise by the network it sits on.
//
// The id is not always available: on the durable path the attach endpoint
// answers "queued" with an empty interface_id, because the id is minted later,
// by the controller. Waiting for an empty id matched nothing and Terraform sat
// there until it gave up - while the NIC was in fact attached and working.
func (r *vmNetworkAttachmentResource) findNic(ctx context.Context, vmUUID, interfaceID, networkUUID string) (*models.VMNic, error) {
	var list models.VMNicListResponse
	if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s/nics", client.VMsEP, vmUUID), nil, &list); err != nil {
		return nil, err
	}
	for i := range list.Items {
		if interfaceID != "" && list.Items[i].UUID == interfaceID {
			return &list.Items[i], nil
		}
		if interfaceID == "" && networkUUID != "" && list.Items[i].NetworkUUID == networkUUID {
			return &list.Items[i], nil
		}
	}
	return nil, nil
}

func (r *vmNetworkAttachmentResource) waitForNic(ctx context.Context, vmUUID, interfaceID, networkUUID string, timeout time.Duration) (*models.VMNic, error) {
	deadline := time.Now().Add(timeout)
	for {
		nic, err := r.findNic(ctx, vmUUID, interfaceID, networkUUID)
		if err != nil {
			return nil, err
		}
		if nic != nil {
			return nic, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timeout waiting for an interface on network %s to appear on VM %s", networkUUID, vmUUID)
		}
		tflog.Debug(ctx, "Waiting for NIC to appear", map[string]any{
			"vm": vmUUID, "interface": interfaceID,
		})
		time.Sleep(1 * time.Second)
	}
}

func (r *vmNetworkAttachmentResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan vmNetworkAttachmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := map[string]any{
		"network_uuid": plan.NetworkID.ValueString(),
		"is_primary":   plan.IsPrimary.ValueBool(),
	}

	var attached models.VMNetworkAttachResponse
	url := fmt.Sprintf("%s/%s/network/attach", client.VMsEP, plan.VMID.ValueString())
	if err := r.c.PostJSON(ctx, url, payload, &attached); err != nil {
		resp.Diagnostics.AddError("Failed to attach network to VM", err.Error())
		return
	}

	// The attach is accepted, so the interface is on its way. Record what
	// identifies it before the wait: a timeout must not leave a NIC that no
	// state file knows about, because that NIC then blocks deleting the network
	// it sits on and nothing in the config can reach it. Read() finds the
	// interface by network when the id is still empty, so this is enough to
	// recover from.
	resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(attached.InterfaceID))
	resp.State.SetAttribute(ctx, path.Root("vm_id"), plan.VMID)
	resp.State.SetAttribute(ctx, path.Root("network_id"), plan.NetworkID)
	resp.State.SetAttribute(ctx, path.Root("is_primary"), plan.IsPrimary)

	nic, err := r.waitForNic(ctx, plan.VMID.ValueString(), attached.InterfaceID,
		plan.NetworkID.ValueString(), 5*time.Minute)
	if err != nil {
		resp.Diagnostics.AddError("Network attach did not complete", err.Error())
		return
	}

	// Whatever the attach call said, the id of record is the one the VM now
	// reports for this network.
	plan.ID = types.StringValue(nic.UUID)
	plan.IPInternal = types.StringValue(nic.IP)
	plan.MACAddress = types.StringValue(nic.MAC)
	plan.IsPrimary = types.BoolValue(nic.IsPrimary)
	resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)
	resp.State.SetAttribute(ctx, path.Root("vm_id"), plan.VMID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vmNetworkAttachmentResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state vmNetworkAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	nic, err := r.findNic(ctx, state.VMID.ValueString(), state.ID.ValueString(), state.NetworkID.ValueString())
	if err != nil {
		// The VM itself is gone or unreadable: let Terraform plan a re-create
		// rather than fail on a resource that no longer has a parent.
		tflog.Warn(ctx, "Could not read VM NICs, dropping attachment from state", map[string]any{
			"vm": state.VMID.ValueString(), "error": err.Error(),
		})
		resp.State.RemoveResource(ctx)
		return
	}
	if nic == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Heal a state written before the platform had minted the interface id:
	// the attach is asynchronous, so the id can arrive after Create returned.
	state.ID = types.StringValue(nic.UUID)
	state.NetworkID = types.StringValue(nic.NetworkUUID)
	state.IPInternal = types.StringValue(nic.IP)
	state.MACAddress = types.StringValue(nic.MAC)
	state.IsPrimary = types.BoolValue(nic.IsPrimary)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update never runs: every attribute is RequiresReplace. It exists because the
// resource interface demands it.
func (r *vmNetworkAttachmentResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan vmNetworkAttachmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vmNetworkAttachmentResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state vmNetworkAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Detach by interface_id, not network_uuid: a VM may hold more than one
	// interface on the same network, and only this one belongs to this resource.
	interfaceID := state.ID.ValueString()
	if interfaceID == "" {
		// State from an attach that returned before the id existed. Look the
		// interface up by network now, or the detach goes out with an empty id
		// and the API rejects the whole destroy.
		nic, err := r.findNic(ctx, state.VMID.ValueString(), "", state.NetworkID.ValueString())
		if err != nil {
			// The VM is gone, and its interfaces with it: nothing to detach.
			tflog.Warn(ctx, "Could not read VM NICs on delete, assuming already detached", map[string]any{
				"vm": state.VMID.ValueString(), "error": err.Error(),
			})
			return
		}
		if nic == nil {
			return // already detached
		}
		interfaceID = nic.UUID
	}

	payload := map[string]any{"interface_id": interfaceID}
	url := fmt.Sprintf("%s/%s/network/detach", client.VMsEP, state.VMID.ValueString())
	if err := r.c.PostJSON(ctx, url, payload, nil); err != nil {
		resp.Diagnostics.AddError("Failed to detach network from VM", err.Error())
		return
	}

	// The detach is queued, not done. Returning now let Terraform start
	// deleting the VM while the platform was still detaching, and the platform
	// refuses to delete a VM with an operation in flight - so a destroy that
	// looked correctly ordered failed anyway. Wait for the interface to go.
	if err := r.waitNicGone(ctx, state.VMID.ValueString(), interfaceID, 5*time.Minute); err != nil {
		resp.Diagnostics.AddError("Failed to detach network from VM", err.Error())
	}
}

func (r *vmNetworkAttachmentResource) waitNicGone(ctx context.Context, vmUUID, interfaceID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		nic, err := r.findNic(ctx, vmUUID, interfaceID, "")
		if err != nil {
			return nil // the VM is unreadable or gone: so is its interface
		}
		if nic == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for interface %s to detach from VM %s", interfaceID, vmUUID)
		}
		tflog.Debug(ctx, "Waiting for NIC to detach", map[string]any{
			"vm": vmUUID, "interface": interfaceID,
		})
		time.Sleep(2 * time.Second)
	}
}

// ImportState takes both ids, because an interface means nothing without the
// VM it hangs off:
//
//	terraform import scamp_vm_network_attachment.second <vm-uuid>/<interface-id>
//
// Read fills network_id, the address and the MAC from the VM's live NIC list.
func (r *vmNetworkAttachmentResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	parts := strings.Split(strings.TrimSpace(req.ID), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import id",
			fmt.Sprintf("Expected \"<vm-uuid>/<interface-id>\", got %q.", req.ID),
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("vm_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}
