package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
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

// ValidateConfig: name or id, never both and never neither - same rule the VM
// resource applies to its class and image.
func (r *volumeResource) ValidateConfig(ctx context.Context, req tfresource.ValidateConfigRequest, resp *tfresource.ValidateConfigResponse) {
	var cfg volumeModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	hasName := !cfg.StorageClass.IsNull() && (cfg.StorageClass.IsUnknown() || cfg.StorageClass.ValueString() != "")
	hasID := !cfg.StorageClassID.IsNull()
	if hasName && hasID {
		resp.Diagnostics.AddError("Both storage_class and storage_class_id are set",
			"Set one of them: storage_class is the readable form, storage_class_id pins an id.")
	}
	if !hasName && !hasID {
		resp.Diagnostics.AddError("Neither storage_class nor storage_class_id is set",
			"Set storage_class (recommended, e.g. storage_class = \"R2\") or storage_class_id.")
	}
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
				Optional:    true,
				Computed:    true,
				Description: "ID of the storage class. Resolved from storage_class when that is set instead.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"storage_class": rschema.StringAttribute{
				Optional:    true,
				Description: "Name of the storage class, e.g. R2 or W3. Case does not matter. Use this or storage_class_id.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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
	StorageClass        types.String `tfsdk:"storage_class"`
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

// volumeReadyStates are every name the platform uses for "exists and is not
// attached to anything": public-api's own, plus the controller's two.
var volumeReadyStates = []string{"created", "provisioned", "detached"}

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

// readVolume is the one-shot read used when an error path still needs to leave
// a complete resource in state.
func (r *volumeResource) readVolume(ctx context.Context, uuid string) (*models.Volume, error) {
	var vol models.Volume
	if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%s", client.VolumesEP, uuid), nil, &vol); err != nil {
		return nil, err
	}
	return &vol, nil
}

func (r *volumeResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan volumeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	classID := plan.StorageClassID.ValueInt64()
	if !plan.StorageClass.IsNull() && plan.StorageClass.ValueString() != "" {
		id, err := resolveStorageClass(ctx, r.c, plan.StorageClass.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Could not resolve storage_class", err.Error())
			return
		}
		classID = int64(id)
	}

	payload := map[string]any{
		"size_gb":          plan.SizeGB.ValueInt64(),
		"storage_class_id": classID,
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
	// The volume exists from here on, whatever happens next. Put its id in
	// state immediately: if the wait or the attach below fails, Terraform still
	// knows about the resource and a later destroy removes it. Returning an
	// error without this left a real, billable volume that no state file
	// referenced - invisible to the user and to terraform destroy alike.
	plan.ID = types.StringValue(createResp.DiskUUID)
	resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)

	// A ready, unattached volume answers with any of three names, and which one
	// you get is not stable: the API returns its own bookkeeping state while the
	// request is still in flight ("created"), then hands over to the controller,
	// which calls the same disk "provisioned" when fresh and "detached" once it
	// has been attached to something before. Waiting for a single one of them
	// hangs for the full timeout on a volume that has been ready for minutes.
	// (The API schema's "provisioning, available, attached" is wrong throughout.)
	vol, err := r.waitForVolumeState(ctx, createResp.DiskUUID, volumeReadyStates, 5*time.Minute)
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
			// Save what we know before failing: the volume is created, only the
			// attach did not happen.
			if vol, rerr := r.readVolume(ctx, createResp.DiskUUID); rerr == nil {
				r.setModelFromVolume(&plan, vol)
				resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
			}
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
	// Imported volumes arrive with only an id, so storage_class is null while
	// the config names it. Translate the id back or every import plans a
	// replacement of a disk that holds data.
	if state.StorageClass.IsNull() || state.StorageClass.ValueString() == "" {
		if n := nameForStorageClass(ctx, r.c, int(state.StorageClassID.ValueInt64())); n != "" {
			state.StorageClass = types.StringValue(n)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ImportState brings an existing disk under management:
//
//	terraform import scamp_volume.data <volume-uuid>
func (r *volumeResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	tfresource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
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
			_, err := r.waitForVolumeState(ctx, uuid, volumeReadyStates, 5*time.Minute)
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
		_, err := r.waitForVolumeState(ctx, uuid, volumeReadyStates, 5*time.Minute)
		if err != nil {
			resp.Diagnostics.AddWarning("Volume detach not confirmed, proceeding with delete", err.Error())
		}
	}

	if err := r.c.Delete(ctx, fmt.Sprintf("%s/%s", client.VolumesEP, uuid)); err != nil {
		resp.Diagnostics.AddError("Failed to delete volume", err.Error())
		return
	}
}
