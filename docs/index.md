---
page_title: "SCAMP Provider"
subcategory: ""
description: |-
  The SCAMP provider allows managing cloud resources such as instances, flavors, images, and SSH keys on ServersCamp platform.
---

# SCAMP Provider

The SCAMP provider is used to manage resources in [ServersCamp](https://serverscamp.com), including instances, SSH keys, flavors, images, and limits.

## Example Usage

```hcl
terraform {
  required_providers {
    scamp = {
      source  = "serverscamp/scamp"
      version = "~> 0.1.0"
    }
  }
}

provider "scamp" {
  base_url = "https://my.serverscamp.com/coreapi"
  token    = var.scamp_token
}