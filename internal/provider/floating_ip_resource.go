package provider

import (
	"context"
	"fmt"
	"time"

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

type floatingIPResource struct {
	c *client.Client
}

func NewFloatingIPResource() tfresource.Resource { return &floatingIPResource{} }

func (r *floatingIPResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_floating_ip"
}

func (r *floatingIPResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		Description: "A floating IPv4 address. Reserved on its own and moved between VMs without " +
			"re-creating anything, which is what makes zero-downtime failover possible. " +
			"Attachment follows the same shape as scamp_volume: set attached_vm_id to attach, " +
			"clear it to detach.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed:    true,
				Description: "UUID of the floating IP.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": rschema.StringAttribute{
				Optional:    true,
				Description: "Name shown in the panel.",
			},
			"attached_vm_id": rschema.StringAttribute{
				Optional: true,
				Description: "UUID of the VM the address points at. Leave unset to reserve the " +
					"address without attaching it.",
			},
			"ip_address": rschema.StringAttribute{
				Computed:    true,
				Description: "The reserved IPv4 address.",
			},
			"ipv6_address": rschema.StringAttribute{
				Computed:    true,
				Description: "IPv6 address that comes with the reservation, if the pool assigned one.",
			},
			"status": rschema.StringAttribute{
				Computed:    true,
				Description: "reserved, attaching, floating or detaching.",
			},
			"created_at": rschema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the address was reserved.",
			},
		},
	}
}

func (r *floatingIPResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

type floatingIPModel struct {
	ID           types.String `tfsdk:"id"`
	DisplayName  types.String `tfsdk:"display_name"`
	AttachedVMID types.String `tfsdk:"attached_vm_id"`
	IPAddress    types.String `tfsdk:"ip_address"`
	IPv6Address  types.String `tfsdk:"ipv6_address"`
	Status       types.String `tfsdk:"status"`
	CreatedAt    types.String `tfsdk:"created_at"`
}

func (r *floatingIPResource) setModelFromIP(m *floatingIPModel, ip *models.FloatingIP) {
	m.ID = types.StringValue(ip.IPUUID)
	m.IPAddress = types.StringValue(ip.IPAddress)
	m.Status = types.StringValue(ip.Status)
	m.CreatedAt = types.StringValue(ip.CreatedAt)

	if ip.IPv6Address != nil {
		m.IPv6Address = types.StringValue(*ip.IPv6Address)
	} else {
		m.IPv6Address = types.StringNull()
	}
	if ip.VMUUID != nil {
		m.AttachedVMID = types.StringValue(*ip.VMUUID)
	} else {
		m.AttachedVMID = types.StringNull()
	}
	// display_name is optional in the config: only overwrite it from the API
	// when the API actually has one, so an unset config field stays null
	// instead of flapping between null and "" on every plan.
	if ip.DisplayName != nil && *ip.DisplayName != "" {
		m.DisplayName = types.StringValue(*ip.DisplayName)
	}
}

// waitForStatus polls until the address reaches one of the target statuses.
// Attach and detach are asynchronous on the platform: the call returns as soon
// as the operation is accepted, and the address is only usable once the fabric
// has the NAT in place.
func (r *floatingIPResource) waitForStatus(ctx context.Context, uuid string, targets []string, timeout time.Duration) (*models.FloatingIP, error) {
	deadline := time.Now().Add(timeout)
	for {
		var ip models.FloatingIP
		if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.FloatingIPsEP, uuid), nil, &ip); err != nil {
			return nil, err
		}
		for _, target := range targets {
			if ip.Status == target {
				return &ip, nil
			}
		}
		if ip.StatusError != nil && *ip.StatusError != "" {
			return &ip, fmt.Errorf("floating IP %s failed: %s", uuid, *ip.StatusError)
		}
		if time.Now().After(deadline) {
			return &ip, fmt.Errorf("timeout waiting for floating IP %s to reach %v (current: %s)", uuid, targets, ip.Status)
		}
		tflog.Debug(ctx, "Waiting for floating IP status", map[string]any{
			"uuid": uuid, "current": ip.Status, "targets": targets,
		})
		time.Sleep(1 * time.Second)
	}
}

func (r *floatingIPResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan floatingIPModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := map[string]any{}
	if !plan.DisplayName.IsNull() {
		payload["display_name"] = plan.DisplayName.ValueString()
	}

	var reserved models.FloatingIPReserveResponse
	if err := r.c.PostJSON(ctx, client.FloatingIPsEP, payload, &reserved); err != nil {
		resp.Diagnostics.AddError("Failed to reserve floating IP", err.Error())
		return
	}

	// Reserved and billable from here on: record it before anything can fail.
	plan.ID = types.StringValue(reserved.IPUUID)
	resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)

	if !plan.AttachedVMID.IsNull() && plan.AttachedVMID.ValueString() != "" {
		attachPayload := map[string]any{"vm_uuid": plan.AttachedVMID.ValueString()}
		var attached models.FloatingIPAttachResponse
		if err := r.c.PostJSON(ctx, fmt.Sprintf("%s/%s/attach", client.FloatingIPsEP, reserved.IPUUID), attachPayload, &attached); err != nil {
			var ip models.FloatingIP
			if rerr := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.FloatingIPsEP, reserved.IPUUID), nil, &ip); rerr == nil {
				r.setModelFromIP(&plan, &ip)
				resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
			}
			resp.Diagnostics.AddError("Failed to attach floating IP to VM", err.Error())
			return
		}
		if _, err := r.waitForStatus(ctx, reserved.IPUUID, []string{"floating"}, 5*time.Minute); err != nil {
			resp.Diagnostics.AddWarning("Floating IP attached but status not confirmed", err.Error())
		}
	}

	wanted := plan.AttachedVMID

	var ip models.FloatingIP
	if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.FloatingIPsEP, reserved.IPUUID), nil, &ip); err != nil {
		resp.Diagnostics.AddError("Failed to read floating IP after create", err.Error())
		return
	}
	r.setModelFromIP(&plan, &ip)

	// attached_vm_id is set in the config, so the state we return has to match
	// it. When the attach did not land, the API reports no VM and overwriting
	// the field with that null makes Terraform abort the whole apply with
	// "provider produced inconsistent result". Keep the configured value: the
	// warning above already says the attach is unconfirmed, and the next plan
	// reads the real state and re-attaches.
	if !wanted.IsNull() && wanted.ValueString() != "" && plan.AttachedVMID.IsNull() {
		plan.AttachedVMID = wanted
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *floatingIPResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state floatingIPModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var ip models.FloatingIP
	if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.FloatingIPsEP, state.ID.ValueString()), nil, &ip); err != nil {
		// Released outside Terraform: drop it from state instead of failing the plan.
		tflog.Warn(ctx, "Floating IP not readable, removing from state", map[string]any{
			"uuid": state.ID.ValueString(), "error": err.Error(),
		})
		resp.State.RemoveResource(ctx)
		return
	}
	r.setModelFromIP(&state, &ip)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *floatingIPResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan, state floatingIPModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := state.ID.ValueString()
	plan.ID = state.ID

	oldVM := state.AttachedVMID.ValueString()
	newVM := plan.AttachedVMID.ValueString()

	if oldVM != newVM {
		// Detach first: an address can only point at one VM, and moving it is
		// exactly the failover this resource exists for.
		if oldVM != "" {
			if err := r.c.PostJSON(ctx, fmt.Sprintf("%s/%s/detach", client.FloatingIPsEP, uuid), nil, nil); err != nil {
				resp.Diagnostics.AddError("Failed to detach floating IP", err.Error())
				return
			}
			if _, err := r.waitForStatus(ctx, uuid, []string{"reserved"}, 5*time.Minute); err != nil {
				resp.Diagnostics.AddWarning("Floating IP detached but status not confirmed", err.Error())
			}
		}
		if newVM != "" {
			attachPayload := map[string]any{"vm_uuid": newVM}
			if err := r.c.PostJSON(ctx, fmt.Sprintf("%s/%s/attach", client.FloatingIPsEP, uuid), attachPayload, nil); err != nil {
				resp.Diagnostics.AddError("Failed to attach floating IP to VM", err.Error())
				return
			}
			if _, err := r.waitForStatus(ctx, uuid, []string{"floating"}, 5*time.Minute); err != nil {
				resp.Diagnostics.AddWarning("Floating IP attached but status not confirmed", err.Error())
			}
		}
	}

	wanted := plan.AttachedVMID

	var ip models.FloatingIP
	if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.FloatingIPsEP, uuid), nil, &ip); err != nil {
		resp.Diagnostics.AddError("Failed to read floating IP after update", err.Error())
		return
	}
	r.setModelFromIP(&plan, &ip)
	// Same reason as in Create: the state has to keep the configured target,
	// or an unconfirmed attach fails the apply instead of the next plan fixing
	// it.
	if !wanted.IsNull() && wanted.ValueString() != "" && plan.AttachedVMID.IsNull() {
		plan.AttachedVMID = wanted
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *floatingIPResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state floatingIPModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := state.ID.ValueString()
	// Release only works on a detached address, so unhook it first if needed.
	if !state.AttachedVMID.IsNull() && state.AttachedVMID.ValueString() != "" {
		if err := r.c.PostJSON(ctx, fmt.Sprintf("%s/%s/detach", client.FloatingIPsEP, uuid), nil, nil); err != nil {
			resp.Diagnostics.AddError("Failed to detach floating IP before release", err.Error())
			return
		}
		if _, err := r.waitForStatus(ctx, uuid, []string{"reserved"}, 5*time.Minute); err != nil {
			resp.Diagnostics.AddWarning("Floating IP detached but status not confirmed", err.Error())
		}
	}

	if err := r.c.Delete(ctx, fmt.Sprintf("%s/%s", client.FloatingIPsEP, uuid)); err != nil {
		resp.Diagnostics.AddError("Failed to release floating IP", err.Error())
		return
	}
}
