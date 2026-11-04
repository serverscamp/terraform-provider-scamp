package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

// The SMTP port with STARTTLS; 465 with TLS works too.
const emailSMTPPort = 587

// The part before @, as the API takes it (public-api services/mail_gateway.py).
var emailLocal = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,38}[a-z0-9])?$`)

var emailReserved = map[string]bool{
	"postmaster": true, "abuse": true, "dmarc": true, "hostmaster": true, "webmaster": true, "root": true,
	"mailer-daemon": true, "bounce": true, "bounces": true,
}

type emailAddressResource struct {
	c *client.Client
}

func NewEmailAddressResource() tfresource.Resource { return &emailAddressResource{} }

type emailAddressModel struct {
	ID              types.String `tfsdk:"id"`
	Name            types.String `tfsdk:"name"`
	Label           types.String `tfsdk:"label"`
	PasswordVersion types.Int64  `tfsdk:"password_version"`
	Address         types.String `tfsdk:"address"`
	Password        types.String `tfsdk:"password"`
	SMTPHost        types.String `tfsdk:"smtp_host"`
	SMTPPort        types.Int64  `tfsdk:"smtp_port"`
	Disabled        types.Bool   `tfsdk:"disabled"`
	DisabledReason  types.String `tfsdk:"disabled_reason"`
}

func (r *emailAddressResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_email_address"
}

func (r *emailAddressResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = rschema.Schema{
		Description: "Manages an address the account sends mail from through the ServersCamp mail gateway, NAME@<your space>.serverscamp-mailer.com, over SMTP with its own login. Transactional mail only, within the daily limit of the account.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true, Description: "The address's id.", PlanModifiers: keep,
			},
			"name": rschema.StringAttribute{
				Required:      true,
				Description:   "The part before @, e.g. gitlab: lowercase letters, digits, dots, hyphens and underscores, starting and ending with a letter or digit, up to 40. Changing it replaces the address.",
				PlanModifiers: replace,
			},
			"label": rschema.StringAttribute{
				Optional:      true,
				Description:   "A note on what sends from it, up to 100 characters. Changing it makes the address again, with a new password.",
				PlanModifiers: replace,
			},
			"password_version": rschema.Int64Attribute{
				Optional:    true,
				Description: "Change it to any other number to get a new SMTP password; the old one stops working. The address stays.",
			},
			"address": rschema.StringAttribute{
				Computed: true, Description: "The full address. It is also the SMTP login.", PlanModifiers: keep,
			},
			"password": rschema.StringAttribute{
				Computed: true, Sensitive: true, Description: "The SMTP password.", PlanModifiers: keep,
			},
			"smtp_host": rschema.StringAttribute{
				Computed: true, Description: "SMTP server of the mail gateway.", PlanModifiers: keep,
			},
			"smtp_port": rschema.Int64Attribute{
				Computed: true, Description: "SMTP port with STARTTLS: 587. Port 465 with TLS works too.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"disabled": rschema.BoolAttribute{
				Computed: true, Description: "True when the address was stopped, for bounced or refused mail or by us.",
			},
			"disabled_reason": rschema.StringAttribute{
				Computed: true, Description: "Why the address was stopped.",
			},
		},
	}
}

func (r *emailAddressResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func (r *emailAddressResource) ValidateConfig(ctx context.Context, req tfresource.ValidateConfigRequest, resp *tfresource.ValidateConfigResponse) {
	var cfg emailAddressModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() || cfg.Name.IsNull() || cfg.Name.IsUnknown() {
		return
	}
	n := cfg.Name.ValueString()
	switch {
	case !emailLocal.MatchString(n) || strings.Contains(n, ".."):
		resp.Diagnostics.AddAttributeError(path.Root("name"), "Bad name",
			fmt.Sprintf("%q: lowercase letters, digits, dots, hyphens and underscores, starting and ending with a letter or digit, up to 40.", n))
	case emailReserved[n]:
		resp.Diagnostics.AddAttributeError(path.Root("name"), "Name is reserved", n+" is kept for the mail system. Pick another one.")
	}
}

func (r *emailAddressResource) overview(ctx context.Context) (*models.EmailOverview, error) {
	var o models.EmailOverview
	err := r.c.GetJSON(ctx, client.EmailEP, nil, &o)
	return &o, err
}

func setFromEmail(m *emailAddressModel, a *models.EmailAddress, host string) {
	m.ID = types.StringValue(a.ID)
	m.Address = types.StringValue(a.Address)
	if i := strings.Index(a.Address, "@"); i > 0 {
		m.Name = types.StringValue(a.Address[:i])
	}
	m.Label = strOrNull(a.Label)
	m.Disabled = types.BoolValue(a.Disabled)
	m.DisabledReason = strOrNull(a.DisabledReason)
	if host != "" {
		m.SMTPHost = types.StringValue(host)
	}
	m.SMTPPort = types.Int64Value(emailSMTPPort)
}

func (r *emailAddressResource) readPassword(ctx context.Context, m *emailAddressModel) {
	var pw models.ServicePassword
	if err := r.c.GetJSON(ctx, client.EmailEP+"/addresses/"+m.ID.ValueString()+"/password", nil, &pw); err != nil {
		tflog.Debug(ctx, "email password not read", map[string]any{"error": err.Error()})
		return
	}
	if pw.Password != nil {
		m.Password = types.StringValue(*pw.Password)
	}
}

func (r *emailAddressResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan emailAddressModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	payload := map[string]any{"name": plan.Name.ValueString()}
	if v := plan.Label; !v.IsNull() && !v.IsUnknown() && v.ValueString() != "" {
		payload["label"] = v.ValueString()
	}
	var created struct {
		models.EmailAddress
		Password string `json:"password"`
		SMTP     struct {
			Host string `json:"host"`
		} `json:"smtp"`
	}
	if err := r.c.PostJSON(ctx, client.EmailEP+"/addresses", payload, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create the address", err.Error())
		return
	}
	label := plan.Label
	setFromEmail(&plan, &created.EmailAddress, created.SMTP.Host)
	// the API keeps an empty label as none: the config's spelling stays
	if label.IsNull() || label.ValueString() == "" {
		plan.Label = label
	}
	plan.Password = types.StringValue(created.Password)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *emailAddressResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state emailAddressModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	o, err := r.overview(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the mail addresses", err.Error())
		return
	}
	for i := range o.Addresses {
		if o.Addresses[i].ID == state.ID.ValueString() {
			label := state.Label
			setFromEmail(&state, &o.Addresses[i], o.SMTP.Host)
			if o.Addresses[i].Label == nil && !label.IsNull() && label.ValueString() == "" {
				state.Label = label
			}
			if state.Password.IsNull() {
				r.readPassword(ctx, &state)
			}
			resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
			return
		}
	}
	resp.State.RemoveResource(ctx)
}

// Update only makes a new password when password_version changes; everything else replaces the address.
func (r *emailAddressResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan, state emailAddressModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID, plan.Address, plan.Password = state.ID, state.Address, state.Password
	plan.SMTPHost, plan.SMTPPort, plan.Disabled, plan.DisabledReason = state.SMTPHost, state.SMTPPort, state.Disabled, state.DisabledReason
	if !plan.PasswordVersion.Equal(state.PasswordVersion) {
		var res struct {
			Password string `json:"password"`
		}
		if err := r.c.PostJSON(ctx, client.EmailEP+"/addresses/"+state.ID.ValueString()+"/password", nil, &res); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("password_version"), "Failed to make a new password", err.Error())
			return
		}
		plan.Password = types.StringValue(res.Password)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *emailAddressResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state emailAddressModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.c.Delete(ctx, client.EmailEP+"/addresses/"+state.ID.ValueString())
	if err != nil && !strings.Contains(err.Error(), "http 404") {
		resp.Diagnostics.AddError("Failed to delete the address", err.Error())
	}
}

// ImportState takes the address's id. The password is read from the API when it was kept.
func (r *emailAddressResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	tfresource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
