# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Provider with `s3`, `azure_blob` and `local` backends, the `IPAM_BACKEND` fallback and `access_propagation_timeout` (`IPAM_ACCESS_PROPAGATION_TIMEOUT`).
- `ipam_pool` resource and data source: root and child pools, IPv4 and IPv6, import by ID or identity.
- `ipam_allocation` resource and data source: next free CIDR or an explicit CIDR, import by `<pool>/<id>` or identity.
- Documentation, examples and a module/provider contract test.
