package provider

import (
    "context"
    "fmt"
    "net/url"
    "time"
    "strings"

    tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
    rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
    "github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
    "github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
    "github.com/hashicorp/terraform-plugin-framework/types"
    diag "github.com/hashicorp/terraform-plugin-framework/diag"

    "scamp/internal/client"
    "scamp/internal/models"
)

type instanceResource struct{ c *client.Client }

func NewInstanceResource() tfresource.Resource { return &instanceResource{} }

func (r *instanceResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
    resp.TypeName = "scamp_instance"
}

func (r *instanceResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
    resp.Schema = rschema.Schema{
        Attributes: map[string]rschema.Attribute{
            "name":     rschema.StringAttribute{Required: true},
            "flavor":   rschema.Int64Attribute{Required: true},
            "image":    rschema.Int64Attribute{Required: true},
            "ssh_key":  rschema.Int64Attribute{
                Required: true,
                PlanModifiers: []planmodifier.Int64{
                    int64planmodifier.RequiresReplace(),
                },
            },
            "dc":       rschema.StringAttribute{Optional: true, Computed: true},
            "password": rschema.StringAttribute{Optional: true, Sensitive: true},
            "running":  rschema.BoolAttribute{Optional: true, Computed: true},
            "id":          rschema.Int64Attribute{Computed: true},
            "os":          rschema.StringAttribute{Computed: true},
            "distro_base": rschema.StringAttribute{Computed: true},
            "ipv4":        rschema.StringAttribute{Computed: true},
            "ipv6":        rschema.StringAttribute{Computed: true},
            "created_at":  rschema.StringAttribute{Computed: true},
            "vmid":        rschema.StringAttribute{Computed: true},
            "price_month": rschema.Float64Attribute{Computed: true},
            "cpus":        rschema.Int64Attribute{Computed: true},
            "ram":         rschema.Float64Attribute{Computed: true},
            "disk":        rschema.Float64Attribute{Computed: true},
            "flavor_id":   rschema.Int64Attribute{Computed: true},
            "image_id":    rschema.Int64Attribute{Computed: true},
            "ssh_key_id":  rschema.Int64Attribute{Computed: true},
            "create_status": rschema.Int64Attribute{Computed: true},
        },
    }
}

func (r *instanceResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
    if req.ProviderData == nil { return }
    r.c = req.ProviderData.(*client.Client)
}

type instanceModel struct {
    ID         types.Int64  `tfsdk:"id"`
    Name       types.String `tfsdk:"name"`
    Flavor     types.Int64  `tfsdk:"flavor"`
    Image      types.Int64  `tfsdk:"image"`
    SSHKey     types.Int64  `tfsdk:"ssh_key"`
    DC         types.String `tfsdk:"dc"`
    Password   types.String `tfsdk:"password"`
    Running    types.Bool   `tfsdk:"running"`
    OS         types.String  `tfsdk:"os"`
    DistroBase types.String  `tfsdk:"distro_base"`
    IPv4       types.String  `tfsdk:"ipv4"`
    IPv6       types.String  `tfsdk:"ipv6"`
    CreatedAt  types.String  `tfsdk:"created_at"`
    VMID       types.String  `tfsdk:"vmid"`
    PriceMonth types.Float64 `tfsdk:"price_month"`
    CPUs       types.Int64   `tfsdk:"cpus"`
    RAM        types.Float64 `tfsdk:"ram"`
    Disk       types.Float64 `tfsdk:"disk"`
    FlavorID   types.Int64   `tfsdk:"flavor_id"`
    ImageID    types.Int64   `tfsdk:"image_id"`
    SSHKeyID   types.Int64   `tfsdk:"ssh_key_id"`
    CreateStatus types.Int64   `tfsdk:"create_status"`
}

// waitSSHKeyVisible polls GET /ssh-keys/{id} until it is visible or timeout.
func (r *instanceResource) waitSSHKeyVisible(ctx context.Context, id int64, timeout time.Duration) error {
    if timeout <= 0 {
        timeout = 60 * time.Second
    }
    deadline := time.Now().Add(timeout)
    for {
        var kr models.SSHKeyResp
        err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%d", client.SSHKeysEP, id), nil, &kr)
        if err == nil && kr.OK && int64(kr.Data.ID) == id {
            return nil
        }
        if time.Now().After(deadline) {
            if err != nil {
                return fmt.Errorf("timeout waiting ssh_key %d visible: %v", id, err)
            }
            return fmt.Errorf("timeout waiting ssh_key %d visible", id)
        }
        time.Sleep(2 * time.Second)
    }
}

// waitInstanceProvisioned polls GET /instances/{id} until instance is strictly provisioned or timeout.
func (r *instanceResource) waitInstanceProvisioned(ctx context.Context, id int64, timeout time.Duration) error {
    if timeout <= 0 {
        timeout = 10 * time.Minute
    }
    deadline := time.Now().Add(timeout)
    for {
        var tmp instanceModel
        var diags diag.Diagnostics
        _ = r.readInstance(ctx, id, &tmp, &diags) // ignore transient diag errors here

        // Consider ready:
        // 1) If create_status exists: require >= 4 (backend marks created+running as 4)
        // 2) Else require both: running == true AND ipv4 != ""
        if !tmp.ID.IsNull() && tmp.ID.ValueInt64() == id {
            if !tmp.CreateStatus.IsNull() {
                cs := tmp.CreateStatus.ValueInt64()
                if cs >= 4 {
                    return nil
                }
            } else {
                if !tmp.Running.IsNull() && tmp.Running.ValueBool() && tmp.IPv4.ValueString() != "" {
                    return nil
                }
            }
        }

        if time.Now().After(deadline) {
            return fmt.Errorf("timeout waiting instance %d to be provisioned", id)
        }
        time.Sleep(2 * time.Second)
    }
}

// isTransientKeyNotFound returns true if the error/message indicates the SSH key is not yet visible to the instances service.
func (r *instanceResource) isTransientKeyNotFound(err error, apiMsg string) bool {
    // Common substrings from backend
    if strings.Contains(apiMsg, "SSH key not found") {
        return true
    }
    if err != nil {
        es := err.Error()
        if strings.Contains(es, "SSH key not found") || strings.Contains(es, "http 404") || strings.Contains(es, "NOT_FOUND") {
            return true
        }
    }
    return false
}

// findInstanceByExactName queries /instances?name=<name> and returns the most recent exact match.
func (r *instanceResource) findInstanceByExactName(ctx context.Context, name string) (*models.Instance, error) {
    q := url.Values{}
    q.Set("name", name)
    var ir models.InstancesResp
    if err := r.c.GetJSON(ctx, client.InstancesEP, q, &ir); err != nil {
        return nil, err
    }
    if len(ir.Data.Instances) == 0 {
        return nil, fmt.Errorf("no instances with name %q", name)
    }
    // Filter exact matches only and pick the most recent by created_at
    var best *models.Instance
    var bestT time.Time
    for i := range ir.Data.Instances {
        it := &ir.Data.Instances[i]
        if it.Name != name {
            continue
        }
        t, _ := time.Parse(time.RFC3339Nano, it.CreatedAt)
        if best == nil || t.After(bestT) {
            cp := *it
            best = &cp
            bestT = t
        }
    }
    if best == nil {
        return nil, fmt.Errorf("no exact match for name %q", name)
    }
    return best, nil
}

func (r *instanceResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
    var plan instanceModel
    resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
    if resp.Diagnostics.HasError() { return }

    // Ensure all required inputs are known (not unknown) and non-null at apply time
    if plan.Flavor.IsUnknown() || plan.Image.IsUnknown() || plan.SSHKey.IsUnknown() || plan.Name.IsUnknown() {
        resp.Diagnostics.AddError("attributes not known", "attributes flavor, image, ssh_key, name must be known at apply (avoid unknown values).")
        return
    }
    if plan.Flavor.IsNull() || plan.Image.IsNull() || plan.SSHKey.IsNull() || plan.Name.IsNull() {
        resp.Diagnostics.AddError("missing required attributes", "attributes flavor, image, ssh_key, name cannot be null at apply.")
        return
    }

    // capture desired values to stabilize post-apply state
    wantName := plan.Name.ValueString()
    wantFlavor := plan.Flavor.ValueInt64()
    wantDCNull := plan.DC.IsNull()

    // validate and extract numeric values for IDs
    flavorID := plan.Flavor.ValueInt64()
    imageID  := plan.Image.ValueInt64()
    sshKeyID := plan.SSHKey.ValueInt64()
    if flavorID <= 0 || imageID <= 0 || sshKeyID <= 0 {
        resp.Diagnostics.AddError("invalid ids", fmt.Sprintf("flavor=%d image=%d ssh_key=%d must be > 0", flavorID, imageID, sshKeyID))
        return
    }

    payload := map[string]any{
        "name":    plan.Name.ValueString(),
        "flavor":  int(flavorID),
        "image":   int(imageID),
        "ssh_key": int(sshKeyID),
    }
    if !plan.DC.IsNull() && plan.DC.ValueString() != "" {
        payload["dc"] = plan.DC.ValueString()
    }
    if !plan.Password.IsNull() && plan.Password.ValueString() != "" {
        payload["password"] = plan.Password.ValueString()
    }

    // ensure SSH key is visible to backend before creating instance
    if sshKeyID > 0 {
        if err := r.waitSSHKeyVisible(ctx, sshKeyID, 60*time.Second); err != nil {
            resp.Diagnostics.AddError("ssh key not visible", err.Error()); return
        }
    }

    var jr struct {
        OK        bool             `json:"ok"`
        JobID     string           `json:"job_id"`
        StatusURL string           `json:"status_url"`
        Message   string           `json:"message"`
        Error     *models.APIError `json:"error,omitempty"`
    }
    // Retry enqueue if backend isn't yet seeing the SSH key (eventual consistency).
    enqueueDeadline := time.Now().Add(2 * time.Minute)
    for {
        jr = struct {
            OK        bool             `json:"ok"`
            JobID     string           `json:"job_id"`
            StatusURL string           `json:"status_url"`
            Message   string           `json:"message"`
            Error     *models.APIError `json:"error,omitempty"`
        }{}
        err := r.c.PostJSON(ctx, client.InstancesEP, payload, &jr)
        if err != nil {
            if time.Now().Before(enqueueDeadline) && r.isTransientKeyNotFound(err, "") {
                time.Sleep(3 * time.Second)
                continue
            }
            resp.Diagnostics.AddError("create failed", err.Error())
            return
        }
        if !jr.OK {
            // Look for backend-provided message about SSH key visibility and retry a bit
            apiMsg := ""
            if jr.Error != nil {
                apiMsg = jr.Error.Message
            } else if jr.Message != "" {
                apiMsg = jr.Message
            }
            if time.Now().Before(enqueueDeadline) && r.isTransientKeyNotFound(nil, apiMsg) {
                time.Sleep(3 * time.Second)
                continue
            }
            resp.Diagnostics.AddError("create enqueue failed", apiMsg)
            return
        }
        break
    }

    // Poll either by status_url or job_id
    var tr *models.TaskResp
    var perr error
    if jr.StatusURL != "" {
        tr, perr = r.c.PollURL(ctx, jr.StatusURL, 2*time.Second)
    } else {
        tr, perr = r.c.PollTask(ctx, jr.JobID, 2*time.Second)
    }
    if perr != nil { resp.Diagnostics.AddError("task poll failed", perr.Error()); return }
    if tr.Data.Error { resp.Diagnostics.AddError("task error", tr.Data.ErrorData); return }

    // Give backend a short time to propagate the new resource_id
    time.Sleep(2 * time.Second)

    id, ok := toID(tr.Data.ResourceID)
    if !ok || id <= 0 {
        resp.Diagnostics.AddError("no resource_id", "task finished without resource_id; backend now supports GET /instances/{id}, so resource_id is required")
        return
    }

    // Wait until instance provisioning completes before first read, so Terraform doesn't finish too early.
    if err := r.waitInstanceProvisioned(ctx, int64(id), 10*time.Minute); err != nil {
        resp.Diagnostics.AddError("provisioning wait failed", err.Error()); return
    }
    if err := r.readInstance(ctx, int64(id), &plan, &resp.Diagnostics); err != nil { return }


    // Stabilize: keep user-requested values to avoid post-apply drift
    if wantDCNull && (plan.DC.IsNull() || plan.DC.ValueString() == "") {
        plan.DC = types.StringNull()
    }
    // keep the requested name (backend may normalize/display shorter)
    if wantName != "" {
        plan.Name = types.StringValue(wantName)
    }
    // keep the requested flavor id for immediate post-apply; Read will converge later
    if wantFlavor > 0 {
        plan.Flavor = types.Int64Value(wantFlavor)
    }
    resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *instanceResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
    var state instanceModel
    resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
    if resp.Diagnostics.HasError() { return }
    if state.ID.IsNull() || state.ID.ValueInt64() <= 0 {
        resp.State.RemoveResource(ctx); return
    }
    if err := r.readInstance(ctx, state.ID.ValueInt64(), &state, &resp.Diagnostics); err != nil { return }
    resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *instanceResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
    var plan, state instanceModel
    resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
    resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
    if resp.Diagnostics.HasError() { return }

    desired := true
    if !plan.Running.IsNull() { desired = plan.Running.ValueBool() }
    current := false
    if !state.Running.IsNull() { current = state.Running.ValueBool() }
    if desired != current {
        act := "start"
        if !desired { act = "stop" }
        var noop any
        if err := r.c.PostJSON(ctx, fmt.Sprintf("%s/%d/%s", client.InstancesEP, state.ID.ValueInt64(), act), map[string]any{}, &noop); err != nil {
            resp.Diagnostics.AddError("power action failed", err.Error()); return
        }
    }

    if err := r.readInstance(ctx, state.ID.ValueInt64(), &state, &resp.Diagnostics); err != nil { return }
    resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *instanceResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
    var state instanceModel
    resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
    if resp.Diagnostics.HasError() { return }
    if state.ID.IsNull() || state.ID.ValueInt64() <= 0 { return }
    if err := r.c.Delete(ctx, fmt.Sprintf("%s/%d", client.InstancesEP, state.ID.ValueInt64())); err != nil {
        resp.Diagnostics.AddError("delete failed", err.Error()); return
    }
}

func (r *instanceResource) readInstance(ctx context.Context, id int64, out *instanceModel, diags *diag.Diagnostics) error {
    // Try GET /instances/{id} first
    var ir models.InstancesResp
    err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%d", client.InstancesEP, id), url.Values{}, &ir)
    if err != nil {
        es := err.Error()
        // Fallback: if 405/404 or METHOD_NOT_ALLOWED/NOT_FOUND -> list all and filter client-side by id
        if strings.Contains(es, "405") || strings.Contains(es, "METHOD_NOT_ALLOWED") || strings.Contains(es, "404") || strings.Contains(es, "NOT_FOUND") {
            var ir2 models.InstancesResp
            if err2 := r.c.GetJSON(ctx, client.InstancesEP, url.Values{}, &ir2); err2 != nil {
                diags.AddError("read instance failed", err2.Error())
                return err2
            }
            ir = ir2
        } else {
            diags.AddError("read instance failed", err.Error())
            return err
        }
    }

    var it *models.Instance
    for i := range ir.Data.Instances {
        cand := &ir.Data.Instances[i]
        if cand.ID != nil && int64(*cand.ID) == id {
            it = cand
            break
        }
    }
    if it == nil {
        out.ID = types.Int64Null()
        return nil
    }
    out.ID = types.Int64Value(toInt64Ptr(it.ID))
    out.Name = types.StringValue(it.Name)
    out.Flavor = types.Int64Value(toInt64Ptr(it.FlavorID))
    out.Image = types.Int64Value(toInt64Ptr(it.ImageID))
    out.SSHKey = types.Int64Value(toInt64Ptr(it.SSHKeyID))
    out.DC = types.StringNull()
    out.Running = types.BoolValue(it.Running)

    out.OS = types.StringValue(it.OS)
    out.DistroBase = types.StringValue(it.DistroBase)
    out.IPv4 = types.StringValue(it.IPv4)
    out.IPv6 = types.StringValue(it.IPv6)
    out.CreatedAt = types.StringValue(it.CreatedAt)
    out.VMID = types.StringValue(it.VMID)
    out.PriceMonth = types.Float64PointerValue(ptrFloat64(it.PriceMonth))
    out.CPUs = types.Int64Value(toInt64Ptr(it.CPUs))
    out.RAM = types.Float64Value(toFloat64Ptr(it.RAM))
    out.Disk = types.Float64Value(toFloat64Ptr(it.Disk))
    out.FlavorID = types.Int64Value(toInt64Ptr(it.FlavorID))
    out.ImageID = types.Int64Value(toInt64Ptr(it.ImageID))
    out.SSHKeyID = types.Int64Value(toInt64Ptr(it.SSHKeyID))
    // Backend model currently has no explicit CreateStatus field in models.Instance
    // Leave as null; wait logic will fall back to running+ipv4 readiness.
    out.CreateStatus = types.Int64Null()
    return nil
}
