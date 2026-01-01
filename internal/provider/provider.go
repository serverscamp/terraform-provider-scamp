package provider

import (
	"context"
	"os"

	fwds "github.com/hashicorp/terraform-plugin-framework/datasource"
	fwprov "github.com/hashicorp/terraform-plugin-framework/provider"
	provschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	fwres "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
)

type scampProvider struct{}

type providerData struct {
	APIURL types.String `tfsdk:"api_url"`
	Token  types.String `tfsdk:"token"`
}

func New() fwprov.Provider { return &scampProvider{} }

func (p *scampProvider) Metadata(_ context.Context, _ fwprov.MetadataRequest, resp *fwprov.MetadataResponse) {
	resp.TypeName = "scamp"
}

func (p *scampProvider) Schema(_ context.Context, _ fwprov.SchemaRequest, resp *fwprov.SchemaResponse) {
	resp.Schema = provschema.Schema{
		Description: "Terraform provider for managing SCAMP cloud resources.",
		Attributes: map[string]provschema.Attribute{
			"api_url": provschema.StringAttribute{
				Optional:    true,
				Description: "Base API URL (default: " + client.DefaultBaseURL + "). Can also be set via SCAMP_API_URL env var.",
			},
			"token": provschema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "API token (starts with sc_). Can also be set via SCAMP_TOKEN env var.",
			},
		},
	}
}

func (p *scampProvider) Configure(ctx context.Context, req fwprov.ConfigureRequest, resp *fwprov.ConfigureResponse) {
	var data providerData
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// API URL: config > env > default
	apiURL := client.DefaultBaseURL
	if envURL := os.Getenv("SCAMP_API_URL"); envURL != "" {
		apiURL = envURL
	}
	if !data.APIURL.IsNull() && data.APIURL.ValueString() != "" {
		apiURL = data.APIURL.ValueString()
	}

	// Token: env > config (env takes precedence for security)
	token := ""
	if !data.Token.IsNull() && data.Token.ValueString() != "" {
		token = data.Token.ValueString()
	}
	if envToken := os.Getenv("SCAMP_TOKEN"); envToken != "" {
		token = envToken
	}

	if token == "" {
		resp.Diagnostics.AddError("Missing API token", "Set token in provider config or SCAMP_TOKEN environment variable.")
		return
	}

	c := client.New(apiURL, token)
	tflog.Info(ctx, "Configured SCAMP client", map[string]any{"api_url": apiURL})
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *scampProvider) DataSources(_ context.Context) []func() fwds.DataSource {
	return []func() fwds.DataSource{
		NewSSHKeyDataSource,
		NewNetworkDataSource,
		NewRouterDataSource,
	}
}

func (p *scampProvider) Resources(_ context.Context) []func() fwres.Resource {
	return []func() fwres.Resource{
		NewSSHKeyResource,
		NewNetworkResource,
		NewRouterResource,
	}
}
