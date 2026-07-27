resource "scamp_volume" "data" {
  display_name  = "web-data"
  size_gb       = 100
  storage_class = "R2"

  # Attaching is a field, not a separate resource: set it to attach, clear it
  # to detach. The disk keeps its data either way.
  attached_vm_id = scamp_vm.web.id
}
