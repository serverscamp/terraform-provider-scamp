---

### 📄 `docs/data-sources/limits.md`

```markdown
---
page_title: "scamp_limits Data Source"
subcategory: ""
description: |-
  Retrieve current account resource limits from SCAMP cloud.
---

# scamp_limits (Data Source)

Use this data source to get account limits.

## Example Usage

```hcl
data "scamp_limits" "all" {}

output "all_limits" {
  value = data.scamp_limits.all.items
}