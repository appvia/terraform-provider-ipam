mock_provider "azurerm" {}

run "single_apply" {
  command = plan

  assert {
    condition     = ipam_pool.root.id == "platform"
    error_message = "ipam_pool.root was not planned."
  }

  assert {
    condition     = module.ipam.provider_config.container == "ipam" && module.ipam.provider_config.account_url == "https://acmeipam.blob.core.windows.net"
    error_message = "provider_config is not known at plan time or has the wrong shape."
  }
}
