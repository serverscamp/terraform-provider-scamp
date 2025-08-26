# SCAMP Terraform Provider

This Terraform provider allows you to manage servers, instances, SSH keys and related resources on [serverscamp.com](https://serverscamp.com).

In addition, serverscamp provides a public API and a CLI utility available at [scli](https://github.com/serverscamp/scli).

Resources:
- scamp_instance (create/start/stop/delete)
- scamp_ssh_key (import/generate, protect/unprotect, delete)

Data sources:
- scamp_flavors
- scamp_images
- scamp_limits

## Build
go mod tidy
go build -o terraform-provider-scamp

## Install
mkdir -p ~/.terraform.d/plugins/registry.terraform.io/serverscamp/scamp/0.1.0/
mv terraform-provider-scamp ~/.terraform.d/plugins/registry.terraform.io/serverscamp/scamp/0.1.0/
