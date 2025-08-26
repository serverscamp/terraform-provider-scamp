package provider

import (
    "context"
    "net/url"
    "os"

    fwds "github.com/hashicorp/terraform-plugin-framework/datasource"
    dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
    fwprov "github.com/hashicorp/terraform-plugin-framework/provider"
    provschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
    fwres "github.com/hashicorp/terraform-plugin-framework/resource"
    "github.com/hashicorp/terraform-plugin-framework/types"
    "github.com/hashicorp/terraform-plugin-log/tflog"

    "github.com/serverscamp/terraform-provider-scamp/internal/client"
    "github.com/serverscamp/terraform-provider-scamp/internal/models"
)

type scampProvider struct{}

type providerData struct {
    BaseURL types.String `tfsdk:"base_url"`
    APIKey  types.String `tfsdk:"api_key"`
}

func New() fwprov.Provider { return &scampProvider{} }

func (p *scampProvider) Metadata(_ context.Context, _ fwprov.MetadataRequest, resp *fwprov.MetadataResponse) {
    resp.TypeName = "scamp"
}

func (p *scampProvider) Schema(_ context.Context, _ fwprov.SchemaRequest, resp *fwprov.SchemaResponse) {
    resp.Schema = provschema.Schema{
        Attributes: map[string]provschema.Attribute{
            "base_url": provschema.StringAttribute{
                Optional:    true,
                Description: "Base API URL for SCAMP (default: " + client.BaseURL + ")",
            },
            "api_key": provschema.StringAttribute{
                Optional:    true,
                Sensitive:   true,
                Description: "API key (or env SCAMP_API_KEY)",
            },
        },
    }
}

func (p *scampProvider) Configure(ctx context.Context, req fwprov.ConfigureRequest, resp *fwprov.ConfigureResponse) {
    var data providerData
    resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
    if resp.Diagnostics.HasError() { return }

    base := client.BaseURL
    if !data.BaseURL.IsNull() && data.BaseURL.ValueString() != "" {
        base = data.BaseURL.ValueString()
    }
    key := os.Getenv("SCAMP_API_KEY")
    if !data.APIKey.IsNull() && data.APIKey.ValueString() != "" {
        key = data.APIKey.ValueString()
    }
    if key == "" {
        resp.Diagnostics.AddError("Missing API key", "Set api_key or SCAMP_API_KEY environment variable.")
        return
    }
    c := client.New(base, key)
    tflog.Info(ctx, "Configured SCAMP client", map[string]any{"base_url": base})
    resp.DataSourceData = c
    resp.ResourceData = c
}

func (p *scampProvider) DataSources(_ context.Context) []func() fwds.DataSource {
    return []func() fwds.DataSource{
        NewFlavorsDataSource,
        NewImagesDataSource,
        NewLimitsDataSource,
        // scamp_instances excluded as requested
    }
}

func (p *scampProvider) Resources(_ context.Context) []func() fwres.Resource {
    return []func() fwres.Resource{
        NewSSHKeyResource,
        NewInstanceResource,
    }
}

// ==== Flavors DS ====
type flavorsDataSource struct{ c *client.Client }
func NewFlavorsDataSource() fwds.DataSource { return &flavorsDataSource{} }
func (d *flavorsDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) { resp.TypeName = "scamp_flavors" }
func (d *flavorsDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
    resp.Schema = dsschema.Schema{
        Attributes: map[string]dsschema.Attribute{
            "items": dsschema.ListNestedAttribute{
                Computed: true,
                NestedObject: dsschema.NestedAttributeObject{
                    Attributes: map[string]dsschema.Attribute{
                        "id": dsschema.Int64Attribute{Computed: true},
                        "name": dsschema.StringAttribute{Computed: true},
                        "vcores": dsschema.Int64Attribute{Computed: true},
                        "ram": dsschema.Float64Attribute{Computed: true},
                        "disk": dsschema.Float64Attribute{Computed: true},
                        "price": dsschema.Float64Attribute{Computed: true},
                    },
                },
            },
        },
    }
}
func (d *flavorsDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
    if req.ProviderData == nil { return }
    d.c = req.ProviderData.(*client.Client)
}

// typed state for scamp_flavors
type flavorItem struct {
    ID     types.Int64   `tfsdk:"id"`
    Name   types.String  `tfsdk:"name"`
    VCores types.Int64   `tfsdk:"vcores"`
    RAM    types.Float64 `tfsdk:"ram"`
    Disk   types.Float64 `tfsdk:"disk"`
    Price  types.Float64 `tfsdk:"price"`
}
type flavorsState struct {
    Items []flavorItem `tfsdk:"items"`
}
func (d *flavorsDataSource) Read(ctx context.Context, _ fwds.ReadRequest, resp *fwds.ReadResponse) {
    var api models.FlavorsResp
    if err := d.c.GetJSON(ctx, client.FlavorsEP, url.Values{}, &api); err != nil {
        resp.Diagnostics.AddError("API error", err.Error())
        return
    }
    out := flavorsState{Items: make([]flavorItem, 0, len(api.Data.Flavors))}
    for _, f := range api.Data.Flavors {
        out.Items = append(out.Items, flavorItem{
            ID:     types.Int64Value(toInt64Ptr(f.ID)),
            Name:   types.StringValue(f.Name),
            VCores: types.Int64Value(toInt64Ptr(f.VCores)),
            RAM:    types.Float64Value(toFloat64Ptr(f.RAM)),
            Disk:   types.Float64Value(toFloat64Ptr(f.Disk)),
            Price:  types.Float64Value(toFloat64Ptr(f.Price)),
        })
    }
    resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}

// ==== Images DS ====
type imagesDataSource struct{ c *client.Client }
func NewImagesDataSource() fwds.DataSource { return &imagesDataSource{} }
func (d *imagesDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) { resp.TypeName = "scamp_images" }
func (d *imagesDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
    resp.Schema = dsschema.Schema{
        Attributes: map[string]dsschema.Attribute{
            "items": dsschema.ListNestedAttribute{
                Computed: true,
                NestedObject: dsschema.NestedAttributeObject{
                    Attributes: map[string]dsschema.Attribute{
                        "id": dsschema.Int64Attribute{Computed: true},
                        "name": dsschema.StringAttribute{Computed: true},
                        "version": dsschema.StringAttribute{Computed: true},
                        "short_name": dsschema.StringAttribute{Computed: true},
                        "distro_family": dsschema.StringAttribute{Computed: true},
                    },
                },
            },
        },
    }
}
func (d *imagesDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
    if req.ProviderData == nil { return }
    d.c = req.ProviderData.(*client.Client)
}

// typed state for scamp_images
type imageItem struct {
    ID           types.Int64  `tfsdk:"id"`
    Name         types.String `tfsdk:"name"`
    Version      types.String `tfsdk:"version"`
    ShortName    types.String `tfsdk:"short_name"`
    DistroFamily types.String `tfsdk:"distro_family"`
}
type imagesState struct {
    Items []imageItem `tfsdk:"items"`
}
func (d *imagesDataSource) Read(ctx context.Context, _ fwds.ReadRequest, resp *fwds.ReadResponse) {
    var api models.ImagesResp
    if err := d.c.GetJSON(ctx, client.ImagesEP, url.Values{}, &api); err != nil {
        resp.Diagnostics.AddError("API error", err.Error())
        return
    }
    out := imagesState{Items: make([]imageItem, 0, len(api.Data.Images))}
    for _, it := range api.Data.Images {
        out.Items = append(out.Items, imageItem{
            ID:           types.Int64Value(toInt64Ptr(it.ID)),
            Name:         types.StringValue(it.Name),
            Version:      types.StringValue(it.Version),
            ShortName:    types.StringValue(it.ShortName),
            DistroFamily: types.StringValue(it.DistroFamily),
        })
    }
    resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}

// ==== Limits DS ====
type limitsDataSource struct{ c *client.Client }
func NewLimitsDataSource() fwds.DataSource { return &limitsDataSource{} }
func (d *limitsDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) { resp.TypeName = "scamp_limits" }
func (d *limitsDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
    resp.Schema = dsschema.Schema{
        Attributes: map[string]dsschema.Attribute{
            "items": dsschema.ListNestedAttribute{
                Computed: true,
                NestedObject: dsschema.NestedAttributeObject{
                    Attributes: map[string]dsschema.Attribute{
                        "id": dsschema.Int64Attribute{Computed: true},
                        "type": dsschema.StringAttribute{Computed: true},
                        "limit": dsschema.Int64Attribute{Computed: true},
                        "used": dsschema.Int64Attribute{Computed: true},
                        "remaining": dsschema.Int64Attribute{Computed: true},
                    },
                },
            },
        },
    }
}
func (d *limitsDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
    if req.ProviderData == nil { return }
    d.c = req.ProviderData.(*client.Client)
}

// typed state for scamp_limits
type limitItem struct {
    ID        types.Int64  `tfsdk:"id"`
    Type      types.String `tfsdk:"type"`
    Limit     types.Int64  `tfsdk:"limit"`
    Used      types.Int64  `tfsdk:"used"`
    Remaining types.Int64  `tfsdk:"remaining"`
}
type limitsState struct {
    Items []limitItem `tfsdk:"items"`
}
func (d *limitsDataSource) Read(ctx context.Context, _ fwds.ReadRequest, resp *fwds.ReadResponse) {
    var api models.LimitsResp
    if err := d.c.GetJSON(ctx, client.LimitsEP, url.Values{}, &api); err != nil {
        resp.Diagnostics.AddError("API error", err.Error())
        return
    }
    out := limitsState{Items: make([]limitItem, 0, len(api.Data.Limits))}
    for _, it := range api.Data.Limits {
        out.Items = append(out.Items, limitItem{
            ID:        types.Int64Value(toInt64Ptr(it.ID)),
            Type:      types.StringValue(it.Type),
            Limit:     types.Int64Value(toInt64Ptr(it.Limit)),
            Used:      types.Int64Value(toInt64Ptr(it.Used)),
            Remaining: types.Int64Value(toInt64Ptr(it.Remaining)),
        })
    }
    resp.Diagnostics.Append(resp.State.Set(ctx, &out)...)
}
