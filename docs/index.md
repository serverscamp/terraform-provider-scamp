---
page_title: "SCAMP Provider"
subcategory: ""
description: |-
  Terraform provider for managing SCAMP cloud resources.
---

# SCAMP Provider

The SCAMP provider allows you to manage resources on the [ServersCamp](https://serverscamp.com) cloud platform.

## Authentication

The provider requires an API token for authentication. You can obtain a token by:

1. Log in to the [SCAMP management panel](https://sandbox.serverscamp.com)
2. Create an API token
3. Save the token from the response (it's shown only once)

Tokens have the format `sc_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx`.

## Example Usage

```hcl
terraform {
  required_providers {
    scamp = {
      source  = "serverscamp/scamp"
      version = "~> 0.2.0"
    }
  }
}

provider "scamp" {
  token = var.scamp_token
}

# Generate a new SSH key
resource "scamp_ssh_key" "generated" {
  key_name = "terraform-generated"
  generate = true
}

# Import an existing public key
resource "scamp_ssh_key" "imported" {
  key_name   = "terraform-imported"
  public_key = file("~/.ssh/id_ed25519.pub")
}

# Read an existing SSH key
data "scamp_ssh_key" "existing" {
  id = 1
}
```

## Configuration

### Provider Arguments

- `api_url` (Optional) - Base API URL. Defaults to `https://platform.serverscamp.com/api/v1`. Can also be set via `SCAMP_API_URL` environment variable.
- `token` (Required) - API token for authentication. Can also be set via `SCAMP_TOKEN` environment variable. Environment variable takes precedence.

### Environment Variables

| Variable | Description |
|----------|-------------|
| `SCAMP_TOKEN` | API token (takes precedence over config) |
| `SCAMP_API_URL` | Base API URL |
