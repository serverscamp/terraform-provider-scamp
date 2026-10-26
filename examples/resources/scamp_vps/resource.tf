resource "scamp_ssh_key" "laptop" {
  key_name   = "laptop"
  public_key = file("~/.ssh/id_ed25519.pub")
}

resource "scamp_vps" "web" {
  name        = "web-1"
  plan        = "vps2-1"
  image       = "debian-13"
  ssh_key_ids = [scamp_ssh_key.laptop.id]
  backups     = true
}

output "web_ipv4" {
  value = scamp_vps.web.ipv4
}
