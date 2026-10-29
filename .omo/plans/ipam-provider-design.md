# ipam-provider-design - Work Plan

## TL;DR (For humans)

**Who this is for and what changes for them:** Platform engineers get a Terraform provider that hands out IP ranges that never clash, even when many separate Terraform projects ask at the same time. Developers get a small Go library and command-line tool over the same data, so a future web UI can build on it.

**What you'll get:** A new client library and CLI repository (`appvia/ipam`) that stores IP pools as readable JSON files in an S3 bucket, Azure Blob container or local folder. The existing provider repository becomes a Terraform provider with pool and allocation resources built on that library. Two small Terraform modules, one for Azure and one for AWS, set up the storage and the access rules in one step, and an `ipam doctor` command checks a setup is safe. Everything comes with tests, docs and release tooling.

**Why this approach:** The bucket is the only moving part, so there is no server or database to run. Each write only succeeds if nobody else changed the file since it was read; on S3 and Azure that is a built-in feature, and it is what guarantees no overlaps. Every allocation gets a generated ID before it is written, so retries never create duplicates and two teams can never claim the same allocation.

**What it will NOT do:** It does not talk to AWS, Azure or Google cloud IPAM services, run a server, or build the web front end. It will not create GitHub repositories, push code or publish releases; you do that.

**Effort:** XL
**Risk:** Medium - correctness under concurrency rests on conditional writes. These are proven with real-backend tests, but S3-compatible emulators vary.
**Decisions to sanity-check:**
- Licences are MPL-2.0 for the provider and Apache-2.0 for the client, as you chose. Note that Appvia's existing Wayfinder provider is Apache-2.0.
- Pool definitions and allocations live in separate files. Only the platform team can change pool definitions; each environment's deployment identity can only write allocations for its own pools.
- Allocations get a generated ID instead of a name you choose, so two teams can never accidentally share one. The ID is created once when the allocation is made; retries never duplicate, and the rare crash at exactly the wrong moment can leave one unused range behind, which is safe and easy to clean up.
- Environment pipelines can only read, and update their own environment's allocation files; they can never delete files or containers.
- Reserving a specific range is simply an allocation with a fixed range; there is no separate reservation type.
- A child pool's range is recorded as an allocation in its parent.
- A local-folder backend is added for development and tests.
- The two modules copy Appvia's Azure module template, which is GPL-3.0 licensed, unlike the Apache-2.0 client and MPL-2.0 provider.

Your next move: start execution with `/ulw-execute ipam-provider-design`, after the review result below. Full execution detail follows below.

---

> TL;DR (machine): XL / Medium risk - new repo github.com/appvia/ipam (root package `ipam` client entrypoint + public `storage/*` backends + `internal/{alloc,document}` + cobra CLI, no `pkg/`) and provider appvia/terraform-provider-ipam (ipam_pool, ipam_allocation resources + data sources), pools/<id>.json (platform-owned definition) + reservations/<id>.json (allocations, ETag-CAS), generated UUIDv7 allocation IDs created in Create.

## Scope
### Affected user and ideal state
**Affected user:**
- **U4 - environment pipelines running restricted identities.** They can read pools and overwrite only their environment's reservations documents; everything must work for them without platform rights.
- **U1 - platform engineers.** They write Terraform across many states, subscriptions and accounts. Today they use hand-kept spreadsheets or `cidrsubnet()`, neither of which coordinates across states. After this work they declare `ipam_allocation` and get a CIDR that is safe to use.
- **U2 - Go developers.** They build the `ipam` CLI now and a web front end later, both on `github.com/appvia/ipam`.
- **U3 - operators.** They inspect and repair pool documents in the bucket.

| Row | Statement | Reason |
| --- | --- | --- |
| IS-1 | Concurrent applies from different states/processes never receive overlapping CIDRs from the same pool tree | Core IPAM promise; state-only allocators cannot coordinate (research claim-graph C1) |
| IS-2 | A retry inside one Create (lost response, timeout, CAS conflict) returns the same CIDR for the same allocation ID; never a duplicate. A hard crash between the write and Terraform saving state can leak at most that one allocation, which is safe (never overlaps) and recoverable (CLI list, then import or release) | Idempotent create; Xtratus provider lacked it (C10) |
| IS-3 | Allocated CIDR is known after apply, stable across plans, refresh shows drift (deleted allocation -> recreate), allocations/pools importable | TF framework guidance (lane a6) |
| IS-4 | Destroy releases the CIDR; a pool with allocations or child pools cannot be deleted | No leaked/orphaned space |
| IS-5 | S3, Azure Blob and local backends behave identically; backend chosen only in provider/CLI config | User: Go interface abstraction |
| IS-6 | Library usable without Terraform: typed Go API + `ipam` CLI with JSON output | User: two repos, CLI/front end |
| IS-7 | Bucket state is human-readable, versioned JSON (schema_version), unknown versions never overwritten | Operator inspection/repair |
| IS-8 | Nothing to operate except a bucket/container and the credentials teams already use for azurerm/azapi (aztfauth, `ARM_*`) or the AWS SDK chain | User: simple |
| IS-9 | Only the platform identity can change pool definitions or delete documents; an environment's deploy identity can only read, and overwrite (never delete) the reservations of that environment's pools, enforceable with plain S3 IAM object ARNs or a data-actions-only Azure custom role with blob-path conditions | User: storage as the permission boundary; pools are per location/environment |
| IS-10 | Two allocations can never share an identity, so a CIDR is never adopted by two owners or two states | Key collisions across teams in a shared pool would silently produce overlapping networks |
| IS-11 | One module call per cloud provisions a hardened backend with the correct platform and per-environment access rules, and its outputs configure the ipam provider directly; `ipam doctor` detects the misconfigurations that silently break the design (versioning off where readable, hierarchical namespace on, ignored conditional writes, wrong write access); on Azure, versioning that cannot be read with container-scoped data roles reports WARN, not FAIL | Platform teams need a simple, repeatable way to stand this up |
| GAP-1 | No allocator or concurrency control exists (repo only has research, commit 94b4c59) | IS-1 unmet |
| GAP-2 | No idempotency contract exists | IS-2 unmet |
| GAP-3 | No Terraform resources exist | IS-3/IS-4 unmet |
| GAP-4 | No deletion guards exist | IS-4 unmet |
| GAP-5 | No storage abstraction or backends exist | IS-5/IS-8 unmet |
| GAP-6 | No client repo, Go API or CLI exists | IS-6 unmet |
| GAP-7 | No document schema exists | IS-7 unmet |
| GAP-8 | No storage layout separates platform-owned data from requestor-writable data | IS-9 unmet |
| GAP-9 | No allocation identity scheme exists | IS-10 unmet |
| GAP-10 | No provisioning module or setup check exists | IS-11 unmet |

### Must have
- Client repo `/Volumes/code/ipam`, module `github.com/appvia/ipam`, Apache-2.0, with these packages:
  - No `pkg/` directory. The module root is the client entrypoint.
  - `ipam` (module root, import path `github.com/appvia/ipam`): `Client`, `New`, `Open`, the public types `Pool`/`Allocation`/`PoolSpec`/`AllocationSpec`, and the typed errors.
  - `storage`: the public `Storage` interface and its errors. It must stay public: it appears in `ipam.New`'s signature, and the provider is a separate module that cannot import `internal/`.
  - `storage/{memory,local,s3,azblob}`: the public backends, so the provider and other callers can build them with typed config.
  - `storage/storagetest`: the shared contract suite, public so third-party backends can run it.
  - `internal/alloc`: the pure CIDR allocator over `netip` types.
  - `internal/document`: the v1 on-disk JSON schema codec, kept separate so the public Go types and the stored format can change independently.
  - `cmd/ipam`: the cobra CLI.
- Provider repo `/Volumes/code/terraform-provider-ipam`, module `github.com/appvia/terraform-provider-ipam`, MPL-2.0, terraform-plugin-framework, protocol 6.0, registry address `registry.terraform.io/appvia/ipam`, with:
  - resources `ipam_pool` and `ipam_allocation`;
  - data sources `ipam_pool` and `ipam_allocation`.
- IPv4 and IPv6. Each pool is single-family and may hold multiple disjoint CIDRs.
- Explicit-CIDR allocation, which is how reservations are expressed.
- Child pools via `parent_pool`.
- **Storage layout** (under one backend root): `pools/<pool_id>.json` holds the pool definition and is written only by the platform identity. `reservations/<pool_id>.json` holds that pool's allocations and is written by the environment's deploy identity and by the platform identity (for carving child pools).
- **Allocation IDs:** UUIDv7, generated exactly once per Create (library `Allocate` generates it before its CAS loop when none is supplied; the provider leaves `id` unknown at plan time and lets Create generate it; the CLI accepts an optional `--id`). The ID is the only identity. Idempotency: same ID with the same request returns the existing allocation; same ID with a different request is `ErrConflict` with Reason `id_reused`.
- **Provisioning modules** in separate repos, `/Volumes/code/terraform-azurerm-ipam` and `/Volumes/code/terraform-aws-ipam`, built from the Appvia module template: hardened storage plus platform and per-environment access rules, outputting the backend URL. Plus a read-only `ipam doctor` CLI command.
- **Documented access patterns** for S3 (IAM object ARNs) and Azure (role-assignment conditions on blob path), with copy-pasteable examples.

### Must NOT have (guardrails, anti-slop, scope boundaries)
- No server, daemon, database, lock service or DynamoDB table.
- No integration with AWS VPC IPAM, Azure/ipam, AVNM or GCP; the research rejected a backend-agnostic cloud-IPAM provider.
- No "preview next CIDR" or next-available data source, because it is advisory and causes duplicates (C1, AWS #25958).
- No ephemeral resources, write-only attributes, actions or TF 1.14 list resources.
- No gocloud.dev: it lacks If-Match writes, so use the native SDKs.
- No web front end and no gRPC/HTTP API.
- No separate reservation object type and no scopes/spaces concept.
- No caller-chosen allocation key and no `owner` attribute; the generated ID is the only identity (labels may carry team/owner as free metadata).
- No allocation ID derived from configuration (a hash of attributes collides for `count` and across states), and no ID generated in `ModifyPlan` (Terraform re-plans during apply and requires identical known values, so a random plan-time ID fails with "Provider produced inconsistent final plan").
- No cloud IAM/RBAC management in the library, CLI or provider; that lives only in the provisioning modules (tasks 18-19). The modules never create pools or configure the ipam provider.
- No Azure hierarchical-namespace (ADLS Gen2) support: renames can bypass path conditions; the azblob backend's docs require a flat-namespace account.
- No in-place update of pool CIDRs, parents or allocation requests: those changes force replacement. `description` and `labels` on pools and allocations update in place (metadata-only CAS write).
- Do not create GitHub repos, push, tag or publish releases.
- Do not modify `.omo/ulw-research/**`.
- Environment identities never get delete permissions on any object, and by default never get management-plane actions (Azure custom role with data actions only). The only exception is the Azure module's explicit `environment_role = "builtin"` opt-in, which is documented as leaving container deletion possible.
- Shared-key auth only for the Azurite well-known development connection string (used by tests); never for real accounts.
- No access-denied retries outside provider `Create`/`Update`/`Delete` (and only with the propagation flag and a deadline); `Read`, data sources, the CLI and `ipam doctor` never retry access-denied; never retry 412 at the SDK layer.
- No home-grown Azure credential chain: Azure auth goes through `github.com/Azure/entrauth/aztfauth` only, and the `ARM_*` env/attribute mapping onto `aztfauth.Option` exists in exactly one place (`azblob.AuthConfig`, task 8) shared by CLI and provider. AWS uses the SDK default credential chain only. The provider's `azure_blob` block mirrors the azapi provider's auth attribute names and `ARM_*` env fallbacks. Secret-bearing attributes (`client_secret`, `client_certificate`, `client_certificate_password`, `oidc_token`, `oidc_request_token`) are `Sensitive`; provider configuration is never stored in state. No secrets in backend URLs or CLI flags (env/files only).

### Constraints (adopted defaults, reversible)
- **Scale:** designed for up to 5,000 allocations and ~10 concurrent writers per pool; beyond that, split pools.
- **Spend:** $0 for verification; real-cloud test legs are opt-in via env vars.
- **Versions:** Terraform >= 1.12 (resource identity); OpenTofu untested and not claimed.
- **Azure cloud:** public cloud only (no `environment` attribute).
- **Platforms:** CLI release builds for linux/darwin/windows; Windows file locking is compiled and unit-tested on a Windows CI runner.

## Verification strategy
> Zero human intervention - all verification is agent-executed.
- **Test decision:** TDD (user decision D4).
  - Allocator and storage contract: write failing table tests first.
  - Provider: `terraform-plugin-testing` acceptance tests (`TF_ACC=1`) against the `local` backend (file path in `t.TempDir()`), using the Terraform CLI already on PATH (`terraform` v1.17.0).
- **Real-backend integration:** gated by Go build tag `integration` plus env vars.
  - Azure Blob: Azurite started with `npx --yes azurite-blob --silent --location <tmp> --blobPort 10000`. Docker is NOT available on this machine.
  - S3: MinIO server binary (`brew install minio/stable/minio` or a direct download into `$TMPDIR`), started with `minio server <tmp> --address :9000`. Optional real AWS via `IPAM_TEST_S3_BUCKET`.
- **Lint:** `golangci-lint run` (on PATH) with the `.golangci.yml` copied from `/Volumes/code/terraform-provider-wayfinder/.golangci.yml`.
- **Evidence:** `<attemptDir>/task-<N>-ipam-provider-design.<ext>`.
  - `attemptDir` is `result.currentAttemptDir` from `await agentToolkit.status()`, run in a JS eval cell after `agentToolkit` is imported with ``await import(`${env("OMO_AGENT_TOOLKIT_SDK_ROOT")}/sdk.js`)``.
  - Outside ulw-loop, use `.omo/evidence/` in the provider repo.

## Execution strategy
### Parallel execution waves
- **Wave 1 (client foundation):** 1, 2, 3, 4, 21. Todo 1 first, then 2, 3 and 21 in parallel, then 4.
- **Wave 2 (client service + backends):** 6, 7, 8 in parallel after 2 (7 and 8 also after 21); then 5 after 2, 3, 4 and 6.
- **Wave 3 (client surface):** 9, then 10, then 20, then 11.
- **Wave 4 (provider):** 12, 13, 14, 15, 16. 12 can start in parallel with Wave 2.
- **Wave 5 (end-to-end):** 17.
- **Independent:** 18 and 19 (separate module repos) can run from Wave 1 onwards, but must finish before 16 (16 tests module outputs against the provider schema).

### Dependency matrix
| Todo | Depends on | Blocks | Can parallelize with |
| --- | --- | --- | --- |
| 1 | - | 2,3,12,21 | 18,19 |
| 2 | 1 | 5,6,7,8 | 3 |
| 3 | 1 | 4,5 | 2 |
| 4 | 3 | 5 | 2 |
| 5 | 2,3,4,6 | 9,10,13 | 7,8 |
| 6 | 2 | 5,9 | 7,8 |
| 7 | 2,21 | 9,17,20 | 5,6,8 |
| 8 | 2,21 | 9,13,17,20 | 5,6,7 |
| 9 | 5,6,7,8 | 10,13 | 12 |
| 10 | 9 | 11,17,20 | 12-15 |
| 11 | 10,18,19,20 | 16 | 12-15 |
| 12 | 1 | 13 | 2-8 |
| 13 | 8,9,12 | 14,15 | 11,20 |
| 14 | 13 | 15,16 | - |
| 15 | 14 | 16,17 | - |
| 16 | 11,15,18,19 | 17 | - |
| 17 | 7,8,10,16,20,21 | F1-F4 | - |
| 18 | - | 16,F1-F4 | 1-15 |
| 19 | - | 16,F1-F4 | 1-15 |
| 20 | 7,8,10,21 | 11,17 | 12-16 |
| 21 | 1 | 7,8,17,20 | 2,3,4,18,19 |

## Todos
> Implementation + Test = ONE todo. Never separate.
<!-- APPEND TASK BATCHES BELOW THIS LINE WITH edit/apply_patch - never rewrite the headers above. -->
- [ ] 1. Scaffold client repo github.com/appvia/ipam
  - **What to do:**
    - `mkdir /Volumes/code/ipam && git init`.
    - Create `go.mod` (`module github.com/appvia/ipam`, `go 1.25` minimum, toolchain as local `go1.27.3` allows).
    - Add `LICENSE` (Apache-2.0 full text, copy from `/Volumes/code/terraform-provider-wayfinder/LICENSE`).
    - Add `.gitignore` (`/dist`, `/ipam`, `.codegraph/`, `*.test`).
    - Add `Makefile` with targets `build`, `test` (`go test ./... -count=1 -race`), `test-integration` (`go test -tags integration ./... -count=1`), `lint` (`golangci-lint run`).
    - Add `.golangci.yml`, copied from the wayfinder repo.
    - Add `mise.toml` pinning go, modelled on `/Volumes/code/terraform-provider-wayfinder/mise.toml`.
    - Add `.github/workflows/test.yml`: go test + lint on push/PR (ubuntu), a `windows-latest` job running `go test ./storage/local/...` (covers the Windows file-lock helper), and an integration job running the task 21 emulator scripts.
    - Add a `README.md` skeleton with sections Overview, Concepts, Storage backends, CLI, Library usage.
    - Add `doc.go` stubs for the root package `ipam` (repo root) and `storage/`. No `pkg/` directory.
    - Run `codegraph init` and make sure `.codegraph/` is gitignored.
  - **Must NOT do:**
    - Do not create a GitHub remote or push.
    - Do not add a `replace` directive.
  - **Closes:** GAP-6
  - **Parallelization:** Wave 1 | Blocked by: - | Blocks: 2,3,12,21
  - **References:**
    - `/Volumes/code/terraform-provider-wayfinder/{Makefile,.golangci.yml,mise.toml,.github/workflows/test.yml,LICENSE}`
    - Draft D2/D4: `/Volumes/code/terraform-provider-ipam/.omo/drafts/ipam-provider-design.md`.
  - **Acceptance criteria:**
    - `cd /Volumes/code/ipam && go build ./... && make lint` both exit 0.
    - `head -1 LICENSE` contains `Apache License`.
  - **QA scenarios:**
    - **Happy:** `go vet ./...` exits 0.
    - **Failure:** `git -C /Volumes/code/ipam remote -v` prints nothing (no remote created).
    - **Evidence:** `<attemptDir>/task-1-ipam-provider-design.txt`
  - **Commit:** Y | `chore: scaffold ipam client repository`
  - **Recommended task executor category:** `quick` (mechanical scaffolding from a sibling repo).

- [ ] 2. Storage interface, errors, memory backend, contract suite
  - **What to do (TDD):** In `storage/storage.go` define:
    ```go
    type Version string // opaque ETag, round-tripped unchanged
    type Storage interface {
      Get(ctx context.Context, key string) ([]byte, Version, error)
      CreateIfAbsent(ctx context.Context, key string, data []byte) (Version, error)
      PutIfMatch(ctx context.Context, key string, data []byte, v Version) (Version, error)
      DeleteIfMatch(ctx context.Context, key string, v Version) error
      List(ctx context.Context, prefix string) ([]string, error) // keys, sorted
    }
    ```
    - Sentinel errors in `storage/errors.go`:
      - `ErrNotFound` for a missing object.
      - `ErrPreconditionFailed` for an ETag mismatch, or an existing object on create.
      - `ErrAccessDenied` for an authorisation failure (HTTP 403 / `fs.ErrPermission`).
      - `ErrUnknownOutcome` for a conditional write or delete whose request may have reached the server but whose result is unknown (client timeout, connection reset after sending, 5xx after the SDK's own retries are exhausted). Backends wrap such errors with it; errors raised before anything was sent (DNS, auth token acquisition, request building) are NOT wrapped.
    - Context helpers in `storage/context.go`: `WithAccessPropagationRetry(ctx) context.Context` and `AccessPropagationRetry(ctx) bool`. Backends use the flag to retry access-denied responses while new cloud role assignments propagate; callers bound the wait with the context deadline. The memory backend ignores it.
    - `storage/memory`:
      - Mutex-guarded map; data is copied on read and write.
      - Monotonic integer version.
      - A hook (`BeforeWrite func(op, key string)`) so tests can inject barriers.
    - `storage/storagetest.Run(t *testing.T, newStore func(t *testing.T) storage.Storage)` covers:
      - Get on a missing key gives `ErrNotFound`.
      - Create then Get round-trips bytes and version.
      - A second Create gives `ErrPreconditionFailed`.
      - PutIfMatch with a stale version gives `ErrPreconditionFailed`, and the data is unchanged.
      - PutIfMatch with the current version succeeds with a new version.
      - DeleteIfMatch with a stale version gives `ErrPreconditionFailed`.
      - DeleteIfMatch on a missing key gives `ErrNotFound` or `ErrPreconditionFailed` and deletes nothing.
      - PutIfMatch on a missing key gives `ErrNotFound` or `ErrPreconditionFailed` and never creates the object (assert Get still returns `ErrNotFound`).
      - List under a prefix returns only that prefix, sorted.
      - 8 goroutines PutIfMatch the same version: exactly 1 succeeds.
    - The memory backend runs the suite.
  - **Must NOT do:**
    - No retries inside backends.
    - No locking API beyond conditional ops.
  - **Closes:** GAP-5
  - **Parallelization:** Wave 1 | Blocked by: 1 | Blocks: 5,6,7,8
  - **References:**
    - Ultrabrain spec, recorded in draft D9: conditional ops, error mapping, monotonic ETag memory store.
    - S3 conditional writes: https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html
  - **Acceptance criteria:** `go test ./storage/... -race -count=1` passes and includes `TestMemoryContract`.
  - **QA scenarios:**
    - **Happy:** the contract suite passes on memory.
    - **Failure:** temporarily breaking the memory backend's version check, in a scratch copy only, makes `storagetest` fail. Record the output, then discard the scratch copy.
    - **Evidence:** `<attemptDir>/task-2-ipam-provider-design.txt`
  - **Commit:** Y | `feat(storage): add storage interface, memory backend and contract suite`
  - **Recommended task executor category:** `deep-low` (concurrency contract correctness).

- [ ] 3. Domain model, canonicalisation and JSON schema v1 (pool and reservations documents)
  - **What to do (TDD):** Public types live in the root package `ipam` (`types.go`, `canonical.go`). The JSON codecs live in `internal/document`, which defines its own v1 structs and must NOT import the root package (no import cycle). The root package converts between public types and documents.
    - **Types:** `Pool{ID, Family(4|6), CIDRs []netip.Prefix, Parent *ParentRef{PoolID, AllocationID}, Description, Labels map[string]string}`, `Allocation{ID, PoolID, Request Request, CIDR netip.Prefix, Description, Labels, CreatedAt time.Time}`, and `Request{Kind:"next"|"explicit", PrefixLength int, CIDR netip.Prefix}`.
    - **Pool ID:** regex `^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`.
    - **Allocation ID:** either a canonical lowercase UUID (any version accepted on input; generated IDs are UUIDv7 via `github.com/google/uuid` `NewV7`) or the reserved form `pool:<pool_id>` used only for child-pool carve-outs. Anything else is `ErrInvalidArgument`.
    - **Canonicalisation:**
      - Use `netip.ParsePrefix` and store `Masked()`.
      - Reject zones.
      - Turn IPv4-mapped `::ffff:a.b.c.d/(96+n)` into `a.b.c.d/n`, and reject a mapped prefix shorter than /96.
      - A pool must have at least 1 CIDR, all one family.
      - Reject duplicate or overlapping CIDRs (adjacent ones are fine) and sort them.
    - **Incarnation:** both documents carry `"incarnation"`: 32 lowercase hex chars from `crypto/rand`, generated once per CreatePool and written identically into both documents. It defeats ETag reuse (S3 single-part ETags are content MD5s; local versions are content hashes), so a deleted-and-recreated pool can never accept a write computed against its predecessor.
    - **Pool document** (`<root>pools/<pool_id>.json`): `{"schema_version":1,"kind":"pool","pool_id":..,"incarnation":..,"family":4,"cidrs":[..],"parent":{"pool_id":..,"allocation_id":"pool:<pool_id>"}|null,"description":..,"labels":{..}}`.
    - **Reservations document** (`<root>reservations/<pool_id>.json`): `{"schema_version":1,"kind":"reservations","pool_id":..,"incarnation":..,"allocations":{"<id>":{"request":{"kind":"next","prefix_length":24}|{"kind":"explicit","cidr":".."},"cidr":"..","description":..,"labels":{..},"created_at":RFC3339}}}`.
    - **Codec rules (both documents):**
      - Encode with sorted keys and two-space indentation, so diffs are stable.
      - Decode rejects a missing `schema_version` or wrong `kind`, and returns `ErrUnsupportedSchema` for any version other than 1.
      - Decode checks `pool_id` matches the object key.
      - Allocations are validated against their pool (same family, wholly inside one pool CIDR, pairwise non-overlapping) by a root-package function `validate(pool, reservations)`, run on every read before mutation; `validate` also requires equal `incarnation` values (mismatch gives `ErrConflict` with Reason `incarnation_mismatch`, retried by the CAS loop as a fresh read).
    - **Typed errors** (`errors.Is`-able, in root `errors.go`; `internal/document` returns its own sentinels, which the root package wraps into `ipam.ErrUnsupportedSchema`/`ErrInvalidCIDR`): `ErrNotFound`, `ErrConflict`, `ErrPoolExhausted`, `ErrInvalidCIDR`, `ErrInvalidArgument`, `ErrUnsupportedSchema`. Also `*OpError{Op, Pool, ID, Reason string, Err error}` with `Unwrap`.
  - **Must NOT do:**
    - Do not put the ETag in either document.
    - Do not store allocations in the pool document.
    - No reservation type, no key, no owner field.
  - **Closes:** GAP-7, GAP-8, GAP-9
  - **Parallelization:** Wave 1 | Blocked by: 1 | Blocks: 4,5
  - **References:** draft D1, D9, D10, D11, D16, D17; ultrabrain spec sections 1 and 6, folded into D9.
  - **Acceptance criteria:**
    - `go test . ./internal/document -run 'TestCanonical|TestCodec|TestValidate' -count=1` passes.
    - Golden files `internal/document/testdata/pool_v1.json` and `internal/document/testdata/reservations_v1.json` round-trip byte-identically.
  - **QA scenarios:**
    - **Happy:** `10.0.0.5/16` is stored as `10.0.0.0/16`; `::ffff:10.0.0.0/104` is stored as `10.0.0.0/8`; a generated UUIDv7 ID round-trips.
    - **Failure:** `schema_version: 2` gives `ErrUnsupportedSchema`; a mixed-family pool gives `ErrInvalidCIDR`; a reservations document containing an allocation outside the pool gives `ErrInvalidCIDR`; an ID `vnet-main` gives `ErrInvalidArgument`.
    - **Evidence:** `<attemptDir>/task-3-ipam-provider-design.txt`
  - **Commit:** Y | `feat(ipam): add pool and reservations models with v1 document codecs`
  - **Recommended task executor category:** `deep-low` (exact invariants).

- [ ] 4. Pure CIDR allocator (next-available and explicit)
  - **What to do (TDD):** Write `internal/alloc/alloc.go` as pure functions over `netip` types only, with no I/O and no import of the root package. Signatures: `Next(parents, used []netip.Prefix, length int) (netip.Prefix, error)` and `CheckExplicit(parents, used []netip.Prefix, cidr netip.Prefix) error`. It returns `alloc.ErrInvalidCIDR`, `alloc.ErrExhausted` and `alloc.ErrOverlap`, which the root package maps to `ipam.ErrInvalidCIDR`, `ipam.ErrPoolExhausted` and `ipam.ErrConflict` (Reason `overlap`).
    - **`Next`:**
      - `length` must be within the family's width and `>=` the bits of at least one parent CIDR, otherwise `ErrInvalidCIDR`.
      - For each eligible parent CIDR on its own:
        1. Build a `go4.org/netipx.IPSetBuilder`, add the parent, and remove every existing allocation CIDR.
        2. From the free set's `Prefixes()`, take every free block `f` with `f.Bits() <= length`; the candidate is `netip.PrefixFrom(f.Addr(), length).Masked()`.
      - Pick the candidate with the lowest address across all parents.
      - No candidate gives `ErrPoolExhausted`.
    - **`CheckExplicit`:**
      - The CIDR must sit wholly inside one parent CIDR and match the family, otherwise `ErrInvalidCIDR`.
      - Any intersection with an existing allocation gives `ErrConflict` with Reason `overlap`.
    - **Tests:** the ultrabrain matrix rows for Next IPv4/IPv6, multiple parents and explicit allocation.
      - The result never spans adjacent parents.
      - An unaligned explicit allocation still yields aligned next results.
      - A `/64` in a `2001:db8::/48` pool works.
      - Include a property test: 500 random sequences of next/explicit/release on a `/16` must never produce overlaps.
  - **Must NOT do:** no storage access and no randomness in selection.
  - **Closes:** GAP-1
  - **Parallelization:** Wave 1 | Blocked by: 3 | Blocks: 5
  - **References:** draft D6, D9; go4.org/netipx `IPSetBuilder`, `IPSet.Prefixes`; Go `net/netip`.
  - **Acceptance criteria:** `go test ./internal/alloc -count=1 -race` passes, including `TestNext`, `TestExplicit` and `TestProperty`.
  - **QA scenarios:**
    - **Happy:** in an empty `10.0.0.0/16`, next `/24` gives `10.0.0.0/24`, then `10.0.1.0/24`; with an explicit `10.0.0.128/25` held, next `/24` gives `10.0.1.0/24`.
    - **Failure:** a `/24` pool asked for a `/16` gives `ErrInvalidCIDR`; a full pool gives `ErrPoolExhausted`.
    - **Evidence:** `<attemptDir>/task-4-ipam-provider-design.txt`
  - **Commit:** Y | `feat(alloc): add deterministic CIDR allocator`
  - **Recommended task executor category:** `deep-low` (algorithmic, spec given).

- [ ] 5. Client service with CAS loop, generated IDs, split documents and child pools
  - **Location:** the root package `ipam` (`client.go`, `pool.go`, `allocation.go`), using `internal/alloc` and `internal/document`.
  - **Constructor:** `ipam.New(store storage.Storage, opts ...Option) *Client`, with options `WithMaxAttempts(8)` (0 = until deadline), `WithBackoff(base 10ms, cap 250ms)`, `WithClock`, `WithRand`, `WithSleeper`, `WithIDGenerator`, all injectable for tests.
  - **Exported helper:** `ipam.NewAllocationID() string` (UUIDv7, canonical lowercase). The provider calls it once at the start of Create.
  - **Attempt bound:** `WithMaxAttempts(n)`; `n = 0` means "retry until the context deadline". The library default is 8; the provider uses 0 with its operation deadline, so concurrent applies against one pool are bounded by time, not by attempt count.
  - **Methods** (all `context.Context` first):
    - `CreatePool(ctx, PoolSpec{ID, CIDRs | (ParentPool + Request), Description, Labels}) (Pool, error)`
    - `GetPool`
    - `ListPools` (List over `pools/`)
    - `DeletePool`
    - `Allocate(ctx, poolID, AllocationSpec{ID (optional), PrefixLength | CIDR, Description, Labels}) (Allocation, error)`. An empty ID means the client generates one before the CAS loop starts, so its own retries stay idempotent.
    - `GetAllocation(ctx, poolID, id)`
    - `ListAllocations(ctx, poolID)`
    - `Release(ctx, poolID, id) error`
    - `UpdatePoolMetadata(ctx, poolID, description, labels)`: platform-only write to the pool document (CAS on the pool document version; never changes CIDRs, parent or incarnation).
    - `UpdateAllocationMetadata(ctx, poolID, id, description, labels)`: CAS write to the reservations document changing only that allocation's description and labels.
  - **Reads:** every mutation of a pool's allocations reads `pools/<id>.json` and `reservations/<id>.json` (with version), then runs `validate` (including the incarnation check). Only the reservations document is written.
  - **CAS loop:**
    - Each attempt: Get, decode, mutate as a pure step, then `PutIfMatch` on the reservations document.
    - Retry set (full-jitter backoff `uniform(0, min(base*2^n, cap))`, cancellable through ctx):
      - `storage.ErrPreconditionFailed`: re-read and recompute.
      - `ErrConflict` with Reason `incarnation_mismatch` (raced between the two document reads): re-read and recompute.
      - `storage.ErrUnknownOutcome`: re-read; if the reservations document already contains this operation's allocation ID with the same request (or, for Release, no longer contains it), return success without writing; otherwise recompute and retry. This is safe because the allocation ID is fixed before the loop starts.
      - Every other error is returned unchanged.
    - After max attempts, return `ErrConflict` (Reason `contention`) wrapping the last error.
  - **In-process serialisation:** a per-pool `sync.Mutex` (map guarded by a mutex) serialises mutations from one `Client`, so parallel Terraform resources in one apply do not burn CAS attempts. Correctness still comes from CAS.
  - **Idempotency (by ID):**
    - Look up the ID before checking availability.
    - Same ID with the same canonical request: return the existing allocation with zero writes.
    - Same ID with a different request: `ErrConflict` (Reason `id_reused`), with both descriptors in the message.
    - User-supplied IDs of the form `pool:...` are `ErrInvalidArgument`.
  - **Release:** an absent ID in an existing pool is a no-op; an absent pool gives `ErrNotFound`.
  - **CreatePool (root, no parent):**
    1. Generate a fresh incarnation.
    2. `CreateIfAbsent(reservations/<id>.json)` with an empty allocations map. If it already exists and is empty, adopt its incarnation (convergent retry or a concurrent creator with the same spec; the pool document check in step 3 then decides); if it has allocations, `ErrConflict` (Reason `pool_exists`).
    3. `CreateIfAbsent(pools/<id>.json)` with the same incarnation. If it exists with an identical spec and incarnation, return it; otherwise `ErrConflict` (Reason `pool_exists`).
  - **CreatePool with parent:**
    1. `Allocate(parent, {ID:"pool:<id>", ...})`. This is idempotent.
    2. Then the two root steps above, with `Parent` set and `CIDRs` set to the carved prefix.
  - **Allocate on a pool whose pool or reservations document is missing:** `ErrNotFound`. Never create a reservations document from `Allocate`, so a deleted pool cannot be resurrected by a racing requestor.
  - **DeletePool** (order chosen so no crash point can strand a parent carve-out):
    1. Read the pool document (keep its parent reference). If it is missing, go to step 5.
    2. Read the reservations document. If it has allocations (child pools appear as `pool:<child>` allocations), `ErrConflict` (Reason `pool_not_empty`).
    3. `DeleteIfMatch(reservations)` with the version read (retry from step 1 on precondition failure). A missing reservations document is tolerated (half-deleted pool).
    4. If the pool has a parent, `Release(parent, "pool:<id>")` (idempotent; a missing parent is tolerated).
    5. Delete the pool document (read, then `DeleteIfMatch`). If the pool document was already missing at step 1, scan every pool's reservations document (`ListPools`) for an allocation `pool:<id>` and release it, so a crash after step 5 of an earlier attempt still converges. Return `ErrNotFound` only if nothing at all was found.
  - **Tests** (memory store + barrier hook, no sleeps):
    - K=8 goroutines allocating without IDs all succeed, and the result equals the first 8 non-overlapping `/24`s.
    - 8 goroutines with the same explicit ID and request converge on one CIDR.
    - Same ID with a different request gives `id_reused`.
    - 8 concurrent CreatePool calls with the same spec: all return the same pool; with different specs: one wins and the rest get `ErrConflict`.
    - Injected permanent 412 is attempted exactly MaxAttempts times, then wraps the error.
    - Lost response: a store wrapper that commits a `PutIfMatch` and then returns an error wrapping `storage.ErrUnknownOutcome` once; `Allocate` succeeds, returns the committed allocation, and exactly one allocation exists.
    - An allocate racing a DeletePool either succeeds before the delete (and the delete fails `pool_not_empty`) or fails `ErrNotFound`; no allocation is ever lost or orphaned.
    - A crash between the parent allocation and the child create (simulated by a store error), followed by a retry, converges.
    - A crash after each DeletePool step (3, 4, 5), followed by a retry, completes and leaves no `pool:<id>` carve-out in the parent.
    - ABA race: an Allocate reads pool P (incarnation A) and its empty reservations doc; P is deleted and recreated with different CIDRs (incarnation B) whose empty reservations doc would be byte-identical without the incarnation token (so on the local backend, whose versions are content hashes, the ETag would be reused); the Allocate's PutIfMatch is then rejected or its retry recomputes against B. Never is a CIDR outside B's CIDRs committed Also run a negative control that neutralises the token itself: inject `WithRand` so the old and new pools get the SAME incarnation, and interleave so the Allocate reads the OLD pool document before the recreate and the NEW (byte-identical) reservations document after it; with equal tokens the stale computation commits a CIDR outside B, and the test asserts that this corruption DOES occur, proving the distinct per-incarnation token is what prevents it in the real configuration.
    - `UpdateAllocationMetadata` changes only description/labels and preserves the CIDR and ID.
    - A reservations document with `schema_version: 2` makes `Allocate` return `ErrUnsupportedSchema` with zero writes (recording store).
    - Deleting the parent while a child exists gives `pool_not_empty`.
    - `Allocate` never writes `pools/*` (assert via a store wrapper that records written keys).
    - Use two `Client` instances on the same memory store to prove the cross-process path; the mutex does not cover it.
  - **Must NOT do:**
    - No global locks in storage.
    - No `time.Sleep` in tests: use the injected sleeper.
  - **Closes:** GAP-1, GAP-2, GAP-4, GAP-8, GAP-9
  - **Parallelization:** Wave 2 | Blocked by: 2,3,4,6 | Blocks: 9,10,13
  - **References:** draft D7, D9, D11, D16, D17; ultrabrain spec sections 3-5 and 7; research C1/C10 in `.omo/ulw-research/20261028-150017/claim-graph.md`.
  - **Acceptance criteria:** `go test . -race -count=3` passes (root package). Three runs prove there is no flakiness.
  - **QA scenarios:**
    - **Happy:** the 8-goroutine test passes with `-race`.
    - **Failure:** a permanent-412 store returns `errors.Is(err, ipam.ErrConflict)` after exactly 8 Put calls (asserted counter); `Allocate` on a pool with no reservations document returns `ErrNotFound` and writes nothing.
    - **Evidence:** `<attemptDir>/task-5-ipam-provider-design.txt`
  - **Commit:** Y | `feat(ipam): add client service with optimistic concurrency, generated IDs and child pools`
  - **Recommended task executor category:** `deep-high` (correctness argued from concurrency invariants).

- [ ] 6. Local filesystem backend
  - **What to do:** `storage/local.New(dir string)`.
    - Object key maps to a file under `dir`, with `/` becoming nested dirs; reject keys containing `..`.
    - The version is the hex sha256 of the content.
    - Each conditional op holds an exclusive `flock` on `<dir>/.lock`, using `golang.org/x/sys/unix` on unix and `windows.LockFileEx` on Windows, behind a build-tagged helper.
    - Writes go to a temp file, then fsync, then rename. (The local backend has no unknown-outcome case: errors after the rename are impossible, and errors before it leave the file unchanged.)
    - `List` walks the directory sorted, ignoring `.lock` and temp files.
    - Runs `storagetest.Run`.
    - Add a cross-process test: re-exec the test binary 4 times, each child doing 25 CAS increments on one key; the final counter must be 100.
  - **Must NOT do:** never pretend the backend is safe on network filesystems; the doc comment states local disks only.
  - **Closes:** GAP-5
  - **Parallelization:** Wave 2 | Blocked by: 2 | Blocks: 5,9
  - **References:** draft D12; `storagetest` from todo 2.
  - **Acceptance criteria:** `go test ./storage/local -race -count=1` passes, including `TestCrossProcess`.
  - **QA scenarios:**
    - **Happy:** the contract suite plus the cross-process counter equals 100.
    - **Failure:** key `../x` gives `ErrInvalidArgument`-style error; PutIfMatch with a stale sha gives `ErrPreconditionFailed`.
    - **Evidence:** `<attemptDir>/task-6-ipam-provider-design.txt`
  - **Commit:** Y | `feat(storage): add local filesystem backend`
  - **Recommended task executor category:** `unspecified-low` (contained backend, contract provided).

- [ ] 7. S3 backend (aws-sdk-go-v2)
  - **What to do:** `storage/s3.New(ctx, Config{Bucket, Prefix, Region, Endpoint, UsePathStyle})`.
    - Load credentials with `config.LoadDefaultConfig` (with an explicit region, so no IMDS call happens at construction).
    - When `Endpoint` is set (S3-compatible emulators), set `RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired` and `ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired`.
    - **Get:** `GetObject`, returning the ETag verbatim. NoSuchKey/404 gives `ErrNotFound`.
    - **CreateIfAbsent:** `PutObject` with `IfNoneMatch: aws.String("*")`.
    - **PutIfMatch:** `PutObject` with `IfMatch`.
    - **DeleteIfMatch:** `DeleteObject` with `IfMatch`.
    - **Error mapping:**
      - HTTP 412 / `PreconditionFailed` gives `ErrPreconditionFailed`.
      - HTTP 409 `ConditionalRequestConflict` is retried by the client as a precondition failure, so map it to `ErrPreconditionFailed`.
      - DeleteIfMatch or PutIfMatch on a missing key: 404 gives `ErrNotFound` (a 412 is also acceptable per the task 2 contract).
      - A 403 on Get may mean "missing key without ListBucket"; the module grants ListBucket unconditionally (task 19) so missing keys surface as 404.
    - **Access-denied mapping:** HTTP 403 `AccessDenied` gives `ErrAccessDenied`.
    - **Unknown-outcome mapping:** for PutObject/DeleteObject, a timeout, connection reset after the request was sent, or a final 5xx after SDK retries is wrapped with `storage.ErrUnknownOutcome` (unit-tested with an `httptest` server that hangs past the client timeout).
    - **Access-propagation retry:** when `storage.AccessPropagationRetry(ctx)` is true, pass a per-call option (`func(o *s3.Options){ o.Retryer = ... }`) using `retry.NewStandard` with `MaxAttempts` 10, `MaxBackoff` 30s, and an extra `Retryables` entry that retries `AccessDenied`. Without the flag, use the client's default retryer (access denied fails immediately). Never retry 412/409/404 beyond the SDK defaults.
    - **List:** `ListObjectsV2` paginated.
    - Use the S3 emulator selected and recorded by task 21 (`.omo/evidence/emulators.json` in the client repo).
    - Run the contract suite's concurrency case with `-count=20` against the emulator to separate emulator bugs from backend bugs.
    - Run `storagetest` under build tag `integration` with env `IPAM_TEST_S3_ENDPOINT`.
  - **Must NOT do:**
    - Do not use multipart upload: documents are small.
    - Do not use gocloud.dev.
    - Do not hardcode credentials.
  - **Closes:** GAP-5
  - **Parallelization:** Wave 2 | Blocked by: 2,21 | Blocks: 9,17,20
  - **References:**
    - https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html (If-Match / If-None-Match on PutObject, SigV4 required).
    - aws-sdk-go-v2 `service/s3` `PutObjectInput.IfMatch`, `IfNoneMatch`, `DeleteObjectInput.IfMatch`.
    - Draft D3, D8.
  - **Acceptance criteria:**
    - `go test ./storage/s3 -count=1` passes; unit tests use an `httptest` server that asserts the headers.
    - `IPAM_TEST_S3_ENDPOINT=http://127.0.0.1:9000 go test -tags integration ./storage/s3 -count=1` passes on the emulator that passed the probe.
  - **QA scenarios:**
    - **Happy:** the integration contract suite is green.
    - **Failure:** the `httptest` server returns 412 and the backend returns `ErrPreconditionFailed`; a missing bucket gives a non-precondition error propagated unchanged; a server returning 403 twice then 200 succeeds with the propagation flag (3 requests) and fails with `ErrAccessDenied` after exactly 1 request without it.
    - **Evidence:** `<attemptDir>/task-7-ipam-provider-design.txt`, including probe output.
  - **Commit:** Y | `feat(storage): add S3 backend with conditional writes`
  - **Recommended task executor category:** `unspecified-high` (external API + emulator uncertainty).

- [ ] 8. Azure Blob backend (azblob)
  - **What to do:** `storage/azblob.New(ctx, Config{AccountURL, Container, Prefix})`.
    - `Config` takes `Credential azcore.TokenCredential` (required unless the Azurite connection string is used).
    - **Single auth mapping (`storage/azblob/auth.go`):** exported `type AuthConfig struct` with one field per azapi auth attribute (`TenantID`, `ClientID`, `ClientIDFilePath`, `ClientSecret`, `ClientSecretFilePath`, `ClientCertificate`, `ClientCertificatePath`, `ClientCertificatePassword`, `OIDCToken`, `OIDCTokenFilePath`, `OIDCRequestToken`, `OIDCRequestURL`, `OIDCAzureServiceConnectionID`, `UseOIDC`, `UseMSI`, `UseCLI`, `UseAKSWorkloadIdentity`), plus:
      - `AuthConfigFromEnv() AuthConfig`: reads the azapi-documented env vars (`ARM_TENANT_ID`, `ARM_CLIENT_ID`, `ARM_CLIENT_ID_FILE_PATH`, `ARM_CLIENT_SECRET`, `ARM_CLIENT_SECRET_FILE_PATH`, `ARM_CLIENT_CERTIFICATE`, `ARM_CLIENT_CERTIFICATE_PATH`, `ARM_CLIENT_CERTIFICATE_PASSWORD`, `ARM_OIDC_TOKEN`, `ARM_OIDC_TOKEN_FILE_PATH`/`AZURE_FEDERATED_TOKEN_FILE`, `ARM_OIDC_REQUEST_TOKEN`/`ACTIONS_ID_TOKEN_REQUEST_TOKEN`/`SYSTEM_ACCESSTOKEN`, `ARM_OIDC_REQUEST_URL`/`ACTIONS_ID_TOKEN_REQUEST_URL`/`SYSTEM_OIDCREQUESTURI`, `ARM_ADO_PIPELINE_SERVICE_CONNECTION_ID`/`ARM_OIDC_AZURE_SERVICE_CONNECTION_ID`/`AZURESUBSCRIPTION_SERVICE_CONNECTION_ID`, `ARM_USE_OIDC`, `ARM_USE_MSI`, `ARM_USE_CLI` (default true), `ARM_USE_AKS_WORKLOAD_IDENTITY`).
      - `(AuthConfig) Merge(override AuthConfig) AuthConfig`: non-zero override fields win (the provider passes its attributes as the override over `AuthConfigFromEnv()`).
      - `(AuthConfig) Credential(logger) (azcore.TokenCredential, error)`: the ONLY mapping onto `aztfauth.Option` (`Use*` flags enabled when the explicit flag is true or the corresponding material is present, matching azapi defaults), then `aztfauth.NewCredential`.
      - One table test covers env-only, attribute-only and mixed precedence.
    - aztfauth itself does not read the environment (verified in its source), so this mapping is necessary; keep it to this one file.
    - Pin `github.com/Azure/entrauth` to the same pseudo-version azapi uses at execution time (currently `v0.0.0-20260702010043-480422e02b6c`); it has no release tags.
    - Constructing the credential and the backend must not touch the network (tokens are fetched lazily on the first request). Assert this with an injected `ClientOptions.Transport` that fails the test on ANY request, building the backend with `UseMSI = true`; construction must succeed with zero requests.
    - Use a connection string ONLY when env `AZURE_STORAGE_CONNECTION_STRING` is set AND it targets a loopback endpoint (Azurite); otherwise reject it with `ErrInvalidArgument` (no shared-key auth against real accounts).
    - **Get:** `DownloadStream`, ETag verbatim.
    - **CreateIfAbsent:** `UploadBuffer` with `AccessConditions.ModifiedAccessConditions.IfNoneMatch = to.Ptr(azcore.ETagAny)`.
    - **PutIfMatch:** `IfMatch`.
    - **DeleteIfMatch:** `DeleteBlob` with IfMatch.
    - **Error mapping:**
      - `bloberror.HasCode(err, bloberror.ConditionNotMet, bloberror.BlobAlreadyExists)` gives `ErrPreconditionFailed`.
      - `BlobNotFound` gives `ErrNotFound`.
    - **Access-denied mapping:** 403 with `AuthorizationPermissionMismatch`/`AuthorizationFailure` gives `ErrAccessDenied`.
    - **Unknown-outcome mapping:** for uploads and deletes, a timeout, connection reset after the request was sent, or a final 5xx after azcore retries is wrapped with `storage.ErrUnknownOutcome` (unit-tested with an `httptest` server that hangs past `TryTimeout`).
    - **Retry policy:** set `azblob.ClientOptions.ClientOptions.Retry` = `policy.RetryOptions{MaxRetries: 10, RetryDelay: 2s, MaxRetryDelay: 30s, ShouldRetry: shouldRetry}`. `ShouldRetry` replaces the SDK's default status-code AND error checks, so `shouldRetry` must reproduce them:
      - `err != nil` gives true (azcore evaluates context and non-retriable errors before calling it).
      - `resp.StatusCode` in {408, 429, 500, 502, 503, 504} gives true.
      - `resp.StatusCode == 403` with header `x-ms-error-code` in {`AuthorizationPermissionMismatch`, `AuthorizationFailure`} AND `storage.AccessPropagationRetry(resp.Request.Context())` gives true.
      - Everything else (notably 412, 409, 404) gives false.
      - Table-test `shouldRetry` directly.
    - **List:** `NewListBlobsFlatPager`.
    - Run `storagetest` under the `integration` tag against Azurite as started by task 21 (`npx --yes azurite-blob --silent --skipApiVersionCheck --location $TMPDIR/azurite --blobPort 10000`, well-known dev connection string), including the concurrency case with `-count=20`.
    - Use the current `azblob` version (1.8.2 as of 2026-10-01).
    - The package doc states that accounts with hierarchical namespace are unsupported (path-based access conditions can be bypassed by rename).
  - **Must NOT do:**
    - Do not use blob leases.
    - Do not accept account keys as config attributes.
  - **Closes:** GAP-5
  - **Parallelization:** Wave 2 | Blocked by: 2,21 | Blocks: 9,13,17,20
  - **References:** draft D3, D8, D19; https://raw.githubusercontent.com/Azure/azure-sdk-for-go/main/sdk/storage/azblob/CHANGELOG.md; aztfauth source https://github.com/Azure/entrauth/blob/main/aztfauth/aztfauth.go; azapi provider auth attributes https://registry.terraform.io/providers/Azure/azapi/latest/docs
  - **Acceptance criteria:** `AZURE_STORAGE_CONNECTION_STRING=<azurite> go test -tags integration ./storage/azblob -count=1` passes, and `go test ./storage/azblob` (unit, no network) passes.
  - **QA scenarios:**
    - **Happy:** the Azurite contract suite is green.
    - **Failure:** a stale ETag PUT against Azurite gives `ErrPreconditionFailed`, asserted in the suite and captured in the log; an `httptest` server returning 403 `AuthorizationPermissionMismatch` twice then 201 succeeds with the propagation flag and fails with `ErrAccessDenied` after exactly 1 request without it; a 412 is never retried by the SDK (request count 1).
    - **Evidence:** `<attemptDir>/task-8-ipam-provider-design.txt`
  - **Commit:** Y | `feat(storage): add Azure Blob backend with ETag conditions`
  - **Recommended task executor category:** `unspecified-high` (external SDK + emulator).

- [ ] 9. Root `ipam.Open` URL entrypoint shared by CLI and provider
  - **What to do:** In the root package (`open.go`):
    - `ipam.OpenStorage(ctx, rawURL string) (storage.Storage, error)`.
    - `ipam.Open(ctx, rawURL string, opts ...Option) (*Client, error)`, which is `OpenStorage` followed by `New`.
    - The root imports the backends. This is a one-way edge with no cycle, because backends import only `storage`.
    - Supported URLs:
    - `s3://<bucket>/<prefix>?region=..&endpoint=..&path_style=true`
    - `azblob://<account>.blob.core.windows.net/<container>/<prefix>` (credential from `azblob.AuthConfigFromEnv().Credential(...)`, i.e. aztfauth over `ARM_*`), or `azblob://<container>/<prefix>` with `AZURE_STORAGE_CONNECTION_STRING`
    - `file:///abs/path`
    - `mem://`, tests only
  - Unknown schemes give `ErrInvalidArgument`.
  - Table tests cover parsing.
  - **Must NOT do:** no credentials in the URL. Reject userinfo and secrets in the query string.
  - **Closes:** GAP-5, GAP-6
  - **Parallelization:** Wave 3 | Blocked by: 5,6,7,8 | Blocks: 10,13
  - **References:** draft D13; todos 6-8 constructors.
  - **Acceptance criteria:** `go test . -run TestOpen -count=1` passes.
  - **QA scenarios:**
    - **Happy:** `file:///tmp/x` gives a local store.
    - **Failure:** `s3://user:pass@b/p` gives an error; `ftp://x` gives `ErrInvalidArgument`.
    - **Evidence:** `<attemptDir>/task-9-ipam-provider-design.txt`
  - **Commit:** Y | `feat(ipam): add URL-based Open entrypoint`
  - **Recommended task executor category:** `quick` (parsing + table tests).

- [ ] 10. `ipam` CLI (cobra)
  - **What to do:** `cmd/ipam`, using `github.com/spf13/cobra`.
    - Global `--backend` flag (env `IPAM_BACKEND`, required) and `-o json|table` (default table).
    - **Commands:**
      - `pool create <id> --cidr .. [--cidr ..] | --parent <id> (--prefix-length N | --cidr ..) [--description] [--label k=v]`
      - `pool get <id>`
      - `pool list`
      - `pool delete <id>`
      - `alloc create <pool> (--prefix-length N | --cidr ..) [--id <uuid>] [--description] [--label k=v]`; prints the generated or supplied ID
      - `alloc get <pool> <id>`
      - `alloc list <pool>` (shows ID, CIDR, created_at, description, labels)
      - `alloc release <pool> <id>`
      - `alloc update <pool> <id> [--description] [--label k=v]` (metadata only)
      - `pool update <id> [--description] [--label k=v]` (metadata only)
      - `version`
    - **Exit codes:** 0 ok, 1 generic error, 2 usage, 3 not found, 4 conflict, 5 exhausted.
    - **Tests:** a test harness that executes the root command against `mem://` and `file://` covers every subcommand and the JSON output schema.
    - Add `.goreleaser.yml` for the CLI binary, adapted from wayfinder's, with no signing/publish needed for snapshot.
  - **Must NOT do:**
    - No interactive prompts.
    - No config file format (flags and env only).
  - **Closes:** GAP-6
  - **Parallelization:** Wave 3 | Blocked by: 9 | Blocks: 11,17,20
  - **References:** draft D2; `/Volumes/code/terraform-provider-wayfinder/.goreleaser.yml`.
  - **Acceptance criteria:**
    - `go test ./cmd/... -count=1` passes.
    - `goreleaser release --snapshot --clean --skip=sign,publish` exits 0.
  - **QA scenarios:**
    - **Happy:** with `IPAM_BACKEND=file://$TMP`, `ipam pool create p --cidr 10.0.0.0/16 && ipam alloc create p --prefix-length 24 -o json` prints `"cidr":"10.0.0.0/24"` and a UUIDv7 `"id"`, captured from the real binary.
    - **Failure:** `ipam pool delete p` with an allocation present exits 4 with a `pool_not_empty` message.
    - **Evidence:** `<attemptDir>/task-10-ipam-provider-design.txt`
  - **Commit:** Y | `feat(cli): add ipam command line interface`
  - **Recommended task executor category:** `unspecified-low` (thin CLI over library).

- [ ] 11. Client library documentation and examples
  - **What to do:**
    - Complete `README.md`:
      - Concepts: pool, parent pool, allocation, generated allocation IDs and idempotency by ID, explicit CIDR as reservation.
      - Both v1 JSON documents (`pools/<id>.json`, `reservations/<id>.json`), with examples.
      - **Access model:** the platform identity writes `pools/*` and `reservations/*`; each environment's deploy identity reads `pools/*` and writes only `reservations/<pool>.json` for its environment's pools. Include copy-pasteable examples: an S3 IAM policy with exact object ARNs, and the Azure role-assignment condition exactly as task 18 renders it (the golden `prod_condition.txt`: `blobs/write` only, on the environment's `reservations/<pool>.json` paths, with the data-actions-only custom role). Azure accounts must not have hierarchical namespace enabled (rename can bypass path conditions). Recommend bucket/blob versioning and soft delete for recovery.
      - **Recovering a leaked allocation:** `ipam alloc list` then either `terraform import` or `ipam alloc release`.
      - Concurrency model (ETag CAS, retries, per-pool doc contention guidance: split hot pools).
      - Backend URLs and credentials: Azure via aztfauth over `ARM_*` env vars (same as azurerm/azapi), AWS via the SDK default chain.
      - Required permissions per role: platform needs read/write/delete on `pools/*` and `reservations/*` plus list; environment identities need read on `pools/*` and `reservations/*`, overwrite (no delete) on their own `reservations/<pool>.json`, and unconditional `s3:ListBucket` (so missing keys return 404, not 403). On Azure, environment identities use the data-actions-only custom role from task 18, never the built-in Contributor role (which includes management-plane container delete).
      - CLI usage.
      - Library example.
    - Add `example_test.go` runnable examples (`ExampleClient_Allocate`) against `mem://`.
    - Add a `CHANGELOG.md` with an Unreleased section.
  - **Must NOT do:** do not document features absent from the code.
  - **Closes:** GAP-6, GAP-7
  - **Parallelization:** Wave 3 | Blocked by: 10,18,19,20 | Blocks: 16
  - **References:** todos 2-10 and 20 public APIs; task 18 `test/testdata/prod_condition.txt` and task 19 environment policy (copied exactly); draft Decisions.
  - **Acceptance criteria:** `go test ./... -run Example -count=1` passes, and the README mentions every exported constructor in `storage/*` (`grep` check script in evidence).
  - **QA scenarios:**
    - **Happy:** examples run.
    - **Failure:** a script cross-checks commands and flags in BOTH directions (every README command/flag exists in `ipam <cmd> --help`, and every command/flag in `--help` is documented in the README, including `doctor`); zero mismatches, and a deliberately renamed flag in a scratch copy of the README makes it fail.
    - **Evidence:** `<attemptDir>/task-11-ipam-provider-design.txt`
  - **Commit:** Y | `docs: document concepts, storage backends and CLI`
  - **Recommended task executor category:** `writing` (docs).

- [ ] 12. Scaffold Terraform provider repo
  - **What to do,** in `/Volumes/code/terraform-provider-ipam` (keep `.omo/`, `.codegraph/`, `.gitignore`):
    - `go.mod` (`module github.com/appvia/terraform-provider-ipam`).
    - `main.go` serving `providerserver.NewProtocol6WithError`, address `registry.terraform.io/appvia/ipam`, with a `version` ldflag.
    - `LICENSE` (MPL-2.0 full text).
    - `terraform-registry-manifest.json` `{"version":1,"metadata":{"protocol_versions":["6.0"]}}`.
    - `.goreleaser.yml`, `Makefile` (`build`/`test`/`testacc`/`docs`/`lint`), `.golangci.yml`, `mise.toml`, `.github/workflows/{test.yml,release.yml}`.
      - All adapted from `/Volumes/code/terraform-provider-wayfinder`, minus its wayfinder-specific scripts and manifestgen.
    - Add `.gitignore` entries for `/dist`, `/terraform-provider-ipam`.
    - Add the dependency on `github.com/appvia/ipam` via `replace github.com/appvia/ipam => ../ipam`. Mark it with a comment `// TODO(release): replace with tagged version once github.com/appvia/ipam is published`.
    - Add `README.md`.
  - **Must NOT do:**
    - Do not delete or modify `.omo/**`.
    - Do not copy wayfinder engine code.
  - **Closes:** GAP-3
  - **Parallelization:** Wave 4 (can start in Wave 2) | Blocked by: 1 | Blocks: 13
  - **References:** `/Volumes/code/terraform-provider-wayfinder/{main.go,Makefile,.goreleaser.yml,terraform-registry-manifest.json,.github/workflows/*.yml,mise.toml}`; current framework versions there: plugin-framework v1.19.0, plugin-testing v1.16.0, plugin-docs v0.25.0. Upgrade to the latest with `go get -u` at execution.
  - **Acceptance criteria:**
    - `go build ./...` exits 0.
    - `git status --porcelain .omo` shows no modifications to research files.
  - **QA scenarios:**
    - **Happy:** `go run . -h` (or building the binary) exits 0.
    - **Failure:** `grep -c TODO\(release\) go.mod` equals 1, which documents the temporary replace.
    - **Evidence:** `<attemptDir>/task-12-ipam-provider-design.txt`
  - **Commit:** Y | `chore: scaffold terraform provider`
  - **Recommended task executor category:** `quick` (sibling-repo scaffolding).

- [ ] 13. Provider configuration and client wiring
  - **What to do:** In `internal/provider/provider.go`, the schema has exactly one of three single nested attributes:
    - `s3 = { bucket (req), prefix, region, endpoint, use_path_style }`
    - `azure_blob = { account_url | (use_connection_string_env bool), container (req), prefix, tenant_id, client_id, client_id_file_path, client_secret (Sensitive), client_secret_file_path, client_certificate (Sensitive), client_certificate_path, client_certificate_password (Sensitive), oidc_token (Sensitive), oidc_token_file_path, oidc_request_token (Sensitive), oidc_request_url, oidc_azure_service_connection_id, use_oidc, use_msi, use_cli, use_aks_workload_identity }`. Names and `ARM_*` env fallbacks match the azapi provider exactly. Build the credential with `azblob.AuthConfigFromEnv().Merge(<attributes as AuthConfig>).Credential(...)` (task 8) and pass it to `azblob.New`; the provider never touches `aztfauth.Option` directly.
    - `local = { path (req) }`
  - Validate with `objectvalidator.AtMostOneOf` (from `terraform-plugin-framework-validators`) across `s3`, `azure_blob` and `local`; in Configure, if none is set and `IPAM_BACKEND` is empty, return an error diagnostic "one of s3, azure_blob or local must be configured, or IPAM_BACKEND set".
  - Top-level attribute `access_propagation_timeout` (Optional duration string, default `"5m"`, env `IPAM_ACCESS_PROPAGATION_TIMEOUT`): how long `Create`/`Update`/`Delete` keep retrying access-denied while new role assignments propagate (Read and data sources never retry).
  - **Configure is offline:** it builds clients and credentials only, with no network call (no token fetch, no container/bucket check). This lets pools be created in the same apply as the storage, provided the provider configuration uses values known at plan time.
  - Env fallback `IPAM_BACKEND`, a URL in todo 9's format, used when no block is set.
  - Configure builds the backend with its public typed constructor (`s3.New`, `azblob.New` or `local.New` from `github.com/appvia/ipam/storage/...`), or with `ipam.OpenStorage` for `IPAM_BACKEND`, then calls `ipam.New` and passes `*ipam.Client` as `ResourceData`/`DataSourceData`.
  - Unknown values at configure time are deferred with a clear diagnostic.
  - Add `internal/provider/errors.go`, mapping ipam typed errors to diagnostics: summary, plus detail with `Reason`.
  - The `azure_blob` auth attributes are copied into an `azblob.AuthConfig` and merged over `azblob.AuthConfigFromEnv()` (task 8); the provider contains no other auth mapping.
  - `use_connection_string_env` is accepted only for loopback (Azurite) endpoints, matching task 8; it exists for tests and local development only.
  - **Unit tests:** schema validation (two blocks error; zero blocks with no `IPAM_BACKEND` errors in Configure; zero blocks with `IPAM_BACKEND` works); env fallback.
  - **Must NOT do:**
    - No `access_key`/`secret_key`/`account_key` attributes and no storage shared-key or SAS auth.
    - Configure must not perform any network call (no token fetch, no container check). Unit test: configure with `use_msi = true` and an injected transport that fails the test on any request; Configure succeeds with zero requests.
  - **Closes:** GAP-3, GAP-5
  - **Parallelization:** Wave 4 | Blocked by: 8,9,12 | Blocks: 14,15
  - **References:** draft D13, D19; aztfauth (https://github.com/Azure/entrauth/blob/main/aztfauth/aztfauth.go); azapi provider docs for attribute names/env vars (https://registry.terraform.io/providers/Azure/azapi/latest/docs); lane a6 (`.omo/ulw-research/20261028-150017/wave-1-lane-a6-tf-framework-patterns.md`, section 2: provider auth belongs in provider config); wayfinder `internal/provider/` for structure.
  - **Acceptance criteria:** `go test ./internal/provider -run TestProvider -count=1` passes.
  - **QA scenarios:**
    - **Happy:** a config with `local { path = "<tmp>" }` configures.
    - **Failure:** setting both `s3` and `local` gives the `AtMostOneOf` diagnostic; no block and no `IPAM_BACKEND` gives the Configure error.
    - **Evidence:** `<attemptDir>/task-13-ipam-provider-design.txt`
  - **Commit:** Y | `feat(provider): add backend configuration`
  - **Recommended task executor category:** `unspecified-low` (framework boilerplate with validators).

- [ ] 14. `ipam_pool` resource and data source
  - **Resource attributes:**
    - `id` (Required, the pool id, RequiresReplace)
    - `cidrs` (Optional+Computed set of string, RequiresReplace). Configured for a root pool; computed (the single carved CIDR) for a child.
    - `parent_pool` (Optional, RequiresReplace)
    - `prefix_length` (Optional, RequiresReplace, requires `parent_pool`)
    - `cidr` (Optional, RequiresReplace, explicit child CIDR, requires `parent_pool`)
    - `family` (Computed)
    - `description` (Optional, updated in place)
    - `labels` (Optional map, updated in place)
  - **Validators:**
    - Root pool: `cidrs` set and no parent fields.
    - Child pool: `parent_pool` plus exactly one of `prefix_length`/`cidr`, and `cidrs` not configured.
  - Computed attributes use `UseStateForUnknown`.
  - **Lifecycle:**
    - Create calls `CreatePool` with `storage.WithAccessPropagationRetry` and a deadline of `access_propagation_timeout`.
    - Read calls `GetPool`; NotFound removes it from state.
    - Update calls `UpdatePoolMetadata` (description/labels only; every other attribute RequiresReplace), with the propagation context.
    - Delete calls `DeletePool` (same propagation context as Create); `pool_not_empty` becomes an error diagnostic; `ErrNotFound` is treated as success (already gone).
    - Read never sets the propagation flag (fail fast).
  - **ImportState:** passthrough `id`. Implement resource identity (`id`) per framework identity support.
  - **Data source `ipam_pool`:** input `id`; outputs `cidrs`, `family`, `parent_pool`, `description`, `labels`, and `allocations` (map of allocation ID to cidr).
  - **Acceptance tests** (`resource.Test`, `ProtoV6ProviderFactories`, local backend in `t.TempDir()`):
    - Create root pool, then import-verify, then destroy.
    - Create parent and child (`prefix_length=20`), check the child's `cidrs[0]` is inside the parent, then destroy (Terraform orders child before parent).
    - Changing `cidrs` forces replacement (plan check `plancheck.ExpectResourceAction(..., plancheck.ResourceActionReplace)`).
    - Pool deleted out-of-band through the library: the next plan is non-empty (create), and a destroy of a resource whose pool is already gone succeeds.
    - Changing `labels` is an in-place update (plancheck `ResourceActionUpdate`), and the CIDR is unchanged afterwards.
  - **Must NOT do:** no `Update` that touches CIDRs, parent, or incarnation.
  - **Closes:** GAP-3, GAP-4
  - **Parallelization:** Wave 4 | Blocked by: 13 | Blocks: 15,16
  - **References:** lane a6 section 1 (`UseStateForUnknown` on computed, `RequiresReplace` on immutable inputs) and section 4 (identity/import); research rec for split request/result fields in `SYNTHESIS.md`.
  - **Acceptance criteria:** `TF_ACC=1 go test ./internal/... -run 'TestAccPool' -count=1` passes with `terraform` v1.17.0 on PATH.
  - **QA scenarios:**
    - **Happy:** the parent/child test passes.
    - **Failure:** a config with both `cidrs` and `parent_pool` gives `ExpectError` matching `regexp.MustCompile("cannot be configured")`, or the validator message.
    - **Evidence:** `<attemptDir>/task-14-ipam-provider-design.txt`
  - **Commit:** Y | `feat(provider): add ipam_pool resource and data source`
  - **Recommended task executor category:** `unspecified-high` (framework semantics + acceptance tests).

- [ ] 15. `ipam_allocation` resource and data source (ID generated in Create)
  - **Resource attributes:**
    - `id` (Computed, `UseStateForUnknown`): the allocation's UUIDv7; unknown at plan time for new resources
    - `pool` (Required, RequiresReplace)
    - `prefix_length` (Optional, RequiresReplace)
    - `cidr` (Optional+Computed, RequiresReplace when configured, `UseStateForUnknown`); setting it requests an explicit allocation
    - `description`, `labels` (Optional, updated in place)
    - `created_at` (Computed, `UseStateForUnknown`)
  - Exactly one of `prefix_length` or a configured `cidr`. Use a ConfigValidator that inspects raw config, so a computed `cidr` does not count as configured.
  - **No `ModifyPlan` ID generation.** Terraform calls PlanResourceChange again during apply and requires every value known in the first plan to be identical (terraform-plugin-go `tfprotov6/resource.go`), so a random plan-time ID would fail with "Provider produced inconsistent final plan". `id` stays unknown in the plan.
  - **Lifecycle:**
    - Create generates the ID once with `ipam.NewAllocationID()`, then calls `Allocate` with that ID, `WithMaxAttempts(0)` semantics (bounded by the operation deadline) and the propagation context. Retries inside Create (SDK retries after a lost response, CAS conflicts) reuse the same ID, so they return the same allocation instead of duplicating it. If Create fails after the write may have committed, it still writes `id` into state with the error diagnostic, so Terraform keeps it as tainted rather than leaking it.
    - Read calls `GetAllocation`; NotFound removes it from state.
    - Update calls `UpdateAllocationMetadata` (description/labels only), with the propagation context.
    - Delete calls `Release`; `ErrNotFound` (pool already gone) is treated as success.
    - Create, Update and Delete set `storage.WithAccessPropagationRetry` with a deadline of `access_propagation_timeout`; Read never does.
    - Unit test with a store wrapper that records whether the flag was present on each call: present on Create/Update/Delete, absent on Read.
  - **Import:** ID `<pool>/<id>`, split on the FIRST `/` (pool IDs cannot contain `/`). Implement resource identity `{pool,id}` (Terraform >= 1.12).
  - **Data source `ipam_allocation`:** inputs `pool`, `id`; outputs `cidr`, `prefix_length`, `description`, `labels`, `created_at`.
  - **Acceptance tests:**
    - Allocate a `/24`, check `cidr` and that `id` parses as a UUID, import-verify (ImportStateIdFunc returning `<pool>/<id>`), destroy, and check the library shows it released.
    - The planned `id` of a new resource is unknown: `plancheck.ExpectUnknownValue("ipam_allocation.a", tfjsonpath.New("id"))`.
    - `count = 20` allocations in one apply (default parallelism 10): all IDs distinct, all CIDRs distinct and non-overlapping, checked with a `TestCheckFunc` using `netip`.
    - Explicit `cidr` allocation plus a next allocation do not overlap.
    - Changing `prefix_length` replaces the resource with a NEW id (plancheck replace action + id changes).
    - Changing `labels` is an in-place update (plancheck `ResourceActionUpdate`); `id` and `cidr` are unchanged.
    - Allocation released out-of-band through the library: the next plan recreates it with a new id.
    - Pool deleted out-of-band: destroy of the allocation succeeds.
    - Exhausted pool gives `ExpectError` "exhausted".
    - **Lost-response idempotency** (unit test at the resource level, local backend wrapped by a store that commits a `PutIfMatch` and then returns an error wrapping `storage.ErrUnknownOutcome` once; task 5's CAS loop re-reads on that error): Create succeeds, exactly ONE allocation exists in the pool, and its ID equals the state's `id`.
  - **Must NOT do:**
    - No `key` or `owner` attribute.
    - No ID derived from configuration and no ID set in `ModifyPlan`.
    - No data source that previews or picks the next CIDR.
  - **Closes:** GAP-1, GAP-2, GAP-3, GAP-4, GAP-9
  - **Parallelization:** Wave 4 | Blocked by: 14 | Blocks: 16,17
  - **References:** research C1 (allocation is an apply-time managed write), C10 (idempotency), lane a6 sections 1 and 4; todo 5 semantics; draft D17, D21; terraform-plugin-go `tfprotov6/resource.go` PlanResourceChange doc comment (planned values must be identical on the apply-time re-plan).
  - **Acceptance criteria:** `TF_ACC=1 go test ./internal/... -run 'TestAccAllocation|TestLostResponse' -count=1 -race` passes.
  - **QA scenarios:**
    - **Happy:** the `count = 20` test passes and the planned id is unknown at plan time.
    - **Failure:** the lost-response test yields exactly one allocation; the exhausted-pool test passes with the expected error; a replace produces a different id.
    - **Evidence:** `<attemptDir>/task-15-ipam-provider-design.txt`
  - **Commit:** Y | `feat(provider): add ipam_allocation resource and data source`
  - **Recommended task executor category:** `unspecified-high` (core resource + concurrency acceptance).

- [ ] 16. Provider docs and examples
  - **What to do:**
    - Add `examples/provider/provider.tf`, with one example per backend.
    - Add `examples/resources/ipam_pool/{resource.tf,import.sh}` and `examples/resources/ipam_allocation/{resource.tf,import.sh}`.
    - Add `examples/data-sources/*/data-source.tf`.
    - Add `templates/index.md.tmpl` explaining:
      - the concurrency model (bucket CAS);
      - that the backend bucket/container must exist beforehand and must not be the Terraform state bucket key prefix;
      - required IAM;
      - Azure authentication: identical to azurerm/azapi (`ARM_*` env vars, OIDC for GitHub Actions and Azure DevOps, managed identity, Azure CLI), via aztfauth;
      - allocation IDs: generated once in Create (unknown at plan time); retries inside Create never duplicate; a hard crash between the write and state save can leak one allocation, recovered with `ipam alloc list` then `terraform import` or `ipam alloc release`;
      - the access model: platform identity vs per-environment deploy identity (read + overwrite only, no delete; Azure via the data-actions-only custom role), with the S3 IAM policy and Azure path-condition examples from the client README, and the typical split (platform state owns `ipam_pool`; requestor states use `data.ipam_pool` and `ipam_allocation`).
    - Add `examples/guides/single-apply.md` (rendered via `templates/guides/`): one root configuration with the provisioning module, `provider "ipam" { azure_blob = module.ipam.provider_config }`, and `ipam_pool` resources with `depends_on = [module.ipam]`. Explain: provider configuration must be known at plan time; `access_propagation_timeout` covers role-assignment delay; avoid `data.ipam_*` reads in that same first apply (or give them `depends_on`) because data sources read at plan time.
    - **Module/provider contract test** (each of `examples/single-apply/azure/` and `examples/single-apply/aws/` has its own `tests/single_apply.tftest.hcl`, `command = plan`): one configuration per cloud in `examples/single-apply/azure/` and `examples/single-apply/aws/` that calls the local module (`source = "../../../../terraform-azurerm-ipam"` / `"../../../../terraform-aws-ipam"`, i.e. four levels up from those directories to `/Volumes/code`), configures `provider "ipam"` from `module.ipam.provider_config`, and declares an `ipam_pool` with `depends_on`. Use `mock_provider "azurerm"` / `mock_provider "aws"`. Make the unpublished ipam provider installable by `terraform init` through a `provider_installation { filesystem_mirror { path = "<tmp>/mirror", include = ["registry.terraform.io/appvia/ipam"] } direct { exclude = ["registry.terraform.io/appvia/ipam"] } }` CLI config (`TF_CLI_CONFIG_FILE`), with the freshly built binary placed in the mirror layout (`registry.terraform.io/appvia/ipam/0.0.0-dev/<os>_<arch>/terraform-provider-ipam_v0.0.0-dev`), and the test configuration pins `version = "0.0.0-dev"` exactly in `required_providers` (Terraform selects a pre-release only when pinned exactly). `dev_overrides` alone is not enough, because `init` still tries to fetch the provider. The plan must succeed, proving the module outputs match task 13's schema and are known at plan time.
    - Run `make docs` (`tfplugindocs generate` + `validate` via `go run`).
    - Complete the README and `CHANGELOG.md`.
  - **Must NOT do:** do not hand-edit generated `docs/` except via templates.
  - **Closes:** GAP-3, GAP-6
  - **Parallelization:** Wave 4 | Blocked by: 11,15,18,19 | Blocks: 17
  - **References:** wayfinder `Makefile` docs target, `/Volumes/code/terraform-provider-wayfinder/templates/`.
  - **Acceptance criteria:**
    - `make docs` exits 0.
    - `git diff --exit-code docs/` after a second `make docs` (docs are reproducible).
    - `terraform fmt -check -recursive examples` exits 0.
    - `terraform -chdir=examples/single-apply/azure test` and `terraform -chdir=examples/single-apply/aws test` both pass.
  - **QA scenarios:**
    - **Happy:** `tfplugindocs validate` passes; the single-apply contract test passes.
    - **Failure:** removing an example file, in a scratch copy only, makes `validate` or the check fail. Capture it, then discard the scratch copy.
    - **Evidence:** `<attemptDir>/task-16-ipam-provider-design.txt`
  - **Commit:** Y | `docs: add provider documentation and examples`
  - **Recommended task executor category:** `writing` (docs generation).

- [ ] 17. Cross-repo end-to-end with real CLI and emulators
  - **What to do:** Add `scripts/e2e.sh` (with `set -euo pipefail`) in the provider repo. It must:
    1. Build the provider binary and write a `dev_overrides` CLI config (`TF_CLI_CONFIG_FILE`).
    2. Build the `ipam` CLI from `../ipam`.
    3. Start Azurite (`npx --yes azurite-blob --silent --skipApiVersionCheck ...`) and the S3 emulator recorded by task 21. Turn on bucket versioning on the S3 emulator (`aws s3api put-bucket-versioning` or the Go SDK) so doctor's versioning check can pass there.
    4. For each backend (local, azblob, s3), run the same flow:
       - Create a pool with the CLI.
       - Run `ipam doctor --expect-write <pool>` and require exit 0 (WARN allowed where versioning is unreadable, e.g. Azurite).
       - Run TWO separate Terraform working dirs, each with its own state and `count = 10` allocations from the same pool, with IDENTICAL configuration, applied concurrently: `apply_a & pa=$!; apply_b & pb=$!; wait "$pa"; wait "$pb"` (each `wait` checked individually).
       - Assert `ipam alloc list -o json` contains EXACTLY 20 allocations, with 20 distinct IDs, and that the CIDRs are non-overlapping via `go run ./scripts/overlapcheck`.
       - `terraform destroy` both, then assert `ipam alloc list` is empty and `ipam pool delete` succeeds.
       - Print `E2E PASS <backend>`.
    5. **Restricted identity leg (S3 emulator only, if it supports users and policies, e.g. MinIO):** create a user whose policy is generated from task 19's environment policy for one pool. As that user, the Terraform allocation flow must succeed, `ipam doctor --expect-write <pool> --expect-no-write <other-pool>` must exit 0, and a CLI `pool create` must fail with access denied. Print `E2E PASS s3-restricted`, or `E2E SKIP s3-restricted (reason)` if the emulator lacks user/policy support.
    6. Exit non-zero on any assertion.
    7. Kill the emulators on exit (`trap`).
  - **`scripts/overlapcheck`** has its own Go unit tests: overlapping input exits 1, disjoint input exits 0, duplicate CIDRs exit 1.
  - **Must NOT do:**
    - Do not use real cloud accounts unless the `IPAM_TEST_S3_BUCKET` / `AZURE_STORAGE_ACCOUNT_URL` envs are set. With `AZURE_STORAGE_ACCOUNT_URL` set, also run the azblob flow against the real account using aztfauth credentials from `ARM_*` (this is the only exercise of the real Azure credential path; otherwise F4 marks IS-8's Azure credential path as unverified).
    - Do not leave background processes running.
  - **Closes:** GAP-1, GAP-5, GAP-6, GAP-8
  - **Parallelization:** Wave 5 | Blocked by: 7,8,10,16,20,21 | Blocks: F1-F4
  - **References:** todos 7, 8, 10, 15, 19, 20, 21; Terraform `dev_overrides` docs (https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers).
  - **Acceptance criteria:** `bash scripts/e2e.sh` exits 0 and prints `E2E PASS local`, `E2E PASS azblob`, and `E2E PASS s3`, plus `E2E PASS s3-restricted` or a SKIP line with a reason. If task 21 recorded no working S3 emulator and no AWS env is set, it prints `E2E SKIP s3 (reason)` instead, and local and azblob must still pass. `go test ./scripts/overlapcheck -count=1` passes.
  - **QA scenarios:**
    - **Happy:** the full script run log.
    - **Failure:** the overlapcheck unit tests prove the checker rejects overlaps (independently of decode validation); with `IPAM_E2E_FORCE_OVERLAP=1`, a hook writes an overlapping allocation straight into the local backend's reservations file, and the next `ipam alloc list` exits non-zero with `ErrInvalidCIDR` (proving decode validation also guards).
    - **Evidence:** `<attemptDir>/task-17-ipam-provider-design.log`
  - **Commit:** Y | `test: add cross-repo end-to-end script`
  - **Recommended task executor category:** `unspecified-high` (multi-process orchestration).

- [ ] 18. Provisioning module terraform-azurerm-ipam (storage + access control)
  - **What to do:** Create `/Volumes/code/terraform-azurerm-ipam` by copying `/Volumes/code/terraform-azurerm-module-template` (Makefile, `.tflint.hcl`, `.terraform-docs.yml`, `.checkov.yml`, `.pre-commit-config.yaml`, `.github/workflows`, `LICENSE`, `test/`, `examples/basic`), `git init`, and replace the module body.
    - **Inputs:** `resource_group_name`, `location`, `storage_account_name`, `environment_role` (`"custom"` | `"builtin"`, default `"custom"`, validated), `container_name` (default `"ipam"`), `platform_principal_ids` (list), `environments` (`map(object({ pool_ids = list(string), principal_ids = list(string) }))`), `reader_principal_ids` (optional list), `soft_delete_retention_days` (default 30), `tags`.
    - **Storage account (`azurerm_storage_account`):** `is_hns_enabled = false`, `min_tls_version = "TLS1_2"`, `shared_access_key_enabled = false`, `allow_nested_items_to_be_public = false`, `blob_properties { versioning_enabled = true, delete_retention_policy { days = var.soft_delete_retention_days }, container_delete_retention_policy { days = var.soft_delete_retention_days } }`.
    - **Container:** `azurerm_storage_container` named `var.container_name`, private.
    - **Platform:** "Storage Blob Data Contributor" on the container scope for each `platform_principal_ids`.
    - **Each environment, for each principal:**
      - "Storage Blob Data Reader" on the container scope (reads `pools/*` and reservations).
      - A custom role definition created by the module, named `ipam-reservations-writer-<storage_account_name>-<container>` (custom role names must be unique per tenant) (`azurerm_role_definition`, assignable scope = the container), with `actions = []`, `not_actions = []` and `data_actions` = exactly `Microsoft.Storage/storageAccounts/blobServices/containers/blobs/read` and `.../blobs/write` (no `add/action`, no `delete`, no management-plane actions). The built-in "Storage Blob Data Contributor" is NOT used for environments, because it also grants management-plane `containers/delete` and `containers/write`, which path conditions do not cover.
      - **`environment_role = "custom"` (default):** create the custom role above and assign it on the container scope with `condition_version = "2.0"` and a condition that allows the `blobs/write` action only when `@Resource[Microsoft.Storage/storageAccounts/blobServices/containers/blobs:path]` equals one of `reservations/<pool_id>.json` for that environment's `pool_ids`. Build the path test as an OR chain of `StringEquals` clauses from the list. (Reservations documents are created by the platform identity, so environments never need `add/action`; Release is an overwrite, so they never need `delete`.)
      - **`environment_role = "builtin"`:** create no role definition; assign the built-in "Storage Blob Data Contributor" on the container scope with a condition that (a) allows `blobs/write` only on the environment's `reservations/<pool_id>.json` paths and (b) never allows `blobs/delete`, `blobs/add/action` or `blobs/move/action` (each guarded by a clause that cannot match, e.g. `@Resource[...blobs:path] StringEquals '__ipam_never__'`). The README and the variable description state plainly that this mode still lets environment identities delete or modify the container through the role's management actions (`containers/delete`, `containers/write`), which conditions cannot restrict; container soft delete is then the only protection.
    - **Readers:** "Storage Blob Data Reader" on the container scope.
    - **Outputs:** `backend_url` (`azblob://<account>.blob.core.windows.net/<container>/`), `storage_account_id`, `container_name`, `provider_config` (object matching the ipam provider's `azure_blob` block). `backend_url` and `provider_config` must be built ONLY from input variables (e.g. `"https://${var.storage_account_name}.blob.core.windows.net"`), never from computed attributes such as `primary_blob_endpoint`, so they are known at plan time and the ipam provider can be configured in the same apply.
    - **README** (terraform-docs) explaining: grants need only pool IDs, so they can be applied before the pools exist; the platform state creates pools with the ipam provider using `backend_url`; hierarchical namespace must stay off.
    - **Tests** (`test/*.tftest.hcl`, `mock_provider "azurerm"`, `command = plan`):
      - HNS off, versioning on, shared keys off.
      - The rendered condition for env `prod` with pools `["uks-prod","ukw-prod"]` equals a golden string stored in `test/testdata/prod_condition.txt` (exact match, not substring).
      - The custom role's `permissions[0].actions` is empty and `data_actions` equals exactly the two read/write actions (no delete, no add).
      - With `environment_role = "builtin"`: no `azurerm_role_definition` is planned, the environment assignment uses the built-in role, and its rendered condition equals the golden `test/testdata/prod_condition_builtin.txt` (write on own paths only; delete/add/move never).
      - `environment_role = "other"` fails variable validation.
      - An environment with zero pools gets no conditioned write assignment.
      - A pool ID that fails `^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$` fails variable validation.
      - In plan mode, `output.backend_url` and `output.provider_config` equal the expected literal values (proves they are known at plan time).
  - **Must NOT do:**
    - Do not create pools or configure the ipam provider inside the module.
    - Do not grant account-scope roles.
    - Do not enable shared-key access.
    - In the default `custom` mode, do not grant environments delete, add, or any management-plane action; the `builtin` mode is the only exception and exists solely behind the explicit input.
  - **Closes:** GAP-10
  - **Parallelization:** Wave 1 (independent repo) | Blocked by: - | Blocks: 16,F1-F4
  - **References:** `/Volumes/code/terraform-azurerm-module-template` (structure, Makefile targets `validate`, `tests`, `lint`, `security`, `documentation`); Azure ABAC attributes https://learn.microsoft.com/en-us/azure/storage/blobs/storage-auth-abac-attributes and examples https://learn.microsoft.com/en-us/azure/storage/blobs/storage-auth-abac-examples; security note on renames https://docs.azure.cn/en-us/storage/blobs/storage-auth-abac-security; draft D16, D18.
  - **Acceptance criteria:** `terraform init -backend=false && terraform validate && terraform test -test-directory=test` pass; `terraform fmt -check -recursive` passes; `tflint` and `checkov` (if on PATH) report no failures, or the absence is recorded.
  - **QA scenarios:**
    - **Happy:** the plan-mode test asserting both prod paths passes; `terraform -chdir=examples/basic validate` passes.
    - **Failure:** a test with pool ID `UKS_Prod` fails validation (`expect_failures = [var.environments]`).
    - **Evidence:** `<attemptDir>/task-18-ipam-provider-design.txt`
  - **Commit:** Y | `feat: add ipam storage and access control module for Azure`
  - **Recommended task executor category:** `unspecified-high` (Azure RBAC/ABAC correctness).

- [ ] 19. Provisioning module terraform-aws-ipam (storage + access control)
  - **What to do:** Create `/Volumes/code/terraform-aws-ipam` with the same repo tooling as task 18 (copied from `/Volumes/code/terraform-azurerm-module-template`, with the provider swapped to `hashicorp/aws`), `git init`.
    - **Inputs:** `bucket_name`, `region` (required; used for `provider_config` and `backend_url` so outputs are known at plan time), `platform_role_names` (list), `environments` (`map(object({ pool_ids = list(string), role_names = list(string) }))`), `reader_role_names` (optional), `kms_key_arn` (optional; default SSE-S3), `noncurrent_version_expiration_days` (default 30), `tags`.
    - **Bucket:** `aws_s3_bucket`, `aws_s3_bucket_versioning` (Enabled), `aws_s3_bucket_public_access_block` (all true), `aws_s3_bucket_ownership_controls` (BucketOwnerEnforced), `aws_s3_bucket_server_side_encryption_configuration`, a lifecycle rule expiring noncurrent versions after N days, and a bucket policy denying `aws:SecureTransport = false`.
    - **Platform policy** (`aws_iam_policy` + `aws_iam_role_policy_attachment` per role): `s3:GetObject`/`PutObject`/`DeleteObject` on `arn:aws:s3:::<bucket>/pools/*` and `.../reservations/*`, and `s3:ListBucket` on the bucket.
    - **Environment policy** (one per environment, attached to its roles):
      - `s3:GetObject` on `pools/*`.
      - `s3:GetObject` on `reservations/*`.
      - `s3:PutObject` (no `DeleteObject`) on the exact ARNs `arn:aws:s3:::<bucket>/reservations/<pool_id>.json`.
      - `s3:ListBucket` on the bucket WITHOUT a prefix condition (S3 returns 404 for a missing key only to callers holding ListBucket; otherwise 403, which would hide drift and make Create wait out the propagation timeout on a mistyped pool).
      - `s3:GetBucketVersioning` on the bucket (so `ipam doctor` can read versioning).
    - **Reader policy:** Get on `pools/*` and `reservations/*`, plus ListBucket and GetBucketVersioning.
    - **KMS:** when `kms_key_arn` is set, every policy (platform, environment, reader) also grants `kms:Decrypt` on that key, and the platform and environment policies grant `kms:GenerateDataKey` on it; a test asserts these statements exist only when `kms_key_arn` is non-null.
    - The platform policy also includes `s3:GetBucketVersioning`.
    - **Outputs:** `backend_url` (`s3://<bucket>/`), `bucket_arn`, `region`, `provider_config` (object matching the ipam provider's `s3` block), and the policy ARNs. `backend_url` and `provider_config` must be built only from input variables, so they are known at plan time (same reason as task 18).
    - **README** mirroring task 18.
    - **Tests** (`mock_provider "aws"`, `command = plan`):
      - Versioning enabled, public access fully blocked.
      - The prod policy JSON (decode `jsondecode`) contains both exact reservation ARNs for `s3:PutObject`, no `s3:DeleteObject` anywhere, and no write action on a `reservations/*` wildcard.
      - The platform policy contains both wildcards.
      - An invalid pool ID fails validation.
      - In plan mode, `output.backend_url` and `output.provider_config` equal the expected literal values.
  - **Must NOT do:**
    - Do not create pools.
    - Do not create IAM roles (attach to given role names only).
    - Do not grant `s3:*`, and never grant environments `s3:DeleteObject`.
  - **Closes:** GAP-10
  - **Parallelization:** Wave 1 (independent repo) | Blocked by: - | Blocks: 16,F1-F4
  - **References:** task 18 structure; S3 conditional writes require only `s3:PutObject` (https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html); draft D16, D18.
  - **Acceptance criteria:** `terraform init -backend=false && terraform validate && terraform test -test-directory=test` pass; `terraform fmt -check -recursive` passes; `tflint`/`checkov` as in task 18.
  - **QA scenarios:**
    - **Happy:** the policy-content test passes; `examples/basic` validates.
    - **Failure:** a test asserting that an environment policy never contains `reservations/*` fails when that wildcard is injected (scratch copy only, then discarded); the invalid pool ID test raises the expected validation failure.
    - **Evidence:** `<attemptDir>/task-19-ipam-provider-design.txt`
  - **Commit:** Y | `feat: add ipam storage and access control module for AWS`
  - **Recommended task executor category:** `unspecified-high` (IAM policy correctness).

- [ ] 20. `ipam doctor` read-only backend check
  - **What to do:**
    - **`storage` additions:**
      - Uses `storage.ErrAccessDenied` (task 2; mapped by tasks 6-8; local maps `fs.ErrPermission`). Doctor never sets the access-propagation flag, so denied probes fail fast.
      - Optional interface `storage.Inspector { Inspect(ctx) (Info, error) }` with `Info{Versioning, SoftDelete, HierarchicalNamespace TriState}` (`Yes`, `No`, `Unknown`).
        - s3: `GetBucketVersioning`; an AccessDenied result gives `Unknown`.
        - azblob: `GetAccountInfo` gives `IsHierarchicalNamespaceEnabled`. Blob service properties give only soft delete (`DeleteRetentionPolicy`); container-scoped data roles usually cannot read them, so forbidden gives `Unknown`. Versioning is not exposed there (it is configured through Azure Resource Manager), so detect it from data access: a read of the probed `reservations/<pool>.json` that returns an `x-ms-version-id` header means versioning `Yes`; no header means `No`; no readable document means `Unknown`.
        - local and memory return `Unknown` for versioning and soft delete (a local folder has no such feature; doctor reports this as WARN with the note "not applicable to file:// backends"), and `No` for hierarchical namespace.
    - **CLI:** `ipam doctor [--expect-write <pool>...] [--expect-no-write <pool>...] [-o json|table]`. Checks:
      1. The backend is reachable and `pools/` is listable/readable.
      2. Versioning or soft delete: `Yes` is PASS, `No` is FAIL, `Unknown` is WARN.
      3. Hierarchical namespace (azblob): `Yes` is FAIL.
      4. Write permission per pool, via a non-mutating probe: read `reservations/<pool>.json` (bytes + version), then `PutIfMatch` the SAME bytes with a deliberately wrong version (`"doctor-probe"`). If the document cannot be read (missing or denied), report WARN for that pool and do NOT attempt the write.
         - `ErrPreconditionFailed` means write allowed and conditional writes honoured.
         - `ErrAccessDenied` means no write.
         - Success means the backend IGNORES conditional writes: FAIL with a critical message. The bytes were identical, so nothing changed.
         - Compare each result with `--expect-write` / `--expect-no-write`.
    - **Exit codes:** 0 if no FAIL, 6 if any FAIL.
    - Never creates, deletes or modifies data.
  - **Must NOT do:**
    - No provisioning or IAM changes.
    - No writes that could change object content.
  - **Closes:** GAP-10
  - **Parallelization:** Wave 3 | Blocked by: 7,8,10,21 | Blocks: 11,17
  - **References:** tasks 2, 6-8, 10; Azure `GetAccountInfo` / blob service properties in azblob; S3 `GetBucketVersioning`.
  - **Acceptance criteria:** `go test ./storage/... ./cmd/... -run 'Doctor|AccessDenied|Inspect' -count=1` passes, and `go test -tags integration ./cmd/... -run Doctor` passes against Azurite and the S3 emulator recorded by task 21 (with versioning enabled on the emulator bucket by the test setup).
  - **QA scenarios:**
    - **Happy:** against Azurite with a created pool, `ipam doctor --expect-write p` exits 0 with a PASS write probe, and the reservations blob ETag is unchanged afterwards (asserted).
    - **Failure:** a fake store that ignores versions makes doctor exit 6 with the "conditional writes ignored" FAIL; a fake store returning `ErrAccessDenied` with `--expect-write p` exits 6; a pool with no reservations document yields WARN and zero write calls (asserted by a recording store).
    - **Evidence:** `<attemptDir>/task-20-ipam-provider-design.txt`
  - **Commit:** Y | `feat(cli): add read-only doctor command`
  - **Recommended task executor category:** `unspecified-high` (cross-backend probes, non-mutating guarantee).

- [ ] 21. Emulator preflight (Wave 1)
  - **What to do,** in the client repo (`/Volumes/code/ipam/scripts/emulators/`):
    - `start-azurite.sh`: `npx --yes azurite-blob --silent --skipApiVersionCheck --location "$TMPDIR/azurite" --blobPort 10000`, waits for the port, prints the well-known dev connection string.
    - `start-s3.sh`: locate or install a MinIO server binary without Docker. Check PATH, then `brew install minio/stable/minio`, then a direct download from `https://dl.min.io/server/minio/release/<os>-<arch>/minio` into `$TMPDIR`. Record the exact source used. Start on `:9000` with a temp dir and create a test bucket.
    - `probe/` (a small Go program using aws-sdk-go-v2 and azblob) runs against each emulator:
      - stale `If-Match` PUT gives 412;
      - `If-None-Match: *` on an existing key gives 412;
      - stale `If-Match` DELETE gives 412;
      - `If-Match` PUT on a missing key gives 404 or 412 and creates nothing;
      - `If-Match` DELETE on a missing key gives 404 or 412;
      - 20 concurrent same-version PUTs: exactly one succeeds.
    - If MinIO fails any probe, try `github.com/johannesboyne/gofakes3` (in-process) with the same probe; if both fail, record that S3 integration runs only against real AWS (`IPAM_TEST_S3_BUCKET`).
    - Write the result to `.omo/evidence/emulators.json`: `{"azurite": {"ok": bool, "version": ".."}, "s3": {"kind": "minio|gofakes3|none", "source": "..", "ok": bool, "failed_probes": [..]}}`.
  - **Must NOT do:**
    - Do not install Docker.
    - Do not require real cloud accounts.
  - **Closes:** GAP-5
  - **Parallelization:** Wave 1 | Blocked by: 1 | Blocks: 7,8,17,20
  - **References:** S3 conditional writes https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html; Azurite CLI options; task 2 contract.
  - **Acceptance criteria:** `bash scripts/emulators/start-azurite.sh && bash scripts/emulators/start-s3.sh && go run ./scripts/emulators/probe` exits 0 when every selected emulator passes, and `.omo/evidence/emulators.json` exists and parses with `jq`.
  - **QA scenarios:**
    - **Happy:** the probe report shows all checks passing for Azurite and the chosen S3 emulator.
    - **Failure:** pointing the probe at a plain HTTP server that ignores conditional headers makes it exit non-zero and list the failed probes.
    - **Evidence:** `<attemptDir>/task-21-ipam-provider-design.txt`
  - **Commit:** Y | `test: add emulator preflight and conditional-write probe`
  - **Recommended task executor category:** `unspecified-high` (environment setup + emulator fidelity).

## Final verification wave
> Runs in parallel after ALL todos. ALL must APPROVE. Surface results and wait for the user's explicit okay before declaring complete.
- [ ] F1. Plan compliance audit
  - Every todo's acceptance command is re-run, and every Must NOT have is checked:
    - `go list -m all` in both repos shows no `gocloud.dev` module.
    - `terraform providers schema -json | jq` shows no `access_key`, `secret_key` or `account_key` provider attributes, no `key` or `owner` on `ipam_allocation`, and no data source whose name contains `preview` or `next`.
    - A Go AST search (`ast-grep` or `go/ast`) over non-test `.go` files finds no call to `azidentity.NewDefaultAzureCredential` and no `ModifyPlan` that sets `id` (prose in docs is excluded on purpose).
    - The module tests confirm no environment grant includes delete.
    - No git remote was added.
    - `.omo/ulw-research` is unchanged.
  - **Recommended task executor category:** `unspecified-high`
- [ ] F2. Code quality review
  - `golangci-lint run` passes in both repos.
  - `go test -race ./...` passes in both repos.
  - Review the CAS loop and allocator for error wrapping and for silent swallowing.
  - **Recommended task executor category:** `unspecified-high`
- [ ] F3. Real manual QA
  - Run `scripts/e2e.sh`.
  - Run the CLI happy and failure flows from todo 10 on the built binary.
  - Run `terraform plan` twice after apply and confirm no diff.
  - **Recommended task executor category:** `unspecified-high`
- [ ] F4. Ideal-state fidelity
  - Check each IS-1..IS-11 row against the evidence in Success criteria, 1:1. Any part of a row proven only by documentation (IS-9 real-cloud enforcement) or not exercised (IS-8 real Azure credential path when no `AZURE_STORAGE_ACCOUNT_URL` was provided) is reported explicitly as UNVERIFIED, never as passed.
  - Any shortfall becomes new numbered todos.
  - **Recommended task executor category:** `unspecified-high`

## Commit strategy
- One commit per todo, using Conventional Commits with the message on each todo's Commit line.
- The client repo (`/Volumes/code/ipam`) gets commits from todos 1-11, 20 and 21; the provider repo gets commits from todos 12-17; `/Volumes/code/terraform-azurerm-ipam` from todo 18; `/Volumes/code/terraform-aws-ipam` from todo 19.
- No pushes, tags or remotes: the user creates `github.com/appvia/ipam` and the two module repos, publishes v0.1.0, and then removes the provider's `replace` directive. That is a follow-up outside this plan.

## Success criteria
> One row per IS row. The plan is complete only when every IS row has a delivering todo and a proving QA scenario; F4 checks the delivered behavior against these rows 1:1, and a shortfall becomes new `- [ ] N.` rows, never a note.
| IS | Delivering todo(s) | Proving QA scenario | Evidence |
| --- | --- | --- | --- |
| IS-1 | 4,5,15,17 | 5 concurrent generated-ID allocations; 15 `count=20`; 17 two-state concurrent apply | task-5/15/17 |
| IS-2 | 5,15 | 5 same-ID convergence + `id_reused`; 15 lost-response test leaves exactly one allocation | task-5/15 |
| IS-3 | 14,15 | import-verify, out-of-band delete recreates, labels update in place, empty second plan | task-14/15, F3 |
| IS-4 | 5,14,15 | `pool_not_empty` guard; DeletePool crash points leave no carve-out; destroy releases (library check) | task-5/14/15 |
| IS-5 | 2,6,7,8,17 | contract suite on memory/local/S3/Azurite; e2e per backend | task-2/6/7/8/17 |
| IS-6 | 5,9,10,11 | CLI happy/failure on real binary; runnable examples | task-10/11 |
| IS-7 | 3,5,11 | golden round-trip; `schema_version 2` rejected by the codec; 5: an Allocate against a pool whose reservations doc is `schema_version: 2` returns `ErrUnsupportedSchema` and performs zero writes (recording store) | task-3/5 |
| IS-8 | 8,9,13,17 | e2e uses only a bucket/container + env credentials; 8/13 no-network construction tests; real Azure credential path only with `AZURE_STORAGE_ACCOUNT_URL` (else UNVERIFIED in F4) | task-8/13/17 |
| IS-9 | 3,5,11,16,17,18,19,20 | 5: `Allocate` never writes `pools/*` (recorded-keys assertion); 18/19: golden/exact policy and condition tests, no environment delete, custom Azure role has no management actions; 17: restricted MinIO user can allocate but not create pools (or SKIP with reason); 20: doctor write probes. Real-cloud enforcement is UNVERIFIED unless run against real accounts | task-5/17/18/19/20 |
| IS-10 | 3,5,15,17 | 15: 20 distinct ids, planned id unknown at plan time; 17: identical configs in two states get 20 distinct ids and non-overlapping CIDRs | task-15/17 |
| IS-11 | 16,18,19,20,21 | 18/19 terraform test plan assertions + examples validate; 16 single-apply contract test configures the provider from module outputs; 20 doctor PASS on emulators (WARN where versioning unreadable) and FAIL on version-ignoring/denied fakes | task-16/18/19/20 |
