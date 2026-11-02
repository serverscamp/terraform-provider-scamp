package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

const storageTimeout = 20 * time.Minute

// Every box is one S3 bucket with this name, in this region, behind its own host.
const (
	storageBucket   = "storage"
	storageRegion   = "us-east-1"
	storageMainUser = "main"
)

var storageProtocols = []string{"s3", "sftp", "ftp", "webdav"}

type storageBoxResource struct {
	c *client.Client
}

func NewStorageBoxResource() tfresource.Resource { return &storageBoxResource{} }

type storageBoxModel struct {
	ID         types.String `tfsdk:"id"`
	Name       types.String `tfsdk:"name"`
	Package    types.String `tfsdk:"package"`
	GrowUpTo   types.String `tfsdk:"grow_up_to"`
	NetworkID  types.String `tfsdk:"network_id"`
	Protocols  types.Set    `tfsdk:"protocols"`
	Host       types.String `tfsdk:"host"`
	S3Endpoint types.String `tfsdk:"s3_endpoint"`
	Bucket     types.String `tfsdk:"bucket"`
	Region     types.String `tfsdk:"region"`
	User       types.String `tfsdk:"user"`
	Password   types.String `tfsdk:"password"`
	AccessKey  types.String `tfsdk:"access_key"`
	SecretKey  types.String `tfsdk:"secret_key"`
	IP         types.String `tfsdk:"ip"`
	PrivateIP  types.String `tfsdk:"private_ip"`
	State      types.String `tfsdk:"state"`
}

func (r *storageBoxResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_storage_box"
}

func (r *storageBoxResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	protoDefault := make([]attr.Value, len(storageProtocols))
	for i, p := range storageProtocols {
		protoDefault[i] = types.StringValue(p)
	}
	resp.Schema = rschema.Schema{
		Description: "Manages a Storage Box: one file tree on its own server, served over S3, SFTP, FTPS and WebDAV. Its S3 side works as a Terraform state backend, with the lock file (use_lockfile).",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true, Description: "The box's id.", PlanModifiers: keep,
			},
			"name": rschema.StringAttribute{
				Required:      true,
				Description:   "Name of the box: 3 to 32 lowercase letters, digits and hyphens, starting with a letter. Changing it replaces the box and every file in it.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"package": rschema.StringAttribute{
				Required:      true,
				Description:   "Package by slug or name: st-50 (50 GB) up to st-10000 (10 TB). The box moves up by itself only to grow_up_to; a package is never moved down, so changing it replaces the box.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"grow_up_to": rschema.StringAttribute{
				Optional:    true,
				Description: "A bigger package the box moves up to by itself when it is full. Changed in place.",
			},
			"network_id": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "UUID of the network, attached to a router. Without it the account's default network. Changing it replaces the box and every file in it.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"protocols": rschema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Default:     setdefault.StaticValue(types.SetValueMust(types.StringType, protoDefault)),
				Description: "Protocols the box serves: s3, sftp, ftp, webdav. All of them by default. Changed in place.",
			},
			"host": rschema.StringAttribute{
				Computed: true, Description: "The box's name in DNS, NAME-xxxxxx.storage.serverscamp.com.", PlanModifiers: keep,
			},
			"s3_endpoint": rschema.StringAttribute{
				Computed: true, Description: "S3 endpoint, https://HOST. Use path-style addressing.", PlanModifiers: keep,
			},
			"bucket": rschema.StringAttribute{
				Computed: true, Description: "The box's one S3 bucket: storage.", PlanModifiers: keep,
			},
			"region": rschema.StringAttribute{
				Computed: true, Description: "S3 region to sign with: us-east-1.", PlanModifiers: keep,
			},
			"user": rschema.StringAttribute{
				Computed: true, Description: "The box's main user, with access to everything.", PlanModifiers: keep,
			},
			"password": rschema.StringAttribute{
				Computed: true, Sensitive: true, Description: "Password of the main user for SFTP, FTPS and WebDAV.", PlanModifiers: keep,
			},
			"access_key": rschema.StringAttribute{
				Computed: true, Description: "S3 access key of the main user.", PlanModifiers: keep,
			},
			"secret_key": rschema.StringAttribute{
				Computed: true, Sensitive: true, Description: "S3 secret key of the main user.", PlanModifiers: keep,
			},
			"ip": rschema.StringAttribute{
				Computed: true, Description: "Public address of the box.",
			},
			"private_ip": rschema.StringAttribute{
				Computed: true, Description: "Address of the box in its network.",
			},
			"state": rschema.StringAttribute{
				Computed: true, Description: "creating, configuring, active or failed.",
			},
		},
	}
}

func (r *storageBoxResource) ValidateConfig(ctx context.Context, req tfresource.ValidateConfigRequest, resp *tfresource.ValidateConfigResponse) {
	var cfg storageBoxModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	checkServiceName(cfg.Name, &resp.Diagnostics)
}

func (r *storageBoxResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func (r *storageBoxResource) packageID(ctx context.Context, want string) (int64, error) {
	var cat struct {
		Packages []models.StoragePackage `json:"packages"`
	}
	if err := r.c.GetJSON(ctx, client.StorageEP+"/catalog", nil, &cat); err != nil {
		return 0, err
	}
	labels := make([][]string, len(cat.Packages))
	for i, p := range cat.Packages {
		labels[i] = []string{p.Slug, p.Name}
	}
	i, err := pickByLabels(want, labels)
	if err != nil {
		return 0, err
	}
	if cat.Packages[i].SoldOut {
		return 0, fmt.Errorf("package %s is out of capacity right now", cat.Packages[i].Slug)
	}
	return cat.Packages[i].ID, nil
}

func (r *storageBoxResource) get(ctx context.Context, id string) (*models.StorageBox, error) {
	var b models.StorageBox
	err := r.c.GetJSON(ctx, client.StorageEP+"/"+id, nil, &b)
	return &b, err
}

// protocolKeys reads the protocols the box serves, as keys or as objects with a key.
func protocolKeys(in []any) []string {
	var out []string
	for _, p := range in {
		switch v := p.(type) {
		case string:
			out = append(out, v)
		case map[string]any:
			if k, ok := v["key"].(string); ok {
				out = append(out, k)
			}
		}
	}
	sort.Strings(out)
	return out
}

func setFromBox(m *storageBoxModel, b *models.StorageBox) {
	m.ID = types.StringValue(b.ID)
	m.Name = types.StringValue(b.Name)
	m.NetworkID = types.StringValue(b.Network)
	m.Host = types.StringValue(b.Host)
	m.S3Endpoint = types.StringValue("https://" + b.Host)
	m.Bucket = types.StringValue(storageBucket)
	m.Region = types.StringValue(storageRegion)
	m.User = types.StringValue(storageMainUser)
	m.IP = strOrNull(b.IP)
	m.PrivateIP = strOrNull(b.PrivateIP)
	m.State = types.StringValue(b.State)
	same := func(cur types.String, p *models.StoragePackage) bool {
		return !cur.IsNull() && !cur.IsUnknown() && p != nil && (norm(cur.ValueString()) == norm(p.Slug) || norm(cur.ValueString()) == norm(p.Name))
	}
	// the box moves up by itself: the package it started on stays as written, a move up is not a change
	if b.Package != nil && (m.Package.IsNull() || m.Package.IsUnknown()) {
		m.Package = types.StringValue(b.Package.Slug)
	}
	if b.GrowUpTo == nil {
		m.GrowUpTo = types.StringNull()
	} else if !same(m.GrowUpTo, b.GrowUpTo) {
		m.GrowUpTo = types.StringValue(b.GrowUpTo.Slug)
	}
	if keys := protocolKeys(b.Protocols); len(keys) > 0 {
		vals := make([]attr.Value, len(keys))
		for i, k := range keys {
			vals[i] = types.StringValue(k)
		}
		m.Protocols = types.SetValueMust(types.StringType, vals)
	}
}

func (r *storageBoxResource) login(ctx context.Context, m *storageBoxModel) {
	var l models.StorageLogin
	if err := r.c.GetJSON(ctx, client.StorageEP+"/"+m.ID.ValueString()+"/users/"+storageMainUser+"/login", nil, &l); err != nil {
		tflog.Debug(ctx, "storage login not read", map[string]any{"error": err.Error()})
		return
	}
	if l.Password != nil {
		m.Password = types.StringValue(*l.Password)
	}
	if l.AccessKey != nil {
		m.AccessKey = types.StringValue(*l.AccessKey)
	}
	if l.S3Secret != nil {
		m.SecretKey = types.StringValue(*l.S3Secret)
	}
}

func (r *storageBoxResource) waitActive(ctx context.Context, id string) (*models.StorageBox, error) {
	deadline := time.Now().Add(storageTimeout)
	for {
		b, err := r.get(ctx, id)
		if err == nil {
			switch b.State {
			case "active":
				return b, nil
			case "failed":
				msg := "the box could not be created"
				if b.Error != nil {
					msg = *b.Error
				}
				return b, fmt.Errorf("%s", msg)
			}
		} else {
			tflog.Debug(ctx, "storage read failed while waiting", map[string]any{"error": err.Error()})
		}
		if time.Now().After(deadline) {
			return b, fmt.Errorf("the box did not become active within %s", storageTimeout)
		}
		select {
		case <-ctx.Done():
			return b, ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}

func (r *storageBoxResource) protocols(ctx context.Context, m *storageBoxModel) ([]string, error) {
	var ps []string
	if d := m.Protocols.ElementsAs(ctx, &ps, false); d.HasError() {
		return nil, fmt.Errorf("protocols: %v", d)
	}
	for _, p := range ps {
		ok := false
		for _, s := range storageProtocols {
			ok = ok || p == s
		}
		if !ok {
			return nil, fmt.Errorf("unknown protocol %q; available: %s", p, strings.Join(storageProtocols, ", "))
		}
	}
	if len(ps) == 0 {
		return nil, fmt.Errorf("pick at least one protocol")
	}
	return ps, nil
}

func (r *storageBoxResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan storageBoxModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pid, err := r.packageID(ctx, plan.Package.ValueString())
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("package"), "Package not available", err.Error())
		return
	}
	protos, err := r.protocols(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("protocols"), "Bad protocols", err.Error())
		return
	}
	network, err := networkOrDefault(ctx, r.c, plan.NetworkID)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("network_id"), "No network", err.Error())
		return
	}
	suffix := make([]byte, 3)
	_, _ = rand.Read(suffix)
	payload := map[string]any{
		"name":         plan.Name.ValueString(),
		"suffix":       hex.EncodeToString(suffix),
		"package_id":   pid,
		"network_uuid": network,
		"protocols":    protos,
	}
	if !plan.GrowUpTo.IsNull() && plan.GrowUpTo.ValueString() != "" {
		gid, err := r.packageID(ctx, plan.GrowUpTo.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("grow_up_to"), "Package not available", err.Error())
			return
		}
		payload["grow_up_to_package_id"] = gid
	}
	var created struct {
		ID   string `json:"id"`
		Host string `json:"host"`
	}
	if err := r.c.PostJSON(ctx, client.StorageEP, payload, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create the storage box", err.Error())
		return
	}
	plan.ID = types.StringValue(created.ID)
	plan.NetworkID = types.StringValue(network)
	plan.Host = types.StringValue(created.Host)
	plan.S3Endpoint = types.StringValue("https://" + created.Host)
	plan.Bucket, plan.Region, plan.User = types.StringValue(storageBucket), types.StringValue(storageRegion), types.StringValue(storageMainUser)
	plan.Password, plan.AccessKey, plan.SecretKey = types.StringNull(), types.StringNull(), types.StringNull()
	plan.IP, plan.PrivateIP = types.StringNull(), types.StringNull()
	plan.State = types.StringValue("creating")
	// saved before the wait, so a box that fails to come up is still in the state and destroy removes it
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	b, err := r.waitActive(ctx, created.ID)
	if b != nil && b.ID != "" {
		setFromBox(&plan, b)
		r.login(ctx, &plan)
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("The storage box did not come up", err.Error())
	}
}

func (r *storageBoxResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state storageBoxModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	b, err := r.get(ctx, state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "http 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the storage box", err.Error())
		return
	}
	setFromBox(&state, b)
	r.login(ctx, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *storageBoxResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan, state storageBoxModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	if !plan.GrowUpTo.Equal(state.GrowUpTo) {
		body := map[string]any{"grow_up_to_package_id": nil}
		if !plan.GrowUpTo.IsNull() && plan.GrowUpTo.ValueString() != "" {
			gid, err := r.packageID(ctx, plan.GrowUpTo.ValueString())
			if err != nil {
				resp.Diagnostics.AddAttributeError(path.Root("grow_up_to"), "Package not available", err.Error())
				return
			}
			body["grow_up_to_package_id"] = gid
		}
		if err := r.c.PutJSON(ctx, client.StorageEP+"/"+id+"/grow", body, nil); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("grow_up_to"), "Failed to change grow_up_to", err.Error())
			return
		}
	}
	if !plan.Protocols.Equal(state.Protocols) {
		protos, err := r.protocols(ctx, &plan)
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("protocols"), "Bad protocols", err.Error())
			return
		}
		if err := r.c.PutJSON(ctx, client.StorageEP+"/"+id+"/protocols", map[string]any{"protocols": protos}, nil); err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("protocols"), "Failed to change the protocols", err.Error())
			return
		}
	}
	b, err := r.waitActive(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("The storage box is not active after the change", err.Error())
		return
	}
	plan.Password, plan.AccessKey, plan.SecretKey = state.Password, state.AccessKey, state.SecretKey
	setFromBox(&plan, b)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *storageBoxResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state storageBoxModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	deadline := time.Now().Add(storageTimeout)
	// a box still being made cannot be deleted yet: wait for it
	for {
		err := r.c.Delete(ctx, client.StorageEP+"/"+id)
		if err == nil || strings.Contains(err.Error(), "http 404") {
			return
		}
		if !strings.Contains(err.Error(), "http 409") || time.Now().After(deadline) {
			resp.Diagnostics.AddError("Failed to delete the storage box", err.Error())
			return
		}
		time.Sleep(10 * time.Second)
	}
}

func (r *storageBoxResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	tfresource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
