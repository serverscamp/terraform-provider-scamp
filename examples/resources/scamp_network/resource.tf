# A private network is isolated: only the VMs you attach to it can talk over it.
resource "scamp_network" "internal" {
  name = "internal"
  type = "private"
  cidr = "10.77.20.0/24"
}
