# The contract test installs a locally built provider from a filesystem mirror
# as 0.0.0-dev; Terraform selects a pre-release only when pinned exactly.
terraform {
  required_version = ">= 1.11"

  required_providers {
    ipam = {
      source  = "appvia/ipam"
      version = "0.0.0-dev"
    }
    azurerm = {
      source  = "hashicorp/azurerm"
      version = ">= 4.14.0, < 6.0.0"
    }
  }
}
