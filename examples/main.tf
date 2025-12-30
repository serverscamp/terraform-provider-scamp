terraform {
  required_providers {
    scamp = {
      source  = "serverscamp/scamp"
      version = "~> 0.2.0"
    }
  }
}

variable "scamp_token" {
  description = "API token for SCAMP provider (starts with sc_)"
  type        = string
  sensitive   = true
}

provider "scamp" {
  token = var.scamp_token
}

# Generate a new SSH key pair
resource "scamp_ssh_key" "generated" {
  key_name = "terraform-generated-key"
  generate = true
}

# Import an existing public key
resource "scamp_ssh_key" "imported" {
  key_name   = "terraform-imported-key"
  public_key = file("~/.ssh/id_ed25519.pub")
}

# Read an existing SSH key by ID
data "scamp_ssh_key" "existing" {
  id = scamp_ssh_key.generated.id
}

# Outputs
output "generated_key_id" {
  description = "ID of the generated SSH key"
  value       = scamp_ssh_key.generated.id
}

output "generated_key_fingerprint" {
  description = "Fingerprint of the generated SSH key"
  value       = scamp_ssh_key.generated.fingerprint
}

output "generated_public_key" {
  description = "Public key of the generated SSH key"
  value       = scamp_ssh_key.generated.public_key
}

output "generated_private_key" {
  description = "Private key of the generated SSH key (sensitive)"
  value       = scamp_ssh_key.generated.private_key
  sensitive   = true
}

output "imported_key_id" {
  description = "ID of the imported SSH key"
  value       = scamp_ssh_key.imported.id
}

output "imported_key_fingerprint" {
  description = "Fingerprint of the imported SSH key"
  value       = scamp_ssh_key.imported.fingerprint
}
