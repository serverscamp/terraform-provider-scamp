package provider

import (
	"context"
	"fmt"
	"time"

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

type volumeResource struct {
	c *client.Client
}

func NewVolumeResource() tfresource.Resource { return &volumeResource{} }

func (r *volumeResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_volume"
}

func (r *volumeResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	resp.Schema = rschema.Schema{
		Description: "Manages a volume (disk) in SCAMP.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed:    true,
				Description: "The UUID of the volume.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"display_name": rschema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Display name of the volume (max 100 characters).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"size_gb": rschema.Int64Attribute{
				Required:    true,
				Description: "Size of the volume in GB (1-10000).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"storage_class_id": rschema.Int64Attribute{
				Required:    true,
				Description: "ID of the storage class.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"attached_vm_id": rschema.StringAttribute{
				Optional:    true,
				Description: "UUID of the VM to attach the volume to. If not set, volume is created but not attached.",
			},
			// Computed fields
			"state": rschema.StringAttribute{
				Computed:    true,
				Description: "State of the volume (queued, provisioning, available, attached, etc.).",
			},
			"sds_pool_name": rschema.StringAttribute{
				Computed:    true,
				Description: "Name of the SDS pool.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"read_iops_limit": rschema.Int64Attribute{
				Computed:    true,
				Description: "Read IOPS limit.",
			},
			"write_iops_limit": rschema.Int64Attribute{
				Computed:    true,
				Description: "Write IOPS limit.",
			},
			"read_bandwidth_limit": rschema.Int64Attribute{
				Computed:    true,
				Description: "Read bandwidth limit (MB/s).",
			},
			"write_bandwidth_limit": rschema.Int64Attribute{
				Computed:    true,
				Description: "Write bandwidth limit (MB/s).",
			},
			"created_at": rschema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the volume was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *volumeResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

type volumeModel struct {
	ID                  types.String `tfsdk:"id"`
	DisplayName         types.String `tfsdk:"display_name"`
	SizeGB              types.Int64  `tfsdk:"size_gb"`
	StorageClassID      types.Int64  `tfsdk:"storage_class_id"`
	AttachedVMID        types.String `tfsdk:"attached_vm_id"`
	State               types.String `tfsdk:"state"`
	SDSPoolName         types.String `tfsdk:"sds_pool_name"`
	ReadIOPSLimit       types.Int64  `tfsdk:"read_iops_limit"`
	WriteIOPSLimit      types.Int64  `tfsdk:"write_iops_limit"`
	ReadBandwidthLimit  types.Int64  `tfsdk:"read_bandwidth_limit"`
	WriteBandwidthLimit types.Int64  `tfsdk:"write_bandwidth_limit"`
	CreatedAt           types.String `tfsdk:"created_at"`
}

func (r *volumeResource) setModelFromVolume(m *volumeModel, vol *models.Volume) {
	m.ID = types.StringValue(vol.DiskUUID)
	m.DisplayName = types.StringValue(vol.DisplayName)
	m.SizeGB = types.Int64Value(int64(vol.SizeGB))
	m.StorageClassID = types.Int64Value(int64(vol.StorageClassID))
	m.State = types.StringValue(vol.State)
	m.SDSPoolName = types.StringValue(vol.SDSPoolName)

	if vol.VMUUID != nil {
		m.AttachedVMID = types.StringValue(*vol.VMUUID)
	} else {
		m.AttachedVMID = types.StringNull()
	}

	if vol.Limits != nil {
		m.ReadIOPSLimit = types.Int64Value(int64(vol.Limits.ReadIOPSLimit))
		m.WriteIOPSLimit = types.Int64Value(int64(vol.Limits.WriteIOPSLimit))
		m.ReadBandwidthLimit = types.Int64Value(int64(vol.Limits.ReadBandwidthLimit))
		m.WriteBandwidthLimit = types.Int64Value(int64(vol.Limits.WriteBandwidthLimit))
	}

	if vol.CreatedAt != "" {
		m.CreatedAt = types.StringValue(vol.CreatedAt)
	}
}

func (r *volumeResource) waitForVolumeState(ctx context.Context, uuid string, targetStates []string, timeout time.Duration) (*models.Volume, error) {
	deadline := time.Now().Add(timeout)
	for {
		var vol models.Volume
		err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.VolumesEP, uuid), nil, &vol)
		if err != nil {
			return nil, err
		}
		for _, target := range targetStates {
			if vol.State == target {
				return &vol, nil
			}
		}
		if vol.State == "error" {
			return &vol, fmt.Errorf("volume %s entered error state", uuid)
		}
		if time.Now().After(deadline) {
			return &vol, fmt.Errorf("timeout waiting for volume %s to reach state %v (current: %s)", uuid, targetStates, vol.State)
		}
		tflog.Debug(ctx, "Waiting for volume state", map[string]any{
			"uuid":          uuid,
			"current":       vol.State,
			"target_states": targetStates,
		})
		time.Sleep(1 * time.Second)
	}
}

func (r *volumeResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan volumeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := map[string]any{
		"size_gb":          plan.SizeGB.ValueInt64(),
		"storage_class_id": plan.StorageClassID.ValueInt64(),
	}

	if !plan.DisplayName.IsNull() && plan.DisplayName.ValueString() != "" {
		payload["display_name"] = plan.DisplayName.ValueString()
	}

	var createResp models.VolumeCreateResponse
	if err := r.c.PostJSON(ctx, client.VolumesEP, payload, &createResp); err != nil {
		resp.Diagnostics.AddError("Failed to create volume", err.Error())
		return
	}

	plan.ID = types.StringValue(createResp.DiskUUID)

	// Save attached_vm_id from plan (not returned by API until attached)
	wantAttachVMID := plan.AttachedVMID

	// Wait for volume to become provisioned
	vol, err := r.waitForVolumeState(ctx, createResp.DiskUUID, []string{"provisioned"}, 5*time.Minute)
	if err != nil {
		resp.Diagnostics.AddWarning("Volume created but not yet available", err.Error())
	} else {
		r.setModelFromVolume(&plan, vol)
	}

	// Attach to VM if attached_vm_id is set
	if !wantAttachVMID.IsNull() && wantAttachVMID.ValueString() != "" {
		attachPayload := map[string]any{
			"vm_uuid": wantAttachVMID.ValueString(),
		}
		var attachResp models.VolumeAttachResponse
		if err := r.c.PostJSON(ctx, fmt.Sprintf("%s/%s/attach", client.VolumesEP, createResp.DiskUUID), attachPayload, &attachResp); err != nil {
			resp.Diagnostics.AddError("Failed to attach volume to VM", err.Error())
			return
		}

		// Wait for attached state
		vol, err = r.waitForVolumeState(ctx, createResp.DiskUUID, []string{"attached"}, 5*time.Minute)
		if err != nil {
			resp.Diagnostics.AddWarning("Volume attached but state not confirmed", err.Error())
		} else {
			r.setModelFromVolume(&plan, vol)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *volumeResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state volumeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := state.ID.ValueString()
	if uuid == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	var vol models.Volume
	err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.VolumesEP, uuid), nil, &vol)
	if err != nil {
		// Assume 404 - resource deleted
		resp.State.RemoveResource(ctx)
		return
	}

	r.setModelFromVolume(&state, &vol)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *volumeResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan volumeModel
	var state volumeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := state.ID.ValueString()

	// Check if attached_vm_id changed
	oldVMID := state.AttachedVMID.ValueString()
	newVMID := plan.AttachedVMID.ValueString()

	if oldVMID != newVMID {
		// Detach from old VM if was attached
		if oldVMID != "" {
			if err := r.c.PostJSON(ctx, fmt.Sprintf("%s/%s/detach", client.VolumesEP, uuid), nil, nil); err != nil {
				resp.Diagnostics.AddError("Failed to detach volume from VM", err.Error())
				return
			}
			// Wait for detached/provisioned state
			_, err := r.waitForVolumeState(ctx, uuid, []string{"provisioned", "detached"}, 5*time.Minute)
			if err != nil {
				resp.Diagnostics.AddWarning("Volume detached but state not confirmed", err.Error())
			}
		}

		// Attach to new VM if specified
		if newVMID != "" {
			attachPayload := map[string]any{
				"vm_uuid": newVMID,
			}
			if err := r.c.PostJSON(ctx, fmt.Sprintf("%s/%s/attach", client.VolumesEP, uuid), attachPayload, nil); err != nil {
				resp.Diagnostics.AddError("Failed to attach volume to VM", err.Error())
				return
			}
			// Wait for attached state
			_, err := r.waitForVolumeState(ctx, uuid, []string{"attached"}, 5*time.Minute)
			if err != nil {
				resp.Diagnostics.AddWarning("Volume attached but state not confirmed", err.Error())
			}
		}
	}

	// Read final state
	var vol models.Volume
	err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.VolumesEP, uuid), nil, &vol)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read volume after update", err.Error())
		return
	}

	r.setModelFromVolume(&plan, &vol)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *volumeResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state volumeModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	uuid := state.ID.ValueString()
	if uuid == "" {
		return
	}

	// Detach from VM if attached
	if !state.AttachedVMID.IsNull() && state.AttachedVMID.ValueString() != "" {
		if err := r.c.PostJSON(ctx, fmt.Sprintf("%s/%s/detach", client.VolumesEP, uuid), nil, nil); err != nil {
			resp.Diagnostics.AddError("Failed to detach volume before deletion", err.Error())
			return
		}
		// Wait for detached/provisioned state
		_, err := r.waitForVolumeState(ctx, uuid, []string{"provisioned", "detached"}, 5*time.Minute)
		if err != nil {
			resp.Diagnostics.AddWarning("Volume detach not confirmed, proceeding with delete", err.Error())
		}
	}

	if err := r.c.Delete(ctx, fmt.Sprintf("%s/%s", client.VolumesEP, uuid)); err != nil {
		resp.Diagnostics.AddError("Failed to delete volume", err.Error())
		return
	}
}
