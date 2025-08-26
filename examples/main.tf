terraform {
  required_providers {
    scamp = {
      source  = "serverscamp/scamp"
      version = "0.1.0"
    }
  }
}

variable "scamp_token" {
  description = "API token for SCAMP provider"
  type        = string
  sensitive   = true
}

provider "scamp" {
  api_key = var.scamp_token
}

data "scamp_flavors" "all_flavors" {}
data "scamp_images" "all_images" {}
data "scamp_limits" "all_limits" {}

resource "scamp_ssh_key" "example_key_generated" {
  name = "key-generated"
  protected = false
}

resource "scamp_ssh_key" "example_key_imported" {
  name       = "key-imported"
  public_key = file("~/.ssh/id_rsa.pub")
  protected  = false
}

locals {
  sc_mini_id = one([
    for f in data.scamp_flavors.all_flavors.items : f.id
    if f.name == "sc-mini"
  ])
  ubuntu24_id = one([
    for i in data.scamp_images.all_images.items : i.id
    if lower(i.distro_family) == "ubuntu" && startswith(i.version, "24")
  ])
}

resource "scamp_instance" "example_vm1" {
  name    = "example-tf1"
  flavor  = local.sc_mini_id
  image   = local.ubuntu24_id
  ssh_key = scamp_ssh_key.example_key_imported.id
  running = true
  depends_on = [scamp_ssh_key.example_key_imported]
}

resource "scamp_instance" "example_vm2" {
  name    = "example-tf2"
  flavor  = local.sc_mini_id
  image   = local.ubuntu24_id
  ssh_key = scamp_ssh_key.example_key_imported.id
  running = true
  depends_on = [scamp_ssh_key.example_key_imported]
}

resource "scamp_instance" "example_vm3" {
  name    = "example-tf3"
  flavor  = local.sc_mini_id
  image   = local.ubuntu24_id
  ssh_key = scamp_ssh_key.example_key_imported.id
  running = true
  depends_on = [scamp_ssh_key.example_key_imported]
}

output "example_vm1_ipv4" {
  value = scamp_instance.example_vm1.ipv4
}
output "example_vm2_ipv4" {
  value = scamp_instance.example_vm2.ipv4
}
output "example_vm3_ipv4" {
  value = scamp_instance.example_vm3.ipv4
}

output "example_vm1_ipv6" {
  value = scamp_instance.example_vm1.ipv6
}
output "example_vm2_ipv6" {
  value = scamp_instance.example_vm2.ipv6
}
output "example_vm3_ipv6" {
  value = scamp_instance.example_vm3.ipv6
}