---

### 📄 `docs/resources/instance.md`

```markdown
---
page_title: "scamp_instance Resource"
subcategory: ""
description: |-
  Manages a compute instance in SCAMP cloud.
---

# scamp_instance (Resource)

Provides a SCAMP compute instance.

## Example Usage

```hcl
resource "scamp_instance" "example" {
  name    = "example-vm"
  flavor  = 2
  image   = 1
  ssh_key = scamp_ssh_key.mykey.id
  running = true
}