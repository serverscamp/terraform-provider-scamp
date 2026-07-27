# Classes, images and storage are named, not numbered: write what you would say
# out loud. `terraform plan` lists the valid names when one does not match.
resource "scamp_vm" "web" {
  display_name    = "web-1"
  vm_class        = "bs-burst-xs"
  image           = "ubuntu-26.04"
  root_disk_class = "R1"
  root_disk_gb    = 25

  # Public IPv4 + IPv6 on the VM itself. Leave it false for a private machine
  # and reach it through a bastion or a floating address.
  assign_public_ips = true

  # primary_network_id is optional: every organisation has a default network,
  # and the provider uses it when nothing is named here.
}

output "web_ip" {
  value = scamp_vm.web.public_ip_v4
}
