---
page_title: "scamp_database_db Resource - scamp"
subcategory: ""
description: |-
  Manages a database on a PostgreSQL or MySQL server of scamp_database.
---

# scamp_database_db (Resource)

Manages a database on a PostgreSQL or MySQL server of scamp_database. The server's owner user has full access to it; give other users access with `scamp_database_user`.

## Example Usage

```terraform
resource "scamp_database" "main" {
  name   = "main-db"
  engine = "postgresql"
  plan   = "db-bs-s"
}

resource "scamp_database_db" "shop" {
  database_id = scamp_database.main.id
  name        = "shop"
}
```

`terraform apply` returns once the server has made the database.

## Schema

### Required

- `database_id` (String) ID of the scamp_database server. Changing it replaces the database.
- `name` (String) Name of the database: lowercase letters, digits and underscores, starting with a letter. Changing it replaces the database, with its data.

### Optional

- `force_destroy` (Boolean) Destroy closes the open connections to the database. Default false: destroy fails while there are any.

### Read-Only

- `id` (String) `<database_id>/<name>`.

## Import

```shell
terraform import scamp_database_db.shop 4e5b1e18-9802-4d1a-962b-d03a32f5b5d5/shop
```
