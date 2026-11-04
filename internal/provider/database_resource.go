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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

// A database takes a few minutes; a resize restarts the server and waits for it.
const databaseTimeout = 40 * time.Minute

var pitrDays = []int64{3, 7, 14, 30}

type databaseResource struct {
	c *client.Client
}

func NewDatabaseResource() tfresource.Resource { return &databaseResource{} }

type databaseModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Engine          types.String `tfsdk:"engine"`
	Version         types.String `tfsdk:"version"`
	Plan            types.String `tfsdk:"plan"`
	DiskGB          types.Int64  `tfsdk:"disk_gb"`
	NetworkID       types.String `tfsdk:"network_id"`
	SecurityGroupID types.String `tfsdk:"security_group_id"`
	Public          types.Bool   `tfsdk:"public"`
	RequireTLS      types.Bool   `tfsdk:"require_tls"`
	Durability      types.String `tfsdk:"durability"`
	Backup          types.Bool   `tfsdk:"backup"`
	BackupDays      types.Int64  `tfsdk:"backup_days"`
	Host            types.String `tfsdk:"host"`
	Port            types.Int64  `tfsdk:"port"`
	PoolPort        types.Int64  `tfsdk:"pool_port"`
	PrivateIP       types.String `tfsdk:"private_ip"`
	PublicHost      types.String `tfsdk:"public_host"`
	PublicPort      types.Int64  `tfsdk:"public_port"`
	User            types.String `tfsdk:"user"`
	Password        types.String `tfsdk:"password"`
	DefaultDatabase types.String `tfsdk:"default_database"`
	State           types.String `tfsdk:"state"`
}

func (r *databaseResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_database"
}

func (r *databaseResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	keepInt := []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}
	resp.Schema = rschema.Schema{
		Description: "Manages a managed database: PostgreSQL, MySQL, Valkey, OpenSearch, ClickHouse, VictoriaMetrics, VictoriaLogs or Qdrant on its own server, run, backed up and updated by ServersCamp. The database has its own name under the database domain with a certificate we renew.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true, Description: "The database's id.", PlanModifiers: keep,
			},
			"name": rschema.StringAttribute{
				Required:      true,
				Description:   "Name of the database server: 3 to 32 lowercase letters, digits and hyphens, starting with a letter. It is the start of its address. Changing it replaces the database.",
				PlanModifiers: replace,
			},
			"engine": rschema.StringAttribute{
				Required:      true,
				Description:   "postgresql, mysql, valkey, opensearch, clickhouse, victoriametrics, victorialogs or qdrant. Changing it replaces the database.",
				PlanModifiers: replace,
			},
			"version": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Engine version, e.g. 18 for PostgreSQL. Without it the engine's default. Changing it replaces the database.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"plan": rschema.StringAttribute{
				Required:    true,
				Description: "Plan, e.g. db-bs-s. Changed in place to another plan of the same engine and line (Basic or High Frequency); the server restarts.",
			},
			"disk_gb": rschema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Disk size in GB. Without it the disk that comes with the plan. Grows in place, online; it never shrinks. The disk also grows by itself when it fills up, so leave it unset unless you want a bigger start.",
				PlanModifiers: keepInt,
			},
			"network_id": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "UUID of the network, attached to a router. Without it the account's default network. Changing it replaces the database.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"security_group_id": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "UUID of a security group. Without it a new group is made with the engine's ports open and everything from the private network and VPN Gateway. Changing it replaces the database.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"public": rschema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Ports on the router's public address. Who may connect is the security group's to say. Changed in place.",
			},
			"require_tls": rschema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Every network connection over TLS. Changed in place.",
			},
			"durability": rschema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("standard"),
				Description: "Write mode: standard, fast or extreme (no backups in extreme). Changed in place.",
			},
			"backup": rschema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(true),
				Description: "Point-in-time backups into our S3, for engines that have them. Changed in place; turned off, the stored backups are deleted.",
			},
			"backup_days": rschema.Int64Attribute{
				Optional: true, Computed: true, Default: int64default.StaticInt64(3),
				Description: "Days of point-in-time restore: 3, 7, 14 or 30. Changed in place.",
			},
			"host": rschema.StringAttribute{
				Computed: true, Description: "The database's name, for connections from the private network and over VPN Gateway.", PlanModifiers: keep,
			},
			"port":      rschema.Int64Attribute{Computed: true, Description: "The engine's port.", PlanModifiers: keepInt},
			"pool_port": rschema.Int64Attribute{Computed: true, Description: "Port of the connection pooler (PgBouncer, transaction mode), where the engine has one."},
			"private_ip": rschema.StringAttribute{
				Computed: true, Description: "The server's address in its network.",
			},
			"public_host": rschema.StringAttribute{Computed: true, Description: "Public address, empty when public is off."},
			"public_port": rschema.Int64Attribute{Computed: true, Description: "Public port of the engine, empty when public is off."},
			"user": rschema.StringAttribute{
				Computed: true, Description: "The owner user.", PlanModifiers: keep,
			},
			"password": rschema.StringAttribute{
				Computed: true, Sensitive: true, Description: "The owner user's password.", PlanModifiers: keep,
			},
			"default_database": rschema.StringAttribute{
				Computed: true, Description: "The database made with the server, where the engine has one.", PlanModifiers: keep,
			},
			"state": rschema.StringAttribute{
				Computed: true, Description: "creating, configuring, running, updating or failed.",
			},
		},
	}
}

func (r *databaseResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func (r *databaseResource) ValidateConfig(ctx context.Context, req tfresource.ValidateConfigRequest, resp *tfresource.ValidateConfigResponse) {
	var cfg databaseModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	checkServiceName(cfg.Name, &resp.Diagnostics)
	if v := cfg.Durability; !v.IsNull() && !v.IsUnknown() {
		switch v.ValueString() {
		case "standard", "fast", "extreme":
		default:
			resp.Diagnostics.AddAttributeError(path.Root("durability"), "Unknown durability", "Use standard, fast or extreme.")
		}
	}
	if v := cfg.BackupDays; !v.IsNull() && !v.IsUnknown() {
		ok := false
		for _, d := range pitrDays {
			ok = ok || d == v.ValueInt64()
		}
		if !ok {
			resp.Diagnostics.AddAttributeError(path.Root("backup_days"), "Unknown backup_days", "Use 3, 7, 14 or 30.")
		}
	}
}

// ModifyPlan stops a smaller disk at plan time: a database disk only grows.
func (r *databaseResource) ModifyPlan(ctx context.Context, req tfresource.ModifyPlanRequest, resp *tfresource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var plan, state databaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !plan.DiskGB.IsUnknown() && !plan.DiskGB.IsNull() && plan.DiskGB.ValueInt64() < state.DiskGB.ValueInt64() {
		resp.Diagnostics.AddAttributeError(path.Root("disk_gb"), "A disk only grows",
			fmt.Sprintf("The disk is %d GB; it cannot go down to %d GB.", state.DiskGB.ValueInt64(), plan.DiskGB.ValueInt64()))
	}
}

func (r *databaseResource) get(ctx context.Context, id string) (*models.Database, error) {
	var d models.Database
	err := r.c.GetJSON(ctx, client.DatabasesEP+"/"+id, nil, &d)
	return &d, err
}

func (r *databaseResource) waitRunning(ctx context.Context, id string) (*models.Database, error) {
	var last *models.Database
	err := waitServiceState(ctx, databaseTimeout, func() (string, string, error) {
		d, err := r.get(ctx, id)
		if err != nil {
			tflog.Debug(ctx, "database read failed while waiting", map[string]any{"error": err.Error()})
			return "", "", err
		}
		last = d
		msg := ""
		if d.Error != nil {
			msg = *d.Error
		}
		return d.State, msg, nil
	}, "running")
	return last, err
}

func int64OrNull(p *int64) types.Int64 {
	if p == nil {
		return types.Int64Null()
	}
	return types.Int64Value(*p)
}

func setFromDatabase(m *databaseModel, d *models.Database) {
	m.ID = types.StringValue(d.ID)
	m.Name = types.StringValue(d.Name)
	m.Engine = types.StringValue(d.Engine)
	m.Version = types.StringValue(d.Version)
	if m.Plan.IsNull() || m.Plan.IsUnknown() || norm(m.Plan.ValueString()) != norm(d.Plan) {
		m.Plan = types.StringValue(d.Plan)
	}
	m.DiskGB = types.Int64Value(d.DiskGB)
	m.NetworkID = types.StringValue(d.NetworkUUID)
	if d.Options.SGUUID != nil && *d.Options.SGUUID != "" {
		m.SecurityGroupID = types.StringValue(*d.Options.SGUUID)
	}
	m.Public = types.BoolValue(d.Public.Enabled)
	m.RequireTLS = types.BoolValue(d.Public.RequireSSL)
	// an engine without the setting keeps what the config says, so it does not plan a change forever
	if hasCap(d, "durability") {
		m.Durability = types.StringValue(d.Durability)
	}
	if hasCap(d, "backup") && d.Durability != "extreme" {
		m.Backup = types.BoolValue(d.Backup.Enabled)
		if d.Options.PITRDays != nil {
			m.BackupDays = types.Int64Value(*d.Options.PITRDays)
		}
	}
	m.Host = types.StringValue(d.Host)
	m.Port = types.Int64Value(d.Port)
	m.PoolPort = int64OrNull(d.PoolPort)
	m.PrivateIP = strOrNull(d.IP)
	m.PublicHost, m.PublicPort = types.StringNull(), types.Int64Null()
	if d.Public.Enabled {
		m.PublicHost = strOrNull(d.Public.Host)
		m.PublicPort = int64OrNull(d.Public.Port)
	}
	m.User = types.StringValue(d.User)
	m.DefaultDatabase = strOrNull(d.Database)
	m.State = types.StringValue(d.State)
}

func hasCap(d *models.Database, c string) bool {
	for _, x := range d.Caps {
		if x == c {
			return true
		}
	}
	return false
}

func (r *databaseResource) password(ctx context.Context, m *databaseModel) {
	var pw models.ServicePassword
	if err := r.c.GetJSON(ctx, client.DatabasesEP+"/"+m.ID.ValueString()+"/password", nil, &pw); err != nil {
		tflog.Debug(ctx, "database password not read", map[string]any{"error": err.Error()})
		return
	}
	if pw.Password != nil {
		m.Password = types.StringValue(*pw.Password)
	}
	if pw.User != nil && *pw.User != "" {
		m.User = types.StringValue(*pw.User)
	}
}

func (r *databaseResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan databaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	version := ""
	if !plan.Version.IsNull() && !plan.Version.IsUnknown() {
		version = plan.Version.ValueString()
	}
	offer, err := resolveServiceOffer(ctx, r.c, client.DatabasesEP, plan.Engine.ValueString(), plan.Plan.ValueString(), version)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("plan"), "Plan not available", err.Error())
		return
	}
	network, err := networkOrDefault(ctx, r.c, plan.NetworkID)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("network_id"), "No network", err.Error())
		return
	}
	payload := map[string]any{
		"name":          plan.Name.ValueString(),
		"engine":        offer.Engine.Key,
		"version":       offer.Version,
		"plan_id":       offer.Plan.ID,
		"network_uuid":  network,
		"durability":    plan.Durability.ValueString(),
		"public_access": plan.Public.ValueBool(),
		"require_ssl":   plan.RequireTLS.ValueBool(),
		"backup":        plan.Backup.ValueBool(),
		"pitr_days":     plan.BackupDays.ValueInt64(),
	}
	if !plan.DiskGB.IsNull() && !plan.DiskGB.IsUnknown() {
		payload["disk_gb"] = plan.DiskGB.ValueInt64()
	}
	if v := plan.SecurityGroupID; !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		payload["sg_uuid"] = v.ValueString()
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := r.c.PostJSON(ctx, client.DatabasesEP, payload, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create the database", err.Error())
		return
	}

	plan.ID = types.StringValue(created.ID)
	plan.Version = types.StringValue(offer.Version)
	plan.NetworkID = types.StringValue(network)
	plan.State = types.StringValue("creating")
	if plan.DiskGB.IsUnknown() {
		plan.DiskGB = types.Int64Value(offer.Plan.DiskIncludedGB)
	}
	if plan.SecurityGroupID.IsUnknown() {
		plan.SecurityGroupID = types.StringNull()
	}
	plan.Host, plan.User, plan.Password, plan.DefaultDatabase = types.StringNull(), types.StringNull(), types.StringNull(), types.StringNull()
	plan.PrivateIP, plan.PublicHost = types.StringNull(), types.StringNull()
	plan.Port, plan.PoolPort, plan.PublicPort = types.Int64Null(), types.Int64Null(), types.Int64Null()
	// saved before the wait, so a database that fails to come up is still in the state and destroy removes it
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	d, err := r.waitRunning(ctx, created.ID)
	if d != nil && d.ID != "" {
		setFromDatabase(&plan, d)
		r.password(ctx, &plan)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("The database did not come up", err.Error())
	}
}

func (r *databaseResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state databaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	d, err := r.get(ctx, state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "http 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the database", err.Error())
		return
	}
	if d.State == "deleting" {
		resp.State.RemoveResource(ctx)
		return
	}
	setFromDatabase(&state, d)
	if state.Password.IsNull() {
		r.password(ctx, &state)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update changes the plan and disk (one resize), public access and TLS, the write mode and backups.
func (r *databaseResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan, state databaseModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	ep := client.DatabasesEP + "/" + id
	cur, err := r.waitRunning(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("The database is not running", err.Error())
		return
	}

	resized := false
	diskUp := !plan.DiskGB.IsUnknown() && !plan.DiskGB.IsNull() && plan.DiskGB.ValueInt64() > state.DiskGB.ValueInt64()
	if norm(plan.Plan.ValueString()) != norm(state.Plan.ValueString()) || diskUp {
		offer, err := resolveServiceOffer(ctx, r.c, client.DatabasesEP, state.Engine.ValueString(), plan.Plan.ValueString(), state.Version.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("plan"), "Plan not available", err.Error())
			return
		}
		body := map[string]any{"plan_id": offer.Plan.ID}
		if diskUp {
			body["disk_gb"] = plan.DiskGB.ValueInt64()
		}
		if err := r.c.PostJSON(ctx, ep+"/resize", body, nil); err != nil {
			resp.Diagnostics.AddError("Failed to resize the database", err.Error())
			return
		}
		resized = true
	}
	if plan.Public.ValueBool() != state.Public.ValueBool() || plan.RequireTLS.ValueBool() != state.RequireTLS.ValueBool() {
		if err := r.c.PutJSON(ctx, ep+"/public", map[string]any{
			"enabled": plan.Public.ValueBool(), "require_ssl": plan.RequireTLS.ValueBool(),
		}, nil); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("public"), "Failed to change public access", err.Error())
			return
		}
	}
	if hasCap(cur, "durability") && plan.Durability.ValueString() != state.Durability.ValueString() {
		if err := r.c.PutJSON(ctx, ep+"/durability", map[string]any{"mode": plan.Durability.ValueString()}, nil); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("durability"), "Failed to change the write mode", err.Error())
			return
		}
	}
	if hasCap(cur, "backup") && plan.Durability.ValueString() != "extreme" &&
		(plan.Backup.ValueBool() != state.Backup.ValueBool() || plan.BackupDays.ValueInt64() != state.BackupDays.ValueInt64()) {
		if err := r.c.PutJSON(ctx, ep+"/backup", map[string]any{
			"enabled": plan.Backup.ValueBool(), "pitr_days": plan.BackupDays.ValueInt64(),
		}, nil); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("backup"), "Failed to change backups", err.Error())
			return
		}
	}

	if resized {
		// the controller takes the resize on its next pass: give it a moment before waiting for running
		time.Sleep(20 * time.Second)
	}
	d, err := r.waitRunning(ctx, id)
	plan.Password = state.Password
	if d != nil && d.ID != "" {
		setFromDatabase(&plan, d)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if err != nil {
		resp.Diagnostics.AddError("The database did not settle", err.Error())
	}
}

func (r *databaseResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state databaseModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	deadline := time.Now().Add(databaseTimeout)
	// a database still being created refuses the delete: wait for it
	for {
		err := r.c.Delete(ctx, client.DatabasesEP+"/"+id)
		if err == nil || strings.Contains(err.Error(), "http 404") {
			return
		}
		if !strings.Contains(err.Error(), "http 409") || time.Now().After(deadline) {
			resp.Diagnostics.AddError("Failed to delete the database", err.Error())
			return
		}
		time.Sleep(10 * time.Second)
	}
}

// ImportState takes the database's id. The owner password is read from the API.
func (r *databaseResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	tfresource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
