---
page_title: "Provisioning storage and pools in one apply"
subcategory: ""
description: |-
  Create the IPAM storage and access with a module and declare pools in the same root configuration.
---

# Provisioning storage and pools in one apply

You can create the storage account or bucket, the access grants and the first pools in a single `terraform apply`. Call [terraform-azurerm-ipam](https://github.com/appvia/terraform-azurerm-ipam) or [terraform-aws-ipam](https://github.com/appvia/terraform-aws-ipam), feed its `provider_config` output to the provider, and make the pools depend on the module.

## Azure

```terraform
# Copyright 2026 Appvia Ltd.
# SPDX-License-Identifier: Apache-2.0

module "ipam" {
  source = "../../../../terraform-azurerm-ipam"

  resource_group_name    = "rg-ipam"
  location               = "uksouth"
  storage_account_name   = "acmeipam"
  platform_principal_ids = ["00000000-0000-0000-0000-000000000001"]

  environments = {
    prod = {
      pool_ids      = ["uks-prod"]
      principal_ids = ["00000000-0000-0000-0000-000000000002"]
    }
  }
}

provider "ipam" {
  azure_blob = module.ipam.provider_config
}

resource "ipam_pool" "root" {
  id    = "platform"
  cidrs = ["10.0.0.0/8"]

  depends_on = [module.ipam]
}

resource "ipam_pool" "uks_prod" {
  id            = "uks-prod"
  parent_pool   = ipam_pool.root.id
  prefix_length = 16
}
```

## AWS

```terraform
# Copyright 2026 Appvia Ltd.
# SPDX-License-Identifier: Apache-2.0

module "ipam" {
  source = "../../../../terraform-aws-ipam"

  bucket_name         = "acme-ipam"
  region              = "eu-west-2"
  platform_role_names = ["platform-deploy"]

  environments = {
    prod = {
      pool_ids   = ["uks-prod"]
      role_names = ["prod-deploy"]
    }
  }
}

provider "ipam" {
  s3 = module.ipam.provider_config
}

resource "ipam_pool" "root" {
  id    = "platform"
  cidrs = ["10.0.0.0/8"]

  depends_on = [module.ipam]
}

resource "ipam_pool" "uks_prod" {
  id            = "uks-prod"
  parent_pool   = ipam_pool.root.id
  prefix_length = 16
}
```

## Why it works

- **The provider configuration must be known at plan time.** `provider_config` is built only from module inputs (account name and container, or bucket and region), so it is. If you feed the provider a value that's only known after apply, planning fails with "Provider configuration is unknown".
- **`depends_on = [module.ipam]`** makes the pools wait for the container or bucket and the role assignments.
- **`access_propagation_timeout`** covers the delay before a new Azure role assignment or IAM policy takes effect. Creates retry access denied until it runs out (default `5m`).
- **Avoid `data.ipam_*` in the same first apply.** Data sources read at plan time, before the storage exists, and reads don't wait for access to propagate. Leave them out of the first apply or give them `depends_on = [module.ipam]` so the read is deferred to apply.
