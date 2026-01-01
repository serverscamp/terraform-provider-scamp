---
page_title: "scamp_network Data Source - SCAMP Provider"
subcategory: ""
description: |-
  Retrieves information about an existing network.
---

# scamp_network (Data Source)

Use this data source to retrieve information about an existing network by its UUID.

## Example Usage

```hcl
data "scamp_network" "existing" {
  id = "b4add215-d138-4e46-9600-28c594b83983"
}

output "network_cidr" {
  value = data.scamp_network.existing.cidr
}

output "network_type" {
  value = data.scamp_network.existing.network_type
}
```

## Argument Reference

- `id` (Required) - The UUID of the network to retrieve.

## Attribute Reference

The following attributes are exported:

- `id` - The UUID of the network.
- `name` - Name of the network.
- `cidr` - CIDR block of the network.
- `router_uuid` - UUID of the attached router, if any.
- `network_type` - Type of network: `private` or `public`.
- `status` - Current status of the network.
- `created_at` - Timestamp when the network was created.
