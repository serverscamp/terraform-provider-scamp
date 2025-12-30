---
page_title: "scamp_ssh_key Resource - SCAMP Provider"
subcategory: ""
description: |-
  Manages an SSH key in SCAMP cloud.
---

# scamp_ssh_key (Resource)

Manages an SSH key in SCAMP. Supports two modes:

1. **Generate** - Creates a new Ed25519 key pair on the server. The private key is returned only once at creation time.
2. **Import** - Imports an existing public key. Supports ed25519, rsa, and ecdsa key types.

## Example Usage

### Generate a new key pair

```hcl
resource "scamp_ssh_key" "generated" {
  key_name = "my-terraform-key"
  generate = true
}

# Save the private key to a file
resource "local_sensitive_file" "private_key" {
  content         = scamp_ssh_key.generated.private_key
  filename        = "${path.module}/my-terraform-key.pem"
  file_permission = "0600"
}

output "public_key" {
  value = scamp_ssh_key.generated.public_key
}
```

### Import an existing public key

```hcl
resource "scamp_ssh_key" "imported" {
  key_name   = "my-existing-key"
  public_key = file("~/.ssh/id_ed25519.pub")
}
```

### Import with inline public key

```hcl
resource "scamp_ssh_key" "imported_inline" {
  key_name   = "imported-inline"
  public_key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGxxxxxx user@host"
}
```

## Argument Reference

- `key_name` (Optional) - Name of the SSH key (max 255 characters). If not provided, an auto-generated name in format `key-{random}` will be assigned.
- `generate` (Optional) - Set to `true` to generate a new Ed25519 key pair. Mutually exclusive with `public_key`. Changing this forces a new resource.
- `public_key` (Optional) - Public key in OpenSSH format for import. Mutually exclusive with `generate`. Changing this forces a new resource.

~> **Note:** You must specify either `generate = true` OR `public_key`, but not both.

## Attribute Reference

In addition to the arguments above, the following attributes are exported:

- `id` - The unique identifier of the SSH key.
- `key_type` - Type of SSH key (`ed25519`, `rsa`, `ecdsa-sha2-nistp256`, etc.).
- `public_key` - Public key in OpenSSH format (computed for generated keys).
- `private_key` - Private key in PEM format. Only available for generated keys, returned only at creation time. Marked as sensitive.
- `fingerprint` - SHA256 fingerprint of the key.
- `has_private_key` - Whether the server stores the private key (`true` for generated keys, `false` for imported).
- `created_at` - Timestamp when the key was created.

## Import

SSH keys can be imported using their ID:

```shell
terraform import scamp_ssh_key.example 123
```

~> **Note:** When importing a generated key, the `private_key` attribute will not be available as it's only returned at creation time.
