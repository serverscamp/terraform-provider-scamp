# A floating address outlives the machine behind it, which is what makes
# failover possible: point it at another VM and the address keeps working.
resource "scamp_floating_ip" "web" {
  display_name   = "web-vip"
  attached_vm_id = scamp_vm.web.id
}
