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
