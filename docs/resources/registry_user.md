---
page_title: "scamp_registry_user Resource - scamp"
subcategory: ""
description: |-
  Manages a user of a scamp_registry besides main, with read or write access by repository.
---

# scamp_registry_user (Resource)

Manages a user of a scamp_registry besides main, with read or write access by repository. The password is made by the registry. A CI pipeline usually gets its own user with write access to its repositories.

## Example Usage

```terraform
resource "scamp_registry_user" "ci" {
  registry_id = scamp_registry.images.id
  name        = "ci"
  access = [
    { repos = "backend/*", level = "write" },
    { repos = "base/*", level = "read" },
  ]
}

output "ci_password" {
  value     = scamp_registry_user.ci.password
  sensitive = true
}
```

`access` and `disabled` are changed in place; changing `name` or `registry_id` replaces the user.

## Schema

### Required

- `name` (String) Name of the user: 2 to 32 lowercase letters, digits, - and _. `main` and `anonymous` are taken. Changing it replaces the user.
- `registry_id` (String) ID of the scamp_registry. Changing it replaces the user.

### Optional

- `access` (Attributes List) What the user may do, up to 50 rules. Without it the user may only log in. (see below)
- `disabled` (Boolean) The user is kept but cannot log in. Default false.

### Nested Schema for `access`

- `repos` (String) A repository like `app`, a pattern like `team/*`, or `*` for every repository.
- `level` (String) `read` (pull) or `write` (pull and push).

### Read-Only

- `id` (String) `<registry_id>/<name>`.
- `password` (String, Sensitive) The user's password.

## Import

```shell
terraform import scamp_registry_user.ci 4e5b1e18-9802-4d1a-962b-d03a32f5b5d5/ci
```
