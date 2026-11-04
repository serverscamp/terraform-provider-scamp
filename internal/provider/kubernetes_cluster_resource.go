package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	tfresource "github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

// A cluster takes a few minutes; a pool of big nodes or a slow drain takes longer.
const k8sTimeout = 45 * time.Minute

// The API's limits on pools (public-api services/kubernetes.py).
const (
	k8sMaxPools   = 5
	k8sMaxPodsMin = 8
	k8sMaxPodsMax = 250
)

type kubernetesClusterResource struct {
	c *client.Client
}

func NewKubernetesClusterResource() tfresource.Resource { return &kubernetesClusterResource{} }

type k8sClusterModel struct {
	ID            types.String   `tfsdk:"id"`
	Name          types.String   `tfsdk:"name"`
	Version       types.String   `tfsdk:"version"`
	NetworkID     types.String   `tfsdk:"network_id"`
	StorageBoxID  types.String   `tfsdk:"storage_box_id"`
	NodePools     []k8sPoolModel `tfsdk:"node_pool"`
	Hostname      types.String   `tfsdk:"hostname"`
	APIURL        types.String   `tfsdk:"api_url"`
	APIPrivateURL types.String   `tfsdk:"api_private_url"`
	Kubeconfig    types.String   `tfsdk:"kubeconfig"`
	State         types.String   `tfsdk:"state"`
}

type k8sPoolModel struct {
	Name      types.String `tfsdk:"name"`
	Plan      types.String `tfsdk:"plan"`
	NodeCount types.Int64  `tfsdk:"node_count"`
	MaxPods   types.Int64  `tfsdk:"max_pods"`
	Labels    types.Map    `tfsdk:"labels"`
}

func (r *kubernetesClusterResource) Metadata(_ context.Context, _ tfresource.MetadataRequest, resp *tfresource.MetadataResponse) {
	resp.TypeName = "scamp_kubernetes_cluster"
}

func (r *kubernetesClusterResource) Schema(_ context.Context, _ tfresource.SchemaRequest, resp *tfresource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = rschema.Schema{
		Description: "Manages a Kubernetes cluster: a control plane run by ServersCamp, free, and node pools of servers in your private network. The API has its own name under the cluster domain with a certificate we renew. Nodes have no public address; services reach the internet through a load balancer.",
		Attributes: map[string]rschema.Attribute{
			"id": rschema.StringAttribute{
				Computed: true, Description: "The cluster's id.", PlanModifiers: keep,
			},
			"name": rschema.StringAttribute{
				Required:      true,
				Description:   "Name of the cluster: 3 to 32 lowercase letters, digits and hyphens, starting with a letter. It is the start of the API address. Changing it replaces the cluster.",
				PlanModifiers: replace,
			},
			"version": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Kubernetes version, e.g. 1.36. Without it the default version. Changing it replaces the cluster.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"network_id": rschema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "UUID of the network, attached to a router. Without it the account's default network. Changing it replaces the cluster.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown(), stringplanmodifier.RequiresReplace()},
			},
			"storage_box_id": rschema.StringAttribute{
				Optional:      true,
				Description:   "UUID of a Storage Box on the same router, shared by the pods as a ReadWriteMany storage class. Changing it replaces the cluster.",
				PlanModifiers: replace,
			},
			"node_pool": rschema.ListNestedAttribute{
				Required:    true,
				Description: "Node pools, 1 to 5. A pool grows, shrinks and changes its pods per node and labels in place; shrinking drains and deletes the newest nodes. A pool keeps its plan: to change it, add a pool under a new name and remove the old one.",
				NestedObject: rschema.NestedAttributeObject{
					Attributes: map[string]rschema.Attribute{
						"name": rschema.StringAttribute{
							Required:    true,
							Description: "Name of the pool, same rule as the cluster name. Nodes are named after it.",
						},
						"plan": rschema.StringAttribute{
							Required:    true,
							Description: "Server plan of every node, e.g. basic-4g. 2 GB of memory or more.",
						},
						"node_count": rschema.Int64Attribute{
							Required:    true,
							Description: "Number of nodes, 1 or more.",
						},
						"max_pods": rschema.Int64Attribute{
							Optional:    true,
							Description: "Pods a node takes, 8 to 250. Without it by the node's memory: 16 a GB, 110 at most. A change restarts the kubelet of the nodes; running pods stay.",
						},
						"labels": rschema.MapAttribute{
							ElementType: types.StringType,
							Optional:    true,
							Description: "Node labels, on every node of the pool. Pods already placed stay where they are.",
						},
					},
				},
			},
			"hostname": rschema.StringAttribute{
				Computed: true, Description: "The API's name.", PlanModifiers: keep,
			},
			"api_url": rschema.StringAttribute{
				Computed: true, Description: "https:// address of the Kubernetes API.",
			},
			"api_private_url": rschema.StringAttribute{
				Computed: true, Description: "Address of the Kubernetes API in the private network.",
			},
			"kubeconfig": rschema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				Description:   "An admin kubeconfig, made once when the cluster is ready, with a client certificate for a year.",
				PlanModifiers: keep,
			},
			"state": rschema.StringAttribute{
				Computed: true, Description: "active once the cluster runs, failed or deleting; anything else while it is being made.",
			},
		},
	}
}

func (r *kubernetesClusterResource) Configure(_ context.Context, req tfresource.ConfigureRequest, _ *tfresource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	r.c = req.ProviderData.(*client.Client)
}

func (r *kubernetesClusterResource) ValidateConfig(ctx context.Context, req tfresource.ValidateConfigRequest, resp *tfresource.ValidateConfigResponse) {
	var cfg k8sClusterModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	if resp.Diagnostics.HasError() {
		return
	}
	checkServiceName(cfg.Name, &resp.Diagnostics)
	if len(cfg.NodePools) > k8sMaxPools {
		resp.Diagnostics.AddAttributeError(path.Root("node_pool"), "Too many node pools",
			fmt.Sprintf("A cluster has up to %d node pools.", k8sMaxPools))
	}
	seen := map[string]bool{}
	for i, p := range cfg.NodePools {
		at := path.Root("node_pool").AtListIndex(i)
		if v := p.Name; !v.IsNull() && !v.IsUnknown() {
			if !serviceName.MatchString(v.ValueString()) {
				resp.Diagnostics.AddAttributeError(at.AtName("name"), "Bad pool name", fmt.Sprintf("%q: %s", v.ValueString(), serviceNameRule))
			}
			if seen[v.ValueString()] {
				resp.Diagnostics.AddAttributeError(at.AtName("name"), "Two pools with one name", fmt.Sprintf("Two pools are named %s.", v.ValueString()))
			}
			seen[v.ValueString()] = true
		}
		if v := p.NodeCount; !v.IsNull() && !v.IsUnknown() && v.ValueInt64() < 1 {
			resp.Diagnostics.AddAttributeError(at.AtName("node_count"), "Bad node count", "A pool has 1 node or more.")
		}
		if v := p.MaxPods; !v.IsNull() && !v.IsUnknown() && (v.ValueInt64() < k8sMaxPodsMin || v.ValueInt64() > k8sMaxPodsMax) {
			resp.Diagnostics.AddAttributeError(at.AtName("max_pods"), "Bad max_pods",
				fmt.Sprintf("Pods per node: %d to %d.", k8sMaxPodsMin, k8sMaxPodsMax))
		}
	}
}

// ModifyPlan stops a plan that changes the plan of an existing pool: the API keeps a pool's node size.
func (r *kubernetesClusterResource) ModifyPlan(ctx context.Context, req tfresource.ModifyPlanRequest, resp *tfresource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var plan, state k8sClusterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	was := map[string]string{}
	for _, p := range state.NodePools {
		was[p.Name.ValueString()] = p.Plan.ValueString()
	}
	for i, p := range plan.NodePools {
		if p.Name.IsUnknown() || p.Plan.IsUnknown() {
			continue
		}
		if old, ok := was[p.Name.ValueString()]; ok && norm(old) != norm(p.Plan.ValueString()) {
			resp.Diagnostics.AddAttributeError(path.Root("node_pool").AtListIndex(i).AtName("plan"), "A pool keeps its plan",
				fmt.Sprintf("Pool %s runs on %s. To move it to %s, add a pool under a new name with that plan and remove this one.",
					p.Name.ValueString(), old, p.Plan.ValueString()))
		}
	}
}

func (r *kubernetesClusterResource) get(ctx context.Context, id string) (*models.K8sCluster, error) {
	var c models.K8sCluster
	err := r.c.GetJSON(ctx, client.KubernetesEP+"/"+id, nil, &c)
	return &c, err
}

// catalog gives the size id of every plan nodes can run on, by name, and the default version.
func (r *kubernetesClusterResource) catalog(ctx context.Context) (map[string]models.AppSize, []string, string, error) {
	var cat models.K8sCatalog
	if err := r.c.GetJSON(ctx, client.KubernetesEP+"/catalog", nil, &cat); err != nil {
		return nil, nil, "", err
	}
	sizes := map[string]models.AppSize{}
	var names []string
	for _, s := range cat.Sizes {
		sizes[norm(s.Name)] = s.AppSize
		names = append(names, s.Name)
	}
	def := ""
	for _, v := range cat.Versions {
		if v.Default || def == "" {
			def = v.Version
		}
	}
	return sizes, names, def, nil
}

func sizeFor(sizes map[string]models.AppSize, names []string, plan string) (int64, error) {
	s, ok := sizes[norm(plan)]
	switch {
	case !ok:
		return 0, fmt.Errorf("plan %q is not one nodes run on; available: %s", plan, strings.Join(names, ", "))
	case s.Locked:
		return 0, fmt.Errorf("plan %s is above the limits of your account", s.Name)
	case s.SoldOut:
		return 0, fmt.Errorf("plan %s is out of capacity right now", s.Name)
	}
	return s.ID, nil
}

func poolLabels(ctx context.Context, p k8sPoolModel) map[string]string {
	out := map[string]string{}
	if !p.Labels.IsNull() && !p.Labels.IsUnknown() {
		_ = p.Labels.ElementsAs(ctx, &out, false)
	}
	return out
}

func poolMaxPods(p k8sPoolModel) any {
	if p.MaxPods.IsNull() || p.MaxPods.IsUnknown() {
		return nil
	}
	return p.MaxPods.ValueInt64()
}

// setFromCluster fills the model from the API. Pools keep the order and the plan spelling of the config.
func setFromCluster(m *k8sClusterModel, c *models.K8sCluster) {
	m.ID = types.StringValue(c.ID)
	m.Name = types.StringValue(c.Name)
	m.Version = types.StringValue(c.Version)
	m.NetworkID = types.StringValue(c.NetworkUUID)
	if c.Storage != nil && c.Storage.ID != "" {
		m.StorageBoxID = types.StringValue(c.Storage.ID)
	}
	m.Hostname = types.StringValue(c.Hostname)
	m.APIURL = strOrNull(c.APIURL)
	m.APIPrivateURL = strOrNull(c.APIPrivateURL)
	m.State = types.StringValue(c.State)

	byName := map[string]models.K8sPool{}
	for _, p := range c.Pools {
		byName[p.Name] = p
	}
	var pools []k8sPoolModel
	done := map[string]bool{}
	for _, old := range m.NodePools {
		if p, ok := byName[old.Name.ValueString()]; ok {
			pools = append(pools, poolFromAPI(old, p))
			done[p.Name] = true
		}
	}
	for _, p := range c.Pools {
		if !done[p.Name] {
			pools = append(pools, poolFromAPI(k8sPoolModel{Labels: types.MapNull(types.StringType)}, p))
		}
	}
	m.NodePools = pools
}

func poolFromAPI(old k8sPoolModel, p models.K8sPool) k8sPoolModel {
	out := k8sPoolModel{Name: types.StringValue(p.Name), NodeCount: types.Int64Value(p.Count), Plan: old.Plan}
	if p.Plan != nil && (old.Plan.IsNull() || old.Plan.IsUnknown() || norm(old.Plan.ValueString()) != norm(*p.Plan)) {
		out.Plan = types.StringValue(*p.Plan)
	}
	out.MaxPods = types.Int64Null()
	if p.MaxPodsSet {
		out.MaxPods = types.Int64Value(p.MaxPods)
	}
	if len(p.Labels) == 0 && (old.Labels.IsNull() || old.Labels.IsUnknown() || len(old.Labels.Elements()) == 0) {
		// no labels reads back as the config wrote it: unset or {}
		out.Labels = old.Labels
		if old.Labels.IsUnknown() {
			out.Labels = types.MapNull(types.StringType)
		}
	} else {
		vals := map[string]attr.Value{}
		for k, v := range p.Labels {
			vals[k] = types.StringValue(v)
		}
		out.Labels = types.MapValueMust(types.StringType, vals)
	}
	return out
}

// waitSettled polls until the cluster runs, and with nodes also until every node of its pools is ready.
// A failed cluster stops the wait.
func (r *kubernetesClusterResource) waitSettled(ctx context.Context, id string, nodes bool) (*models.K8sCluster, error) {
	deadline := time.Now().Add(k8sTimeout)
	for {
		c, err := r.get(ctx, id)
		if err == nil {
			if c.State == "failed" {
				msg := "the cluster could not be created"
				if c.Error != nil && *c.Error != "" {
					msg = *c.Error
				}
				return c, fmt.Errorf("%s", msg)
			}
			if c.State == "active" && (!nodes || settled(c)) {
				return c, nil
			}
		} else {
			tflog.Debug(ctx, "cluster read failed while waiting", map[string]any{"error": err.Error()})
		}
		if time.Now().After(deadline) {
			return c, fmt.Errorf("the cluster did not settle within %s", k8sTimeout)
		}
		select {
		case <-ctx.Done():
			return c, ctx.Err()
		case <-time.After(15 * time.Second):
		}
	}
}

// settled: every pool has its nodes, all ready, and no node of a removed pool is left.
func settled(c *models.K8sCluster) bool {
	want := map[string]int64{}
	for _, p := range c.Pools {
		want[p.Name] = p.Count
	}
	have := map[string]int64{}
	for _, n := range c.NodeList {
		if n.State != "ready" {
			return false
		}
		if _, ok := want[n.Pool]; !ok {
			return false
		}
		have[n.Pool]++
	}
	for name, n := range want {
		if have[name] != n {
			return false
		}
	}
	return true
}

// kubeconfig is fetched once: every download makes a new client certificate.
func (r *kubernetesClusterResource) kubeconfig(ctx context.Context, m *k8sClusterModel) {
	kc, err := r.c.GetText(ctx, client.KubernetesEP+"/"+m.ID.ValueString()+"/kubeconfig")
	if err != nil {
		tflog.Debug(ctx, "kubeconfig not read", map[string]any{"error": err.Error()})
		return
	}
	m.Kubeconfig = types.StringValue(kc)
}

func (r *kubernetesClusterResource) Create(ctx context.Context, req tfresource.CreateRequest, resp *tfresource.CreateResponse) {
	var plan k8sClusterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sizes, names, def, err := r.catalog(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the Kubernetes catalog", err.Error())
		return
	}
	var pools []map[string]any
	for i, p := range plan.NodePools {
		id, err := sizeFor(sizes, names, p.Plan.ValueString())
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("node_pool").AtListIndex(i).AtName("plan"), "Plan not available", err.Error())
			return
		}
		pools = append(pools, map[string]any{
			"name": p.Name.ValueString(), "vm_class_id": id, "node_count": p.NodeCount.ValueInt64(),
			"max_pods": poolMaxPods(p), "labels": poolLabels(ctx, p),
		})
	}
	network, err := networkOrDefault(ctx, r.c, plan.NetworkID)
	if err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("network_id"), "No network", err.Error())
		return
	}
	plan.NetworkID = types.StringValue(network)
	if plan.Version.IsNull() || plan.Version.IsUnknown() || plan.Version.ValueString() == "" {
		plan.Version = types.StringValue(def)
	}
	payload := map[string]any{
		"name":         plan.Name.ValueString(),
		"slug":         appSlug(),
		"version":      plan.Version.ValueString(),
		"pools":        pools,
		"network_uuid": network,
	}
	if !plan.StorageBoxID.IsNull() && plan.StorageBoxID.ValueString() != "" {
		payload["storage_box_id"] = plan.StorageBoxID.ValueString()
	}
	var created struct {
		ID   string `json:"id"`
		Host string `json:"host"`
	}
	if err := r.c.PostJSON(ctx, client.KubernetesEP, payload, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create the cluster", err.Error())
		return
	}

	plan.ID = types.StringValue(created.ID)
	plan.Hostname = types.StringValue(created.Host)
	plan.State = types.StringValue("provisioning")
	plan.APIURL, plan.APIPrivateURL, plan.Kubeconfig = types.StringNull(), types.StringNull(), types.StringNull()
	// saved before the wait, so a cluster that fails to come up is still in the state and destroy removes it
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	c, err := r.waitSettled(ctx, created.ID, true)
	if c != nil && c.ID != "" {
		setFromCluster(&plan, c)
		if c.State == "active" {
			r.kubeconfig(ctx, &plan)
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
	if err != nil {
		resp.Diagnostics.AddError("The cluster did not come up", err.Error())
	}
}

func (r *kubernetesClusterResource) Read(ctx context.Context, req tfresource.ReadRequest, resp *tfresource.ReadResponse) {
	var state k8sClusterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	c, err := r.get(ctx, state.ID.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "http 404") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the cluster", err.Error())
		return
	}
	if c.State == "deleting" {
		resp.State.RemoveResource(ctx)
		return
	}
	setFromCluster(&state, c)
	if state.Kubeconfig.IsNull() && c.State == "active" {
		r.kubeconfig(ctx, &state)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update changes the node pools: new ones are added first, so a pool can be swapped for another under a new name
// without the cluster ever going below one pool; then the changed ones; then the removed ones.
func (r *kubernetesClusterResource) Update(ctx context.Context, req tfresource.UpdateRequest, resp *tfresource.UpdateResponse) {
	var plan, state k8sClusterModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	ep := client.KubernetesEP + "/" + id + "/pools"
	// pool changes are taken only while the cluster runs
	if _, err := r.waitSettled(ctx, id, false); err != nil {
		resp.Diagnostics.AddError("The cluster is not running", err.Error())
		return
	}
	was := map[string]k8sPoolModel{}
	for _, p := range state.NodePools {
		was[p.Name.ValueString()] = p
	}
	keep := map[string]bool{}
	var sizes map[string]models.AppSize
	var names []string
	for i, p := range plan.NodePools {
		name := p.Name.ValueString()
		keep[name] = true
		at := path.Root("node_pool").AtListIndex(i)
		old, ok := was[name]
		if !ok {
			if sizes == nil {
				var err error
				if sizes, names, _, err = r.catalog(ctx); err != nil {
					resp.Diagnostics.AddError("Failed to read the Kubernetes catalog", err.Error())
					return
				}
			}
			sid, err := sizeFor(sizes, names, p.Plan.ValueString())
			if err != nil {
				resp.Diagnostics.AddAttributeError(at.AtName("plan"), "Plan not available", err.Error())
				return
			}
			if err := r.c.PostJSON(ctx, ep, map[string]any{
				"name": name, "vm_class_id": sid, "node_count": p.NodeCount.ValueInt64(),
				"max_pods": poolMaxPods(p), "labels": poolLabels(ctx, p),
			}, nil); err != nil {
				resp.Diagnostics.AddAttributeError(at, "Failed to add the node pool", err.Error())
				return
			}
			continue
		}
		change := map[string]any{}
		if p.NodeCount.ValueInt64() != old.NodeCount.ValueInt64() {
			change["node_count"] = p.NodeCount.ValueInt64()
		}
		if !p.MaxPods.Equal(old.MaxPods) {
			change["max_pods"] = poolMaxPods(p)
		}
		if nl, ol := poolLabels(ctx, p), poolLabels(ctx, old); fmt.Sprint(nl) != fmt.Sprint(ol) {
			change["labels"] = nl
		}
		if len(change) == 0 {
			continue
		}
		if err := r.c.PatchJSON(ctx, ep+"/"+name, change, nil); err != nil {
			resp.Diagnostics.AddAttributeError(at, "Failed to change the node pool", err.Error())
			return
		}
	}
	for name := range was {
		if keep[name] {
			continue
		}
		if err := r.c.Delete(ctx, ep+"/"+name); err != nil && !strings.Contains(err.Error(), "http 404") {
			resp.Diagnostics.AddError("Failed to delete the node pool "+name, err.Error())
			return
		}
	}

	c, err := r.waitSettled(ctx, id, true)
	plan.Kubeconfig = state.Kubeconfig
	if c != nil && c.ID != "" {
		setFromCluster(&plan, c)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if err != nil {
		resp.Diagnostics.AddError("The node pools did not settle", err.Error())
	}
}

func (r *kubernetesClusterResource) Delete(ctx context.Context, req tfresource.DeleteRequest, resp *tfresource.DeleteResponse) {
	var state k8sClusterModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	deadline := time.Now().Add(k8sTimeout)
	// a cluster busy with a change may refuse the delete for a while: try again
	for {
		err := r.c.Delete(ctx, client.KubernetesEP+"/"+id)
		if err == nil || strings.Contains(err.Error(), "http 404") {
			break
		}
		if !strings.Contains(err.Error(), "http 409") || time.Now().After(deadline) {
			resp.Diagnostics.AddError("Failed to delete the cluster", err.Error())
			return
		}
		time.Sleep(10 * time.Second)
	}
	// the cluster reads back as deleting until its servers are gone
	for {
		_, err := r.get(ctx, id)
		if err != nil && strings.Contains(err.Error(), "http 404") {
			return
		}
		if time.Now().After(deadline) {
			resp.Diagnostics.AddError("The cluster is still there", fmt.Sprintf("%s still reads back after %s", id, k8sTimeout))
			return
		}
		time.Sleep(10 * time.Second)
	}
}

// ImportState takes the cluster's id. The kubeconfig is made on the first read.
func (r *kubernetesClusterResource) ImportState(ctx context.Context, req tfresource.ImportStateRequest, resp *tfresource.ImportStateResponse) {
	tfresource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
