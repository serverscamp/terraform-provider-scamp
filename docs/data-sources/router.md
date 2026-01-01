---
page_title: "scamp_router Data Source - SCAMP Provider"
subcategory: ""
description: |-
  Retrieves information about an existing router.
---

# scamp_router (Data Source)

Use this data source to retrieve information about an existing router by its UUID.

## Example Usage

```hcl
data "scamp_router" "existing" {
  id = "a1b2c3d4-5678-90ab-cdef-1234567890ab"
}

output "router_ipv4" {
  value = data.scamp_router.existing.ipv4_address
}

output "router_ipv6" {
  value = data.scamp_router.existing.ipv6_address
}
```

## Argument Reference

- `id` (Required) - The UUID of the router to retrieve.

## Attribute Reference

The following attributes are exported:

- `id` - The UUID of the router.
- `name` - Name of the router.
- `ipv4_address` - Public IPv4 address of the router.
- `ipv6_address` - Public IPv6 address of the router.
- `status` - Current status of the router.
- `created_at` - Timestamp when the router was created.
