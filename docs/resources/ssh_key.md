---

### 📄 `docs/resources/ssh_key.md`

```markdown
---
page_title: "scamp_ssh_key Resource"
subcategory: ""
description: |-
  Manages an SSH key in SCAMP cloud.
---

# scamp_ssh_key (Resource)

Provides an SSH key resource for use with instances.

## Example Usage

```hcl
resource "scamp_ssh_key" "mykey" {
  name       = "mykey"
  public_key = file("~/.ssh/id_rsa.pub")
  protected  = false
}