# ServersCamp Terraform Provider

Manage [ServersCamp](https://serverscamp.com) infrastructure as code: virtual
machines, networks, disks and floating addresses.

Registry: [`serverscamp/scamp`](https://registry.terraform.io/providers/serverscamp/scamp/latest)
· Full reference: [`docs/`](docs/)
· See also [scli](https://github.com/serverscamp/scli), the CLI.

## Quick start

```hcl
terraform {
  required_providers {
    scamp = {
      source  = "serverscamp/scamp"
      version = "~> 1.4"
    }
  }
}

provider "scamp" {
  # or export SCAMP_TOKEN=sc_...
  token = var.scamp_token
}

resource "scamp_vm" "web" {
  display_name      = "web-1"
  vm_class          = "bs-burst-xs"
  image             = "ubuntu-26.04"
  root_disk_class   = "R1"
  root_disk_gb      = 25
  assign_public_ips = true
}

output "web_ip" {
  value = scamp_vm.web.public_ip_v4
}
```

That is a complete, working configuration. Everything else is optional.

**Names, not numbers.** `vm_class`, `image` and `root_disk_class` take the same
names you see in the panel. When a name does not match, the error lists what is
actually available, so you can find the right one without leaving the terminal.

**The network is optional.** Every organisation has a default network, and a VM
lands there unless `primary_network_id` says otherwise.

## A fuller example

```hcl
resource "scamp_network" "internal" {
  name = "internal"
  type = "private"
  cidr = "10.77.20.0/24"
}

resource "scamp_vm" "web" {
  display_name      = "web-1"
  vm_class          = "bs-burst-xs"
  image             = "ubuntu-26.04"
  root_disk_class   = "R1"
  root_disk_gb      = 25
  assign_public_ips = true
}

# A second network on the same machine.
resource "scamp_vm_network_attachment" "internal" {
  vm_id      = scamp_vm.web.id
  network_id = scamp_network.internal.id
}

# An extra disk. Attaching is a field, not a separate resource.
resource "scamp_volume" "data" {
  display_name   = "web-data"
  size_gb        = 100
  storage_class  = "R2"
  attached_vm_id = scamp_vm.web.id
}

# An address that outlives the machine behind it: point it at another VM and it
# keeps working, which is what makes failover possible.
resource "scamp_floating_ip" "vip" {
  display_name   = "web-vip"
  attached_vm_id = scamp_vm.web.id
}
```

## Resources

| Resource | What it is |
|---|---|
| `scamp_vm` | Virtual machine |
| `scamp_volume` | Extra disk, attached or standalone |
| `scamp_network` | Private or public network |
| `scamp_vm_network_attachment` | An additional network interface on a VM |
| `scamp_floating_ip` | Floating address, moved between VMs without re-creating it |
| `scamp_ssh_key` | SSH key available to new machines |
| `scamp_router` | Router providing public connectivity to a network |

## Data sources

Look things up instead of hard-coding them: `scamp_vm_classes` /
`scamp_vm_class`, `scamp_storage_classes` / `scamp_storage_class`,
`scamp_vm_templates` / `scamp_vm_template`, plus `scamp_vm`, `scamp_volume`,
`scamp_network`, `scamp_router` and `scamp_ssh_key` for things that already
exist.

```hcl
data "scamp_vm_classes" "all" {}

output "available_classes" {
  value = [for c in data.scamp_vm_classes.all.items : c.name]
}
```

## Adopting existing infrastructure

Everything the provider manages can be imported, so a machine created in the
panel moves under Terraform without being rebuilt:

```bash
terraform import scamp_vm.web 75279184-491e-44cb-af17-838ff438a8f3
```

The provider translates the ids the platform returns back into the names a
config is written with, so `terraform plan` straight after an import comes back
empty instead of proposing to replace a running machine. The exact command for
each resource is on its page under [`docs/resources/`](docs/resources/).

## Authentication

The provider needs an API token, created in the Cloud Panel and shown once.

| Setting | Provider argument | Environment variable |
|---|---|---|
| Token | `token` | `SCAMP_TOKEN` |
| API endpoint | `api_url` | `SCAMP_API_URL` |

## Building from source

```bash
go build -o terraform-provider-scamp
```

To run Terraform against that build instead of the registry, put this in
`~/.terraformrc` - and then skip `terraform init` entirely, it would install the
published version over the top:

```hcl
provider_installation {
  dev_overrides { "serverscamp/scamp" = "/path/to/this/directory" }
  direct {}
}
```

Everything under `docs/` is generated from the provider schema plus the files in
`examples/`, so the schema descriptions are the single source of truth:

```bash
go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest generate --provider-name scamp
```
