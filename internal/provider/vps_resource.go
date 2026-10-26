package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

const vpsTimeout = 15 * time.Minute

type vpsResource struct {
	c *client.Client
}

func NewVPSResource() tfresource.Resource { return &vpsResource{} }

type vpsModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Plan       types.String `tfsdk:"plan"`
	Image      types.String `tfsdk:"image"`
	SSHKeyIDs  types.List   `tfsdk:"ssh_key_ids"`
	Password   types.String `tfsdk:"password"`
	Backups    types.Bool   `tfsdk:"backups"`
	ReverseDNS types.String `tfsdk:"reverse_dns"`
	Hostname   types.String `tfsdk:"hostname"`
	Status     types.String `tfsdk:"status"`
	IPv4       types.String `tfsdk:"ipv4"`
	IPv6Prefix types.String `tfsdk:"ipv6_prefix"`
	CPUModel   types.String `tfsdk:"cpu_model"`
	CreatedAt  types.String `tfsdk:"created_at"`
}

func (r *vpsResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_vps"
}

// Schema: ssh_key_ids and password replace the server when they change, except when the state has no value:
// the API never returns them, so after an import they are null and filling them in must not destroy the server.
func (r *vpsResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	keepAfterImport := "the API does not return it, so after an import a value set in the config is only recorded"
	resp.Schema = rschema.Schema{
		Description: "Manages a Simple VPS: a server with a fixed plan, a public IPv4 address and an IPv6 prefix.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed:      true,
				Description:   "The UUID of the VPS.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": rschema.StringAttribute{
				Required:      true,
				Description:   "Name of the server: 3 to 24 lowercase letters, digits and hyphens, starting with a letter. Changing it replaces the server.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"plan": rschema.StringAttribute{
				Required:    true,
				Description: "Plan id, e.g. vps2-1 (see the scamp_vps_plans data source). Changed in place to a bigger plan, the server restarts; a smaller plan is refused because the disk cannot shrink.",
			},
			"image": rschema.StringAttribute{
				Required:      true,
				Description:   "Image id, e.g. debian-13 (see the scamp_vps_images data source). Changing it replaces the server.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"ssh_key_ids": rschema.ListAttribute{
				ElementType: types.Int64Type,
				Optional:    true,
				Description: "IDs of SSH keys saved in the account (scamp_ssh_key) to put on the server. Set this, password, or both. The keys go onto the server once, when it is made, so changing the list replaces the server; to keep the server, add lifecycle { ignore_changes = [ssh_key_ids] }. " + strings.ToUpper(keepAfterImport[:1]) + keepAfterImport[1:] + ".",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.ListRequest, resp *listplanmodifier.RequiresReplaceIfFuncResponse) {
						resp.RequiresReplace = !req.StateValue.IsNull()
					}, "Replaces the server when the keys change.", "Replaces the server when the keys change."),
				},
			},
			"password": rschema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Root password, 8 to 128 characters. Kept in the state like any sensitive value. Changing it replaces the server; " + keepAfterImport + ".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIf(func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
						resp.RequiresReplace = !req.StateValue.IsNull()
					}, "Replaces the server when the password changes.", "Replaces the server when the password changes."),
				},
			},
			"backups": rschema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Daily backups; they add the plan's backup share to the price. Turning them off deletes every backup of the server.",
			},
			"reverse_dns": rschema.StringAttribute{
				Optional:    true,
				Description: "Reverse DNS of the IPv4 and IPv6 addresses, e.g. mail.example.com. Without it the default name (hostname) is used.",
			},
			"hostname": rschema.StringAttribute{
				Computed:      true,
				Description:   "The default name of the server, NAME-xxxxxx.vm.scamp.pro.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status": rschema.StringAttribute{
				Computed:    true,
				Description: "running, stopped, paused, unpaid or error.",
			},
			"ipv4": rschema.StringAttribute{
				Computed:      true,
				Description:   "Public IPv4 address.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ipv6_prefix": rschema.StringAttribute{
				Computed:      true,
				Description:   "The server's IPv6 prefix, e.g. 2a01:ea05:0:2400::/56.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cpu_model": rschema.StringAttribute{
				Computed:      true,
				Description:   "CPU model of the host.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"created_at": rschema.StringAttribute{
				Computed:      true,
				Description:   "When the server was made.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *vpsResource) ValidateConfig(ctx context.Context, req tfresource.ValidateConfigRequest, resp *tfresource.ValidateConfigResponse) {
	var cfg vpsModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// unknown counts as set: it comes from another resource and will be there by apply
	if cfg.SSHKeyIDs.IsNull() && cfg.Password.IsNull() {
		resp.Diagnostics.AddError("Neither ssh_key_ids nor password is set",
			"Set ssh_key_ids, password, or both, so that you can sign in to the server.")
	}
	if !cfg.Password.IsNull() && !cfg.Password.IsUnknown() {
		if n := len(cfg.Password.ValueString()); n < 8 || n > 128 {
			resp.Diagnostics.AddAttributeError(path.Root("password"), "Password length", "The root password is 8 to 128 characters.")
		}
	}
}

func (r *vpsResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func (r *vpsResource) get(ctx context.Context, id string) (*models.VPS, error) {
	var v models.VPS
	err := r.c.GetJSON(ctx, client.VPSEP+"/"+id, nil, &v)
	return &v, err
}

// setFromVPS copies what the API returns. The password and the SSH keys never come back and stay as they are.
func setFromVPS(m *vpsModel, v *models.VPS) {
	m.ID = types.StringValue(v.UUID)
	m.Name = types.StringValue(v.Name)
	m.Plan = types.StringValue(v.PlanID)
	m.Image = types.StringValue(v.Image)
	m.Backups = types.BoolValue(v.Backups)
	m.Status = types.StringValue(v.Status)
	m.IPv4 = types.StringValue(v.IPv4)
	m.IPv6Prefix = types.StringValue(v.IPv6Prefix)
	m.CPUModel = types.StringValue(v.CPUModel)
	m.CreatedAt = types.StringValue(v.CreatedAt)
	if v.PTRCustom {
		m.ReverseDNS = types.StringValue(v.PTR)
	} else {
		m.ReverseDNS = types.StringNull()
		if v.PTR != "" {
			m.Hostname = types.StringValue(v.PTR)
		}
	}
	if m.Hostname.IsUnknown() {
		m.Hostname = types.StringNull()
	}
}

// waitRunning polls until the server runs. An error status stops the wait with the API's reason.
func (r *vpsResource) waitRunning(ctx context.Context, id string) (*models.VPS, error) {
	deadline := time.Now().Add(vpsTimeout)
	for {
		v, err := r.get(ctx, id)
		if err == nil {
			switch v.Status {
			case "running":
				return v, nil
			case "error":
				return v, fmt.Errorf("the server is in error: %s", v.Error)
			case "unpaid":
				return v, fmt.Errorf("the server is stopped: the balance ran out")
			}
		} else if !strings.Contains(err.Error(), "http 404") {
			tflog.Debug(ctx, "VPS read failed while waiting", map[string]any{"error": err.Error()})
		}
		if time.Now().After(deadline) {
			return v, fmt.Errorf("the server did not reach running within %s", vpsTimeout)
		}
		select {
		case <-ctx.Done():
			return v, ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

// waitRebooted polls until the server's uptime is below what it was before, i.e. it has booted again.
func (r *vpsResource) waitRebooted(ctx context.Context, id string, before int64) error {
	deadline := time.Now().Add(vpsTimeout)
	for {
		if v, err := r.get(ctx, id); err == nil && v.Status == "running" && v.Uptime != nil && *v.Uptime < before {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("no restart seen within %s", vpsTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}
}

func (r *vpsResource) setPTR(ctx context.Context, id, hostname string) error {
	return r.c.PostJSON(ctx, client.VPSEP+"/"+id+"/ptr", map[string]string{"hostname": hostname}, nil)
}

func idempotencyKey() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return "tf-" + hex.EncodeToString(b)
}

func (r *vpsResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan vpsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := map[string]any{
		"name":    plan.Name.ValueString(),
		"plan_id": plan.Plan.ValueString(),
		"image":   plan.Image.ValueString(),
		"backups": plan.Backups.ValueBool(),
	}
	if !plan.SSHKeyIDs.IsNull() {
		var ids []int64
		resp.Diagnostics.Append(plan.SSHKeyIDs.ElementsAs(ctx, &ids, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		// the API skips keys it does not find without a word, and the server would come up without them
		for _, id := range ids {
			if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%d", client.SSHKeysEP, id), nil, &models.SSHKey{}); err != nil {
				resp.Diagnostics.AddAttributeError(path.Root("ssh_key_ids"), "SSH key not found",
					fmt.Sprintf("SSH key %d is not in this account: %s", id, err.Error()))
				return
			}
		}
		payload["ssh_key_ids"] = ids
	}
	if !plan.Password.IsNull() && plan.Password.ValueString() != "" {
		payload["password"] = plan.Password.ValueString()
	}

	// The same key on every try: a lost answer never makes a second server.
	key := idempotencyKey()
	var created models.VPSCreated
	var err error
	for try := 0; try < 4; try++ {
		err = r.c.PostJSONWith(ctx, client.VPSEP, payload, map[string]string{"Idempotency-Key": key}, &created)
		if err == nil || strings.HasPrefix(err.Error(), "http 4") {
			break
		}
		tflog.Warn(ctx, "VPS create got no answer, asking again with the same Idempotency-Key", map[string]any{"error": err.Error()})
		time.Sleep(time.Duration(2<<try) * time.Second)
	}
	if err != nil {
		resp.Diagnostics.AddError("Failed to create the VPS", err.Error())
		return
	}

	plan.ID = types.StringValue(created.UUID)
	plan.Hostname = types.StringValue(created.Hostname)
	// saved before the wait, so a server that fails to come up is still in the state and destroy removes it
	plan.Status = types.StringValue("creating")
	plan.IPv4, plan.IPv6Prefix, plan.CPUModel, plan.CreatedAt = types.StringNull(), types.StringNull(), types.StringNull(), types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	v, err := r.waitRunning(ctx, created.UUID)
	if err != nil {
		resp.Diagnostics.AddError("The VPS did not come up", err.Error())
		return
	}
	want := plan.ReverseDNS
	if !want.IsNull() && want.ValueString() != "" {
		if err := r.setPTR(ctx, created.UUID, want.ValueString()); err != nil {
			resp.Diagnostics.AddError("Failed to set reverse DNS", err.Error())
			return
		}
		if v, err = r.get(ctx, created.UUID); err != nil {
			resp.Diagnostics.AddError("Failed to read the VPS", err.Error())
			return
		}
	}
	setFromVPS(&plan, v)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vpsResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state vpsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	v, err := r.get(ctx, state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "http 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the VPS", err.Error())
		return
	}
	setFromVPS(&state, v)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vpsResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan, state vpsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()

	if plan.Plan.ValueString() != state.Plan.ValueString() {
		var before int64 = -1
		if v, err := r.get(ctx, id); err == nil && v.Uptime != nil {
			before = *v.Uptime
		}
		var changed struct {
			Rebooted bool `json:"rebooted"`
		}
		if err := r.c.PostJSON(ctx, client.VPSEP+"/"+id+"/plan", map[string]string{"plan_id": plan.Plan.ValueString()}, &changed); err != nil {
			resp.Diagnostics.AddError("Failed to change the plan", err.Error())
			return
		}
		// the status stays running through the restart: wait for the uptime to drop, so the apply ends
		// with the server back on the new plan and not in the middle of its reboot
		if changed.Rebooted && before >= 0 {
			if err := r.waitRebooted(ctx, id, before); err != nil {
				resp.Diagnostics.AddError("The VPS did not restart after the plan change", err.Error())
				return
			}
		}
		if _, err := r.waitRunning(ctx, id); err != nil {
			resp.Diagnostics.AddError("The VPS did not come back after the plan change", err.Error())
			return
		}
	}
	if plan.Backups.ValueBool() != state.Backups.ValueBool() {
		if err := r.c.PostJSON(ctx, client.VPSEP+"/"+id+"/backups", map[string]bool{"enabled": plan.Backups.ValueBool()}, nil); err != nil {
			resp.Diagnostics.AddError("Failed to change backups", err.Error())
			return
		}
	}
	if !plan.ReverseDNS.Equal(state.ReverseDNS) {
		if err := r.setPTR(ctx, id, plan.ReverseDNS.ValueString()); err != nil {
			resp.Diagnostics.AddError("Failed to set reverse DNS", err.Error())
			return
		}
	}

	v, err := r.get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the VPS", err.Error())
		return
	}
	plan.Hostname = state.Hostname
	setFromVPS(&plan, v)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vpsResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state vpsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if err := r.c.Delete(ctx, client.VPSEP+"/"+id); err != nil && !strings.Contains(err.Error(), "http 404") {
		resp.Diagnostics.AddError("Failed to delete the VPS", err.Error())
		return
	}
	deadline := time.Now().Add(vpsTimeout)
	for {
		_, err := r.get(ctx, id)
		if err != nil && strings.Contains(err.Error(), "http 404") {
			return
		}
		if time.Now().After(deadline) {
			resp.Diagnostics.AddError("The VPS is still there", fmt.Sprintf("%s still reads back after %s", id, vpsTimeout))
			return
		}
		time.Sleep(3 * time.Second)
	}
}

// ImportState takes the VPS UUID. password and ssh_key_ids stay empty: the API does not return them.
func (r *vpsResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	tfresource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
