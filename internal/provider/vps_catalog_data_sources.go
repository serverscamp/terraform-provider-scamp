package provider

import (
	"context"

	fwds "github.com/hashicorp/terraform-plugin-framework/datasource"
	dsschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

// scamp_vps_plans

type vpsPlansDataSource struct {
	c *client.Client
}

func NewVPSPlansDataSource() fwds.DataSource { return &vpsPlansDataSource{} }

func (d *vpsPlansDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_vps_plans"
}

func (d *vpsPlansDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Retrieves the Simple VPS plans.",
		Attributes: map[string]dsschema.Attribute{
			"items": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "List of plans.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id":            dsschema.StringAttribute{Computed: true, Description: "Plan id, the value for scamp_vps.plan."},
						"name":          dsschema.StringAttribute{Computed: true, Description: "Name of the plan."},
						"vcpu":          dsschema.Int64Attribute{Computed: true, Description: "Number of vCPUs."},
						"ram_gb":        dsschema.Float64Attribute{Computed: true, Description: "Memory in GB."},
						"disk_gb":       dsschema.Int64Attribute{Computed: true, Description: "Disk in GB."},
						"traffic_gb":    dsschema.Int64Attribute{Computed: true, Description: "Outbound traffic included a month, in GB."},
						"net_mbit":      dsschema.Int64Attribute{Computed: true, Description: "Port speed in Mbit/s."},
						"price_monthly": dsschema.Float64Attribute{Computed: true, Description: "Price a month in EUR."},
						"backup_share":  dsschema.Float64Attribute{Computed: true, Description: "Daily backups add this share of the price."},
					},
				},
			},
		},
	}
}

func (d *vpsPlansDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type vpsPlanModel struct {
	ID           types.String  `tfsdk:"id"`
	Name         types.String  `tfsdk:"name"`
	VCPU         types.Int64   `tfsdk:"vcpu"`
	RAMGB        types.Float64 `tfsdk:"ram_gb"`
	DiskGB       types.Int64   `tfsdk:"disk_gb"`
	TrafficGB    types.Int64   `tfsdk:"traffic_gb"`
	NetMbit      types.Int64   `tfsdk:"net_mbit"`
	PriceMonthly types.Float64 `tfsdk:"price_monthly"`
	BackupShare  types.Float64 `tfsdk:"backup_share"`
}

func (d *vpsPlansDataSource) Read(ctx context.Context, _ fwds.ReadRequest, resp *fwds.ReadResponse) {
	var list struct {
		Items []models.VPSPlan `json:"items"`
	}
	if err := d.c.GetJSON(ctx, client.VPSEP+"/plans", nil, &list); err != nil {
		resp.Diagnostics.AddError("Failed to read VPS plans", err.Error())
		return
	}
	var state struct {
		Items []vpsPlanModel `tfsdk:"items"`
	}
	for _, p := range list.Items {
		state.Items = append(state.Items, vpsPlanModel{
			ID:           types.StringValue(p.ID),
			Name:         types.StringValue(p.Name),
			VCPU:         types.Int64Value(int64(p.VCPU)),
			RAMGB:        types.Float64Value(p.RAMGB),
			DiskGB:       types.Int64Value(int64(p.DiskGB)),
			TrafficGB:    types.Int64Value(int64(p.TrafficGB)),
			NetMbit:      types.Int64Value(int64(p.NetMbit)),
			PriceMonthly: types.Float64Value(p.PriceMonthly),
			BackupShare:  types.Float64Value(p.BackupShare),
		})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// scamp_vps_images

type vpsImagesDataSource struct {
	c *client.Client
}

func NewVPSImagesDataSource() fwds.DataSource { return &vpsImagesDataSource{} }

func (d *vpsImagesDataSource) Metadata(_ context.Context, _ fwds.MetadataRequest, resp *fwds.MetadataResponse) {
	resp.TypeName = "scamp_vps_images"
}

func (d *vpsImagesDataSource) Schema(_ context.Context, _ fwds.SchemaRequest, resp *fwds.SchemaResponse) {
	resp.Schema = dsschema.Schema{
		Description: "Retrieves the systems a Simple VPS can be installed with.",
		Attributes: map[string]dsschema.Attribute{
			"items": dsschema.ListNestedAttribute{
				Computed:    true,
				Description: "List of images.",
				NestedObject: dsschema.NestedAttributeObject{
					Attributes: map[string]dsschema.Attribute{
						"id":      dsschema.StringAttribute{Computed: true, Description: "Image id, the value for scamp_vps.image."},
						"os":      dsschema.StringAttribute{Computed: true, Description: "Operating system."},
						"version": dsschema.StringAttribute{Computed: true, Description: "Version of the system."},
					},
				},
			},
		},
	}
}

func (d *vpsImagesDataSource) Configure(_ context.Context, req fwds.ConfigureRequest, _ *fwds.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	d.c = req.ProviderData.(*client.Client)
}

type vpsImageModel struct {
	ID      types.String `tfsdk:"id"`
	OS      types.String `tfsdk:"os"`
	Version types.String `tfsdk:"version"`
}

func (d *vpsImagesDataSource) Read(ctx context.Context, _ fwds.ReadRequest, resp *fwds.ReadResponse) {
	var list struct {
		Items []models.VPSImage `json:"items"`
	}
	if err := d.c.GetJSON(ctx, client.VPSEP+"/images", nil, &list); err != nil {
		resp.Diagnostics.AddError("Failed to read VPS images", err.Error())
		return
	}
	var state struct {
		Items []vpsImageModel `tfsdk:"items"`
	}
	for _, i := range list.Items {
		state.Items = append(state.Items, vpsImageModel{ID: types.StringValue(i.ID), OS: types.StringValue(i.OS), Version: types.StringValue(i.Version)})
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
