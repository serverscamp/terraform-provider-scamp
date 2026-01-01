---
page_title: "scamp_router Resource - SCAMP Provider"
subcategory: ""
description: |-
  Manages a router in SCAMP cloud.
---

# scamp_router (Resource)

Manages a router in SCAMP. Routers provide internet access for attached private networks.

## Example Usage

### Create a basic router

```hcl
resource "scamp_router" "main" {
  name = "main-router"
}
```

### Create a router with attached network

```hcl
resource "scamp_router" "main" {
  name = "main-router"
}

resource "scamp_network" "internal" {
  name        = "internal-network"
  cidr        = "10.50.0.0/24"
  router_uuid = scamp_router.main.id
}
```

## Argument Reference

- `name` (Optional) - Name of the router (1-64 characters). If not provided, an auto-generated name will be assigned.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `id` - The UUID of the router.
- `ipv4_address` - Public IPv4 address assigned to the router (with CIDR notation, e.g., `194.110.174.50/24`).
- `ipv6_address` - Public IPv6 address assigned to the router (with CIDR notation).
- `status` - Current status of the router (`provision_queued`, `active`, etc.).
- `created_at` - Timestamp when the router was created.

## Import

Routers can be imported using their UUID:

```shell
terraform import scamp_router.example a1b2c3d4-5678-90ab-cdef-1234567890ab
```

## Notes

- Routers cannot be deleted if they have attached networks. Detach all networks first.
- Each router gets a unique public IPv4 and IPv6 address.
- Use `scamp_network` with `router_uuid` to attach networks to the router.
