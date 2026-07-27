resource "scamp_ssh_key" "main" {
  key_name   = "workstation"
  public_key = file("~/.ssh/id_ed25519.pub")
}
