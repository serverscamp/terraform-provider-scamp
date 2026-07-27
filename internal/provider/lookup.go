package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

// Catalogue lookups by name.
//
// Every one of these exists so a config can say what it means - "bs-burst-xs",
// "R1", "Ubuntu 26" - instead of an integer that means nothing on review and
// changes between environments. The alternative was a data source per lookup,
// which buries a three-line VM in twenty lines of plumbing.
//
// All of them fail with the list of valid values, because a wrong name is a
// typo nine times out of ten and the fix should be in the error message.

func norm(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// pickByLabels returns the one entry that answers to `want`. Each entry brings
// every name it is known by (a template has a slug and a display name), and a
// match on any of them counts. Exact wins; a unique substring is accepted so
// "ubuntu 26" finds "ubuntu-26.04"; several substring hits are an error rather
// than a coin toss. Everything is compared lowercased.
func pickByLabels(want string, labels [][]string) (int, error) {
	w := norm(want)

	for i, set := range labels {
		for _, l := range set {
			if norm(l) == w {
				return i, nil
			}
		}
	}

	var hits []int
	for i, set := range labels {
		for _, l := range set {
			if strings.Contains(norm(l), w) {
				hits = append(hits, i)
				break
			}
		}
	}

	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return -1, fmt.Errorf("no match for %q; available: %s", want, strings.Join(primaryLabels(labels), ", "))
	default:
		var amb []string
		for _, i := range hits {
			amb = append(amb, labels[i][0])
		}
		sort.Strings(amb)
		return -1, fmt.Errorf("%q matches more than one entry: %s - use the full name", want, strings.Join(amb, ", "))
	}
}

// primaryLabels is the first label of each entry, for error messages.
func primaryLabels(labels [][]string) []string {
	out := make([]string, 0, len(labels))
	for _, set := range labels {
		if len(set) > 0 {
			out = append(out, set[0])
		}
	}
	sort.Strings(out)
	return out
}

// pickByName is the single-label case.
func pickByName(want string, names []string) (int, error) {
	labels := make([][]string, len(names))
	for i, n := range names {
		labels[i] = []string{n}
	}
	return pickByLabels(want, labels)
}

func resolveVMClass(ctx context.Context, c *client.Client, name string) (int, error) {
	var list models.VMClassesListResponse
	if err := c.GetJSON(ctx, client.VMClassesEP, nil, &list); err != nil {
		return 0, fmt.Errorf("could not read VM classes: %w", err)
	}
	names := make([]string, len(list.Items))
	for i, it := range list.Items {
		names[i] = it.Name
	}
	idx, err := pickByName(name, names)
	if err != nil {
		return 0, fmt.Errorf("vm_class: %w", err)
	}
	return list.Items[idx].ID, nil
}

func resolveStorageClass(ctx context.Context, c *client.Client, name string) (int, error) {
	var list models.StorageClassesListResponse
	if err := c.GetJSON(ctx, client.StorageClassesEP, nil, &list); err != nil {
		return 0, fmt.Errorf("could not read storage classes: %w", err)
	}
	names := make([]string, len(list.Items))
	for i, it := range list.Items {
		names[i] = it.Name
	}
	idx, err := pickByName(name, names)
	if err != nil {
		return 0, fmt.Errorf("root_disk_class: %w", err)
	}
	return list.Items[idx].ID, nil
}

func resolveTemplate(ctx context.Context, c *client.Client, name string) (int, error) {
	var list models.VMTemplatesListResponse
	if err := c.GetJSON(ctx, client.VMTemplatesEP, nil, &list); err != nil {
		return 0, fmt.Errorf("could not read VM templates: %w", err)
	}
	labels := make([][]string, len(list.Items))
	for i, it := range list.Items {
		// Slug first: it is the stable identifier and what the error messages
		// should teach people to write. The API prefixes it with the storage
		// path ("templates/ubuntu-26.04"), which nobody should have to type,
		// so the trimmed form is accepted as well.
		slug := strings.TrimPrefix(it.APIName, "templates/")
		labels[i] = []string{slug, it.APIName, it.Name}
	}
	idx, err := pickByLabels(name, labels)
	if err != nil {
		return 0, fmt.Errorf("image: %w", err)
	}
	return list.Items[idx].ID, nil
}

// defaultNetworkName is the network every organisation is provisioned with and
// cannot delete from the panel. The API has no "is default" flag, so the name
// is the only marker there is - hence the fallbacks below.
const defaultNetworkName = "default-network"

// resolveDefaultNetwork finds the network to use when the config does not name
// one: the default network if it is there, otherwise the only network the
// organisation has. Anything else is ambiguous and says so.
func resolveDefaultNetwork(ctx context.Context, c *client.Client) (string, error) {
	var list models.NetworksListResponse
	if err := c.GetJSON(ctx, client.NetworksEP, nil, &list); err != nil {
		return "", fmt.Errorf("could not read networks: %w", err)
	}
	if len(list.Items) == 0 {
		return "", fmt.Errorf("this organisation has no networks yet; create a scamp_network and set primary_network_id")
	}
	for _, n := range list.Items {
		if norm(n.Name) == defaultNetworkName {
			return n.NetworkUUID, nil
		}
	}
	if len(list.Items) == 1 {
		return list.Items[0].NetworkUUID, nil
	}
	var names []string
	for _, n := range list.Items {
		names = append(names, n.Name)
	}
	sort.Strings(names)
	return "", fmt.Errorf("no %q network and more than one to choose from (%s); set primary_network_id",
		defaultNetworkName, strings.Join(names, ", "))
}

// stripPrefixLen drops the CIDR suffix the API attaches to addresses:
// "194.110.174.110/24" -> "194.110.174.110". Anything without a slash is
// returned untouched.
func stripPrefixLen(addr string) string {
	if i := strings.IndexByte(addr, '/'); i >= 0 {
		return addr[:i]
	}
	return addr
}

// ── reverse lookups ─────────────────────────────────────────────────────────
//
// Import goes the other way round: the platform hands back numeric class and
// template ids, while the config is written with names. Without translating
// them back, an imported VM plans as "replace" - every name attribute reads as
// null against a config that spells them out.
//
// All three are best-effort: a class that no longer exists in the catalogue
// leaves the name empty rather than failing the import, because the resource
// itself is real and the user still wants it under management.

func nameForVMClass(ctx context.Context, c *client.Client, id int) string {
	var list models.VMClassesListResponse
	if err := c.GetJSON(ctx, client.VMClassesEP, nil, &list); err != nil {
		return ""
	}
	for _, it := range list.Items {
		if it.ID == id {
			return it.Name
		}
	}
	return ""
}

func nameForStorageClass(ctx context.Context, c *client.Client, id int) string {
	var list models.StorageClassesListResponse
	if err := c.GetJSON(ctx, client.StorageClassesEP, nil, &list); err != nil {
		return ""
	}
	for _, it := range list.Items {
		if it.ID == id {
			return it.Name
		}
	}
	return ""
}

// nameForTemplate returns the slug ("ubuntu-26.04"), which is what `image`
// takes and what the docs teach.
func nameForTemplate(ctx context.Context, c *client.Client, id int) string {
	var list models.VMTemplatesListResponse
	if err := c.GetJSON(ctx, client.VMTemplatesEP, nil, &list); err != nil {
		return ""
	}
	for _, it := range list.Items {
		if it.ID == id {
			return strings.TrimPrefix(it.APIName, "templates/")
		}
	}
	return ""
}
