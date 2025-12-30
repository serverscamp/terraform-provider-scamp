---
page_title: "scamp_ssh_key Data Source - SCAMP Provider"
subcategory: ""
description: |-
  Retrieves information about an existing SSH key.
---

# scamp_ssh_key (Data Source)

Use this data source to retrieve information about an existing SSH key by its ID.

## Example Usage

```hcl
data "scamp_ssh_key" "existing" {
  id = 1
}

output "key_fingerprint" {
  value = data.scamp_ssh_key.existing.fingerprint
}

output "key_public" {
  value = data.scamp_ssh_key.existing.public_key
}
```

## Argument Reference

- `id` (Required) - The ID of the SSH key to retrieve.

## Attribute Reference

The following attributes are exported:

- `id` - The unique identifier of the SSH key.
- `key_name` - Name of the SSH key.
- `key_type` - Type of SSH key (`ed25519`, `rsa`, `ecdsa-sha2-nistp256`, etc.).
- `public_key` - Public key in OpenSSH format.
- `fingerprint` - SHA256 fingerprint of the key.
- `has_private_key` - Whether the server stores the private key.
- `created_at` - Timestamp when the key was created.
