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

type networkResource struct {
	c *client.Client
}

func NewNetworkResource() tfresource.Resource { return &networkResource{} }

func (r *networkResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_network"
}

func (r *networkResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		Description: "Manages a network in SCAMP. Use type='private' for isolated networks, type='public' with router_uuid for internet-connected networks.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed:    true,
				Description: "The UUID of the network.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": rschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Name of the network (1-64 characters). Auto-generated if not provided.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cidr": rschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "CIDR block for the network (e.g., 10.50.0.0/24). Auto-generated if not provided.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": rschema.StringAttribute{
				Required:    true,
				Description: "Type of network: 'private' (isolated) or 'public' (attached to router, requires router_uuid).",
			},
			"router_uuid": rschema.StringAttribute{
				Optional:    true,
				Description: "UUID of the router to attach this network to. Required when type='public'.",
			},
			"description": rschema.StringAttribute{
				Optional:    true,
				Description: "Description of the network.",
			},
			"tags": rschema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Tags to assign to the network as key-value pairs.",
			},
			"status": rschema.StringAttribute{
				Computed:    true,
				Description: "Current status of the network (provision_queued, active, etc.).",
			},
			"created_at": rschema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the network was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *networkResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

type networkModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	CIDR        types.String `tfsdk:"cidr"`
	Type        types.String `tfsdk:"type"`
	RouterUUID  types.String `tfsdk:"router_uuid"`
	Description types.String `tfsdk:"description"`
	Tags        types.Map    `tfsdk:"tags"`
	Status      types.String `tfsdk:"status"`
	CreatedAt   types.String `tfsdk:"created_at"`
}

func (r *networkResource) setModelFromNetwork(m *networkModel, n *models.Network) {
	m.ID = types.StringValue(n.NetworkUUID)
	m.Name = types.StringValue(n.Name)
	m.CIDR = types.StringValue(n.CIDR)
	m.Type = types.StringValue(n.NetworkType)
	m.Status = types.StringValue(n.Status)
	if n.CreatedAt != "" {
		m.CreatedAt = types.StringValue(n.CreatedAt)
	}
	if n.RouterUUID != nil && *n.RouterUUID != "" {
		m.RouterUUID = types.StringValue(*n.RouterUUID)
	} else {
		m.RouterUUID = types.StringNull()
	}
}

// waitForNetworkActive polls until network status is "active" or timeout.
func (r *networkResource) waitForNetworkActive(ctx context.Context, uuid string, timeout time.Duration) (*models.Network, error) {
	deadline := time.Now().Add(timeout)
	for {
		var network models.Network
		err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.NetworksEP, uuid), nil, &network)
		if err != nil {
			return nil, err
		}
		if network.Status == "active" {
			return &network, nil
		}
		if time.Now().After(deadline) {
			return &network, fmt.Errorf("timeout waiting for network %s to become active (current status: %s)", uuid, network.Status)
		}
		tflog.Debug(ctx, "Waiting for network to become active", map[string]any{"uuid": uuid, "status": network.Status})
		time.Sleep(2 * time.Second)
	}
}

func (r *networkResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan networkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	networkType := plan.Type.ValueString()
	routerUUID := ""
	if !plan.RouterUUID.IsNull() {
		routerUUID = plan.RouterUUID.ValueString()
	}

	// Validate type and router_uuid combination
	if networkType != "private" && networkType != "public" {
		resp.Diagnostics.AddError("Invalid network type", "type must be 'private' or 'public'")
		return
	}
	if networkType == "public" && routerUUID == "" {
		resp.Diagnostics.AddError("Missing router_uuid", "router_uuid is required when type='public'")
		return
	}
	if networkType == "private" && routerUUID != "" {
		resp.Diagnostics.AddError("Invalid configuration", "router_uuid must not be set when type='private'")
		return
	}

	// Build payload
	payload := map[string]any{}
	if !plan.Name.IsNull() && plan.Name.ValueString() != "" {
		payload["name"] = plan.Name.ValueString()
	}
	if !plan.CIDR.IsNull() && plan.CIDR.ValueString() != "" {
		payload["cidr"] = plan.CIDR.ValueString()
	}

	// Create network
	var network models.Network
	if err := r.c.PostJSON(ctx, client.NetworksEP, payload, &network); err != nil {
		resp.Diagnostics.AddError("Failed to create network", err.Error())
		return
	}

	// Wait for network to become active
	activeNetwork, err := r.waitForNetworkActive(ctx, network.NetworkUUID, 2*time.Minute)
	if err != nil {
		resp.Diagnostics.AddWarning("Network created but not yet active", err.Error())
		r.setModelFromNetwork(&plan, &network)
	} else {
		r.setModelFromNetwork(&plan, activeNetwork)
	}

	// Attach to router if public
	if networkType == "public" {
		attachPayload := map[string]any{
			"router_uuid": routerUUID,
		}
		var attachResp models.NetworkAttachResponse
		if err := r.c.PostJSON(ctx, fmt.Sprintf("%s/%s/attach", client.NetworksEP, network.NetworkUUID), attachPayload, &attachResp); err != nil {
			resp.Diagnostics.AddError("Failed to attach network to router", err.Error())
			return
		}
		plan.RouterUUID = types.StringValue(attachResp.RouterUUID)
		plan.Type = types.StringValue("public")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := state.ID.ValueString()
	if uuid == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	var network models.Network
	err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.NetworksEP, uuid), nil, &network)
	if err != nil {
		// Assume 404 - resource deleted
		resp.State.RemoveResource(ctx)
		return
	}

	// Save local-only fields before overwriting
	savedDescription := state.Description
	savedTags := state.Tags

	r.setModelFromNetwork(&state, &network)

	// Restore local-only fields (not in API)
	state.Description = savedDescription
	state.Tags = savedTags

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *networkResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan, state networkModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := state.ID.ValueString()
	oldType := state.Type.ValueString()
	newType := plan.Type.ValueString()
	newRouterUUID := ""
	if !plan.RouterUUID.IsNull() {
		newRouterUUID = plan.RouterUUID.ValueString()
	}

	// Validate new configuration
	if newType != "private" && newType != "public" {
		resp.Diagnostics.AddError("Invalid network type", "type must be 'private' or 'public'")
		return
	}
	if newType == "public" && newRouterUUID == "" {
		resp.Diagnostics.AddError("Missing router_uuid", "router_uuid is required when type='public'")
		return
	}
	if newType == "private" && newRouterUUID != "" {
		resp.Diagnostics.AddError("Invalid configuration", "router_uuid must not be set when type='private'")
		return
	}

	// Handle type changes
	if oldType != newType {
		if oldType == "public" && newType == "private" {
			// Detach from router
			if err := r.c.Delete(ctx, fmt.Sprintf("%s/%s/detach", client.NetworksEP, uuid)); err != nil {
				resp.Diagnostics.AddError("Failed to detach network from router", err.Error())
				return
			}
		} else if oldType == "private" && newType == "public" {
			// Attach to router
			attachPayload := map[string]any{
				"router_uuid": newRouterUUID,
			}
			var attachResp models.NetworkAttachResponse
			if err := r.c.PostJSON(ctx, fmt.Sprintf("%s/%s/attach", client.NetworksEP, uuid), attachPayload, &attachResp); err != nil {
				resp.Diagnostics.AddError("Failed to attach network to router", err.Error())
				return
			}
		}
	} else if newType == "public" {
		// Same type but maybe different router
		oldRouter := state.RouterUUID.ValueString()
		if oldRouter != newRouterUUID {
			// Detach from old, attach to new
			if err := r.c.Delete(ctx, fmt.Sprintf("%s/%s/detach", client.NetworksEP, uuid)); err != nil {
				resp.Diagnostics.AddError("Failed to detach network from router", err.Error())
				return
			}
			attachPayload := map[string]any{
				"router_uuid": newRouterUUID,
			}
			var attachResp models.NetworkAttachResponse
			if err := r.c.PostJSON(ctx, fmt.Sprintf("%s/%s/attach", client.NetworksEP, uuid), attachPayload, &attachResp); err != nil {
				resp.Diagnostics.AddError("Failed to attach network to router", err.Error())
				return
			}
		}
	}

	// Re-read network to get updated state
	var network models.Network
	if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.NetworksEP, uuid), nil, &network); err != nil {
		resp.Diagnostics.AddError("Failed to read network after update", err.Error())
		return
	}

	// Save local-only fields before overwriting
	savedDescription := plan.Description
	savedTags := plan.Tags

	r.setModelFromNetwork(&plan, &network)

	// Restore local-only fields (not in API)
	plan.Description = savedDescription
	plan.Tags = savedTags

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *networkResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state networkModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := state.ID.ValueString()
	if uuid == "" {
		return
	}

	// Detach from router first if public
	if state.Type.ValueString() == "public" {
		_ = r.c.Delete(ctx, fmt.Sprintf("%s/%s/detach", client.NetworksEP, uuid))
		// Ignore error - might already be detached
	}

	// Delete network
	if err := r.c.Delete(ctx, fmt.Sprintf("%s/%s", client.NetworksEP, uuid)); err != nil {
		resp.Diagnostics.AddError("Failed to delete network", err.Error())
		return
	}
}
