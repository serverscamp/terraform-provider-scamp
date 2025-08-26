package main

import (
    "context"

    "github.com/hashicorp/terraform-plugin-framework/providerserver"
    "github.com/hashicorp/terraform-plugin-log/tflog"

    "scamp/internal/provider"
)

var version = "0.1.0"

func main() {
    ctx := context.Background()
    tflog.Info(ctx, "Starting SCAMP Terraform Provider", map[string]any{"version": version})
    providerserver.Serve(ctx, provider.New, providerserver.ServeOpts{
        Address: "registry.terraform.io/you/scamp",
    })
}
