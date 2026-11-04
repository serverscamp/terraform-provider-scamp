package provider

import (
	"context"
	"net/url"
	"strings"

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

type brokerUserResource struct {
	c *client.Client
}

func NewBrokerUserResource() tfresource.Resource { return &brokerUserResource{} }

type brokerUserModel struct {
	ID        types.String `tfsdk:"id"`
	BrokerID  types.String `tfsdk:"broker_id"`
	Name      types.String `tfsdk:"name"`
	Publish   types.List   `tfsdk:"publish"`
	Subscribe types.List   `tfsdk:"subscribe"`
	JetStream types.Bool   `tfsdk:"jetstream"`
	VHost     types.String `tfsdk:"vhost"`
	Level     types.String `tfsdk:"level"`
	Topics    types.String `tfsdk:"topics"`
	Password  types.String `tfsdk:"password"`
}

func (r *brokerUserResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_broker_user"
}

func (r *brokerUserResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = rschema.Schema{
		Description: "Manages a user of a scamp_broker besides admin, with what it may do: NATS subjects and JetStream, a RabbitMQ virtual host and level, or Kafka topics and level. Each engine takes its own attributes and ignores the rest. The password is made by the broker.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true, Description: "<broker_id>/<name>.", PlanModifiers: keep,
			},
			"broker_id": rschema.StringAttribute{
				Required: true, Description: "ID of the scamp_broker. Changing it replaces the user.", PlanModifiers: replace,
			},
			"name": rschema.StringAttribute{
				Required:      true,
				Description:   "Name of the user: 2 to 32 lowercase letters, digits, - and _, starting with a letter; admin is taken. Changing it replaces the user.",
				PlanModifiers: replace,
			},
			"publish": rschema.ListAttribute{
				ElementType: types.StringType, Optional: true,
				Description: "NATS: subjects the user may publish to, * for one token and > at the end, e.g. orders.>.",
			},
			"subscribe": rschema.ListAttribute{
				ElementType: types.StringType, Optional: true,
				Description: "NATS: subjects the user may subscribe to.",
			},
			"jetstream": rschema.BoolAttribute{
				Optional: true, Description: "NATS: the user may use JetStream streams and consumers.",
			},
			"vhost": rschema.StringAttribute{
				Optional: true, Computed: true, PlanModifiers: keep,
				Description: "RabbitMQ: the virtual host of the user, / or up to 64 letters, digits, - _ and .",
			},
			"level": rschema.StringAttribute{
				Optional: true, Computed: true, PlanModifiers: keep,
				Description: "RabbitMQ and Kafka: read or write.",
			},
			"topics": rschema.StringAttribute{
				Optional: true, Computed: true, PlanModifiers: keep,
				Description: "Kafka: * for every topic, or a prefix of topic names.",
			},
			"password": rschema.StringAttribute{
				Computed: true, Sensitive: true, Description: "The user's password.", PlanModifiers: keep,
			},
		},
	}
}

func (r *brokerUserResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func listStrings(ctx context.Context, l types.List) []string {
	out := []string{}
	if !l.IsNull() && !l.IsUnknown() {
		_ = l.ElementsAs(ctx, &out, false)
	}
	return out
}

func optString(v types.String) any {
	if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
		return nil
	}
	return v.ValueString()
}

func (r *brokerUserResource) put(ctx context.Context, m brokerUserModel) error {
	return r.c.PutJSON(ctx, client.BrokersEP+"/"+m.BrokerID.ValueString()+"/users/"+url.PathEscape(m.Name.ValueString()), map[string]any{
		"publish":   listStrings(ctx, m.Publish),
		"subscribe": listStrings(ctx, m.Subscribe),
		"jetstream": m.JetStream.ValueBool(),
		"vhost":     optString(m.VHost),
		"level":     optString(m.Level),
		"topics":    optString(m.Topics),
	}, nil)
}

func (r *brokerUserResource) find(ctx context.Context, m brokerUserModel) (*models.BrokerUser, error) {
	var b models.BrokerUsers
	if err := r.c.GetJSON(ctx, client.BrokersEP+"/"+m.BrokerID.ValueString(), nil, &b); err != nil {
		return nil, err
	}
	if b.State == "deleting" {
		return nil, nil
	}
	for i := range b.Users {
		if b.Users[i].Name == m.Name.ValueString() {
			return &b.Users[i], nil
		}
	}
	return nil, nil
}

// setFromBrokerUser takes what the broker keeps. Lists and the NATS switch read back as the config wrote them when
// the broker has nothing there, so an engine that ignores them does not plan a change forever.
func setFromBrokerUser(ctx context.Context, m *brokerUserModel, u *models.BrokerUser) {
	if len(u.Publish) > 0 || !m.Publish.IsNull() {
		m.Publish, _ = types.ListValueFrom(ctx, types.StringType, nonNil(u.Publish))
	}
	if len(u.Subscribe) > 0 || !m.Subscribe.IsNull() {
		m.Subscribe, _ = types.ListValueFrom(ctx, types.StringType, nonNil(u.Subscribe))
	}
	if u.JetStream || !m.JetStream.IsNull() {
		m.JetStream = types.BoolValue(u.JetStream)
	}
	m.VHost = strOrNull(u.VHost)
	m.Level = strOrNull(u.Level)
	m.Topics = strOrNull(u.Topics)
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func (r *brokerUserResource) password(ctx context.Context, m *brokerUserModel) {
	var pw models.ServicePassword
	ep := client.BrokersEP + "/" + m.BrokerID.ValueString() + "/users/" + url.PathEscape(m.Name.ValueString()) + "/password"
	if err := r.c.GetJSON(ctx, ep, nil, &pw); err != nil {
		tflog.Debug(ctx, "broker user password not read", map[string]any{"error": err.Error()})
		return
	}
	if pw.Password != nil {
		m.Password = types.StringValue(*pw.Password)
	}
}

func (r *brokerUserResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan brokerUserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, plan); err != nil {
		resp.Diagnostics.AddError("Failed to create the user", err.Error())
		return
	}
	plan.ID = types.StringValue(plan.BrokerID.ValueString() + "/" + plan.Name.ValueString())
	plan.Password = types.StringNull()
	if u, err := r.find(ctx, plan); err == nil && u != nil {
		setFromBrokerUser(ctx, &plan, u)
	}
	for _, v := range []*types.String{&plan.VHost, &plan.Level, &plan.Topics} {
		if v.IsUnknown() {
			*v = types.StringNull()
		}
	}
	r.password(ctx, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *brokerUserResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state brokerUserModel
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
		resp.Diagnostics.AddError("Failed to read the broker", err.Error())
		return
	}
	if u == nil {
		// the broker's list follows the controller with a delay: the controller itself still knows a user just made
		var pw models.ServicePassword
		ep := client.BrokersEP + "/" + state.BrokerID.ValueString() + "/users/" + url.PathEscape(state.Name.ValueString()) + "/password"
		if err := r.c.GetJSON(ctx, ep, nil, &pw); err != nil || pw.Password == nil {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}
	setFromBrokerUser(ctx, &state, u)
	if state.Password.IsNull() {
		r.password(ctx, &state)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *brokerUserResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan, state brokerUserModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, plan); err != nil {
		resp.Diagnostics.AddError("Failed to change the user", err.Error())
		return
	}
	plan.Password = state.Password
	if u, err := r.find(ctx, plan); err == nil && u != nil {
		setFromBrokerUser(ctx, &plan, u)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *brokerUserResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state brokerUserModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.c.Delete(ctx, client.BrokersEP+"/"+state.BrokerID.ValueString()+"/users/"+url.PathEscape(state.Name.ValueString()))
	if err != nil && !strings.Contains(err.Error(), "http 404") {
		resp.Diagnostics.AddError("Failed to delete the user", err.Error())
	}
}

func (r *brokerUserResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	brokerID, name, err := splitPair(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Bad import id", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("broker_id"), brokerID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}
