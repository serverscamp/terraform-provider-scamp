terraform {
  required_providers {
    scamp = {
      source  = "serverscamp/scamp"
      version = "~> 1.4"
    }
  }
}

# The token is created in the Cloud Panel under API tokens and shown once.
# Prefer the environment variable: SCAMP_TOKEN=sc_...
provider "scamp" {}
