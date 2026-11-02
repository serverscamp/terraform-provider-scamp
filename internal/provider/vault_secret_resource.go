package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

type vaultSecretResource struct {
	c *client.Client
}

func NewVaultSecretResource() tfresource.Resource { return &vaultSecretResource{} }

type vaultSecretModel struct {
	ID      types.String `tfsdk:"id"`
	VaultID types.String `tfsdk:"vault_id"`
	Name    types.String `tfsdk:"name"`
	Value   types.String `tfsdk:"value"`
	Version types.Int64  `tfsdk:"version"`
}

func (r *vaultSecretResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_vault_secret"
}

func (r *vaultSecretResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = rschema.Schema{
		Description: "Manages a secret in a Secrets Vault. A new value is stored as a new version; destroy deletes the secret with all its versions.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed:      true,
				Description:   "VAULT_ID/NAME.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"vault_id": rschema.StringAttribute{
				Required: true, Description: "The vault's id (scamp_vault). Changing it replaces the secret.", PlanModifiers: replace,
			},
			"name": rschema.StringAttribute{
				Required:      true,
				Description:   "Name of the secret: lowercase letters, digits, - _ . and / between parts, e.g. gitlab/admin. Changing it replaces the secret.",
				PlanModifiers: replace,
			},
			"value": rschema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "The value, up to 64 KB. For several fields use jsonencode({...}). A value changed outside Terraform shows as a change and is set back on apply.",
			},
			"version": rschema.Int64Attribute{
				Computed: true, Description: "The secret's current version.",
			},
		},
	}
}

func (r *vaultSecretResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func secretEP(vault, name string) string {
	return client.VaultEP + "/" + vault + "/secrets/" + name
}

func (r *vaultSecretResource) put(ctx context.Context, m *vaultSecretModel) error {
	var out models.VaultSecret
	if err := r.c.PutJSON(ctx, secretEP(m.VaultID.ValueString(), m.Name.ValueString()), map[string]any{"value": m.Value.ValueString()}, &out); err != nil {
		return err
	}
	m.ID = types.StringValue(m.VaultID.ValueString() + "/" + m.Name.ValueString())
	m.Version = types.Int64Value(out.Version)
	if out.Version == 0 {
		// the answer to a put may not carry the version: read it back
		var cur models.VaultSecret
		if err := r.c.GetJSON(ctx, secretEP(m.VaultID.ValueString(), m.Name.ValueString()), nil, &cur); err == nil {
			m.Version = types.Int64Value(cur.Version)
		}
	}
	return nil
}

func (r *vaultSecretResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan vaultSecretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Failed to store the secret", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vaultSecretResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state vaultSecretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var cur models.VaultSecret
	err := r.c.GetJSON(ctx, secretEP(state.VaultID.ValueString(), state.Name.ValueString()), nil, &cur)
	if err != nil {
		if strings.Contains(err.Error(), "http 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the secret", err.Error())
		return
	}
	state.Value = types.StringValue(cur.Value)
	state.Version = types.Int64Value(cur.Version)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *vaultSecretResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan vaultSecretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.put(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Failed to store the secret", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *vaultSecretResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state vaultSecretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := r.c.Delete(ctx, secretEP(state.VaultID.ValueString(), state.Name.ValueString()))
	if err != nil && !strings.Contains(err.Error(), "http 404") {
		resp.Diagnostics.AddError("Failed to delete the secret", err.Error())
	}
}

// ImportState takes VAULT_ID/NAME, e.g. terraform import scamp_vault_secret.x 0a1b2c.../gitlab/admin
func (r *vaultSecretResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	vault, name, ok := strings.Cut(req.ID, "/")
	if !ok || vault == "" || name == "" {
		resp.Diagnostics.AddError("Bad import id", fmt.Sprintf("expected VAULT_ID/NAME, got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("vault_id"), vault)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), name)...)
}
