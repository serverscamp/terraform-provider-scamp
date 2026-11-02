package provider

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

// GitLab takes about ten minutes to install; the API gives up on an install after an hour.
const managedAppTimeout = 65 * time.Minute

type managedAppResource struct {
	c *client.Client
}

func NewManagedAppResource() tfresource.Resource { return &managedAppResource{} }

type managedAppModel struct {
	ID              types.String `tfsdk:"id"`
	App             types.String `tfsdk:"app"`
	Name            types.String `tfsdk:"name"`
	Plan            types.String `tfsdk:"plan"`
	NetworkID       types.String `tfsdk:"network_id"`
	SecurityGroupID types.String `tfsdk:"security_group_id"`
	SSHKeyID        types.Int64  `tfsdk:"ssh_key_id"`
	Public          types.Bool   `tfsdk:"public"`
	Maintenance     types.String `tfsdk:"maintenance"`
	Hostname        types.String `tfsdk:"hostname"`
	URL             types.String `tfsdk:"url"`
	AdminUser       types.String `tfsdk:"admin_user"`
	AdminPassword   types.String `tfsdk:"admin_password"`
	IPv4            types.String `tfsdk:"ipv4"`
	IPv6            types.String `tfsdk:"ipv6"`
	PrivateIP       types.String `tfsdk:"private_ip"`
	SSHUser         types.String `tfsdk:"ssh_user"`
	SSHPassword     types.String `tfsdk:"ssh_password"`
	VMID            types.String `tfsdk:"vm_id"`
	State           types.String `tfsdk:"state"`
}

func (r *managedAppResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_managed_app"
}

func (r *managedAppResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = rschema.Schema{
		Description: "Manages a One-Click App: GitLab or Docker with Portainer on its own server, installed and kept up to date by ServersCamp. The app has its own name under apps.scamp.pro with a certificate we renew. Without a public address it is reached from the private network and over VPN Gateway.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true, Description: "The app's id.", PlanModifiers: keep,
			},
			"app": rschema.StringAttribute{
				Required: true, Description: "Which app: gitlab or docker. Changing it replaces the app.", PlanModifiers: replace,
			},
			"name": rschema.StringAttribute{
				Required:      true,
				Description:   "Name of the app: 3 to 32 lowercase letters, digits and hyphens, starting with a letter. It is the start of the address, NAME-xxxxxx.APP.apps.scamp.pro. Changing it replaces the app.",
				PlanModifiers: replace,
			},
			"plan": rschema.StringAttribute{
				Required:      true,
				Description:   "Server plan, e.g. basic-8g-dedicated. Apps run on dedicated Basic plans; GitLab needs 8 GB or more. Changing it replaces the app.",
				PlanModifiers: replace,
			},
			"network_id": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "UUID of the network, attached to a router. Without it the account's default network. Changing it replaces the app.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"security_group_id": rschema.StringAttribute{
				Optional:      true,
				Description:   "UUID of a security group. Without it a new group is made with SSH, HTTP and HTTPS open. Changing it replaces the app.",
				PlanModifiers: replace,
			},
			"ssh_key_id": rschema.Int64Attribute{
				Optional:      true,
				Description:   "ID of an SSH key saved in the account, for root over SSH. Without it the server has a password (see the panel). Changing it replaces the app.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"public": rschema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Public IPv4 and IPv6. Off: the app's name points at its private address and it is reached from the private network and over VPN Gateway. Changed in place; turned on again it gets a new address.",
			},
			"maintenance": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString("manual"),
				Description:   "App updates: manual (we tell you, you start them) or off. Changing it replaces the app.",
				PlanModifiers: replace,
			},
			"hostname": rschema.StringAttribute{
				Computed: true, Description: "The app's name, NAME-xxxxxx.APP.apps.scamp.pro.", PlanModifiers: keep,
			},
			"url": rschema.StringAttribute{
				Computed: true, Description: "https:// address of the app.", PlanModifiers: keep,
			},
			"admin_user": rschema.StringAttribute{
				Computed: true, Description: "The app's administrator.", PlanModifiers: keep,
			},
			"admin_password": rschema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				Description:   "The administrator's first password. Once it is changed in the app this one stops working; the state keeps the first one.",
				PlanModifiers: keep,
			},
			"ipv4": rschema.StringAttribute{
				Computed: true, Description: "Public IPv4, empty when public is off.",
			},
			"ipv6": rschema.StringAttribute{
				Computed: true, Description: "Public IPv6, empty when public is off.",
			},
			"private_ip": rschema.StringAttribute{
				Computed: true, Description: "The server's address in its network: SSH and the app over VPN Gateway.",
			},
			"ssh_user": rschema.StringAttribute{
				Computed: true, Description: "The server's user for SSH, with sudo.", PlanModifiers: keep,
			},
			"ssh_password": rschema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				Description:   "The password of ssh_user, as the server was made with it.",
				PlanModifiers: keep,
			},
			"vm_id": rschema.StringAttribute{
				Computed: true, Description: "UUID of the app's server.", PlanModifiers: keep,
			},
			"state": rschema.StringAttribute{
				Computed: true, Description: "creating, installing, running, failed or deleting.",
			},
		},
	}
}

func (r *managedAppResource) ValidateConfig(ctx context.Context, req tfresource.ValidateConfigRequest, resp *tfresource.ValidateConfigResponse) {
	var cfg managedAppModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	checkServiceName(cfg.Name, &resp.Diagnostics)
	if v := cfg.Maintenance; !v.IsNull() && !v.IsUnknown() && v.ValueString() != "manual" && v.ValueString() != "off" {
		resp.Diagnostics.AddAttributeError(path.Root("maintenance"), "Unknown maintenance", "Use manual or off.")
	}
}

func (r *managedAppResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func (r *managedAppResource) get(ctx context.Context, id string) (*models.ManagedApp, error) {
	var a models.ManagedApp
	err := r.c.GetJSON(ctx, client.ManagedAppsEP+"/"+id, nil, &a)
	return &a, err
}

func strOrNull(p *string) types.String {
	if p == nil || *p == "" {
		return types.StringNull()
	}
	return types.StringValue(*p)
}

func setFromApp(m *managedAppModel, a *models.ManagedApp) {
	m.ID = types.StringValue(a.ID)
	m.App = types.StringValue(a.App)
	m.Name = types.StringValue(a.Name)
	if a.Plan != nil {
		m.Plan = types.StringValue(*a.Plan)
	}
	m.Hostname = types.StringValue(a.Hostname)
	m.URL = types.StringValue(a.URL)
	m.AdminUser = types.StringValue(a.AppUser)
	m.IPv4 = strOrNull(a.IPv4)
	m.IPv6 = strOrNull(a.IPv6)
	m.PrivateIP = strOrNull(a.Private)
	m.VMID = strOrNull(a.VMUUID)
	m.State = types.StringValue(a.State)
	// while the server is made it has no address yet; once it has one, public follows it
	if a.State != "creating" {
		m.Public = types.BoolValue(a.Public)
	}
}

// sizeID finds the plan among the sizes the app can run on, with what is wrong when it is not one of them.
func (r *managedAppResource) sizeID(ctx context.Context, app, plan string) (int64, error) {
	var cat models.AppCatalog
	if err := r.c.GetJSON(ctx, client.ManagedAppsEP+"/catalog", nil, &cat); err != nil {
		return 0, err
	}
	var kind *models.AppKind
	var keys []string
	for i := range cat.Apps {
		keys = append(keys, cat.Apps[i].Key)
		if cat.Apps[i].Key == app {
			kind = &cat.Apps[i]
		}
	}
	if kind == nil {
		return 0, fmt.Errorf("no app %q; available: %s", app, strings.Join(keys, ", "))
	}
	var fits []string
	for _, s := range cat.Sizes {
		ok := s.HWSlug == kind.Hardware && s.MemoryMB >= kind.MinMemoryMB
		modeOK := false
		for _, m := range kind.CPUModes {
			modeOK = modeOK || m == s.CPUMode
		}
		if !ok || !modeOK {
			continue
		}
		fits = append(fits, s.Name)
		if norm(s.Name) == norm(plan) {
			switch {
			case s.Locked:
				return 0, fmt.Errorf("plan %s is above the limits of your account", s.Name)
			case s.SoldOut:
				return 0, fmt.Errorf("plan %s is out of capacity right now", s.Name)
			}
			return s.ID, nil
		}
	}
	return 0, fmt.Errorf("plan %q is not one %s runs on; available: %s", plan, app, strings.Join(fits, ", "))
}

// appSlug is the random part of the address: 6 lowercase letters and digits.
func appSlug() string {
	const abc = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = abc[int(b[i])%len(abc)]
	}
	return string(b)
}

// waitRunning polls until the app runs. A failed install stops the wait with the step that broke.
func (r *managedAppResource) waitRunning(ctx context.Context, id string) (*models.ManagedApp, error) {
	deadline := time.Now().Add(managedAppTimeout)
	for {
		a, err := r.get(ctx, id)
		if err == nil {
			switch a.State {
			case "running":
				return a, nil
			case "failed":
				msg := "the install failed"
				if a.Error != nil {
					msg = *a.Error
				}
				return a, fmt.Errorf("%s", msg)
			}
		} else {
			tflog.Debug(ctx, "app read failed while waiting", map[string]any{"error": err.Error()})
		}
		if time.Now().After(deadline) {
			return a, fmt.Errorf("the app did not reach running within %s", managedAppTimeout)
		}
		select {
		case <-ctx.Done():
			return a, ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

// passwords fills the app administrator's first password and the server's SSH user and password.
// What the API does not give keeps its value.
func (r *managedAppResource) passwords(ctx context.Context, m *managedAppModel) {
	var pw models.ManagedAppPasswords
	if err := r.c.GetJSON(ctx, client.ManagedAppsEP+"/"+m.ID.ValueString()+"/password", nil, &pw); err != nil {
		tflog.Debug(ctx, "app passwords not read", map[string]any{"error": err.Error()})
		return
	}
	if pw.App.Password != nil {
		m.AdminPassword = types.StringValue(*pw.App.Password)
	}
	if pw.Server.User != "" {
		m.SSHUser = types.StringValue(pw.Server.User)
	}
	if pw.Server.Password != nil {
		m.SSHPassword = types.StringValue(*pw.Server.Password)
	}
}

func (r *managedAppResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan managedAppModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sizeID, err := r.sizeID(ctx, plan.App.ValueString(), plan.Plan.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("plan"), "Plan not available", err.Error())
		return
	}
	network, err := networkOrDefault(ctx, r.c, plan.NetworkID)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("network_id"), "No network", err.Error())
		return
	}
	plan.NetworkID = types.StringValue(network)
	payload := map[string]any{
		"app":          plan.App.ValueString(),
		"name":         plan.Name.ValueString(),
		"slug":         appSlug(),
		"vm_class_id":  sizeID,
		"network_uuid": network,
		"maintenance":  plan.Maintenance.ValueString(),
		"public":       plan.Public.ValueBool(),
	}
	if !plan.SecurityGroupID.IsNull() && plan.SecurityGroupID.ValueString() != "" {
		payload["sg_uuid"] = plan.SecurityGroupID.ValueString()
	}
	if !plan.SSHKeyID.IsNull() {
		payload["ssh_key_id"] = plan.SSHKeyID.ValueInt64()
	}
	var created struct {
		ID       string `json:"id"`
		Hostname string `json:"hostname"`
		URL      string `json:"url"`
	}
	if err := r.c.PostJSON(ctx, client.ManagedAppsEP, payload, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create the app", err.Error())
		return
	}

	plan.ID = types.StringValue(created.ID)
	plan.Hostname = types.StringValue(created.Hostname)
	plan.URL = types.StringValue(created.URL)
	plan.State = types.StringValue("creating")
	plan.AdminUser, plan.AdminPassword = types.StringNull(), types.StringNull()
	plan.SSHUser, plan.SSHPassword, plan.PrivateIP = types.StringNull(), types.StringNull(), types.StringNull()
	plan.IPv4, plan.IPv6, plan.VMID = types.StringNull(), types.StringNull(), types.StringNull()
	// saved before the wait, so an app that fails to install is still in the state and destroy removes it
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	a, err := r.waitRunning(ctx, created.ID)
	if a != nil && a.ID != "" {
		setFromApp(&plan, a)
		r.passwords(ctx, &plan)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("The app did not come up", err.Error())
	}
}

func (r *managedAppResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state managedAppModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	a, err := r.get(ctx, state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "http 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the app", err.Error())
		return
	}
	if a.State == "deleting" {
		resp.State.RemoveResource(ctx)
		return
	}
	setFromApp(&state, a)
	if state.AdminPassword.IsNull() || state.SSHPassword.IsNull() {
		r.passwords(ctx, &state)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update only turns the public address on or off; everything else replaces the app.
func (r *managedAppResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan, state managedAppModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if plan.Public.ValueBool() != state.Public.ValueBool() {
		vm := state.VMID.ValueString()
		if vm == "" {
			resp.Diagnostics.AddError("The app has no server yet", "Apply again once the app is running.")
			return
		}
		ep := client.VMsEP + "/" + vm + "/public-ip"
		var err error
		if plan.Public.ValueBool() {
			err = r.c.PostJSON(ctx, ep, nil, nil)
		} else {
			err = r.c.Delete(ctx, ep)
		}
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("public"), "Failed to change the public address", err.Error())
			return
		}
	}
	a, err := r.get(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the app", err.Error())
		return
	}
	plan.AdminPassword, plan.SSHUser, plan.SSHPassword = state.AdminPassword, state.SSHUser, state.SSHPassword
	setFromApp(&plan, a)
	plan.Public = types.BoolValue(a.Public)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *managedAppResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state managedAppModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	deadline := time.Now().Add(managedAppTimeout)
	// an app whose server is still being made cannot be deleted yet: wait for it
	for {
		err := r.c.Delete(ctx, client.ManagedAppsEP+"/"+id)
		if err == nil || strings.Contains(err.Error(), "http 404") {
			break
		}
		if !strings.Contains(err.Error(), "http 409") || time.Now().After(deadline) {
			resp.Diagnostics.AddError("Failed to delete the app", err.Error())
			return
		}
		time.Sleep(10 * time.Second)
	}
	for {
		_, err := r.get(ctx, id)
		if err != nil && strings.Contains(err.Error(), "http 404") {
			return
		}
		if time.Now().After(deadline) {
			resp.Diagnostics.AddError("The app is still there", fmt.Sprintf("%s still reads back after %s", id, managedAppTimeout))
			return
		}
		time.Sleep(5 * time.Second)
	}
}

// ImportState takes the app's id. The first password is read from the API.
func (r *managedAppResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	tfresource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
