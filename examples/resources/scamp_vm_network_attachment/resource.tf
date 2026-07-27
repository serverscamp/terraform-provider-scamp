# The VM's first network is set on the VM itself. Every additional network is
# an attachment, so one machine can sit in several networks at once.
resource "scamp_vm_network_attachment" "internal" {
  vm_id      = scamp_vm.web.id
  network_id = scamp_network.internal.id
}
