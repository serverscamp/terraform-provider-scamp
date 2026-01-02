# SCAMP Terraform Provider

Terraform provider for managing cloud resources on [serverscamp.com](https://serverscamp.com).

See also:
- [SCAMP API Documentation](https://serverscamp.com/docs/api)
- [scli - CLI utility](https://github.com/serverscamp/scli)

## Resources

| Resource | Description |
|----------|-------------|
| `scamp_vm` | Virtual machine with root disk |
| `scamp_volume` | Additional disk (attach/detach to VM) |
| `scamp_network` | Private or public network |
| `scamp_router` | Router for public network access |
| `scamp_ssh_key` | SSH key (import or generate) |

## Data Sources

| Data Source | Description |
|-------------|-------------|
| `scamp_vm` | Get VM by UUID |
| `scamp_volume` | Get volume by UUID |
| `scamp_network` | Get network by UUID or name |
| `scamp_router` | Get router by UUID or name |
| `scamp_ssh_key` | Get SSH key by ID or name |
| `scamp_vm_classes` | List all VM classes |
| `scamp_vm_class` | Get VM class by name |
| `scamp_storage_classes` | List all storage classes |
| `scamp_storage_class` | Get storage class by name |
| `scamp_network_classes` | List all network classes |
| `scamp_network_class` | Get network class by name |
| `scamp_vm_templates` | List all VM templates |
| `scamp_vm_template` | Get VM template by OS type |

## Example

```hcl
terraform {
  required_providers {
    scamp = {
      source  = "serverscamp/scamp"
      version = "0.2.0"
    }
  }
}

provider "scamp" {
  # token = "sc_..." # or set SCAMP_TOKEN env var
}

# Classes
data "scamp_vm_class" "small" { name = "burst-s" }
data "scamp_storage_class" "standard" { name = "standart-storage" }
data "scamp_network_class" "baseline" { name = "baseline-network" }
data "scamp_vm_template" "ubuntu" { os_type = "Ubuntu" }

# SSH key
resource "scamp_ssh_key" "main" {
  key_name   = "my-key"
  public_key = file("~/.ssh/id_rsa.pub")
}

# Router (provides public IPs)
resource "scamp_router" "default" {
  name = "default-router"
}

# Network
resource "scamp_network" "public" {
  name        = "public-network"
  cidr        = "10.50.0.0/24"
  type        = "public"
  router_uuid = scamp_router.default.id
}

# VM
resource "scamp_vm" "web" {
  display_name             = "web-server"
  vm_class_id              = data.scamp_vm_class.small.id
  vm_template_id           = data.scamp_vm_template.ubuntu.id
  ssh_key_id               = scamp_ssh_key.main.id
  root_disk_class_id       = data.scamp_storage_class.standard.id
  root_disk_gb             = 50
  primary_network_id       = scamp_network.public.id
  primary_network_class_id = data.scamp_network_class.baseline.id
  assign_public_ips        = true
}

# Additional volume
resource "scamp_volume" "data" {
  display_name     = "data-disk"
  size_gb          = 100
  storage_class_id = data.scamp_storage_class.standard.id
  attached_vm_id   = scamp_vm.web.id  # optional
}
```

## Build

```bash
go mod tidy
go build -o terraform-provider-scamp
```
