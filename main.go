package main

import (
    "context"
    "github.com/hashicorp/terraform-plugin-framework/providerserver"
    "github.com/hashicorp/terraform-plugin-log/tflog"
    "github.com/serverscamp/terraform-provider-scamp/internal/provider"
)

// version is stamped by goreleaser at build time (-ldflags "-X main.version").
// The default only shows up in local builds.
var version = "dev"

func main() {
    ctx := context.Background()
    tflog.Info(ctx, "Starting SCAMP Terraform Provider", map[string]any{"version": version})
    providerserver.Serve(ctx, provider.New, providerserver.ServeOpts{
        Address: "registry.terraform.io/serverscamp/scamp",
    })
}
