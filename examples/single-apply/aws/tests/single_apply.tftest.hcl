mock_provider "aws" {}

run "single_apply" {
  command = plan

  assert {
    condition     = ipam_pool.root.id == "platform"
    error_message = "ipam_pool.root was not planned."
  }

  assert {
    condition     = module.ipam.provider_config.bucket == "acme-ipam" && module.ipam.provider_config.region == "eu-west-2"
    error_message = "provider_config is not known at plan time or has the wrong shape."
  }
}
