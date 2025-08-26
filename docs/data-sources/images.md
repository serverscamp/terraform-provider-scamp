---

### 📄 `docs/data-sources/images.md`

```markdown
---
page_title: "scamp_images Data Source"
subcategory: ""
description: |-
  Retrieve available images from SCAMP cloud.
---

# scamp_images (Data Source)

Use this data source to list available images.

## Example Usage

```hcl
data "scamp_images" "all" {}

output "all_images" {
  value = data.scamp_images.all.items
}