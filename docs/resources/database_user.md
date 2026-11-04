---
page_title: "scamp_database_user Resource - scamp"
subcategory: ""
description: |-
  Manages a user of a PostgreSQL or MySQL server of scamp_database, with its access to the server's databases.
---

# scamp_database_user (Resource)

Manages a user of a PostgreSQL or MySQL server of scamp_database, with its access to the server's databases. The password is made by the server.

## Example Usage

```terraform
resource "scamp_database_user" "shop_app" {
  database_id = scamp_database.main.id
  name        = "shop_app"
  grants = {
    (scamp_database_db.shop.name) = "write"
  }
}

output "shop_dsn" {
  value     = "postgresql://shop_app:${scamp_database_user.shop_app.password}@${scamp_database.main.host}:${scamp_database.main.port}/shop"
  sensitive = true
}
```

## Schema

### Required

- `database_id` (String) ID of the scamp_database server. Changing it replaces the user.
- `name` (String) Name of the user: lowercase letters, digits and underscores, starting with a letter. Changing it replaces the user.

### Optional

- `grants` (Map of String) Access by database name: `read`, `write` or `admin`. Without it the user has no database. Changed in place.

### Read-Only

- `id` (String) `<database_id>/<name>`.
- `password` (String, Sensitive) The user's password.

## Import

```shell
terraform import scamp_database_user.shop_app 4e5b1e18-9802-4d1a-962b-d03a32f5b5d5/shop_app
```
