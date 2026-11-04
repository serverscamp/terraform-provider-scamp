---
page_title: "scamp_registry Resource - scamp"
subcategory: ""
description: |-
  Manages a container registry on its own server, for Docker and OCI images and Helm charts.
---

# scamp_registry (Resource)

Manages a container registry on its own server, for Docker and OCI images and Helm charts, with its own name under registry.serverscamp.com and a certificate we renew. The main user has the whole registry; add others with `scamp_registry_user`.

## Example Usage

```terraform
resource "scamp_registry" "images" {
  name         = "images"
  package      = "rg-50"
  grow_up_to   = "rg-250"
  public_repos = ["public/*"]
}

output "registry_host" {
  value = scamp_registry.images.host   # docker login <host> -u main
}

output "registry_password" {
  value     = scamp_registry.images.password
  sensitive = true
}
```

`terraform apply` returns once the registry runs, usually in a few minutes.

## Packages

`rg-10` (10 GB), `rg-25`, `rg-50`, `rg-100`, `rg-250`, `rg-500`, `rg-1000` (1 TB). A full registry moves up by
itself, one package at a time, up to `grow_up_to`. That move is not a change in the plan: `package` keeps the
package the registry started on. A package is never moved down, so changing `package` replaces the registry.

## Schema

### Required

- `name` (String) Name of the registry: 3 to 32 lowercase letters, digits and hyphens, starting with a letter. It is the start of its address, NAME-xxxxxx.registry.serverscamp.com. Changing it replaces the registry and every image in it.
- `package` (String) Package by slug or name: rg-10 (10 GB) up to rg-1000 (1 TB). Changing it replaces the registry.

### Optional

- `grow_up_to` (String) A bigger package the registry moves up to by itself when it is full. Changed in place.
- `network_id` (String) UUID of the network, attached to a router. Without it the account's default network. Changing it replaces the registry and every image in it.
- `public_repos` (List of String) Repositories anyone may pull without logging in: names like `app` or patterns like `public/*`. Changed in place.

### Read-Only

- `host` (String) The registry's name, NAME-xxxxxx.registry.serverscamp.com.
- `id` (String) The registry's id.
- `ip` (String) Public address of the registry.
- `password` (String, Sensitive) The main user's password.
- `private_ip` (String) Address of the registry in its network.
- `state` (String) creating, configuring, active or failed.
- `user` (String) The main user, with the whole registry: `main`.

## Import

```shell
terraform import scamp_registry.images 4e5b1e18-9802-4d1a-962b-d03a32f5b5d5
```
