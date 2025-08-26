---

### 📄 `docs/data-sources/flavors.md`

```markdown
---
page_title: "scamp_flavors Data Source"
subcategory: ""
description: |-
  Retrieve available flavors from SCAMP cloud.
---

# scamp_flavors (Data Source)

Use this data source to list available flavors.

## Example Usage

```hcl
data "scamp_flavors" "all" {}

output "all_flavors" {
  value = data.scamp_flavors.all.items
}