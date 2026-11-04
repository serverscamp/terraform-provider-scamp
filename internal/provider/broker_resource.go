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

// A broker takes a few minutes; Kafka is the slowest.
const brokerTimeout = 30 * time.Minute

type brokerResource struct {
	c *client.Client
}

func NewBrokerResource() tfresource.Resource { return &brokerResource{} }

type brokerModel struct {
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
	Hostname        types.String `tfsdk:"hostname"`
	Port            types.Int64  `tfsdk:"port"`
	PrivateIP       types.String `tfsdk:"private_ip"`
	PublicHost      types.String `tfsdk:"public_host"`
	PublicPort      types.Int64  `tfsdk:"public_port"`
	User            types.String `tfsdk:"user"`
	Password        types.String `tfsdk:"password"`
	State           types.String `tfsdk:"state"`
}

func (r *brokerResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_broker"
}

func (r *brokerResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	keepInt := []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}
	resp.Schema = rschema.Schema{
		Description: "Manages a message broker: NATS with JetStream, RabbitMQ or Kafka on its own server, run and updated by ServersCamp. The broker has its own name under the broker domain with a certificate we renew.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true, Description: "The broker's id.", PlanModifiers: keep,
			},
			"name": rschema.StringAttribute{
				Required:      true,
				Description:   "Name of the broker: 3 to 32 lowercase letters, digits and hyphens, starting with a letter. It is the start of its address, NAME-xxxxxx. Changing it replaces the broker.",
				PlanModifiers: replace,
			},
			"engine": rschema.StringAttribute{
				Required:      true,
				Description:   "nats, rabbitmq or kafka. Changing it replaces the broker.",
				PlanModifiers: replace,
			},
			"version": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Engine version. Without it the engine's default. Changing it replaces the broker.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"plan": rschema.StringAttribute{
				Required:    true,
				Description: "Plan, e.g. nats-bs-s. Changed in place to another plan of the same engine; the server restarts.",
			},
			"disk_gb": rschema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Disk size in GB. A broker starts with the disk of its plan; a bigger value grows it in place after it is made. It never shrinks.",
				PlanModifiers: keepInt,
			},
			"network_id": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "UUID of the network, attached to a router. Without it the account's default network. Changing it replaces the broker.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"security_group_id": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "UUID of a security group. Without it a new group is made with the engine's ports open and everything from the private network and VPN Gateway. Changing it replaces the broker.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"public": rschema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Ports on the router's public address. Who may connect is the security group's to say. Changed in place.",
			},
			"require_tls": rschema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false),
				Description: "Every connection over TLS; off: TLS and plain both. Changed in place.",
			},
			"hostname": rschema.StringAttribute{
				Computed: true, Description: "The broker's name, NAME-xxxxxx under the broker domain.", PlanModifiers: keep,
			},
			"port":        rschema.Int64Attribute{Computed: true, Description: "The engine's port.", PlanModifiers: keepInt},
			"private_ip":  rschema.StringAttribute{Computed: true, Description: "The server's address in its network."},
			"public_host": rschema.StringAttribute{Computed: true, Description: "Public address, empty when public is off."},
			"public_port": rschema.Int64Attribute{Computed: true, Description: "Public port, empty when public is off."},
			"user": rschema.StringAttribute{
				Computed: true, Description: "The admin user.", PlanModifiers: keep,
			},
			"password": rschema.StringAttribute{
				Computed: true, Sensitive: true, Description: "The admin user's password.", PlanModifiers: keep,
			},
			"state": rschema.StringAttribute{
				Computed: true, Description: "creating, configuring, running, updating, failed or deleting.",
			},
		},
	}
}

func (r *brokerResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func (r *brokerResource) ValidateConfig(ctx context.Context, req tfresource.ValidateConfigRequest, resp *tfresource.ValidateConfigResponse) {
	var cfg brokerModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	checkServiceName(cfg.Name, &resp.Diagnostics)
}

func (r *brokerResource) ModifyPlan(ctx context.Context, req tfresource.ModifyPlanRequest, resp *tfresource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var plan, state brokerModel
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

func (r *brokerResource) get(ctx context.Context, id string) (*models.Broker, error) {
	var b models.Broker
	err := r.c.GetJSON(ctx, client.BrokersEP+"/"+id, nil, &b)
	return &b, err
}

func (r *brokerResource) waitRunning(ctx context.Context, id string) (*models.Broker, error) {
	var last *models.Broker
	err := waitServiceState(ctx, brokerTimeout, func() (string, string, error) {
		b, err := r.get(ctx, id)
		if err != nil {
			tflog.Debug(ctx, "broker read failed while waiting", map[string]any{"error": err.Error()})
			return "", "", err
		}
		last = b
		msg := ""
		if b.Error != nil {
			msg = *b.Error
		}
		return b.State, msg, nil
	}, "running")
	return last, err
}

func setFromBroker(m *brokerModel, b *models.Broker) {
	m.ID = types.StringValue(b.ID)
	m.Name = types.StringValue(b.Name)
	m.Engine = types.StringValue(b.Engine)
	m.Version = types.StringValue(b.Version)
	if b.Plan != nil && (m.Plan.IsNull() || m.Plan.IsUnknown() || norm(m.Plan.ValueString()) != norm(*b.Plan)) {
		m.Plan = types.StringValue(*b.Plan)
	}
	m.DiskGB = types.Int64Value(b.DiskGB)
	m.NetworkID = types.StringValue(b.NetworkUUID)
	if b.SGUUID != nil && *b.SGUUID != "" {
		m.SecurityGroupID = types.StringValue(*b.SGUUID)
	}
	m.Public = types.BoolValue(b.Public.Enabled)
	m.RequireTLS = types.BoolValue(b.RequireTLS)
	m.Port = int64OrNull(b.Port)
	m.PrivateIP = strOrNull(b.IP)
	m.PublicHost, m.PublicPort = types.StringNull(), types.Int64Null()
	if b.Public.Enabled {
		m.PublicHost = strOrNull(b.Public.Host)
		m.PublicPort = int64OrNull(b.Public.Port)
	}
	m.User = types.StringValue(b.User)
	m.State = types.StringValue(b.State)
}

func (r *brokerResource) password(ctx context.Context, m *brokerModel) {
	var pw models.ServicePassword
	if err := r.c.GetJSON(ctx, client.BrokersEP+"/"+m.ID.ValueString()+"/password", nil, &pw); err != nil {
		tflog.Debug(ctx, "broker password not read", map[string]any{"error": err.Error()})
		return
	}
	if pw.Password != nil {
		m.Password = types.StringValue(*pw.Password)
	}
}

func (r *brokerResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan brokerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	version := ""
	if !plan.Version.IsNull() && !plan.Version.IsUnknown() {
		version = plan.Version.ValueString()
	}
	offer, err := resolveServiceOffer(ctx, r.c, client.BrokersEP, plan.Engine.ValueString(), plan.Plan.ValueString(), version)
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
		"suffix":        hexSlug(),
		"engine":        offer.Engine.Key,
		"version":       offer.Version,
		"plan_id":       offer.Plan.ID,
		"network_uuid":  network,
		"public_access": plan.Public.ValueBool(),
		"require_tls":   plan.RequireTLS.ValueBool(),
	}
	if v := plan.SecurityGroupID; !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		payload["sg_uuid"] = v.ValueString()
	}
	var created struct {
		ID   string `json:"id"`
		Host string `json:"host"`
	}
	if err := r.c.PostJSON(ctx, client.BrokersEP, payload, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create the broker", err.Error())
		return
	}

	wantDisk := plan.DiskGB
	plan.ID = types.StringValue(created.ID)
	plan.Hostname = types.StringValue(created.Host)
	plan.Version = types.StringValue(offer.Version)
	plan.NetworkID = types.StringValue(network)
	plan.DiskGB = types.Int64Value(offer.Plan.DiskIncludedGB)
	plan.State = types.StringValue("creating")
	if plan.SecurityGroupID.IsUnknown() {
		plan.SecurityGroupID = types.StringNull()
	}
	plan.User, plan.Password, plan.PrivateIP, plan.PublicHost = types.StringNull(), types.StringNull(), types.StringNull(), types.StringNull()
	plan.Port, plan.PublicPort = types.Int64Null(), types.Int64Null()
	// saved before the wait, so a broker that fails to come up is still in the state and destroy removes it
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	b, err := r.waitRunning(ctx, created.ID)
	if b != nil && b.ID != "" {
		setFromBroker(&plan, b)
		r.password(ctx, &plan)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("The broker did not come up", err.Error())
		return
	}
	// a broker is made with its plan's disk; a bigger one asked for is a resize once it runs
	if !wantDisk.IsNull() && !wantDisk.IsUnknown() && wantDisk.ValueInt64() > plan.DiskGB.ValueInt64() {
		if err := r.c.PostJSON(ctx, client.BrokersEP+"/"+created.ID+"/resize", map[string]any{
			"plan_id": offer.Plan.ID, "disk_gb": wantDisk.ValueInt64(),
		}, nil); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("disk_gb"), "Failed to grow the disk", err.Error())
			return
		}
		time.Sleep(20 * time.Second)
		b, err := r.waitRunning(ctx, created.ID)
		if b != nil && b.ID != "" {
			setFromBroker(&plan, b)
			resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		}
		if err != nil {
			resp.Diagnostics.AddError("The broker did not settle", err.Error())
		}
	}
}

func (r *brokerResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state brokerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	b, err := r.get(ctx, state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "http 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the broker", err.Error())
		return
	}
	if b.State == "deleting" {
		resp.State.RemoveResource(ctx)
		return
	}
	setFromBroker(&state, b)
	if state.Password.IsNull() {
		r.password(ctx, &state)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update changes the plan and disk (one resize), public access and TLS.
func (r *brokerResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan, state brokerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	ep := client.BrokersEP + "/" + id
	if _, err := r.waitRunning(ctx, id); err != nil {
		resp.Diagnostics.AddError("The broker is not running", err.Error())
		return
	}

	resized := false
	diskUp := !plan.DiskGB.IsUnknown() && !plan.DiskGB.IsNull() && plan.DiskGB.ValueInt64() > state.DiskGB.ValueInt64()
	if norm(plan.Plan.ValueString()) != norm(state.Plan.ValueString()) || diskUp {
		offer, err := resolveServiceOffer(ctx, r.c, client.BrokersEP, state.Engine.ValueString(), plan.Plan.ValueString(), state.Version.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("plan"), "Plan not available", err.Error())
			return
		}
		body := map[string]any{"plan_id": offer.Plan.ID}
		if diskUp {
			body["disk_gb"] = plan.DiskGB.ValueInt64()
		}
		if err := r.c.PostJSON(ctx, ep+"/resize", body, nil); err != nil {
			resp.Diagnostics.AddError("Failed to resize the broker", err.Error())
			return
		}
		resized = true
	}
	if plan.Public.ValueBool() != state.Public.ValueBool() {
		if err := r.c.PutJSON(ctx, ep+"/access", map[string]any{"public_access": plan.Public.ValueBool()}, nil); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("public"), "Failed to change public access", err.Error())
			return
		}
	}
	if plan.RequireTLS.ValueBool() != state.RequireTLS.ValueBool() {
		if err := r.c.PutJSON(ctx, ep+"/tls", map[string]any{"require_tls": plan.RequireTLS.ValueBool()}, nil); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("require_tls"), "Failed to change TLS", err.Error())
			return
		}
	}

	if resized {
		// the controller takes the resize on its next pass: give it a moment before waiting for running
		time.Sleep(20 * time.Second)
	}
	b, err := r.waitRunning(ctx, id)
	plan.Password = state.Password
	if b != nil && b.ID != "" {
		setFromBroker(&plan, b)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if err != nil {
		resp.Diagnostics.AddError("The broker did not settle", err.Error())
	}
}

func (r *brokerResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state brokerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	deadline := time.Now().Add(brokerTimeout)
	for {
		err := r.c.Delete(ctx, client.BrokersEP+"/"+id)
		if err == nil || strings.Contains(err.Error(), "http 404") {
			break
		}
		if !strings.Contains(err.Error(), "http 409") || time.Now().After(deadline) {
			resp.Diagnostics.AddError("Failed to delete the broker", err.Error())
			return
		}
		time.Sleep(10 * time.Second)
	}
	// it reads back as deleting until its server is gone
	for {
		_, err := r.get(ctx, id)
		if err != nil && strings.Contains(err.Error(), "http 404") {
			return
		}
		if time.Now().After(deadline) {
			resp.Diagnostics.AddError("The broker is still there", fmt.Sprintf("%s still reads back after %s", id, brokerTimeout))
			return
		}
		time.Sleep(5 * time.Second)
	}
}

// ImportState takes the broker's id. The admin password is read from the API.
func (r *brokerResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	tfresource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
