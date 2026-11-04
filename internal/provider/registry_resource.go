package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

const (
	registryTimeout  = 20 * time.Minute
	registryMainUser = "main"
)

type registryResource struct {
	c *client.Client
}

func NewRegistryResource() tfresource.Resource { return &registryResource{} }

type registryModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Package     types.String `tfsdk:"package"`
	GrowUpTo    types.String `tfsdk:"grow_up_to"`
	NetworkID   types.String `tfsdk:"network_id"`
	PublicRepos types.List   `tfsdk:"public_repos"`
	Host        types.String `tfsdk:"host"`
	User        types.String `tfsdk:"user"`
	Password    types.String `tfsdk:"password"`
	IP          types.String `tfsdk:"ip"`
	PrivateIP   types.String `tfsdk:"private_ip"`
	State       types.String `tfsdk:"state"`
}

func (r *registryResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_registry"
}

func (r *registryResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = rschema.Schema{
		Description: "Manages a container registry on its own server, for Docker and OCI images and Helm charts, with its own name under registry.serverscamp.com and a certificate we renew. The main user has the whole registry; add others with scamp_registry_user.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true, Description: "The registry's id.", PlanModifiers: keep,
			},
			"name": rschema.StringAttribute{
				Required:      true,
				Description:   "Name of the registry: 3 to 32 lowercase letters, digits and hyphens, starting with a letter. It is the start of its address, NAME-xxxxxx.registry.serverscamp.com. Changing it replaces the registry and every image in it.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"package": rschema.StringAttribute{
				Required:      true,
				Description:   "Package by slug or name: rg-10 (10 GB) up to rg-1000 (1 TB). The registry moves up by itself only to grow_up_to; a package is never moved down, so changing it replaces the registry.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"grow_up_to": rschema.StringAttribute{
				Optional:    true,
				Description: "A bigger package the registry moves up to by itself when it is full. Changed in place.",
			},
			"network_id": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "UUID of the network, attached to a router. Without it the account's default network. Changing it replaces the registry and every image in it.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"public_repos": rschema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Repositories anyone may pull without logging in: names like app or patterns like public/*. Changed in place.",
			},
			"host": rschema.StringAttribute{
				Computed: true, Description: "The registry's name, NAME-xxxxxx.registry.serverscamp.com: docker login HOST.", PlanModifiers: keep,
			},
			"user": rschema.StringAttribute{
				Computed: true, Description: "The main user, with the whole registry.", PlanModifiers: keep,
			},
			"password": rschema.StringAttribute{
				Computed: true, Sensitive: true, Description: "The main user's password.", PlanModifiers: keep,
			},
			"ip": rschema.StringAttribute{
				Computed: true, Description: "Public address of the registry.",
			},
			"private_ip": rschema.StringAttribute{
				Computed: true, Description: "Address of the registry in its network.",
			},
			"state": rschema.StringAttribute{
				Computed: true, Description: "creating, configuring, active or failed.",
			},
		},
	}
}

func (r *registryResource) ValidateConfig(ctx context.Context, req tfresource.ValidateConfigRequest, resp *tfresource.ValidateConfigResponse) {
	var cfg registryModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	checkServiceName(cfg.Name, &resp.Diagnostics)
}

func (r *registryResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func (r *registryResource) packageID(ctx context.Context, want string) (int64, error) {
	var cat struct {
		Packages []models.StoragePackage `json:"packages"`
	}
	if err := r.c.GetJSON(ctx, client.RegistryEP+"/catalog", nil, &cat); err != nil {
		return 0, err
	}
	labels := make([][]string, len(cat.Packages))
	for i, p := range cat.Packages {
		labels[i] = []string{p.Slug, p.Name}
	}
	i, err := pickByLabels(want, labels)
	if err != nil {
		return 0, err
	}
	if cat.Packages[i].SoldOut {
		return 0, fmt.Errorf("package %s is out of capacity right now", cat.Packages[i].Slug)
	}
	return cat.Packages[i].ID, nil
}

func (r *registryResource) get(ctx context.Context, id string) (*models.Registry, error) {
	return getRegistry(ctx, r.c, id)
}

func getRegistry(ctx context.Context, c *client.Client, id string) (*models.Registry, error) {
	var g models.Registry
	err := c.GetJSON(ctx, client.RegistryEP+"/"+id, nil, &g)
	return &g, err
}

func setFromRegistry(ctx context.Context, m *registryModel, g *models.Registry) {
	m.ID = types.StringValue(g.ID)
	m.Name = types.StringValue(g.Name)
	m.NetworkID = types.StringValue(g.Network)
	m.Host = types.StringValue(g.Host)
	m.User = types.StringValue(registryMainUser)
	m.IP = strOrNull(g.IP)
	m.PrivateIP = strOrNull(g.PrivateIP)
	m.State = types.StringValue(g.State)
	same := func(cur types.String, p *models.StoragePackage) bool {
		return !cur.IsNull() && !cur.IsUnknown() && p != nil && (norm(cur.ValueString()) == norm(p.Slug) || norm(cur.ValueString()) == norm(p.Name))
	}
	// the registry moves up by itself: the package it started on stays as written, a move up is not a change
	if g.Package != nil && (m.Package.IsNull() || m.Package.IsUnknown()) {
		m.Package = types.StringValue(g.Package.Slug)
	}
	if g.GrowUpTo == nil {
		m.GrowUpTo = types.StringNull()
	} else if !same(m.GrowUpTo, g.GrowUpTo) {
		m.GrowUpTo = types.StringValue(g.GrowUpTo.Slug)
	}
	// none public reads back as the config wrote it: unset or []
	if len(g.PublicRepos) > 0 || !m.PublicRepos.IsNull() {
		m.PublicRepos, _ = types.ListValueFrom(ctx, types.StringType, nonNil(g.PublicRepos))
	}
}

func (r *registryResource) login(ctx context.Context, m *registryModel) {
	var l models.ServicePassword
	if err := r.c.GetJSON(ctx, client.RegistryEP+"/"+m.ID.ValueString()+"/users/"+registryMainUser+"/login", nil, &l); err != nil {
		tflog.Debug(ctx, "registry login not read", map[string]any{"error": err.Error()})
		return
	}
	if l.Password != nil {
		m.Password = types.StringValue(*l.Password)
	}
}

func (r *registryResource) waitActive(ctx context.Context, id string) (*models.Registry, error) {
	var last *models.Registry
	err := waitServiceState(ctx, registryTimeout, func() (string, string, error) {
		g, err := r.get(ctx, id)
		if err != nil {
			tflog.Debug(ctx, "registry read failed while waiting", map[string]any{"error": err.Error()})
			return "", "", err
		}
		last = g
		msg := ""
		if g.Error != nil {
			msg = *g.Error
		}
		return g.State, msg, nil
	}, "active")
	return last, err
}

func (r *registryResource) putPublic(ctx context.Context, id string, m *registryModel) error {
	return r.c.PutJSON(ctx, client.RegistryEP+"/"+id+"/public", map[string]any{"repos": listStrings(ctx, m.PublicRepos)}, nil)
}

func (r *registryResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan registryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pid, err := r.packageID(ctx, plan.Package.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("package"), "Package not available", err.Error())
		return
	}
	network, err := networkOrDefault(ctx, r.c, plan.NetworkID)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("network_id"), "No network", err.Error())
		return
	}
	payload := map[string]any{
		"name":         plan.Name.ValueString(),
		"suffix":       hexSlug(),
		"package_id":   pid,
		"network_uuid": network,
	}
	if !plan.GrowUpTo.IsNull() && plan.GrowUpTo.ValueString() != "" {
		gid, err := r.packageID(ctx, plan.GrowUpTo.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("grow_up_to"), "Package not available", err.Error())
			return
		}
		payload["grow_up_to_package_id"] = gid
	}
	var created struct {
		ID   string `json:"id"`
		Host string `json:"host"`
	}
	if err := r.c.PostJSON(ctx, client.RegistryEP, payload, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create the registry", err.Error())
		return
	}
	plan.ID = types.StringValue(created.ID)
	plan.NetworkID = types.StringValue(network)
	plan.Host = types.StringValue(created.Host)
	plan.User = types.StringValue(registryMainUser)
	plan.Password, plan.IP, plan.PrivateIP = types.StringNull(), types.StringNull(), types.StringNull()
	plan.State = types.StringValue("creating")
	// saved before the wait, so a registry that fails to come up is still in the state and destroy removes it
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	g, err := r.waitActive(ctx, created.ID)
	if err == nil && len(listStrings(ctx, plan.PublicRepos)) > 0 {
		if perr := r.putPublic(ctx, created.ID, &plan); perr != nil {
			resp.Diagnostics.AddAttributeError(path.Root("public_repos"), "Failed to set the public repositories", perr.Error())
		}
		g, err = r.waitActive(ctx, created.ID)
	}
	if g != nil && g.ID != "" {
		setFromRegistry(ctx, &plan, g)
		r.login(ctx, &plan)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("The registry did not come up", err.Error())
	}
}

func (r *registryResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state registryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	g, err := r.get(ctx, state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "http 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the registry", err.Error())
		return
	}
	if g.State == "gone" {
		resp.State.RemoveResource(ctx)
		return
	}
	setFromRegistry(ctx, &state, g)
	if state.Password.IsNull() {
		r.login(ctx, &state)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *registryResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan, state registryModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if !plan.GrowUpTo.Equal(state.GrowUpTo) {
		body := map[string]any{"grow_up_to_package_id": nil}
		if !plan.GrowUpTo.IsNull() && plan.GrowUpTo.ValueString() != "" {
			gid, err := r.packageID(ctx, plan.GrowUpTo.ValueString())
			if err != nil {
				resp.Diagnostics.AddAttributeError(path.Root("grow_up_to"), "Package not available", err.Error())
				return
			}
			body["grow_up_to_package_id"] = gid
		}
		if err := r.c.PutJSON(ctx, client.RegistryEP+"/"+id+"/grow", body, nil); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("grow_up_to"), "Failed to change grow_up_to", err.Error())
			return
		}
	}
	if fmt.Sprint(listStrings(ctx, plan.PublicRepos)) != fmt.Sprint(listStrings(ctx, state.PublicRepos)) {
		if err := r.putPublic(ctx, id, &plan); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("public_repos"), "Failed to set the public repositories", err.Error())
			return
		}
	}
	g, err := r.waitActive(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("The registry is not active after the change", err.Error())
		return
	}
	plan.Password = state.Password
	setFromRegistry(ctx, &plan, g)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *registryResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state registryModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	deadline := time.Now().Add(registryTimeout)
	// a registry still being made cannot be deleted yet: wait for it
	for {
		err := r.c.Delete(ctx, client.RegistryEP+"/"+id)
		if err == nil || strings.Contains(err.Error(), "http 404") {
			return
		}
		if !strings.Contains(err.Error(), "http 409") || time.Now().After(deadline) {
			resp.Diagnostics.AddError("Failed to delete the registry", err.Error())
			return
		}
		time.Sleep(10 * time.Second)
	}
}

// ImportState takes the registry's id. The main user's password is read from the API.
func (r *registryResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	tfresource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ── scamp_registry_user ───────────────────────────────────────────────────

type registryUserResource struct {
	c *client.Client
}

func NewRegistryUserResource() tfresource.Resource { return &registryUserResource{} }

type registryGrantModel struct {
	Repos types.String `tfsdk:"repos"`
	Level types.String `tfsdk:"level"`
}

type registryUserModel struct {
	ID         types.String         `tfsdk:"id"`
	RegistryID types.String         `tfsdk:"registry_id"`
	Name       types.String         `tfsdk:"name"`
	Access     []registryGrantModel `tfsdk:"access"`
	Disabled   types.Bool           `tfsdk:"disabled"`
	Password   types.String         `tfsdk:"password"`
}

func (r *registryUserResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_registry_user"
}

func (r *registryUserResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = rschema.Schema{
		Description: "Manages a user of a scamp_registry besides main, with read or write access by repository. The password is made by the registry. A CI pipeline usually gets its own user with write access to its repositories.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true, Description: "<registry_id>/<name>.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"registry_id": rschema.StringAttribute{
				Required: true, Description: "ID of the scamp_registry. Changing it replaces the user.", PlanModifiers: replace,
			},
			"name": rschema.StringAttribute{
				Required:      true,
				Description:   "Name of the user: 2 to 32 lowercase letters, digits, - and _. Changing it replaces the user.",
				PlanModifiers: replace,
			},
			"access": rschema.ListNestedAttribute{
				Optional:    true,
				Description: "What the user may do, up to 50 rules. Without it the user may only log in. Changed in place.",
				NestedObject: rschema.NestedAttributeObject{
					Attributes: map[string]rschema.Attribute{
						"repos": rschema.StringAttribute{
							Required: true, Description: "A repository like app, a pattern like team/*, or * for every repository.",
						},
						"level": rschema.StringAttribute{
							Required: true, Description: "read (pull) or write (pull and push).",
						},
					},
				},
			},
			"disabled": rschema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "The user is kept but cannot log in. Changed in place.",
			},
			"password": rschema.StringAttribute{
				Computed: true, Sensitive: true, Description: "The user's password.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *registryUserResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func (r *registryUserResource) ValidateConfig(ctx context.Context, req tfresource.ValidateConfigRequest, resp *tfresource.ValidateConfigResponse) {
	var cfg registryUserModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if v := cfg.Name; !v.IsNull() && !v.IsUnknown() && (v.ValueString() == registryMainUser || v.ValueString() == "anonymous") {
		resp.Diagnostics.AddAttributeError(path.Root("name"), "Name is taken", v.ValueString()+" is the registry's own.")
	}
	for i, g := range cfg.Access {
		if v := g.Level; !v.IsNull() && !v.IsUnknown() && v.ValueString() != "read" && v.ValueString() != "write" {
			resp.Diagnostics.AddAttributeError(path.Root("access").AtListIndex(i).AtName("level"), "Unknown level", "Use read or write.")
		}
	}
}

func grantsOut(gs []registryGrantModel) []map[string]string {
	out := []map[string]string{}
	for _, g := range gs {
		out = append(out, map[string]string{"repos": strings.TrimSpace(g.Repos.ValueString()), "level": g.Level.ValueString()})
	}
	return out
}

func (r *registryUserResource) find(ctx context.Context, m registryUserModel) (*models.RegistryUser, error) {
	g, err := getRegistry(ctx, r.c, m.RegistryID.ValueString())
	if err != nil {
		return nil, err
	}
	for i := range g.Users {
		if g.Users[i].Name == m.Name.ValueString() {
			return &g.Users[i], nil
		}
	}
	return nil, nil
}

func setFromRegistryUser(m *registryUserModel, u *models.RegistryUser) {
	m.Disabled = types.BoolValue(u.Disabled)
	if len(u.Access) == 0 && len(m.Access) == 0 {
		return // none reads back as the config wrote it: unset or []
	}
	m.Access = make([]registryGrantModel, len(u.Access))
	for i, g := range u.Access {
		m.Access[i] = registryGrantModel{Repos: types.StringValue(g.Repos), Level: types.StringValue(g.Level)}
	}
}

func (r *registryUserResource) password(ctx context.Context, m *registryUserModel) {
	var l models.ServicePassword
	ep := client.RegistryEP + "/" + m.RegistryID.ValueString() + "/users/" + url.PathEscape(m.Name.ValueString()) + "/login"
	if err := r.c.GetJSON(ctx, ep, nil, &l); err != nil {
		tflog.Debug(ctx, "registry user password not read", map[string]any{"error": err.Error()})
		return
	}
	if l.Password != nil {
		m.Password = types.StringValue(*l.Password)
	}
}

func (r *registryUserResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan registryUserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	rid, name := plan.RegistryID.ValueString(), plan.Name.ValueString()
	if err := r.c.PostJSON(ctx, client.RegistryEP+"/"+rid+"/users", map[string]any{
		"name": name, "access": grantsOut(plan.Access),
	}, nil); err != nil {
		resp.Diagnostics.AddError("Failed to create the user", err.Error())
		return
	}
	plan.ID = types.StringValue(rid + "/" + name)
	plan.Password = types.StringNull()
	// a user is made enabled: turned off right away when the config says so
	if plan.Disabled.ValueBool() {
		if err := r.c.PutJSON(ctx, client.RegistryEP+"/"+rid+"/users/"+url.PathEscape(name), map[string]any{
			"access": grantsOut(plan.Access), "disabled": true,
		}, nil); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("disabled"), "Failed to turn the user off", err.Error())
		}
	}
	r.password(ctx, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *registryUserResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state registryUserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	u, err := r.find(ctx, state)
	if err != nil {
		if strings.Contains(err.Error(), "http 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the registry", err.Error())
		return
	}
	if u == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	setFromRegistryUser(&state, u)
	if state.Password.IsNull() {
		r.password(ctx, &state)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *registryUserResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan, state registryUserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.c.PutJSON(ctx, client.RegistryEP+"/"+state.RegistryID.ValueString()+"/users/"+url.PathEscape(state.Name.ValueString()),
		map[string]any{"access": grantsOut(plan.Access), "disabled": plan.Disabled.ValueBool()}, nil); err != nil {
		resp.Diagnostics.AddError("Failed to change the user", err.Error())
		return
	}
	plan.Password = state.Password
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *registryUserResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state registryUserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.c.Delete(ctx, client.RegistryEP+"/"+state.RegistryID.ValueString()+"/users/"+url.PathEscape(state.Name.ValueString()))
	if err != nil && !strings.Contains(err.Error(), "http 404") {
		resp.Diagnostics.AddError("Failed to delete the user", err.Error())
	}
}

func (r *registryUserResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	rid, name, err := splitPair(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Bad import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("registry_id"), rid)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("disabled"), false)...)
}
