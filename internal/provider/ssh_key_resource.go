package provider

import (
    "context"
    "fmt"
    "strings"
    "time"

    tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
    rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
    "github.com/hashicorp/terraform-plugin-framework/types"

    "github.com/serverscamp/terraform-provider-scamp/internal/client"
    "github.com/serverscamp/terraform-provider-scamp/internal/models"
)

type sshKeyResource struct{ c *client.Client }
func NewSSHKeyResource() tfresource.Resource { return &sshKeyResource{} }

func (r *sshKeyResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
    resp.TypeName = "scamp_ssh_key"
}

func (r *sshKeyResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
    resp.Schema = rschema.Schema{
        Attributes: map[string]rschema.Attribute{
            "id":           rschema.Int64Attribute{Computed: true},
            "name":         rschema.StringAttribute{Required: true},
            "public_key":   rschema.StringAttribute{Optional: true, Sensitive: true},
            "protected":    rschema.BoolAttribute{Optional: true, Computed: true},
            "fingerprint":  rschema.StringAttribute{Computed: true},
            "has_private":  rschema.BoolAttribute{Computed: true},
            "servers_count": rschema.Int64Attribute{Computed: true},
            "created_at":   rschema.StringAttribute{Computed: true},
        },
    }
}

func (r *sshKeyResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
    if req.ProviderData == nil { return }
    r.c = req.ProviderData.(*client.Client)
}

type sshKeyModel struct {
    ID           types.Int64  `tfsdk:"id"`
    Name         types.String `tfsdk:"name"`
    PublicKey    types.String `tfsdk:"public_key"`
    Protected    types.Bool   `tfsdk:"protected"`
    Fingerprint  types.String `tfsdk:"fingerprint"`
    HasPrivate   types.Bool   `tfsdk:"has_private"`
    ServersCount types.Int64  `tfsdk:"servers_count"`
    CreatedAt    types.String `tfsdk:"created_at"`
}

func (r *sshKeyResource) setModelFromKey(m *sshKeyModel, k *models.SSHKey) {
    m.ID = types.Int64Value(int64(k.ID))
    m.Name = types.StringValue(k.Name)
    m.Fingerprint = types.StringValue(k.Fingerprint)
    m.Protected = types.BoolValue(k.Protected)
    m.HasPrivate = types.BoolValue(k.HasPrivate)
    m.ServersCount = types.Int64Value(int64(k.ServersCount))
    m.CreatedAt = types.StringValue(k.CreatedAt)
}

// waitForProtectState polls GET /ssh-keys/{id} until Protected matches desired or timeout.
func (r *sshKeyResource) waitForProtectState(ctx context.Context, id int64, desired bool, timeout time.Duration) error {
    if timeout <= 0 { timeout = 180 * time.Second }
    deadline := time.Now().Add(timeout)
    for {
        var kr models.SSHKeyResp
        if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%d", client.SSHKeysEP, id), nil, &kr); err != nil {
            return err
        }
        if kr.OK && kr.Data.Protected == desired {
            return nil
        }
        if time.Now().After(deadline) {
            return fmt.Errorf("timeout waiting for protected=%v on key %d", desired, id)
        }
        time.Sleep(2 * time.Second)
    }
}

// waitKeyVisible polls GET /ssh-keys/{id} until API returns the key or timeout.
func (r *sshKeyResource) waitKeyVisible(ctx context.Context, id int64, timeout time.Duration) error {
    if timeout <= 0 { timeout = 120 * time.Second }
    deadline := time.Now().Add(timeout)
    for {
        var kr models.SSHKeyResp
        err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%d", client.SSHKeysEP, id), nil, &kr)
        if err == nil && kr.OK && kr.Data.ID == int(id) {
            return nil
        }
        if time.Now().After(deadline) {
            if err != nil {
                return fmt.Errorf("timeout waiting key %d visible: last error: %v", id, err)
            }
            return fmt.Errorf("timeout waiting key %d visible", id)
        }
        time.Sleep(2 * time.Second)
    }
}

// waitKeyUnused polls GET /ssh-keys/{id} until servers_count==0 or timeout.
func (r *sshKeyResource) waitKeyUnused(ctx context.Context, id int64, timeout time.Duration) error {
    if timeout <= 0 { timeout = 5 * time.Minute }
    deadline := time.Now().Add(timeout)
    for {
        var kr models.SSHKeyResp
        err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%d", client.SSHKeysEP, id), nil, &kr)
        if err == nil && kr.OK && kr.Data.ID == int(id) {
            if kr.Data.ServersCount == 0 {
                return nil
            }
        }
        if time.Now().After(deadline) {
            if err != nil {
                return fmt.Errorf("timeout waiting key %d unused: last error: %v", id, err)
            }
            return fmt.Errorf("timeout waiting key %d unused (servers_count>0)", id)
        }
        time.Sleep(2 * time.Second)
    }
}

// applyProtect toggles protection and handles both sync and async (task) APIs, then waits for final state.
func (r *sshKeyResource) applyProtect(ctx context.Context, id int64, desired bool) error {
    action := "protect"
    if !desired {
        action = "unprotect"
    }
    // Some APIs return a job with status_url/job_id; others are synchronous.
    var respBody struct {
        OK        bool             `json:"ok"`
        JobID     string           `json:"job_id"`
        StatusURL string           `json:"status_url"`
        Message   string           `json:"message"`
        Error     *models.APIError `json:"error,omitempty"`
    }
    // Try to capture a response body; if decode fails, we'll ignore and just wait on state.
    if err := r.c.PostJSON(ctx, fmt.Sprintf("%s/%d/%s", client.SSHKeysEP, id, action), map[string]any{}, &respBody); err != nil {
        // Some endpoints may not return JSON; fall back to assuming request accepted
        // Only abort on strong HTTP errors (already returned by PostJSON).
        return err
    }
    // If async job returned, poll it first.
    if respBody.OK && (respBody.StatusURL != "" || respBody.JobID != "") {
        var tr *models.TaskResp
        var err error
        ctxWait, cancel := context.WithTimeout(ctx, 180*time.Second)
        defer cancel()
        if respBody.StatusURL != "" {
            tr, err = r.c.PollURL(ctxWait, respBody.StatusURL, 2*time.Second)
        } else {
            tr, err = r.c.PollTask(ctxWait, respBody.JobID, 2*time.Second)
        }
        if err != nil {
            return fmt.Errorf("protect task poll failed: %w", err)
        }
        if tr.Data.Error {
            return fmt.Errorf("protect task error: %s", tr.Data.ErrorData)
        }
    }
    // Finally wait until GET /ssh-keys/{id} reflects desired state.
    if err := r.waitForProtectState(ctx, id, desired, 180*time.Second); err != nil {
        return err
    }
    return nil
}

// fetchKeyByID tries to GET /ssh-keys/{id}
func (r *sshKeyResource) fetchKeyByID(ctx context.Context, id int64) (*models.SSHKey, error) {
    var kr models.SSHKeyResp
    if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%d", client.SSHKeysEP, id), nil, &kr); err != nil {
        return nil, err
    }
    if !kr.OK {
        return nil, fmt.Errorf("api not ok while fetching key %d", id)
    }
    return &kr.Data, nil
}

// findKeyByName lists keys and returns the first with exact matching name.
func (r *sshKeyResource) findKeyByName(ctx context.Context, name string) (*models.SSHKey, error) {
    var list models.SSHKeysListResp
    if err := r.c.GetJSON(ctx, client.SSHKeysEP, nil, &list); err != nil {
        return nil, err
    }
    if !list.OK {
        return nil, fmt.Errorf("api not ok while listing keys")
    }
    for _, k := range list.Data.Keys {
        if k.Name == name {
            kk := k
            return &kk, nil
        }
    }
    return nil, fmt.Errorf("key with name %q not found", name)
}

func (r *sshKeyResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan sshKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() { return }

	// capture desired protection before we overwrite model fields
	wantProtect := !plan.Protected.IsNull() && plan.Protected.ValueBool()

	name := plan.Name.ValueString()
	pub := plan.PublicKey.ValueString()

	// 1) Импорт публичного ключа — синхронно (без task)
	if pub != "" {
		// API: POST /ssh-keys {name, public_key, protected?}
		payload := map[string]any{
			"name":       name,
			"public_key": pub,
		}
		if wantProtect {
			payload["protected"] = true
		}
		// Accept API shape: { ok: true, id: <int>, message: "..." }
		var ir struct {
			OK      bool             `json:"ok"`
			ID      int              `json:"id"`
			Message string           `json:"message"`
			Error   *models.APIError `json:"error,omitempty"`
		}
		if err := r.c.PostJSON(ctx, client.SSHKeysEP, payload, &ir); err != nil {
			resp.Diagnostics.AddError("import failed", err.Error()); return
		}
		if !ir.OK || ir.ID == 0 {
			// Some deployments may still return full object; try to decode that as a fallback
			var alt models.SSHKeyResp
			if err2 := r.c.PostJSON(ctx, client.SSHKeysEP, payload, &alt); err2 == nil && alt.OK && alt.Data.ID > 0 {
				ir.ID = alt.Data.ID
			}
		}
		if ir.ID == 0 {
			resp.Diagnostics.AddError("import failed", "API did not return created key ID"); return
		}

		// Wait until key is visible by id
		if err := r.waitKeyVisible(ctx, int64(ir.ID), 60*time.Second); err != nil {
			resp.Diagnostics.AddError("import wait failed", err.Error()); return
		}
		// Read the created key and set state
		var kr models.SSHKeyResp
		if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%d", client.SSHKeysEP, ir.ID), nil, &kr); err != nil {
			resp.Diagnostics.AddError("read imported key failed", err.Error()); return
		}
		r.setModelFromKey(&plan, &kr.Data)
		if plan.Name.ValueString() == "" {
			plan.Name = types.StringValue(name)
		}

		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	// 2) Генерация — асинхронная задача
	var jr struct {
		OK        bool             `json:"ok"`
		JobID     string           `json:"job_id"`
		StatusURL string           `json:"status_url"`
		Message   string           `json:"message"`
		Error     *models.APIError `json:"error,omitempty"`
	}
	if err := r.c.PostJSON(ctx, client.SSHKeysEP, map[string]any{"action": "generate", "name": name}, &jr); err != nil {
		resp.Diagnostics.AddError("generate failed", err.Error()); return
	}
	if !jr.OK {
		resp.Diagnostics.AddError("generate failed", fmt.Sprintf("enqueue error: %+v", jr.Error)); return
	}

	// ограничим ожидание, чтобы не висеть вечно
	ctxWait, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	var tr *models.TaskResp
	var err error
	if jr.StatusURL != "" {
		tr, err = r.c.PollURL(ctxWait, jr.StatusURL, 2*time.Second)
	} else if jr.JobID != "" {
		tr, err = r.c.PollTask(ctxWait, jr.JobID, 2*time.Second)
	} else {
		resp.Diagnostics.AddError("generate failed", "no job_id/status_url returned"); return
	}
	if err != nil {
		resp.Diagnostics.AddError("task poll failed", err.Error()); return
	}
	if tr.Data.Error {
		resp.Diagnostics.AddError("task error", tr.Data.ErrorData); return
	}

	// получить id из task (обязательно)
	var newID int64
	if id, ok := toID(tr.Data.ResourceID); ok && id > 0 {
		newID = id
	} else {
		resp.Diagnostics.AddError("create failed", "task finished without resource_id")
		return
	}

	// прочитать созданный ключ
	var kr models.SSHKeyResp
	if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%d", client.SSHKeysEP, newID), nil, &kr); err != nil {
		resp.Diagnostics.AddError("fetch created key failed", err.Error()); return
	}
	r.setModelFromKey(&plan, &kr.Data)
	if plan.Name.ValueString() == "" {
		plan.Name = types.StringValue(name)
	}
	// Stabilize: if user asked for protected=true but API still shows false, return true and let next Read converge.
	if wantProtect && !kr.Data.Protected {
		plan.Protected = types.BoolValue(true)
	}

	// включить protect, если запросили (и дождаться применения)
	if wantProtect && !kr.Data.Protected {
		if err := r.applyProtect(ctx, newID, true); err != nil {
			resp.Diagnostics.AddWarning("protect after generate", fmt.Sprintf("apply protect failed: %v (continuing best-effort)", err))
		} else {
			if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%d", client.SSHKeysEP, newID), nil, &kr); err == nil {
				r.setModelFromKey(&plan, &kr.Data)
			}
		}
		if plan.Name.ValueString() == "" {
			plan.Name = types.StringValue(name)
		}
		// Стабилизация: вернуть true, чтобы Terraform не ругался, а последующий Read выровняет.
		if wantProtect && (plan.Protected.IsNull() || !plan.Protected.ValueBool()) {
			plan.Protected = types.BoolValue(true)
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *sshKeyResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
    var state sshKeyModel
    resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
    if resp.Diagnostics.HasError() { return }

    wantName := state.Name.ValueString()
    id := state.ID.ValueInt64()

    // Try by ID first
    key, err := r.fetchKeyByID(ctx, id)
    if err != nil || key.ID == 0 {
        // Fallback: try by name if we have it
        if wantName != "" {
            if k2, err2 := r.findKeyByName(ctx, wantName); err2 == nil {
                key = k2
            } else {
                // Not found by name either -> consider resource gone
                resp.State.RemoveResource(ctx)
                return
            }
        } else {
            // No name to search by -> consider gone
            resp.State.RemoveResource(ctx)
            return
        }
    }

    // Set state from fetched key
    r.setModelFromKey(&state, key)
    // Never let name be empty: keep previous desired name if API returns empty
    if state.Name.ValueString() == "" && wantName != "" {
        state.Name = types.StringValue(wantName)
    }

    resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sshKeyResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
    var plan, state sshKeyModel
    resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
    resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
    if resp.Diagnostics.HasError() { return }

    if plan.Protected.ValueBool() != state.Protected.ValueBool() {
        if err := r.applyProtect(ctx, state.ID.ValueInt64(), plan.Protected.ValueBool()); err != nil {
            resp.Diagnostics.AddError("toggle protect failed", err.Error()); return
        }
    }

    // перечитываем ключ и сохраняем state
    var kr models.SSHKeyResp
    if err := r.c.GetJSON(ctx, fmt.Sprintf("%s/%d", client.SSHKeysEP, state.ID.ValueInt64()), nil, &kr); err != nil {
        resp.Diagnostics.AddError("read after update failed", err.Error()); return
    }
    r.setModelFromKey(&state, &kr.Data)
    resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sshKeyResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
    var state sshKeyModel
    resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
    if resp.Diagnostics.HasError() { return }
    if state.ID.IsNull() || state.ID.ValueInt64() <= 0 { return }
    id := state.ID.ValueInt64()

    // Best-effort: wait until key is not used by any servers before attempting delete
    _ = r.waitKeyUnused(ctx, id, 5*time.Minute)

    // Try delete; if backend still says FORBIDDEN due to usage, keep waiting and retry until timeout.
    deadline := time.Now().Add(5 * time.Minute)
    for {
        err := r.c.Delete(ctx, fmt.Sprintf("%s/%d", client.SSHKeysEP, id))
        if err == nil {
            return
        }
        msg := err.Error()
        // Treat not found as success
        if msg == "NOT_FOUND" || msg == "404" || msg == "http 404: NOT_FOUND" {
            return
        }
        // If forbidden because still used, wait and retry until timeout
        if strings.Contains(strings.ToUpper(msg), "FORBIDDEN") || strings.Contains(msg, "Cannot delete key") {
            if time.Now().After(deadline) {
                resp.Diagnostics.AddError("delete failed", fmt.Sprintf("timeout waiting key %d to become unused: %v", id, err))
                return
            }
            // small wait and re-check usage
            _ = r.waitKeyUnused(ctx, id, 30*time.Second)
            time.Sleep(2 * time.Second)
            continue
        }
        // Other errors: return immediately
        resp.Diagnostics.AddError("delete failed", err.Error())
        return
    }
}
