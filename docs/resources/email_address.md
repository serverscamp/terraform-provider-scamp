---
page_title: "scamp_email_address Resource - scamp"
subcategory: ""
description: |-
  Manages an address the account sends mail from through the ServersCamp mail gateway.
---

# scamp_email_address (Resource)

Manages an address the account sends mail from through the ServersCamp mail gateway, NAME@<your space>.serverscamp-mailer.com, over SMTP with its own login. Transactional mail only, within the daily limit of the account.

## Example Usage

```terraform
resource "scamp_email_address" "gitlab" {
  name  = "gitlab"
  label = "GitLab notifications"
}

output "smtp" {
  value = {
    host  = scamp_email_address.gitlab.smtp_host
    port  = scamp_email_address.gitlab.smtp_port   # 587, STARTTLS
    login = scamp_email_address.gitlab.address
  }
}

output "smtp_password" {
  value     = scamp_email_address.gitlab.password
  sensitive = true
}
```

## A new password

Set `password_version` to any number and change it whenever the password should be replaced. The address stays;
the old password stops working.

```terraform
resource "scamp_email_address" "app" {
  name             = "app"
  password_version = 2
}
```

Changing `name` or `label` makes the address again, with a new password. An address an account deleted can be
made again by the same account.

## Schema

### Required

- `name` (String) The part before @, e.g. gitlab: lowercase letters, digits, dots, hyphens and underscores, starting and ending with a letter or digit, up to 40. Changing it replaces the address.

### Optional

- `label` (String) A note on what sends from it, up to 100 characters. Changing it makes the address again, with a new password.
- `password_version` (Number) Change it to any other number to get a new SMTP password; the old one stops working. The address stays.

### Read-Only

- `address` (String) The full address. It is also the SMTP login.
- `disabled` (Boolean) True when the address was stopped, for bounced or refused mail or by us.
- `disabled_reason` (String) Why the address was stopped.
- `id` (String) The address's id.
- `password` (String, Sensitive) The SMTP password.
- `smtp_host` (String) SMTP server of the mail gateway.
- `smtp_port` (Number) SMTP port with STARTTLS: 587. Port 465 with TLS works too.

## Import

```shell
terraform import scamp_email_address.gitlab 4e5b1e18-9802-4d1a-962b-d03a32f5b5d5
```
