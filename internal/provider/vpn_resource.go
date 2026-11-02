package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

// ---- scamp_vpn_device ---------------------------------------------------------

type vpnDeviceResource struct {
	c *client.Client
}

func NewVPNDeviceResource() tfresource.Resource { return &vpnDeviceResource{} }

type vpnDeviceModel struct {
	ID         types.String `tfsdk:"id"`
	RouterID   types.String `tfsdk:"router_id"`
	Name       types.String `tfsdk:"name"`
	FullTunnel types.Bool   `tfsdk:"full_tunnel"`
	Address    types.String `tfsdk:"address"`
	PublicKey  types.String `tfsdk:"public_key"`
	Config     types.String `tfsdk:"config"`
}

func (r *vpnDeviceResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_vpn_device"
}

func (r *vpnDeviceResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = rschema.Schema{
		Description: "Manages a device of VPN Gateway: a WireGuard peer with its own address. The config with the device's private key is made once, at create, and kept only in the state: put it in a vault with scamp_vault_secret or read it with terraform output.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true, Description: "ROUTER_ID/DEVICE_ID.", PlanModifiers: keep,
			},
			"router_id": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "UUID of the router whose VPN Gateway the device joins. Without it the router of the account's default network. Changing it replaces the device.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"name": rschema.StringAttribute{
				Required: true, Description: "Name of the device, up to 64 characters. Changing it replaces the device and its config.", PlanModifiers: replace,
			},
			"full_tunnel": rschema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				Description:   "All traffic of the device through the gateway. Off: only the router's networks. Changing it replaces the device and its config.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
			},
			"address": rschema.StringAttribute{
				Computed: true, Description: "The device's address in the VPN.", PlanModifiers: keep,
			},
			"public_key": rschema.StringAttribute{
				Computed: true, Description: "The device's WireGuard public key.", PlanModifiers: keep,
			},
			"config": rschema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				Description:   "WireGuard config for the device, with its private key. Made once; empty after an import.",
				PlanModifiers: keep,
			},
		},
	}
}

func (r *vpnDeviceResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func (r *vpnDeviceResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan vpnDeviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	router := plan.RouterID.ValueString()
	if plan.RouterID.IsNull() || plan.RouterID.IsUnknown() || router == "" {
		var err error
		if router, err = defaultRouter(ctx, r.c); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("router_id"), "No router", err.Error())
			return
		}
		plan.RouterID = types.StringValue(router)
	}
	payload := map[string]any{"name": plan.Name.ValueString(), "full_tunnel": plan.FullTunnel.ValueBool()}
	var d models.VPNDevice
	if err := r.c.PostJSON(ctx, client.NetsvcEP+"/"+router+"/devices", payload, &d); err != nil {
		resp.Diagnostics.AddError("Failed to add the VPN device", err.Error())
		return
	}
	plan.ID = types.StringValue(fmt.Sprintf("%s/%d", router, d.ID))
	plan.Address = types.StringValue(d.Address)
	plan.Config = types.StringValue(d.Config)
	plan.PublicKey = types.StringNull()
	// the answer has no public key: the list has it
	if cur, err := r.find(ctx, router, d.ID); err == nil && cur != nil {
		plan.PublicKey = types.StringValue(cur.PublicKey)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vpnDeviceResource) find(ctx context.Context, router string, id int64) (*models.VPNDevice, error) {
	var list struct {
		Items []models.VPNDevice `json:"items"`
	}
	if err := r.c.GetJSON(ctx, client.NetsvcEP+"/"+router+"/devices", nil, &list); err != nil {
		return nil, err
	}
	for i := range list.Items {
		if list.Items[i].ID == id {
			return &list.Items[i], nil
		}
	}
	return nil, nil
}

func deviceID(id string) (string, int64, error) {
	router, num, ok := strings.Cut(id, "/")
	n, err := strconv.ParseInt(num, 10, 64)
	if !ok || router == "" || err != nil {
		return "", 0, fmt.Errorf("expected ROUTER_ID/DEVICE_ID, got %q", id)
	}
	return router, n, nil
}

func (r *vpnDeviceResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state vpnDeviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	router, id, err := deviceID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Bad device id", err.Error())
		return
	}
	d, err := r.find(ctx, router, id)
	if err != nil {
		if strings.Contains(err.Error(), "http 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the VPN device", err.Error())
		return
	}
	if d == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	state.RouterID = types.StringValue(router)
	state.Name = types.StringValue(d.Name)
	state.FullTunnel = types.BoolValue(d.FullTunnel)
	state.Address = types.StringValue(d.Address)
	state.PublicKey = types.StringValue(d.PublicKey)
	if state.Config.IsUnknown() {
		state.Config = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vpnDeviceResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan vpnDeviceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vpnDeviceResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state vpnDeviceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	router, id, err := deviceID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Bad device id", err.Error())
		return
	}
	err = r.c.Delete(ctx, fmt.Sprintf("%s/%s/devices/%d", client.NetsvcEP, router, id))
	if err != nil && !strings.Contains(err.Error(), "http 404") {
		resp.Diagnostics.AddError("Failed to remove the VPN device", err.Error())
	}
}

// ImportState takes ROUTER_ID/DEVICE_ID. The config is not there after an import: it is made only once.
func (r *vpnDeviceResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	router, _, err := deviceID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Bad import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("router_id"), router)...)
}
