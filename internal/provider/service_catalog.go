package provider

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/serverscamp/terraform-provider-scamp/internal/client"
	"github.com/serverscamp/terraform-provider-scamp/internal/models"
)

// Managed databases and message brokers share the shape of their catalogs: engines with their versions and plans
// per engine, named by slug (pg-basic-2g, nats-basic-1g).

// serviceOffer is the engine and the plan a config asks for, checked against the catalog at ep.
type serviceOffer struct {
	Engine  models.ServiceEngine
	Plan    models.ServicePlan
	Version string
}

func resolveServiceOffer(ctx context.Context, c *client.Client, ep, engine, plan, version string) (*serviceOffer, error) {
	var cat models.ServiceCatalog
	if err := c.GetJSON(ctx, ep+"/catalog", nil, &cat); err != nil {
		return nil, fmt.Errorf("could not read the catalog: %w", err)
	}
	var eng *models.ServiceEngine
	var keys []string
	for i := range cat.Engines {
		e := cat.Engines[i]
		if !e.Available || (e.Creatable != nil && !*e.Creatable) {
			continue
		}
		keys = append(keys, e.Key)
		if e.Key == norm(engine) {
			eng = &cat.Engines[i]
		}
	}
	if eng == nil {
		return nil, fmt.Errorf("no engine %q; available: %s", engine, strings.Join(keys, ", "))
	}
	out := &serviceOffer{Engine: *eng, Version: eng.DefaultVersion}
	if version != "" {
		ok := false
		for _, v := range eng.Versions {
			ok = ok || v == version
		}
		if !ok {
			return nil, fmt.Errorf("%s has no version %q; available: %s", eng.Key, version, strings.Join(eng.Versions, ", "))
		}
		out.Version = version
	}
	var names []string
	for _, p := range cat.Plans {
		if p.Engine != eng.Key {
			continue
		}
		names = append(names, p.Name)
		if norm(p.Name) == norm(plan) {
			if p.SoldOut {
				return nil, fmt.Errorf("plan %s is out of capacity right now", p.Name)
			}
			out.Plan = p
			return out, nil
		}
	}
	return nil, fmt.Errorf("plan %q is not one of %s; available: %s", plan, eng.Key, strings.Join(names, ", "))
}

// hexSlug is 6 random hex digits: the tail of a broker's address.
func hexSlug() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// waitServiceState polls read until it reports want. A failed service stops the wait with its error.
func waitServiceState(ctx context.Context, timeout time.Duration, read func() (state string, errMsg string, err error), want string) error {
	deadline := time.Now().Add(timeout)
	for {
		st, msg, err := read()
		if err == nil {
			switch st {
			case want:
				return nil
			case "failed":
				if msg == "" {
					msg = "it could not be created"
				}
				return fmt.Errorf("%s", msg)
			}
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("not %s within %s: %w", want, timeout, err)
			}
			return fmt.Errorf("still %s after %s", st, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Second):
		}
	}
}
