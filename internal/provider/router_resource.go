package provider

import (
	"context"
	"fmt"
	"time"

	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

type routerResource struct {
	c *client.Client
}

func NewRouterResource() tfresource.Resource { return &routerResource{} }

func (r *routerResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_router"
}

func (r *routerResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		Description: "Manages a router in SCAMP. Routers provide internet access for attached networks.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed:    true,
				Description: "The UUID of the router.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": rschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Name of the router (1-64 characters). Auto-generated if not provided.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ipv4_address": rschema.StringAttribute{
				Computed:    true,
				Description: "Public IPv4 address assigned to the router.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"ipv6_address": rschema.StringAttribute{
				Computed:    true,
				Description: "Public IPv6 address assigned to the router.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": rschema.StringAttribute{
				Optional:    true,
				Description: "Description of the router.",
			},
			"tags": rschema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Tags to assign to the router as key-value pairs.",
			},
			"status": rschema.StringAttribute{
				Computed:    true,
				Description: "Current status of the router (provision_queued, active, etc.).",
			},
			"created_at": rschema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the router was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *routerResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

type routerModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	IPv4Address types.String `tfsdk:"ipv4_address"`
	IPv6Address types.String `tfsdk:"ipv6_address"`
	Description types.String `tfsdk:"description"`
	Tags        types.Map    `tfsdk:"tags"`
	Status      types.String `tfsdk:"status"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

func (r *routerResource) setModelFromRouter(m *routerModel, rt *models.Router) {
	m.ID = types.StringValue(rt.RouterUUID)
	m.Name = types.StringValue(rt.Name)
	m.IPv4Address = types.StringValue(rt.IPv4Address)
	m.IPv6Address = types.StringValue(rt.IPv6Address)
	m.Status = types.StringValue(rt.Status)
	if rt.CreatedAt != "" {
		m.CreatedAt = types.StringValue(rt.CreatedAt)
	}
}

// waitForRouterActive polls until router status is "active" or timeout.
func (r *routerResource) waitForRouterActive(ctx context.Context, uuid string, timeout time.Duration) (*models.Router, error) {
	deadline := time.Now().Add(timeout)
	for {
		var router models.Router
		err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.RoutersEP, uuid), nil, &router)
		if err != nil {
			return nil, err
		}
		if router.Status == "active" {
			return &router, nil
		}
		if time.Now().After(deadline) {
			return &router, fmt.Errorf("timeout waiting for router %s to become active (current status: %s)", uuid, router.Status)
		}
		tflog.Debug(ctx, "Waiting for router to become active", map[string]any{"uuid": uuid, "status": router.Status})
		time.Sleep(2 * time.Second)
	}
}

func (r *routerResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan routerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Build payload
	payload := map[string]any{}
	if !plan.Name.IsNull() && plan.Name.ValueString() != "" {
		payload["name"] = plan.Name.ValueString()
	}

	// Create router
	var router models.Router
	if err := r.c.PostJSON(ctx, client.RoutersEP, payload, &router); err != nil {
		resp.Diagnostics.AddError("Failed to create router", err.Error())
		return
	}

	// Wait for router to become active
	activeRouter, err := r.waitForRouterActive(ctx, router.RouterUUID, 2*time.Minute)
	if err != nil {
		resp.Diagnostics.AddWarning("Router created but not yet active", err.Error())
		r.setModelFromRouter(&plan, &router)
	} else {
		r.setModelFromRouter(&plan, activeRouter)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *routerResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state routerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := state.ID.ValueString()
	if uuid == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	var router models.Router
	err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.RoutersEP, uuid), nil, &router)
	if err != nil {
		// Assume 404 - resource deleted
		resp.State.RemoveResource(ctx)
		return
	}

	// Save local-only fields before overwriting
	savedDescription := state.Description
	savedTags := state.Tags

	r.setModelFromRouter(&state, &router)

	// Restore local-only fields (not in API)
	state.Description = savedDescription
	state.Tags = savedTags

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *routerResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	// Routers don't support updates via API - just preserve plan values
	var plan routerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *routerResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state routerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := state.ID.ValueString()
	if uuid == "" {
		return
	}

	if err := r.c.Delete(ctx, fmt.Sprintf("%s/%s", client.RoutersEP, uuid)); err != nil {
		resp.Diagnostics.AddError("Failed to delete router", err.Error())
		return
	}
}
