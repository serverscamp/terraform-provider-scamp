---
page_title: "scamp_broker_user Resource - scamp"
subcategory: ""
description: |-
  Manages a user of a scamp_broker besides admin, with what it may do.
---

# scamp_broker_user (Resource)

Manages a user of a scamp_broker besides admin, with what it may do: NATS subjects and JetStream, a RabbitMQ virtual host and level, or Kafka topics and level. Each engine takes its own attributes and ignores the rest. The password is made by the broker. A broker has up to 20 users.

## Example Usage

```terraform
# NATS
resource "scamp_broker_user" "orders" {
  broker_id = scamp_broker.events.id
  name      = "orders"
  publish   = ["orders.>"]
  subscribe = ["orders.>", "_INBOX.>"]
  jetstream = true
}

# RabbitMQ
resource "scamp_broker_user" "worker" {
  broker_id = scamp_broker.queue.id
  name      = "worker"
  vhost     = "/"
  level     = "write"
}

# Kafka
resource "scamp_broker_user" "ingest" {
  broker_id = scamp_broker.stream.id
  name      = "ingest"
  topics    = "ingest."
  level     = "write"
}
```

Every attribute is changed in place; changing `name` or `broker_id` replaces the user.

## Schema

### Required

- `broker_id` (String) ID of the scamp_broker. Changing it replaces the user.
- `name` (String) Name of the user: 2 to 32 lowercase letters, digits, - and _, starting with a letter; admin is taken. Changing it replaces the user.

### Optional

- `jetstream` (Boolean) NATS: the user may use JetStream streams and consumers.
- `level` (String) RabbitMQ and Kafka: read or write.
- `publish` (List of String) NATS: subjects the user may publish to, `*` for one token and `>` at the end.
- `subscribe` (List of String) NATS: subjects the user may subscribe to.
- `topics` (String) Kafka: `*` for every topic, or a prefix of topic names.
- `vhost` (String) RabbitMQ: the virtual host of the user, `/` or up to 64 letters, digits, - _ and .

### Read-Only

- `id` (String) `<broker_id>/<name>`.
- `password` (String, Sensitive) The user's password.

## Import

```shell
terraform import scamp_broker_user.orders 4e5b1e18-9802-4d1a-962b-d03a32f5b5d5/orders
```
