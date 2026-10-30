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
