package provider

import (
	"context"
	"fmt"

	fwds "github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

type sshKeyDataSource struct {
	c *client.Client
}

func NewSSHKeyDataSource() fwds.DataSource { return &sshKeyDataSource{} }

func (d *sshKeyDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_ssh_key"
}

func (d *sshKeyDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Retrieves information about an existing SSH key by ID.",
		Attributes: map[string]dsschema.Attribute{
			"id": dsschema.Int64Attribute{
				Required:    true,
				Description: "The ID of the SSH key to retrieve.",
			},
			"key_name": dsschema.StringAttribute{
				Computed:    true,
				Description: "Name of the SSH key.",
			},
			"key_type": dsschema.StringAttribute{
				Computed:    true,
				Description: "Type of SSH key (ed25519, rsa, etc.).",
			},
			"public_key": dsschema.StringAttribute{
				Computed:    true,
				Description: "Public key in OpenSSH format.",
			},
			"fingerprint": dsschema.StringAttribute{
				Computed:    true,
				Description: "SHA256 fingerprint of the key.",
			},
			"has_private_key": dsschema.BoolAttribute{
				Computed:    true,
				Description: "Whether the server stores the private key.",
			},
			"created_at": dsschema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the key was created.",
			},
		},
	}
}

func (d *sshKeyDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type sshKeyDataSourceModel struct {
	ID            types.Int64  `tfsdk:"id"`
	KeyName       types.String `tfsdk:"key_name"`
	KeyType       types.String `tfsdk:"key_type"`
	PublicKey     types.String `tfsdk:"public_key"`
	Fingerprint   types.String `tfsdk:"fingerprint"`
	HasPrivateKey types.Bool   `tfsdk:"has_private_key"`
	CreatedAt     types.String `tfsdk:"created_at"`
}

func (d *sshKeyDataSource) Read(ctx context.Context, req fwds.ReadRequest, resp *fwds.ReadResponse) {
	var config sshKeyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := config.ID.ValueInt64()

	var key models.SSHKey
	err := d.c.GetJSON(ctx, fmt.Sprintf("%s/%d", client.SSHKeysEP, id), nil, &key)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read SSH key", err.Error())
		return
	}

	config.ID = types.Int64Value(int64(key.ID))
	config.KeyName = types.StringValue(key.KeyName)
	config.KeyType = types.StringValue(key.KeyType)
	config.PublicKey = types.StringValue(key.PublicKey)
	config.Fingerprint = types.StringValue(key.Fingerprint)
	config.HasPrivateKey = types.BoolValue(key.HasPrivateKey)
	config.CreatedAt = types.StringValue(key.CreatedAt)

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
