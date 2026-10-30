# terraform-provider-ipam

A Terraform provider that allocates non-overlapping IP ranges from pools stored in shared object storage: Amazon S3, Azure Blob Storage or a local folder. There's no IPAM server. Workspaces, pipelines and the `ipam` CLI share the pools through conditional writes. It's built on [github.com/appvia/ipam](https://github.com/appvia/ipam).

```terraform
provider "ipam" {
  s3 = {
    bucket = "acme-ipam"
    region = "eu-west-2"
  }
}

data "ipam_pool" "uks" {
  id = "uks"
}

resource "ipam_allocation" "vnet" {
  pool          = data.ipam_pool.uks.id
  prefix_length = 24
}
```

## Documentation

- [Provider](docs/index.md): backends, credentials, `access_propagation_timeout`, the concurrency and access models, and least-privilege IAM.
- Resources: [`ipam_pool`](docs/resources/pool.md), [`ipam_allocation`](docs/resources/allocation.md).
- Data sources: [`ipam_pool`](docs/data-sources/pool.md), [`ipam_allocation`](docs/data-sources/allocation.md).
- Guide: [provisioning storage and pools in one apply](docs/guides/single-apply.md).

The storage and access grants are provisioned by [terraform-aws-ipam](https://github.com/appvia/terraform-aws-ipam) and [terraform-azurerm-ipam](https://github.com/appvia/terraform-azurerm-ipam). Their grants have been tested only in plan mode and under MinIO's IAM engine, not against real AWS IAM or Azure RBAC and ABAC. The real Azure credential path is untested too. The real-cloud legs of `scripts/e2e.sh` use the caller's own credentials and don't test the grants. See the known limits in the [provider docs](docs/index.md).

## Development

Requires Go and Terraform 1.11 or later.

- `make build` builds the provider binary.
- `make test` runs the unit tests, `make testacc` the acceptance tests, and `make lint` runs golangci-lint.
- `make docs` regenerates `docs/` from `templates/`, `examples/` and the schema with tfplugindocs, then validates it. Don't edit `docs/` by hand.

To try a local build, point Terraform at it with `dev_overrides` in a CLI config file:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/appvia/ipam" = "/path/to/terraform-provider-ipam"
  }
  direct {}
}
```

`examples/single-apply/{azure,aws}` hold a contract test between the provisioning modules and this provider (`terraform test`, plan only). They expect the module repositories next to this one and the provider installed from a filesystem mirror as version `0.0.0-dev`.

`go.mod` uses `replace github.com/appvia/ipam => ../ipam` until the client module is published.

## License

MPL-2.0. See [LICENSE](LICENSE).
