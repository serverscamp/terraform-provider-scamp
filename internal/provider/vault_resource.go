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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

const vaultTimeout = 20 * time.Minute

type vaultResource struct {
	c *client.Client
}

func NewVaultResource() tfresource.Resource { return &vaultResource{} }

type vaultModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Package   types.String `tfsdk:"package"`
	GrowUpTo  types.String `tfsdk:"grow_up_to"`
	NetworkID types.String `tfsdk:"network_id"`
	Host      types.String `tfsdk:"host"`
	URL       types.String `tfsdk:"url"`
	IP        types.String `tfsdk:"ip"`
	PrivateIP types.String `tfsdk:"private_ip"`
	RootToken types.String `tfsdk:"root_token"`
	State     types.String `tfsdk:"state"`
}

func (r *vaultResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_vault"
}

func (r *vaultResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = rschema.Schema{
		Description: "Manages a Secrets Vault: a secrets store on its own server. Put secrets in it with scamp_vault_secret.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true, Description: "The vault's id.", PlanModifiers: keep,
			},
			"name": rschema.StringAttribute{
				Required:      true,
				Description:   "Name of the vault: 3 to 32 lowercase letters, digits and hyphens, starting with a letter. Changing it replaces the vault and every secret in it.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"package": rschema.StringAttribute{
				Required:    true,
				Description: "Package by name or slug, e.g. the smallest one in the panel. Changed in place; down only if the secrets and tokens fit.",
			},
			"grow_up_to": rschema.StringAttribute{
				Optional:    true,
				Description: "A bigger package the vault moves up to by itself when it is full. Changed in place.",
			},
			"network_id": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "UUID of the network, attached to a router. Without it the account's default network. Changing it replaces the vault and every secret in it.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"host": rschema.StringAttribute{
				Computed: true, Description: "The vault's name in DNS.", PlanModifiers: keep,
			},
			"url": rschema.StringAttribute{
				Computed: true, Description: "https:// address of the vault API.", PlanModifiers: keep,
			},
			"ip": rschema.StringAttribute{
				Computed: true, Description: "Public address of the vault.",
			},
			"private_ip": rschema.StringAttribute{
				Computed: true, Description: "Address of the vault in its network.",
			},
			"root_token": rschema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The vault's root token, for the vault API. Rotated in the panel, it is read again on the next refresh.",
			},
			"state": rschema.StringAttribute{
				Computed: true, Description: "creating, configuring, active or failed.",
			},
		},
	}
}

func (r *vaultResource) ValidateConfig(ctx context.Context, req tfresource.ValidateConfigRequest, resp *tfresource.ValidateConfigResponse) {
	var cfg vaultModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	checkServiceName(cfg.Name, &resp.Diagnostics)
}

func (r *vaultResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func (r *vaultResource) packageID(ctx context.Context, want string) (int64, error) {
	var cat struct {
		Packages []models.VaultPackage `json:"packages"`
	}
	if err := r.c.GetJSON(ctx, client.VaultEP+"/catalog", nil, &cat); err != nil {
		return 0, err
	}
	labels := make([][]string, len(cat.Packages))
	for i, p := range cat.Packages {
		labels[i] = []string{p.Name, p.Slug}
	}
	i, err := pickByLabels(want, labels)
	if err != nil {
		return 0, err
	}
	if cat.Packages[i].SoldOut {
		return 0, fmt.Errorf("package %s is out of capacity right now", cat.Packages[i].Name)
	}
	return cat.Packages[i].ID, nil
}

func (r *vaultResource) get(ctx context.Context, id string) (*models.Vault, error) {
	var v models.Vault
	err := r.c.GetJSON(ctx, client.VaultEP+"/"+id, nil, &v)
	return &v, err
}

func (r *vaultResource) rootToken(ctx context.Context, id string) types.String {
	var t struct {
		RootToken *string `json:"root_token"`
	}
	if err := r.c.GetJSON(ctx, client.VaultEP+"/"+id+"/root-token", nil, &t); err != nil {
		return types.StringNull()
	}
	return strOrNull(t.RootToken)
}

// setFromVault copies what the API returns. Package names stay as written in the config when they still name
// the same package, so "Small" and "small" do not show as a change.
func setFromVault(m *vaultModel, v *models.Vault) {
	m.ID = types.StringValue(v.ID)
	m.Name = types.StringValue(v.Name)
	m.NetworkID = types.StringValue(v.Network)
	m.Host = types.StringValue(v.Host)
	m.URL = types.StringValue(v.URL)
	m.IP = strOrNull(v.IP)
	m.PrivateIP = strOrNull(v.PrivateIP)
	m.State = types.StringValue(v.State)
	same := func(cur types.String, p *models.VaultPackage) bool {
		return !cur.IsNull() && !cur.IsUnknown() && p != nil && (norm(cur.ValueString()) == norm(p.Name) || norm(cur.ValueString()) == norm(p.Slug))
	}
	if v.Package != nil && !same(m.Package, v.Package) {
		m.Package = types.StringValue(v.Package.Slug)
	}
	if v.GrowUpTo == nil {
		m.GrowUpTo = types.StringNull()
	} else if !same(m.GrowUpTo, v.GrowUpTo) {
		m.GrowUpTo = types.StringValue(v.GrowUpTo.Slug)
	}
}

func (r *vaultResource) waitActive(ctx context.Context, id string) (*models.Vault, error) {
	deadline := time.Now().Add(vaultTimeout)
	for {
		v, err := r.get(ctx, id)
		if err == nil {
			switch v.State {
			case "active":
				return v, nil
			case "failed":
				msg := "the vault could not be created"
				if v.Error != nil {
					msg = *v.Error
				}
				return v, fmt.Errorf("%s", msg)
			}
		} else {
			tflog.Debug(ctx, "vault read failed while waiting", map[string]any{"error": err.Error()})
		}
		if time.Now().After(deadline) {
			return v, fmt.Errorf("the vault did not become active within %s", vaultTimeout)
		}
		select {
		case <-ctx.Done():
			return v, ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

func (r *vaultResource) setGrow(ctx context.Context, id string, grow types.String) error {
	body := map[string]any{"grow_up_to_package_id": nil}
	if !grow.IsNull() && grow.ValueString() != "" {
		gid, err := r.packageID(ctx, grow.ValueString())
		if err != nil {
			return err
		}
		body["grow_up_to_package_id"] = gid
	}
	return r.c.PutJSON(ctx, client.VaultEP+"/"+id+"/grow", body, nil)
}

func (r *vaultResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan vaultModel
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
	suffix := make([]byte, 3)
	_, _ = rand.Read(suffix)
	payload := map[string]any{
		"name":         plan.Name.ValueString(),
		"suffix":       hex.EncodeToString(suffix),
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
	if err := r.c.PostJSON(ctx, client.VaultEP, payload, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create the vault", err.Error())
		return
	}
	plan.ID = types.StringValue(created.ID)
	plan.Host = types.StringValue(created.Host)
	plan.URL = types.StringValue("https://" + created.Host)
	plan.IP, plan.PrivateIP, plan.RootToken = types.StringNull(), types.StringNull(), types.StringNull()
	plan.State = types.StringValue("creating")
	// saved before the wait, so a vault that fails to come up is still in the state and destroy removes it
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	v, err := r.waitActive(ctx, created.ID)
	if v != nil && v.ID != "" {
		setFromVault(&plan, v)
		plan.RootToken = r.rootToken(ctx, created.ID)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("The vault did not come up", err.Error())
	}
}

func (r *vaultResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state vaultModel
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
		resp.Diagnostics.AddError("Failed to read the vault", err.Error())
		return
	}
	setFromVault(&state, v)
	if t := r.rootToken(ctx, v.ID); !t.IsNull() {
		state.RootToken = t
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vaultResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan, state vaultModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if !plan.Package.Equal(state.Package) {
		pid, err := r.packageID(ctx, plan.Package.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("package"), "Package not available", err.Error())
			return
		}
		if err := r.c.PutJSON(ctx, client.VaultEP+"/"+id+"/package", map[string]any{"package_id": pid}, nil); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("package"), "Failed to change the package", err.Error())
			return
		}
	}
	if !plan.GrowUpTo.Equal(state.GrowUpTo) {
		if err := r.setGrow(ctx, id, plan.GrowUpTo); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("grow_up_to"), "Failed to change grow_up_to", err.Error())
			return
		}
	}
	v, err := r.waitActive(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("The vault is not active after the change", err.Error())
		return
	}
	plan.RootToken = state.RootToken
	setFromVault(&plan, v)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vaultResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state vaultModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	deadline := time.Now().Add(vaultTimeout)
	// a vault still being made cannot be deleted yet: wait for it
	for {
		err := r.c.Delete(ctx, client.VaultEP+"/"+id)
		if err == nil || strings.Contains(err.Error(), "http 404") {
			return
		}
		if !strings.Contains(err.Error(), "http 409") || time.Now().After(deadline) {
			resp.Diagnostics.AddError("Failed to delete the vault", err.Error())
			return
		}
		time.Sleep(10 * time.Second)
	}
}

func (r *vaultResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	tfresource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
