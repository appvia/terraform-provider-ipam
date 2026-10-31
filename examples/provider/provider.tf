# Copyright 2026 Appvia Ltd.
# SPDX-License-Identifier: Apache-2.0

terraform {
  required_providers {
    ipam = {
      source = "appvia/ipam"
    }
  }
}

# Amazon S3. Credentials come from the AWS SDK default chain
# (AWS_PROFILE, AWS_ACCESS_KEY_ID, web identity, instance roles).
provider "ipam" {
  s3 = {
    bucket = "acme-ipam"
    region = "eu-west-2"
  }
}

# Azure Blob Storage. Credentials come from ARM_* environment variables,
# exactly like the azurerm and azapi providers (OIDC, managed identity,
# Azure CLI, client secret or certificate).
provider "ipam" {
  alias = "azure"

  azure_blob = {
    account_url = "https://acmeipam.blob.core.windows.net"
    container   = "ipam"
  }

  # Wait up to 10 minutes for new role assignments to take effect.
  access_propagation_timeout = "10m"
}

# A directory on a local disk, for development and single-host use.
provider "ipam" {
  alias = "local"

  local = {
    path = "/var/lib/ipam"
  }
}

# No backend block: IPAM_BACKEND must hold a backend URL, for example
#   export IPAM_BACKEND='s3://acme-ipam/?region=eu-west-2'
provider "ipam" {
  alias = "env"
}
