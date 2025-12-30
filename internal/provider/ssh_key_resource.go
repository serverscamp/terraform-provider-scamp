package provider

import (
	"context"
	"fmt"
	"strings"

	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

type sshKeyResource struct {
	c *client.Client
}

func NewSSHKeyResource() tfresource.Resource { return &sshKeyResource{} }

func (r *sshKeyResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_ssh_key"
}

func (r *sshKeyResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		Description: "Manages an SSH key in SCAMP. Supports both generating new keys and importing existing public keys.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.Int64Attribute{
				Computed:    true,
				Description: "The unique identifier of the SSH key.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"key_name": rschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Name of the SSH key (max 255 chars). If not provided, auto-generated as key-{random}.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"generate": rschema.BoolAttribute{
				Optional:    true,
				Description: "Set to true to generate a new Ed25519 key pair. Mutually exclusive with public_key.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"public_key": rschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Public key in OpenSSH format. Required for import, computed for generated keys.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"private_key": rschema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Private key in PEM format. Only available for generated keys, returned only once at creation.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"key_type": rschema.StringAttribute{
				Computed:    true,
				Description: "Type of SSH key (ed25519, rsa, ecdsa-sha2-nistp256, etc.).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"fingerprint": rschema.StringAttribute{
				Computed:    true,
				Description: "SHA256 fingerprint of the key.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"has_private_key": rschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the server stores the private key (true for generated keys).",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": rschema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the key was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *sshKeyResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

type sshKeyModel struct {
	ID            types.Int64  `tfsdk:"id"`
	KeyName       types.String `tfsdk:"key_name"`
	Generate      types.Bool   `tfsdk:"generate"`
	PublicKey     types.String `tfsdk:"public_key"`
	PrivateKey    types.String `tfsdk:"private_key"`
	KeyType       types.String `tfsdk:"key_type"`
	Fingerprint   types.String `tfsdk:"fingerprint"`
	HasPrivateKey types.Bool   `tfsdk:"has_private_key"`
	CreatedAt     types.String `tfsdk:"created_at"`
}

func (r *sshKeyResource) setModelFromKey(m *sshKeyModel, k *models.SSHKey) {
	m.ID = types.Int64Value(int64(k.ID))
	m.KeyName = types.StringValue(k.KeyName)
	m.KeyType = types.StringValue(k.KeyType)
	m.PublicKey = types.StringValue(k.PublicKey)
	m.Fingerprint = types.StringValue(k.Fingerprint)
	m.HasPrivateKey = types.BoolValue(k.HasPrivateKey)
	m.CreatedAt = types.StringValue(k.CreatedAt)
	// private_key is only set during create for generated keys
	if k.PrivateKey != "" {
		m.PrivateKey = types.StringValue(k.PrivateKey)
	}
}

func (r *sshKeyResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan sshKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	generate := !plan.Generate.IsNull() && plan.Generate.ValueBool()
	hasPublicKey := !plan.PublicKey.IsNull() && plan.PublicKey.ValueString() != ""

	// Validate: either generate or import, not both
	if generate && hasPublicKey {
		resp.Diagnostics.AddError("Invalid configuration", "Cannot set both 'generate = true' and 'public_key'. Choose one.")
		return
	}
	if !generate && !hasPublicKey {
		resp.Diagnostics.AddError("Invalid configuration", "Must set either 'generate = true' or provide 'public_key' for import.")
		return
	}

	keyName := plan.KeyName.ValueString()

	if generate {
		// POST /ssh-keys/generate
		payload := map[string]any{}
		if keyName != "" {
			payload["key_name"] = keyName
		}

		var key models.SSHKey
		if err := r.c.PostJSON(ctx, client.SSHKeysEP+"/generate", payload, &key); err != nil {
			resp.Diagnostics.AddError("Failed to generate SSH key", err.Error())
			return
		}

		r.setModelFromKey(&plan, &key)
		plan.Generate = types.BoolValue(true)
	} else {
		// POST /ssh-keys/import
		// Trim whitespace/newlines from public key (file() often includes trailing newline)
		publicKey := strings.TrimSpace(plan.PublicKey.ValueString())
		payload := map[string]any{
			"public_key": publicKey,
		}
		if keyName != "" {
			payload["key_name"] = keyName
		}

		var key models.SSHKey
		if err := r.c.PostJSON(ctx, client.SSHKeysEP+"/import", payload, &key); err != nil {
			resp.Diagnostics.AddError("Failed to import SSH key", err.Error())
			return
		}

		// Preserve the original public_key from plan (with possible trailing newline)
		origPublicKey := plan.PublicKey
		r.setModelFromKey(&plan, &key)
		plan.PublicKey = origPublicKey
		// Keep generate as null (not false) to match plan
		plan.PrivateKey = types.StringNull() // No private key for imported keys
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sshKeyResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state sshKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueInt64()
	if id <= 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	var key models.SSHKey
	err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%d", client.SSHKeysEP, id), nil, &key)
	if err != nil {
		// Check if 404 - resource was deleted externally
		resp.State.RemoveResource(ctx)
		return
	}

	// Preserve generate, private_key, and public_key from state (not returned by API on read, or may differ in whitespace)
	oldGenerate := state.Generate
	oldPrivateKey := state.PrivateKey
	oldPublicKey := state.PublicKey

	r.setModelFromKey(&state, &key)

	state.Generate = oldGenerate
	state.PrivateKey = oldPrivateKey
	// Keep original public_key if it matches (ignoring whitespace)
	if strings.TrimSpace(oldPublicKey.ValueString()) == strings.TrimSpace(key.PublicKey) {
		state.PublicKey = oldPublicKey
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sshKeyResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	// SSH keys cannot be updated - all mutable attributes require replace
	var state sshKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sshKeyResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state sshKeyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueInt64()
	if id <= 0 {
		return
	}

	err := r.c.Delete(ctx, fmt.Sprintf("%s/%d", client.SSHKeysEP, id))
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete SSH key", err.Error())
		return
	}
}
