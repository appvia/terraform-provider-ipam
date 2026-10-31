---
page_title: "Provider: IPAM"
description: |-
  Allocate non-overlapping IP ranges from pools stored in S3, Azure Blob Storage or a local folder.
---

# IPAM Provider

The IPAM provider hands out non-overlapping CIDRs from pools that live as JSON documents in shared object storage: an S3 bucket, an Azure Blob container or a local directory. There's no server to run. Any number of Terraform workspaces, pipelines and the `ipam` CLI can allocate from the same pools at the same time without handing out the same range twice. It's built on [github.com/appvia/ipam](https://github.com/appvia/ipam).

- [`ipam_pool`](resources/pool.md) declares a root pool or a child pool carved from a parent.
- [`ipam_allocation`](resources/allocation.md) allocates one CIDR from a pool.
- [`data.ipam_pool`](data-sources/pool.md) and [`data.ipam_allocation`](data-sources/allocation.md) read them.

## Example Usage

```terraform
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
```

## Choosing a backend

Configure at most one of `s3`, `azure_blob` or `local`. When none is set, the provider reads a backend URL from `IPAM_BACKEND`, in the same format the `ipam` CLI takes:

| URL | Backend |
| --- | --- |
| `s3://<bucket>/<prefix>?region=..&endpoint=..&path_style=true` | Amazon S3 or an S3-compatible server |
| `azblob://<account>.blob.core.windows.net/<container>/<prefix>` | Azure Blob Storage |
| `file:///absolute/path` | A local directory |

Configuring the provider makes no network calls, so a wrong bucket or missing permission shows up on the first read or write, not in `terraform validate`.

The bucket or container must exist before you apply. The provider never creates or deletes it. Keep it apart from your Terraform state: use a dedicated bucket or container, or at least a `prefix` that doesn't overlap the state key prefix. Pool documents are stored under `pools/<id>.json` and `reservations/<id>.json` below the prefix.

The `local` backend is for development and single-host use. Never point it at a network filesystem.

### Environment variables

| Variable | Purpose |
| --- | --- |
| `IPAM_BACKEND` | Backend URL, used only when no backend block is configured. |
| `IPAM_ACCESS_PROPAGATION_TIMEOUT` | Default for `access_propagation_timeout` when the attribute isn't set. |
| `AWS_*` | AWS SDK credentials and region for `s3`. |
| `ARM_*` | Azure credentials for `azure_blob`. |

## Credentials

Keep credentials in the environment. Nothing in the backend blocks needs a secret.

**AWS** uses the AWS SDK default credential chain: environment variables, shared config and credentials files (`AWS_PROFILE`), web identity (for example GitHub Actions OIDC), ECS and EC2 instance roles. The region comes from `region`, or from `AWS_REGION` or the shared config. A missing region is an error.

**Azure** authenticates exactly like the `azurerm` and `azapi` providers, through aztfauth. Set the usual `ARM_*` variables:

- `ARM_TENANT_ID` and `ARM_CLIENT_ID` with `ARM_CLIENT_SECRET` or `ARM_CLIENT_CERTIFICATE_PATH` for a service principal.
- `ARM_USE_OIDC=true` for GitHub Actions and Azure DevOps workload identity federation. The pipeline token variables (`ACTIONS_ID_TOKEN_REQUEST_*`, `ARM_OIDC_AZURE_SERVICE_CONNECTION_ID`) are picked up.
- `ARM_USE_MSI=true` for a managed identity, or `ARM_USE_AKS_WORKLOAD_IDENTITY=true` on AKS.
- `ARM_USE_CLI` is on by default, so a local `az login` works.

The `azure_blob` block also accepts each of these as an attribute (`tenant_id`, `client_id`, `use_oidc` and so on). The secret ones (`client_secret`, `client_certificate`, `client_certificate_password`, `oidc_token`, `oidc_request_token`) are marked sensitive but still end up in your configuration and plan files, so prefer the environment. Shared keys and SAS tokens aren't supported. `use_connection_string_env` exists only for the Azurite emulator on a loopback address.

## Access propagation timeout

`access_propagation_timeout` (default `5m`, env `IPAM_ACCESS_PROPAGATION_TIMEOUT`) is a Go duration such as `90s` or `10m`. It does two things for Create, Update and Delete:

1. While it runs, an access-denied answer is retried, so a role assignment made earlier in the same apply has time to take effect.
2. It's the deadline for the whole operation, including retries after write conflicts with other writers. When it runs out, the operation fails with `timed out`.

Reads and data sources don't retry access denied. They fail at once, so a missing permission shows up straight away at plan time.

## Concurrency model

Every write is a conditional write against the document's ETag (compare and swap). A new document is written only if it's absent, and an update only if nobody changed it since it was read. When another writer wins, the provider re-reads and tries again with jittered backoff until `access_propagation_timeout` runs out. No locks or leases are used, so a crashed run never leaves a pool locked.

All allocations of a pool live in one document, so writers to one pool contend on one ETag. If many pipelines allocate from the same pool at once, split it into child pools (for example one per region or environment).

## Allocation IDs and leaks

An allocation's ID is a UUIDv7 generated during Create, so it's unknown at plan time. Retries inside one Create reuse the same ID, which turns a write that did land into a success instead of a second allocation.

If Terraform is killed after the write lands but before state is saved, one allocation can leak. Find it and either adopt it or release it:

```sh
ipam alloc list uks
terraform import ipam_allocation.vnet uks/018f22e2-1865-7be8-9f45-35d90607d0f7
# or
ipam alloc release uks 018f22e2-1865-7be8-9f45-35d90607d0f7
```

## Access model

Two kinds of identity use the store:

- The **platform identity** owns the pool hierarchy. It reads, writes and deletes `pools/*` and `reservations/*`, and lists the bucket.
- Each **environment's deploy identity** allocates. It reads `pools/*` and `reservations/*` and may only overwrite `reservations/<pool>.json` for its own pools. It never deletes, because releasing an allocation is an overwrite.

The usual split follows that model. The platform state owns the `ipam_pool` resources. Each environment's state reads its pool with `data.ipam_pool` and owns its `ipam_allocation` resources.

| Role | Needs |
| --- | --- |
| Platform | read, write and delete on `pools/*` and `reservations/*`; list the bucket or container |
| Environment | read on `pools/*` and `reservations/*`; overwrite (no delete) on its own `reservations/<pool>.json`; unconditional `s3:ListBucket` on AWS |
| Reader | read on `pools/*` and `reservations/*`; list |

The [terraform-aws-ipam](https://github.com/appvia/terraform-aws-ipam) and [terraform-azurerm-ipam](https://github.com/appvia/terraform-azurerm-ipam) modules create the storage and these least-privilege grants for you.

### AWS IAM policy for an environment

This is the policy `terraform-aws-ipam` attaches to an environment's roles for bucket `acme-ipam` and pools `uks-prod` and `ukw-prod`. Add `kms:Decrypt` and `kms:GenerateDataKey` on the key if the bucket uses SSE-KMS.

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "ReadDocuments",
      "Effect": "Allow",
      "Action": ["s3:GetObject"],
      "Resource": [
        "arn:aws:s3:::acme-ipam/pools/*",
        "arn:aws:s3:::acme-ipam/reservations/*"
      ]
    },
    {
      "Sid": "WriteOwnReservations",
      "Effect": "Allow",
      "Action": ["s3:PutObject"],
      "Resource": [
        "arn:aws:s3:::acme-ipam/reservations/uks-prod.json",
        "arn:aws:s3:::acme-ipam/reservations/ukw-prod.json"
      ]
    },
    {
      "Sid": "BucketRead",
      "Effect": "Allow",
      "Action": ["s3:ListBucket", "s3:GetBucketVersioning"],
      "Resource": ["arn:aws:s3:::acme-ipam"]
    }
  ]
}
```

`s3:ListBucket` must not have a prefix condition. S3 answers a request for a missing key with 404 only to callers that can list the bucket, and with 403 otherwise, and the provider relies on the 404 to tell a deleted pool from a denied one. The platform policy grants `s3:GetObject`, `s3:PutObject` and `s3:DeleteObject` on `pools/*` and `reservations/*`, plus the same `BucketRead` statement.

### Azure role assignments for an environment

Environment identities get two assignments at container scope. One is `Storage Blob Data Reader`. The other is a data-actions-only custom role holding just `Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read` and `.../blobs/write` (no delete, no add, no management-plane actions), with a version 2.0 condition that allows `blobs/write` only on the environment's own reservations documents. This is the condition `terraform-azurerm-ipam` renders for an environment that owns `uks-prod` and `ukw-prod`:

```text
(
 (
  !(ActionMatches{'Microsoft.Storage/storageAccounts/blobServices/containers/blobs/write'})
 )
 OR
 (
  @Resource[Microsoft.Storage/storageAccounts/blobServices/containers/blobs:path] StringEquals 'reservations/uks-prod.json'
  OR
  @Resource[Microsoft.Storage/storageAccounts/blobServices/containers/blobs:path] StringEquals 'reservations/ukw-prod.json'
 )
)
```

The platform identity gets `Storage Blob Data Contributor` at container scope. Don't give environment identities `Contributor`, which can delete the container. The storage account must not have hierarchical namespace (Data Lake Gen2) enabled.

## Recovery and known limits

- **Versioning.** The documents are the only record of what's allocated. Turn on bucket versioning (S3), or blob versioning and soft delete (Azure). Both modules do this by default.
- **Interrupted pool create.** If an apply is killed while a pool is being created, partial documents can be left behind. The next Create of that pool waits for the whole `access_propagation_timeout` and then fails with `timed out`. `ipam pool get <id>` reports such a partially created pool as not found. For a child pool, the leftover shows in `ipam alloc list <parent>` as a `pool:<child>:<incarnation>` entry. Run `ipam pool delete <id>` to clear the pool's own documents; when the pool's reservations document is the missing one, this also releases the parent's reservation. If `ipam alloc list <parent>` still lists the `pool:<child>:<incarnation>` entry afterwards, release it by its full ID with `ipam alloc release <parent> pool:<child>:<incarnation>`, but only once no create of that child is running anywhere. Then apply again.
- **S3 conditional delete.** Deleting a pool uses a conditional DELETE. The S3 emulators don't enforce it, so it's unproven by the test suite against anything but real AWS.
- **Real-cloud access rules are untested.** The grants from `terraform-aws-ipam` and `terraform-azurerm-ipam` were tested only in plan mode with mocked providers (`terraform test`), and the AWS environment policy only under MinIO's IAM engine. Enforcement by real AWS IAM and by real Azure RBAC and ABAC conditions hasn't been verified, and neither has the Azure credential path (aztfauth) against real Azure. Grant enforcement is checked only by applying the module and running `ipam doctor --expect-write <own pool> --expect-no-write <other pool>` as each identity. `scripts/e2e.sh` has real-cloud legs (`IPAM_TEST_S3_BUCKET` or `AZURE_STORAGE_ACCOUNT_URL`), but they use the caller's own credentials. They prove the real credential path and real conditional PUTs, not the module grants. Conditional DELETE on real S3 is checked by `IPAM_TEST_S3_BUCKET=<bucket> go test -tags integration ./storage/s3` in [appvia/ipam](https://github.com/appvia/ipam).

## Guides

- [Provisioning storage and pools in one apply](guides/single-apply.md)

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `access_propagation_timeout` (String) How long Create, Update and Delete keep retrying access denied while new role assignments propagate; also bounds those operations. Go duration, default "5m". Env: IPAM_ACCESS_PROPAGATION_TIMEOUT.
- `azure_blob` (Attributes) Azure Blob Storage backend (flat namespace accounts only). Authentication attributes and their ARM_* environment fallbacks match the azapi provider. (see [below for nested schema](#nestedatt--azure_blob))
- `local` (Attributes) Directory on a local disk (development and single-host use only; never a network filesystem). (see [below for nested schema](#nestedatt--local))
- `s3` (Attributes) Amazon S3 (or S3-compatible) backend. Credentials come from the AWS SDK default chain. (see [below for nested schema](#nestedatt--s3))

<a id="nestedatt--azure_blob"></a>
### Nested Schema for `azure_blob`

Required:

- `container` (String) Existing container name.

Optional:

- `account_url` (String) Blob service endpoint, https://<account>.blob.core.windows.net. Exactly one of account_url or use_connection_string_env must be set.
- `client_certificate` (String, Sensitive) Base64-encoded PKCS#12 client certificate. Env: ARM_CLIENT_CERTIFICATE.
- `client_certificate_password` (String, Sensitive) Client certificate password. Env: ARM_CLIENT_CERTIFICATE_PASSWORD.
- `client_certificate_path` (String) Path to a PKCS#12 client certificate. Env: ARM_CLIENT_CERTIFICATE_PATH.
- `client_id` (String) Client ID. Env: ARM_CLIENT_ID.
- `client_id_file_path` (String) File containing the client ID. Env: ARM_CLIENT_ID_FILE_PATH.
- `client_secret` (String, Sensitive) Client secret. Env: ARM_CLIENT_SECRET.
- `client_secret_file_path` (String) File containing the client secret. Env: ARM_CLIENT_SECRET_FILE_PATH.
- `oidc_azure_service_connection_id` (String) Azure DevOps service connection ID. Env: ARM_ADO_PIPELINE_SERVICE_CONNECTION_ID, ARM_OIDC_AZURE_SERVICE_CONNECTION_ID.
- `oidc_request_token` (String, Sensitive) Bearer token for the OIDC token request. Env: ARM_OIDC_REQUEST_TOKEN, ACTIONS_ID_TOKEN_REQUEST_TOKEN.
- `oidc_request_url` (String) OIDC token request URL. Env: ARM_OIDC_REQUEST_URL, ACTIONS_ID_TOKEN_REQUEST_URL.
- `oidc_token` (String, Sensitive) OIDC ID token. Env: ARM_OIDC_TOKEN.
- `oidc_token_file_path` (String) File containing an OIDC ID token. Env: ARM_OIDC_TOKEN_FILE_PATH.
- `prefix` (String) Virtual directory under which pool documents are stored.
- `tenant_id` (String) Tenant ID. Env: ARM_TENANT_ID.
- `use_aks_workload_identity` (Boolean) Use AKS workload identity. Env: ARM_USE_AKS_WORKLOAD_IDENTITY.
- `use_cli` (Boolean) Use Azure CLI authentication. Env: ARM_USE_CLI (default true).
- `use_connection_string_env` (Boolean) Use shared-key auth from AZURE_STORAGE_CONNECTION_STRING. Accepted only for the Azurite development account on a loopback endpoint; for tests and local development.
- `use_msi` (Boolean) Use managed identity authentication. Env: ARM_USE_MSI.
- `use_oidc` (Boolean) Use OIDC authentication. Env: ARM_USE_OIDC.


<a id="nestedatt--local"></a>
### Nested Schema for `local`

Required:

- `path` (String) Directory path; created when missing.


<a id="nestedatt--s3"></a>
### Nested Schema for `s3`

Required:

- `bucket` (String) Bucket name.

Optional:

- `endpoint` (String) Custom http(s) endpoint for S3-compatible servers.
- `prefix` (String) Key prefix under which pool documents are stored.
- `region` (String) Bucket region. Defaults to the AWS SDK environment (AWS_REGION, shared config).
- `use_path_style` (Boolean) Use path-style addressing.
