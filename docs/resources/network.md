---
page_title: "scamp_network Resource - SCAMP Provider"
subcategory: ""
description: |-
  Manages a private network in SCAMP cloud.
---

# scamp_network (Resource)

Manages a private network in SCAMP. Networks can be attached to routers to provide internet access.

## Example Usage

### Create a basic network

```hcl
resource "scamp_network" "internal" {
  name = "internal-network"
  cidr = "10.50.0.0/24"
}
```

### Create a network with auto-generated CIDR

```hcl
resource "scamp_network" "auto" {
  name = "auto-network"
}
```

### Create a network attached to a router

```hcl
resource "scamp_network" "public" {
  name        = "public-network"
  cidr        = "10.100.0.0/24"
  router_uuid = scamp_router.main.id
}
```

## Argument Reference

- `name` (Optional) - Name of the network (1-64 characters). If not provided, an auto-generated name will be assigned.
- `cidr` (Optional) - CIDR block for the network (e.g., `10.50.0.0/24`). If not provided, a random CIDR will be generated. Changing this forces a new resource.
- `router_uuid` (Optional) - UUID of the router to attach this network to. Set to attach, remove to detach.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `id` - The UUID of the network.
- `network_type` - Type of network: `private` if not attached to a router, `public` if attached.
- `status` - Current status of the network (`provision_queued`, `active`, etc.).
- `created_at` - Timestamp when the network was created.

## Import

Networks can be imported using their UUID:

```shell
terraform import scamp_network.example b4add215-d138-4e46-9600-28c594b83983
```

## Notes

- Networks cannot be deleted if they have VMs attached. Remove VMs first.
- Networks cannot be deleted if attached to a router. Detach from router first (Terraform handles this automatically).
- Attaching a network to a router changes its `network_type` from `private` to `public`.
