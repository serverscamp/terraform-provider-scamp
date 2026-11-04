package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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

// Users and databases of a PostgreSQL or MySQL server. The controller keeps the wanted state and applies it on its
// next pass, so every change waits until the server says it is applied.
const databaseAccessTimeout = 10 * time.Minute

func readAccess(ctx context.Context, c *client.Client, dbID string) (*models.DatabaseAccess, error) {
	var a models.DatabaseAccess
	err := c.GetJSON(ctx, client.DatabasesEP+"/"+dbID+"/access", nil, &a)
	return &a, err
}

// waitAccessApplied polls until the server has applied the users and databases. An error of the server that names
// what changed stops the wait.
func waitAccessApplied(ctx context.Context, c *client.Client, dbID, name string, done func(*models.DatabaseAccess) bool) error {
	deadline := time.Now().Add(databaseAccessTimeout)
	for {
		a, err := readAccess(ctx, c, dbID)
		if err == nil {
			if a.Applied {
				for _, e := range a.Errors {
					if strings.Contains(e, `"`+name+`"`) || strings.Contains(e, "'"+name+"'") {
						return fmt.Errorf("the server refused it: %s", e)
					}
				}
				if done(a) {
					return nil
				}
			}
		} else {
			tflog.Debug(ctx, "access read failed while waiting", map[string]any{"error": err.Error()})
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("not applied within %s", databaseAccessTimeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

// splitPair takes an import id "<database id>/<name>".
func splitPair(id string) (string, string, error) {
	i := strings.LastIndex(id, "/")
	if i <= 0 || i == len(id)-1 {
		return "", "", fmt.Errorf("use <id of the server>/<name>, got %q", id)
	}
	return id[:i], id[i+1:], nil
}

// ── scamp_database_db ─────────────────────────────────────────────────────

type databaseDBResource struct {
	c *client.Client
}

func NewDatabaseDBResource() tfresource.Resource { return &databaseDBResource{} }

type databaseDBModel struct {
	ID           types.String `tfsdk:"id"`
	DatabaseID   types.String `tfsdk:"database_id"`
	Name         types.String `tfsdk:"name"`
	ForceDestroy types.Bool   `tfsdk:"force_destroy"`
}

func (r *databaseDBResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_database_db"
}

func (r *databaseDBResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = rschema.Schema{
		Description: "Manages a database on a PostgreSQL or MySQL server of scamp_database. The server's owner user has full access to it; give other users access with scamp_database_user.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true, Description: "<database_id>/<name>.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"database_id": rschema.StringAttribute{
				Required: true, Description: "ID of the scamp_database server. Changing it replaces the database.", PlanModifiers: replace,
			},
			"name": rschema.StringAttribute{
				Required:      true,
				Description:   "Name of the database: lowercase letters, digits and underscores, starting with a letter. Changing it replaces the database, with its data.",
				PlanModifiers: replace,
			},
			"force_destroy": rschema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Destroy closes the open connections to the database. Off: destroy fails while there are any.",
			},
		},
	}
}

func (r *databaseDBResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func dbPresent(a *models.DatabaseAccess, name string) bool {
	for _, d := range a.Databases {
		if d.Name == name && d.State == nil {
			return true
		}
	}
	return false
}

func (r *databaseDBResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan databaseDBModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	dbID, name := plan.DatabaseID.ValueString(), plan.Name.ValueString()
	if err := r.c.PostJSON(ctx, client.DatabasesEP+"/"+dbID+"/dbs", map[string]any{"name": name}, nil); err != nil {
		resp.Diagnostics.AddError("Failed to create the database", err.Error())
		return
	}
	plan.ID = types.StringValue(dbID + "/" + name)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if err := waitAccessApplied(ctx, r.c, dbID, name, func(a *models.DatabaseAccess) bool { return dbPresent(a, name) }); err != nil {
		resp.Diagnostics.AddError("The database was not created", err.Error())
	}
}

func (r *databaseDBResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state databaseDBModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	a, err := readAccess(ctx, r.c, state.DatabaseID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "http 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the databases of the server", err.Error())
		return
	}
	found := false
	for _, d := range a.Databases {
		if d.Name == state.Name.ValueString() && (d.State == nil || *d.State != "deleting") {
			found = true
		}
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update only takes force_destroy; everything else replaces the database.
func (r *databaseDBResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan databaseDBModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *databaseDBResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state databaseDBModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	dbID, name := state.DatabaseID.ValueString(), state.Name.ValueString()
	q := url.Values{}
	if state.ForceDestroy.ValueBool() {
		q.Set("force", "true")
	}
	if err := r.c.DeleteQuery(ctx, client.DatabasesEP+"/"+dbID+"/dbs/"+url.PathEscape(name), q); err != nil &&
		!strings.Contains(err.Error(), "http 404") {
		resp.Diagnostics.AddError("Failed to delete the database", err.Error())
		return
	}
	err := waitAccessApplied(ctx, r.c, dbID, name, func(a *models.DatabaseAccess) bool {
		for _, d := range a.Databases {
			if d.Name == name {
				return false
			}
		}
		return true
	})
	if err != nil && !strings.Contains(err.Error(), "http 404") {
		resp.Diagnostics.AddError("The database is still there", err.Error())
	}
}

func (r *databaseDBResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	dbID, name, err := splitPair(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Bad import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database_id"), dbID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("force_destroy"), false)...)
}

// ── scamp_database_user ───────────────────────────────────────────────────

type databaseUserResource struct {
	c *client.Client
}

func NewDatabaseUserResource() tfresource.Resource { return &databaseUserResource{} }

type databaseUserModel struct {
	ID         types.String `tfsdk:"id"`
	DatabaseID types.String `tfsdk:"database_id"`
	Name       types.String `tfsdk:"name"`
	Grants     types.Map    `tfsdk:"grants"`
	Password   types.String `tfsdk:"password"`
}

func (r *databaseUserResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_database_user"
}

func (r *databaseUserResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = rschema.Schema{
		Description: "Manages a user of a PostgreSQL or MySQL server of scamp_database, with its access to the server's databases. The password is made by the server.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true, Description: "<database_id>/<name>.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"database_id": rschema.StringAttribute{
				Required: true, Description: "ID of the scamp_database server. Changing it replaces the user.", PlanModifiers: replace,
			},
			"name": rschema.StringAttribute{
				Required:      true,
				Description:   "Name of the user: lowercase letters, digits and underscores, starting with a letter. Changing it replaces the user.",
				PlanModifiers: replace,
			},
			"grants": rschema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Access by database name: read, write or admin, e.g. { app = \"write\" }. Without it the user has no database. Changed in place.",
			},
			"password": rschema.StringAttribute{
				Computed: true, Sensitive: true, Description: "The user's password.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *databaseUserResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func (r *databaseUserResource) ValidateConfig(ctx context.Context, req tfresource.ValidateConfigRequest, resp *tfresource.ValidateConfigResponse) {
	var cfg databaseUserModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || cfg.Grants.IsNull() || cfg.Grants.IsUnknown() {
		return
	}
	for db, v := range cfg.Grants.Elements() {
		s, ok := v.(types.String)
		if !ok || s.IsUnknown() || s.IsNull() {
			continue
		}
		switch s.ValueString() {
		case "read", "write", "admin":
		default:
			resp.Diagnostics.AddAttributeError(path.Root("grants").AtMapKey(db), "Unknown access", "Use read, write or admin.")
		}
	}
}

func userGrants(ctx context.Context, m databaseUserModel) map[string]string {
	out := map[string]string{}
	if !m.Grants.IsNull() && !m.Grants.IsUnknown() {
		_ = m.Grants.ElementsAs(ctx, &out, false)
	}
	return out
}

func (r *databaseUserResource) put(ctx context.Context, m *databaseUserModel) error {
	dbID, name := m.DatabaseID.ValueString(), m.Name.ValueString()
	var res struct {
		Password *string `json:"password"`
	}
	if err := r.c.PutJSON(ctx, client.DatabasesEP+"/"+dbID+"/users/"+url.PathEscape(name),
		map[string]any{"grants": userGrants(ctx, *m)}, &res); err != nil {
		return err
	}
	if res.Password != nil && *res.Password != "" {
		m.Password = types.StringValue(*res.Password)
	}
	want := fmt.Sprint(userGrants(ctx, *m))
	return waitAccessApplied(ctx, r.c, dbID, name, func(a *models.DatabaseAccess) bool {
		for _, u := range a.Users {
			if u.Name == name {
				return fmt.Sprint(u.Grants) == want || (len(u.Grants) == 0 && want == "map[]")
			}
		}
		return false
	})
}

func (r *databaseUserResource) password(ctx context.Context, m *databaseUserModel) {
	var pw models.ServicePassword
	ep := client.DatabasesEP + "/" + m.DatabaseID.ValueString() + "/users/" + url.PathEscape(m.Name.ValueString()) + "/password"
	if err := r.c.GetJSON(ctx, ep, nil, &pw); err != nil {
		tflog.Debug(ctx, "user password not read", map[string]any{"error": err.Error()})
		return
	}
	if pw.Password != nil {
		m.Password = types.StringValue(*pw.Password)
	}
}

func (r *databaseUserResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan databaseUserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = types.StringValue(plan.DatabaseID.ValueString() + "/" + plan.Name.ValueString())
	plan.Password = types.StringNull()
	err := r.put(ctx, &plan)
	if plan.Password.IsNull() {
		r.password(ctx, &plan)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if err != nil {
		resp.Diagnostics.AddError("The user was not created", err.Error())
	}
}

func (r *databaseUserResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state databaseUserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	a, err := readAccess(ctx, r.c, state.DatabaseID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "http 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the users of the server", err.Error())
		return
	}
	var grants map[string]string
	found := false
	for _, u := range a.Users {
		if u.Name == state.Name.ValueString() {
			found, grants = true, u.Grants
		}
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	if len(grants) == 0 && (state.Grants.IsNull() || len(state.Grants.Elements()) == 0) {
		// no grants reads back as the config wrote it: unset or {}
	} else {
		vals := map[string]attr.Value{}
		for k, v := range grants {
			vals[k] = types.StringValue(v)
		}
		state.Grants = types.MapValueMust(types.StringType, vals)
	}
	if state.Password.IsNull() {
		r.password(ctx, &state)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *databaseUserResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan, state databaseUserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Password = state.Password
	err := r.put(ctx, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if err != nil {
		resp.Diagnostics.AddError("Failed to change the user's access", err.Error())
	}
}

func (r *databaseUserResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state databaseUserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	dbID, name := state.DatabaseID.ValueString(), state.Name.ValueString()
	if err := r.c.Delete(ctx, client.DatabasesEP+"/"+dbID+"/users/"+url.PathEscape(name)); err != nil &&
		!strings.Contains(err.Error(), "http 404") {
		resp.Diagnostics.AddError("Failed to delete the user", err.Error())
		return
	}
	err := waitAccessApplied(ctx, r.c, dbID, name, func(a *models.DatabaseAccess) bool {
		for _, u := range a.Users {
			if u.Name == name {
				return false
			}
		}
		return true
	})
	if err != nil && !strings.Contains(err.Error(), "http 404") {
		resp.Diagnostics.AddError("The user is still there", err.Error())
	}
}

func (r *databaseUserResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	dbID, name, err := splitPair(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Bad import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("database_id"), dbID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}
