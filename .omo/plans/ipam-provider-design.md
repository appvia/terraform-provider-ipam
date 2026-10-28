# ipam-provider-design - Work Plan

## TL;DR (For humans)

**Who this is for and what changes for them:** Platform engineers get a Terraform provider that hands out IP ranges that never clash, even when many separate Terraform projects ask at the same time. Developers get a small Go library and command-line tool over the same data, so a future web UI can build on it.

**What you'll get:** A new client library and CLI repository (`appvia/ipam`) that stores IP pools as readable JSON files in an S3 bucket, Azure Blob container or local folder. The existing provider repository becomes a Terraform provider with pool and allocation resources built on that library. Both come with tests, docs and release tooling.

**Why this approach:** The bucket is the only moving part, so there is no server or database to run. Each write only succeeds if nobody else changed the file since it was read; on S3 and Azure that is a built-in feature, and it is what guarantees no overlaps. Every allocation also carries a caller-chosen key, so retries and crashes can never create duplicates.

**What it will NOT do:** It does not talk to AWS, Azure or Google cloud IPAM services, run a server, or build the web front end. It will not create GitHub repositories, push code or publish releases; you do that.

**Effort:** XL
**Risk:** Medium - correctness under concurrency rests on conditional writes. These are proven with real-backend tests, but S3-compatible emulators vary.
**Decisions to sanity-check:**
- Licences are MPL-2.0 for the provider and Apache-2.0 for the client, as you chose. Note that Appvia's existing Wayfinder provider is Apache-2.0.
- There is no separate "reservation" concept: reserving a specific range is an allocation with a fixed range.
- A child pool's range is recorded as an allocation in its parent.
- A local-folder backend is added for development and tests.

Your next move: start execution with `/ulw-execute ipam-provider-design`, after the review result below. Full execution detail follows below.

---

> TL;DR (machine): XL / Medium risk - new repo github.com/appvia/ipam (lib + storage s3/azblob/local/memory + cobra CLI) and provider appvia/terraform-provider-ipam (ipam_pool, ipam_allocation resources + data sources), ETag-CAS on one JSON doc per pool.

## Scope
### Affected user and ideal state
**Affected user:**
- **U1 - platform engineers.** They write Terraform across many states, subscriptions and accounts. Today they use hand-kept spreadsheets or `cidrsubnet()`, neither of which coordinates across states. After this work they declare `ipam_allocation` and get a CIDR that is safe to use.
- **U2 - Go developers.** They build the `ipam` CLI now and a web front end later, both on `github.com/appvia/ipam`.
- **U3 - operators.** They inspect and repair pool documents in the bucket.

| Row | Statement | Reason |
| --- | --- | --- |
| IS-1 | Concurrent applies from different states/processes never receive overlapping CIDRs from the same pool tree | Core IPAM promise; state-only allocators cannot coordinate (research claim-graph C1) |
| IS-2 | A retry after crash/timeout with the same allocation key returns the same CIDR; never a duplicate | Idempotent create; Xtratus provider lacked it (C10) |
| IS-3 | Allocated CIDR is known after apply, stable across plans, refresh shows drift (deleted allocation -> recreate), allocations/pools importable | TF framework guidance (lane a6) |
| IS-4 | Destroy releases the CIDR; a pool with allocations or child pools cannot be deleted | No leaked/orphaned space |
| IS-5 | S3, Azure Blob and local backends behave identically; backend chosen only in provider/CLI config | User: Go interface abstraction |
| IS-6 | Library usable without Terraform: typed Go API + `ipam` CLI with JSON output | User: two repos, CLI/front end |
| IS-7 | Bucket state is human-readable, versioned JSON (schema_version), unknown versions never overwritten | Operator inspection/repair |
| IS-8 | Nothing to operate except a bucket/container and SDK-default credentials | User: simple |
| GAP-1 | No allocator or concurrency control exists (repo only has research, commit 94b4c59) | IS-1 unmet |
| GAP-2 | No idempotency contract exists | IS-2 unmet |
| GAP-3 | No Terraform resources exist | IS-3/IS-4 unmet |
| GAP-4 | No deletion guards exist | IS-4 unmet |
| GAP-5 | No storage abstraction or backends exist | IS-5/IS-8 unmet |
| GAP-6 | No client repo, Go API or CLI exists | IS-6 unmet |
| GAP-7 | No document schema exists | IS-7 unmet |

### Must have
- Client repo `/Volumes/code/ipam`, module `github.com/appvia/ipam`, Apache-2.0, with these packages:
  - `pkg/storage`: the interface and its errors.
  - `pkg/storage/{memory,local,s3,azblob}`: the backends.
  - `pkg/storage/storagetest`: the shared contract suite.
  - `pkg/storage/config`: a URL-based factory.
  - `pkg/ipam`: model, canonicalisation, allocator and `Client` service.
  - `cmd/ipam`: the cobra CLI.
- Provider repo `/Volumes/code/terraform-provider-ipam`, module `github.com/appvia/terraform-provider-ipam`, MPL-2.0, terraform-plugin-framework, protocol 6.0, registry address `registry.terraform.io/appvia/ipam`, with:
  - resources `ipam_pool` and `ipam_allocation`;
  - data sources `ipam_pool` and `ipam_allocation`.
- IPv4 and IPv6. Each pool is single-family and may hold multiple disjoint CIDRs.
- Explicit-CIDR allocation, which is how reservations are expressed.
- Child pools via `parent_pool`.

### Must NOT have (guardrails, anti-slop, scope boundaries)
- No server, daemon, database, lock service or DynamoDB table.
- No integration with AWS VPC IPAM, Azure/ipam, AVNM or GCP; the research rejected a backend-agnostic cloud-IPAM provider.
- No "preview next CIDR" or next-available data source, because it is advisory and causes duplicates (C1, AWS #25958).
- No ephemeral resources, write-only attributes, actions or TF 1.14 list resources.
- No gocloud.dev: it lacks If-Match writes, so use the native SDKs.
- No web front end and no gRPC/HTTP API.
- No separate reservation object type and no scopes/spaces concept.
- No in-place update of pool CIDRs or allocation requests: every change forces replacement.
- Do not create GitHub repos, push, tag or publish releases.
- Do not modify `.omo/ulw-research/**`.
- Never put credentials in provider or CLI config attributes. Use the SDK default credential chains only.

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
- **Wave 1 (client foundation):** 1, 2, 3, 4. Todo 1 first, then 2 and 3 in parallel, then 4.
- **Wave 2 (client service + backends):** 5, 6, 7, 8. 5 and 6 in parallel after 2-4; 7 and 8 after 2.
- **Wave 3 (client surface):** 9, 10, 11.
- **Wave 4 (provider):** 12, 13, 14, 15, 16. 12 can start in parallel with Wave 2.
- **Wave 5 (end-to-end):** 17.

### Dependency matrix
| Todo | Depends on | Blocks | Can parallelize with |
| --- | --- | --- | --- |
| 1 | - | 2,3,12 | 12 |
| 2 | 1 | 5,6,7,8 | 3 |
| 3 | 1 | 4,5 | 2 |
| 4 | 3 | 5 | 2 |
| 5 | 2,3,4 | 9,10,13 | 6,7,8 |
| 6 | 2 | 9 | 5,7,8 |
| 7 | 2 | 9,17 | 5,6,8 |
| 8 | 2 | 9,17 | 5,6,7 |
| 9 | 5,6,7,8 | 10,13 | 11 |
| 10 | 9 | 11,17 | - |
| 11 | 10 | - | 13 |
| 12 | 1 | 13 | 2-8 |
| 13 | 9,12 | 14,15 | 11 |
| 14 | 13 | 15,16 | - |
| 15 | 14 | 16,17 | - |
| 16 | 15 | 17 | - |
| 17 | 7,8,10,16 | F1-F4 | - |

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
    - Add `.github/workflows/test.yml`: go test + lint on push/PR, with an integration job running Azurite via `npx` and MinIO via its binary.
    - Add a `README.md` skeleton with sections Overview, Concepts, Storage backends, CLI, Library usage.
    - Add `doc.go` stubs for `pkg/ipam`, `pkg/storage`.
    - Run `codegraph init` and make sure `.codegraph/` is gitignored.
  - **Must NOT do:**
    - Do not create a GitHub remote or push.
    - Do not add a `replace` directive.
  - **Closes:** GAP-6
  - **Parallelization:** Wave 1 | Blocked by: - | Blocks: 2,3,12
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
  - **What to do (TDD):** In `pkg/storage/storage.go` define:
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
    - Sentinel errors in `pkg/storage/errors.go`:
      - `ErrNotFound` for a missing object.
      - `ErrPreconditionFailed` for an ETag mismatch, or an existing object on create.
    - `pkg/storage/memory`:
      - Mutex-guarded map; data is copied on read and write.
      - Monotonic integer version.
      - A hook (`BeforeWrite func(op, key string)`) so tests can inject barriers.
    - `pkg/storage/storagetest.Run(t *testing.T, newStore func(t *testing.T) storage.Storage)` covers:
      - Get on a missing key gives `ErrNotFound`.
      - Create then Get round-trips bytes and version.
      - A second Create gives `ErrPreconditionFailed`.
      - PutIfMatch with a stale version gives `ErrPreconditionFailed`, and the data is unchanged.
      - PutIfMatch with the current version succeeds with a new version.
      - DeleteIfMatch with a stale version gives `ErrPreconditionFailed`.
      - DeleteIfMatch on a missing key gives `ErrNotFound`.
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
  - **Acceptance criteria:** `go test ./pkg/storage/... -race -count=1` passes and includes `TestMemoryContract`.
  - **QA scenarios:**
    - **Happy:** the contract suite passes on memory.
    - **Failure:** temporarily breaking the memory backend's version check, in a scratch copy only, makes `storagetest` fail. Record the output, then discard the scratch copy.
    - **Evidence:** `<attemptDir>/task-2-ipam-provider-design.txt`
  - **Commit:** Y | `feat(storage): add storage interface, memory backend and contract suite`
  - **Recommended task executor category:** `deep-low` (concurrency contract correctness).

- [ ] 3. Domain model, canonicalisation and JSON schema v1
  - **What to do (TDD) in `pkg/ipam`:**
    - **Types:** `Pool{ID, Family(4|6), CIDRs []netip.Prefix, Parent *ParentRef{PoolID, Key}, Description, Labels map[string]string, Allocations map[string]Allocation}`, `Allocation{Key, Request Request, CIDR netip.Prefix, Description, Labels, CreatedAt time.Time}`, and `Request{Kind:"next"|"explicit", PrefixLength int, CIDR netip.Prefix}`.
    - **Pool ID:** regex `^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`.
    - **Allocation key:** 1-255 bytes of UTF-8 with no control chars; it may contain `/`.
    - **Canonicalisation:**
      - Use `netip.ParsePrefix` and store `Masked()`.
      - Reject zones.
      - Turn IPv4-mapped `::ffff:a.b.c.d/(96+n)` into `a.b.c.d/n`, and reject a mapped prefix shorter than /96.
      - A pool must have at least 1 CIDR, all one family.
      - Reject duplicate or overlapping CIDRs (adjacent ones are fine) and sort them.
    - **JSON document:**
      - Shape: `{"schema_version":1,"pool_id":..,"family":4,"cidrs":[..],"parent":{"pool_id":..,"key":..}|null,"description":..,"labels":{..},"allocations":{"<key>":{"request":{"kind":"next","prefix_length":24}|{"kind":"explicit","cidr":".."},"cidr":"..","description":..,"labels":{..},"created_at":RFC3339}}}`.
      - Encode with sorted keys and indentation, so diffs are stable.
      - Decode rejects a missing `schema_version` and returns `ErrUnsupportedSchema` for any version other than 1.
      - Decode checks `pool_id` matches the object key and re-validates every invariant (containment, non-overlap).
    - **Object key layout:** `<prefix>pools/<pool_id>.json`.
    - **Typed errors** (`errors.Is`-able, in `pkg/ipam/errors.go`): `ErrNotFound`, `ErrConflict`, `ErrPoolExhausted`, `ErrInvalidCIDR`, `ErrInvalidArgument`, `ErrUnsupportedSchema`. Also `*OpError{Op, Pool, Key, Reason string, Err error}` with `Unwrap`.
  - **Must NOT do:**
    - Do not put the ETag in the document.
    - No reservation type.
  - **Closes:** GAP-7
  - **Parallelization:** Wave 1 | Blocked by: 1 | Blocks: 4,5
  - **References:** draft D1, D9, D10, D11; ultrabrain spec sections 1 and 6, folded into D9.
  - **Acceptance criteria:**
    - `go test ./pkg/ipam -run 'TestCanonical|TestCodec' -count=1` passes.
    - Golden file `pkg/ipam/testdata/pool_v1.json` round-trips byte-identically.
  - **QA scenarios:**
    - **Happy:** `10.0.0.5/16` is stored as `10.0.0.0/16`; `::ffff:10.0.0.0/104` is stored as `10.0.0.0/8`.
    - **Failure:** `schema_version: 2` gives `ErrUnsupportedSchema`; a mixed-family pool gives `ErrInvalidCIDR`; overlapping CIDRs give `ErrInvalidCIDR`.
    - **Evidence:** `<attemptDir>/task-3-ipam-provider-design.txt`
  - **Commit:** Y | `feat(ipam): add pool model, canonicalisation and v1 document codec`
  - **Recommended task executor category:** `deep-low` (exact invariants).

- [ ] 4. Pure CIDR allocator (next-available and explicit)
  - **What to do (TDD):** Write `pkg/ipam/allocator.go` as pure functions with no I/O.
    - **`NextPrefix(pool Pool, length int) (netip.Prefix, error)`:**
      - `length` must be within the family's width and `>=` the bits of at least one parent CIDR, otherwise `ErrInvalidCIDR`.
      - For each eligible parent CIDR on its own:
        1. Build a `go4.org/netipx.IPSetBuilder`, add the parent, and remove every existing allocation CIDR.
        2. From the free set's `Prefixes()`, take every free block `f` with `f.Bits() <= length`; the candidate is `netip.PrefixFrom(f.Addr(), length).Masked()`.
      - Pick the candidate with the lowest address across all parents.
      - No candidate gives `ErrPoolExhausted`.
    - **`CheckExplicit(pool, cidr)`:**
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
  - **Acceptance criteria:** `go test ./pkg/ipam -run 'TestNext|TestExplicit|TestProperty' -count=1 -race` passes.
  - **QA scenarios:**
    - **Happy:** in an empty `10.0.0.0/16`, next `/24` gives `10.0.0.0/24`, then `10.0.1.0/24`; with an explicit `10.0.0.128/25` held, next `/24` gives `10.0.1.0/24`.
    - **Failure:** a `/24` pool asked for a `/16` gives `ErrInvalidCIDR`; a full pool gives `ErrPoolExhausted`.
    - **Evidence:** `<attemptDir>/task-4-ipam-provider-design.txt`
  - **Commit:** Y | `feat(ipam): add deterministic CIDR allocator`
  - **Recommended task executor category:** `deep-low` (algorithmic, spec given).

- [ ] 5. Client service with CAS loop, idempotency and child pools
  - **Constructor:** `ipam.New(store storage.Storage, opts ...Option) *Client`, with options `WithMaxAttempts(8)`, `WithBackoff(base 10ms, cap 250ms)`, `WithClock`, `WithRand`, `WithSleeper`, all injectable for tests.
  - **Methods** (all `context.Context` first):
    - `CreatePool(ctx, PoolSpec{ID, CIDRs | (ParentPool + Request), Description, Labels}) (Pool, error)`
    - `GetPool`
    - `ListPools` (List over `pools/`)
    - `DeletePool`
    - `Allocate(ctx, poolID, AllocationSpec{Key, PrefixLength | CIDR, Description, Labels}) (Allocation, error)`
    - `GetAllocation`
    - `ListAllocations`
    - `Release(ctx, poolID, key) error`
  - **CAS loop:**
    - Each attempt: Get, decode, mutate as a pure step, then `PutIfMatch`.
    - Retry only on `storage.ErrPreconditionFailed`, with full-jitter backoff `uniform(0, min(base*2^n, cap))`, cancellable through ctx.
    - After max attempts, return `ErrConflict` (Reason `contention`) wrapping the last error.
  - **In-process serialisation:** a per-pool `sync.Mutex` (map guarded by a mutex) serialises mutations from one `Client`, so parallel Terraform resources in one apply do not burn CAS attempts. Correctness still comes from CAS.
  - **Idempotency:**
    - Look up the key before checking availability.
    - Same canonical request: return the existing allocation with zero writes.
    - Different request: `ErrConflict` (Reason `key_reused`), with both descriptors in the message.
  - **Release:** an absent key in an existing pool is a no-op; an absent pool gives `ErrNotFound`.
  - **DeletePool:**
    - Refuse with `ErrConflict` (Reason `pool_not_empty`) when allocations exist. Child pools appear as allocations with key `pool:<child>`, so they block deletion too.
    - Otherwise `DeleteIfMatch` (retry on precondition).
    - If the pool has a parent, then `Release(parent, "pool:<id>")`.
  - **CreatePool with parent:**
    1. `Allocate(parent, {Key:"pool:<id>", ...})`. This is idempotent.
    2. `CreateIfAbsent(child doc)` with `Parent` set.
    3. If the object already exists with the same parent ref and CIDR, return it (convergent retry). Otherwise return `ErrConflict` (Reason `pool_exists`).
  - **CreatePool without parent:**
    - `CreateIfAbsent`. On precondition failure, read the existing doc: if its spec is identical return it, otherwise `ErrConflict` (`pool_exists`).
    - Keys starting with `pool:` are rejected for user allocations (`ErrInvalidArgument`).
  - **Tests** (memory store + barrier hook, no sleeps):
    - K=8 goroutines with unique keys all succeed, and the result equals the first 8 non-overlapping `/24`s.
    - 8 goroutines with the same key and request converge on one CIDR.
    - 8 concurrent CreatePool calls with the same spec: all return the same pool; with different specs: one wins and the rest get `ErrConflict`.
    - Injected permanent 412 is attempted exactly MaxAttempts times, then wraps the error.
    - An allocate racing a DeletePool loses no data.
    - A crash between the parent allocation and the child create (simulated by a store error on create), followed by a retry, converges.
    - Deleting the parent while a child exists gives `pool_not_empty`.
    - Use two `Client` instances on the same memory store to prove the cross-process path; the mutex does not cover it.
  - **Must NOT do:**
    - No global locks in storage.
    - No `time.Sleep` in tests: use the injected sleeper.
  - **Closes:** GAP-1, GAP-2, GAP-4
  - **Parallelization:** Wave 2 | Blocked by: 2,3,4 | Blocks: 9,10,13
  - **References:** draft D7, D9, D11; ultrabrain spec sections 3-5 and 7; research C1/C10 in `.omo/ulw-research/20261028-150017/claim-graph.md`.
  - **Acceptance criteria:** `go test ./pkg/ipam -race -count=3` passes. Three runs prove there is no flakiness.
  - **QA scenarios:**
    - **Happy:** the 8-goroutine unique-key test passes with `-race`.
    - **Failure:** a permanent-412 store returns `errors.Is(err, ipam.ErrConflict)` after exactly 8 Put calls (asserted counter).
    - **Evidence:** `<attemptDir>/task-5-ipam-provider-design.txt`
  - **Commit:** Y | `feat(ipam): add client service with optimistic concurrency and child pools`
  - **Recommended task executor category:** `deep-high` (correctness argued from concurrency invariants).

- [ ] 6. Local filesystem backend
  - **What to do:** `pkg/storage/local.New(dir string)`.
    - Object key maps to a file under `dir`, with `/` becoming nested dirs; reject keys containing `..`.
    - The version is the hex sha256 of the content.
    - Each conditional op holds an exclusive `flock` on `<dir>/.lock`, using `golang.org/x/sys/unix` on unix and `windows.LockFileEx` on Windows, behind a build-tagged helper.
    - Writes go to a temp file, then fsync, then rename.
    - `List` walks the directory sorted, ignoring `.lock` and temp files.
    - Runs `storagetest.Run`.
    - Add a cross-process test: re-exec the test binary 4 times, each child doing 25 CAS increments on one key; the final counter must be 100.
  - **Must NOT do:** never pretend the backend is safe on network filesystems; the doc comment states local disks only.
  - **Closes:** GAP-5
  - **Parallelization:** Wave 2 | Blocked by: 2 | Blocks: 9
  - **References:** draft D12; `storagetest` from todo 2.
  - **Acceptance criteria:** `go test ./pkg/storage/local -race -count=1` passes, including `TestCrossProcess`.
  - **QA scenarios:**
    - **Happy:** the contract suite plus the cross-process counter equals 100.
    - **Failure:** key `../x` gives `ErrInvalidArgument`-style error; PutIfMatch with a stale sha gives `ErrPreconditionFailed`.
    - **Evidence:** `<attemptDir>/task-6-ipam-provider-design.txt`
  - **Commit:** Y | `feat(storage): add local filesystem backend`
  - **Recommended task executor category:** `unspecified-low` (contained backend, contract provided).

- [ ] 7. S3 backend (aws-sdk-go-v2)
  - **What to do:** `pkg/storage/s3.New(ctx, Config{Bucket, Prefix, Region, Endpoint, UsePathStyle})`.
    - Load credentials with `config.LoadDefaultConfig`.
    - **Get:** `GetObject`, returning the ETag verbatim. NoSuchKey/404 gives `ErrNotFound`.
    - **CreateIfAbsent:** `PutObject` with `IfNoneMatch: aws.String("*")`.
    - **PutIfMatch:** `PutObject` with `IfMatch`.
    - **DeleteIfMatch:** `DeleteObject` with `IfMatch`.
    - **Error mapping:**
      - HTTP 412 / `PreconditionFailed` gives `ErrPreconditionFailed`.
      - HTTP 409 `ConditionalRequestConflict` is retried by the client as a precondition failure, so map it to `ErrPreconditionFailed`.
      - DeleteIfMatch on a missing key: 404 gives `ErrNotFound`.
    - **List:** `ListObjectsV2` paginated.
    - **First step:** verify the emulator. Start a MinIO server binary and run a 20-line probe (stale If-Match PUT must give 412; If-None-Match on an existing key must give 412; DeleteObject If-Match stale must give 412). Record the result.
      - If MinIO fails any probe, use `github.com/johannesboyne/gofakes3` ONLY if it passes the same probe. Otherwise run the contract suite only against real AWS (`IPAM_TEST_S3_BUCKET`) and record the limitation in the README.
    - Run `storagetest` under build tag `integration` with env `IPAM_TEST_S3_ENDPOINT`.
  - **Must NOT do:**
    - Do not use multipart upload: documents are small.
    - Do not use gocloud.dev.
    - Do not hardcode credentials.
  - **Closes:** GAP-5
  - **Parallelization:** Wave 2 | Blocked by: 2 | Blocks: 9,17
  - **References:**
    - https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html (If-Match / If-None-Match on PutObject, SigV4 required).
    - aws-sdk-go-v2 `service/s3` `PutObjectInput.IfMatch`, `IfNoneMatch`, `DeleteObjectInput.IfMatch`.
    - Draft D3, D8.
  - **Acceptance criteria:**
    - `go test ./pkg/storage/s3 -count=1` passes; unit tests use an `httptest` server that asserts the headers.
    - `IPAM_TEST_S3_ENDPOINT=http://127.0.0.1:9000 go test -tags integration ./pkg/storage/s3 -count=1` passes on the emulator that passed the probe.
  - **QA scenarios:**
    - **Happy:** the integration contract suite is green.
    - **Failure:** the `httptest` server returns 412 and the backend returns `ErrPreconditionFailed`; a missing bucket gives a non-precondition error propagated unchanged.
    - **Evidence:** `<attemptDir>/task-7-ipam-provider-design.txt`, including probe output.
  - **Commit:** Y | `feat(storage): add S3 backend with conditional writes`
  - **Recommended task executor category:** `unspecified-high` (external API + emulator uncertainty).

- [ ] 8. Azure Blob backend (azblob)
  - **What to do:** `pkg/storage/azblob.New(ctx, Config{AccountURL, Container, Prefix})`.
    - Credentials come from `azidentity.NewDefaultAzureCredential`. Use a connection string ONLY when env `AZURE_STORAGE_CONNECTION_STRING` is set (needed for Azurite).
    - **Get:** `DownloadStream`, ETag verbatim.
    - **CreateIfAbsent:** `UploadBuffer` with `AccessConditions.ModifiedAccessConditions.IfNoneMatch = to.Ptr(azcore.ETagAny)`.
    - **PutIfMatch:** `IfMatch`.
    - **DeleteIfMatch:** `DeleteBlob` with IfMatch.
    - **Error mapping:**
      - `bloberror.HasCode(err, bloberror.ConditionNotMet, bloberror.BlobAlreadyExists)` gives `ErrPreconditionFailed`.
      - `BlobNotFound` gives `ErrNotFound`.
    - **List:** `NewListBlobsFlatPager`.
    - Run `storagetest` under the `integration` tag against Azurite (`npx --yes azurite-blob --silent --location $TMPDIR/azurite --blobPort 10000`, well-known dev connection string).
    - Use the current `azblob` version (1.8.2 as of 2026-10-01).
  - **Must NOT do:**
    - Do not use blob leases.
    - Do not accept account keys as config attributes.
  - **Closes:** GAP-5
  - **Parallelization:** Wave 2 | Blocked by: 2 | Blocks: 9,17
  - **References:** draft D3, D8; https://raw.githubusercontent.com/Azure/azure-sdk-for-go/main/sdk/storage/azblob/CHANGELOG.md
  - **Acceptance criteria:** `AZURE_STORAGE_CONNECTION_STRING=<azurite> go test -tags integration ./pkg/storage/azblob -count=1` passes, and `go test ./pkg/storage/azblob` (unit, no network) passes.
  - **QA scenarios:**
    - **Happy:** the Azurite contract suite is green.
    - **Failure:** a stale ETag PUT against Azurite gives `ErrPreconditionFailed`, asserted in the suite and captured in the log.
    - **Evidence:** `<attemptDir>/task-8-ipam-provider-design.txt`
  - **Commit:** Y | `feat(storage): add Azure Blob backend with ETag conditions`
  - **Recommended task executor category:** `unspecified-high` (external SDK + emulator).

- [ ] 9. Backend URL factory shared by CLI and provider
  - **What to do:** `pkg/storage/config.Open(ctx, rawURL string) (storage.Storage, error)` and a struct form `Open(ctx, Config{...})`. Supported URLs:
    - `s3://<bucket>/<prefix>?region=..&endpoint=..&path_style=true`
    - `azblob://<account>.blob.core.windows.net/<container>/<prefix>`, or `azblob://<container>/<prefix>` with `AZURE_STORAGE_CONNECTION_STRING`
    - `file:///abs/path`
    - `mem://`, tests only
  - Unknown schemes give `ErrInvalidArgument`.
  - Table tests cover parsing.
  - **Must NOT do:** no credentials in the URL. Reject userinfo and secrets in the query string.
  - **Closes:** GAP-5, GAP-6
  - **Parallelization:** Wave 3 | Blocked by: 5,6,7,8 | Blocks: 10,13
  - **References:** draft D13; todos 6-8 constructors.
  - **Acceptance criteria:** `go test ./pkg/storage/config -count=1` passes.
  - **QA scenarios:**
    - **Happy:** `file:///tmp/x` gives a local store.
    - **Failure:** `s3://user:pass@b/p` gives an error; `ftp://x` gives `ErrInvalidArgument`.
    - **Evidence:** `<attemptDir>/task-9-ipam-provider-design.txt`
  - **Commit:** Y | `feat(storage): add URL-based backend factory`
  - **Recommended task executor category:** `quick` (parsing + table tests).

- [ ] 10. `ipam` CLI (cobra)
  - **What to do:** `cmd/ipam`, using `github.com/spf13/cobra`.
    - Global `--backend` flag (env `IPAM_BACKEND`, required) and `-o json|table` (default table).
    - **Commands:**
      - `pool create <id> --cidr .. [--cidr ..] | --parent <id> (--prefix-length N | --cidr ..) [--description] [--label k=v]`
      - `pool get <id>`
      - `pool list`
      - `pool delete <id>`
      - `alloc create <pool> <key> (--prefix-length N | --cidr ..) [--description] [--label k=v]`
      - `alloc get <pool> <key>`
      - `alloc list <pool>`
      - `alloc release <pool> <key>`
      - `version`
    - **Exit codes:** 0 ok, 1 generic error, 2 usage, 3 not found, 4 conflict, 5 exhausted.
    - **Tests:** a test harness that executes the root command against `mem://` and `file://` covers every subcommand and the JSON output schema.
    - Add `.goreleaser.yml` for the CLI binary, adapted from wayfinder's, with no signing/publish needed for snapshot.
  - **Must NOT do:**
    - No interactive prompts.
    - No config file format (flags and env only).
  - **Closes:** GAP-6
  - **Parallelization:** Wave 3 | Blocked by: 9 | Blocks: 11,17
  - **References:** draft D2; `/Volumes/code/terraform-provider-wayfinder/.goreleaser.yml`.
  - **Acceptance criteria:**
    - `go test ./cmd/... -count=1` passes.
    - `goreleaser release --snapshot --clean --skip=sign,publish` exits 0.
  - **QA scenarios:**
    - **Happy:** with `IPAM_BACKEND=file://$TMP`, `ipam pool create p --cidr 10.0.0.0/16 && ipam alloc create p a --prefix-length 24 -o json` prints `"cidr":"10.0.0.0/24"`, captured from the real binary.
    - **Failure:** `ipam pool delete p` with an allocation present exits 4 with a `pool_not_empty` message.
    - **Evidence:** `<attemptDir>/task-10-ipam-provider-design.txt`
  - **Commit:** Y | `feat(cli): add ipam command line interface`
  - **Recommended task executor category:** `unspecified-low` (thin CLI over library).

- [ ] 11. Client library documentation and examples
  - **What to do:**
    - Complete `README.md`:
      - Concepts: pool, parent pool, allocation, key idempotency, explicit CIDR as reservation.
      - The JSON document schema v1, with an example.
      - Concurrency model (ETag CAS, retries, per-pool doc contention guidance: split hot pools).
      - Backend URLs and credentials (SDK default chains).
      - Required IAM permissions: S3 `s3:GetObject`, `s3:PutObject`, `s3:DeleteObject`, `s3:ListBucket`; Azure role "Storage Blob Data Contributor".
      - CLI usage.
      - Library example.
    - Add `example_test.go` runnable examples (`ExampleClient_Allocate`) against `mem://`.
    - Add a `CHANGELOG.md` with an Unreleased section.
  - **Must NOT do:** do not document features absent from the code.
  - **Closes:** GAP-6, GAP-7
  - **Parallelization:** Wave 3 | Blocked by: 10 | Blocks: -
  - **References:** todos 2-10 public APIs; draft Decisions.
  - **Acceptance criteria:** `go test ./... -run Example -count=1` passes, and the README mentions every exported constructor in `pkg/storage/*` (`grep` check script in evidence).
  - **QA scenarios:**
    - **Happy:** examples run.
    - **Failure:** every CLI command and flag in the README is cross-checked against `ipam --help` output; zero mismatches.
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
    - `azure_blob = { account_url | (use_connection_string_env bool), container (req), prefix }`
    - `local = { path (req) }`
  - Validate with `objectvalidator.ExactlyOneOf` from `terraform-plugin-framework-validators`.
  - Env fallback `IPAM_BACKEND`, a URL in todo 9's format, used when no block is set.
  - Configure builds `storage` via `pkg/storage/config`, then `ipam.New`, and passes `*ipam.Client` as `ResourceData`/`DataSourceData`.
  - Unknown values at configure time are deferred with a clear diagnostic.
  - Add `internal/provider/errors.go`, mapping ipam typed errors to diagnostics: summary, plus detail with `Reason`.
  - **Unit tests:** schema validation (zero or two blocks are errors); env fallback.
  - **Must NOT do:** no `access_key`/`secret_key`/`account_key` attributes.
  - **Closes:** GAP-3, GAP-5
  - **Parallelization:** Wave 4 | Blocked by: 9,12 | Blocks: 14,15
  - **References:** draft D13; lane a6 (`.omo/ulw-research/20261028-150017/wave-1-lane-a6-tf-framework-patterns.md`, section 2: provider auth belongs in provider config); wayfinder `internal/provider/` for structure.
  - **Acceptance criteria:** `go test ./internal/provider -run TestProvider -count=1` passes.
  - **QA scenarios:**
    - **Happy:** a config with `local { path = "<tmp>" }` configures.
    - **Failure:** setting both `s3` and `local` gives a diagnostic containing "Exactly one of".
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
    - `description` (Optional, RequiresReplace)
    - `labels` (Optional map, RequiresReplace)
  - **Validators:**
    - Root pool: `cidrs` set and no parent fields.
    - Child pool: `parent_pool` plus exactly one of `prefix_length`/`cidr`, and `cidrs` not configured.
  - Computed attributes use `UseStateForUnknown`.
  - **Lifecycle:**
    - Create calls `CreatePool`.
    - Read calls `GetPool`; NotFound removes it from state.
    - There is no in-place update: every attribute RequiresReplace.
    - Delete calls `DeletePool`; `pool_not_empty` becomes an error diagnostic.
  - **ImportState:** passthrough `id`. Implement resource identity (`id`) per framework identity support.
  - **Data source `ipam_pool`:** input `id`; outputs `cidrs`, `family`, `parent_pool`, `description`, `labels`, and `allocations` (map key to cidr).
  - **Acceptance tests** (`resource.Test`, `ProtoV6ProviderFactories`, local backend in `t.TempDir()`):
    - Create root pool, then import-verify, then destroy.
    - Create parent and child (`prefix_length=20`), check the child's `cidrs[0]` is inside the parent, then destroy (Terraform orders child before parent).
    - Changing `cidrs` forces replacement (plan check `plancheck.ExpectResourceAction(..., plancheck.ResourceActionReplace)`).
    - Pool deleted out-of-band through the library: the next plan is non-empty (create).
  - **Must NOT do:** no `Update` that mutates remote state.
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

- [ ] 15. `ipam_allocation` resource and data source
  - **Resource attributes:**
    - `pool` (Required, RequiresReplace)
    - `key` (Required, RequiresReplace; the idempotency key, unique per pool)
    - `prefix_length` (Optional, RequiresReplace)
    - `cidr` (Optional+Computed, RequiresReplace when configured, `UseStateForUnknown`); setting it requests an explicit allocation
    - `description`, `labels` (Optional, RequiresReplace)
    - `id` (Computed `<pool>/<key>`)
  - Exactly one of `prefix_length` or a configured `cidr`. Use a ConfigValidator that inspects raw config, so a computed `cidr` does not count as configured.
  - **Lifecycle:**
    - Create calls `Allocate`. This is idempotent, so a crash-recovered retry adopts the existing allocation.
    - Read calls `GetAllocation`; NotFound removes it from state.
    - Delete calls `Release`.
  - **Import:** ID `<pool>/<key>`, split on the FIRST `/`; pool IDs cannot contain `/`. Implement resource identity `{pool,key}`.
  - **Data source `ipam_allocation`:** inputs `pool`, `key`; outputs `cidr`, `prefix_length`, `description`, `labels`.
  - **Acceptance tests:**
    - Allocate a `/24`, check `cidr`, import-verify (ImportStateId `p/k`), destroy, and check the library shows it released.
    - `count = 20` allocations in one apply (default parallelism 10): all distinct and non-overlapping, checked with a `TestCheckFunc` using `netip`.
    - Explicit `cidr` allocation plus a next allocation do not overlap.
    - Recovery: pre-create an allocation with the same key/request through the library, then apply. Apply succeeds with the same CIDR, and the plan afterwards is empty.
    - Same key with a different `prefix_length` pre-existing gives `ExpectError` `key_reused`.
    - Allocation deleted out-of-band: the next plan recreates it.
    - Exhausted pool gives `ExpectError` "exhausted".
  - **Must NOT do:** no data source that previews or picks the next CIDR.
  - **Closes:** GAP-1, GAP-2, GAP-3, GAP-4
  - **Parallelization:** Wave 4 | Blocked by: 14 | Blocks: 16,17
  - **References:** research C1 (allocation is an apply-time managed write), C10 (idempotency), lane a6 sections 1 and 4; todo 5 semantics.
  - **Acceptance criteria:** `TF_ACC=1 go test ./internal/... -run 'TestAccAllocation' -count=1 -race` passes.
  - **QA scenarios:**
    - **Happy:** the `count = 20` test passes.
    - **Failure:** the `key_reused` test passes with the expected error, and the exhausted-pool test passes.
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
      - the `key` idempotency guidance (use stable keys; changing a key replaces the allocation).
    - Run `make docs` (`tfplugindocs generate` + `validate` via `go run`).
    - Complete the README and `CHANGELOG.md`.
  - **Must NOT do:** do not hand-edit generated `docs/` except via templates.
  - **Closes:** GAP-3, GAP-6
  - **Parallelization:** Wave 4 | Blocked by: 15 | Blocks: 17
  - **References:** wayfinder `Makefile` docs target, `/Volumes/code/terraform-provider-wayfinder/templates/`.
  - **Acceptance criteria:**
    - `make docs` exits 0.
    - `git diff --exit-code docs/` after a second `make docs` (docs are reproducible).
    - `terraform fmt -check -recursive examples` exits 0.
  - **QA scenarios:**
    - **Happy:** `tfplugindocs validate` passes.
    - **Failure:** removing an example file, in a scratch copy only, makes `validate` or the check fail. Capture it, then discard the scratch copy.
    - **Evidence:** `<attemptDir>/task-16-ipam-provider-design.txt`
  - **Commit:** Y | `docs: add provider documentation and examples`
  - **Recommended task executor category:** `writing` (docs generation).

- [ ] 17. Cross-repo end-to-end with real CLI and emulators
  - **What to do:** Add `scripts/e2e.sh` in the provider repo. It must:
    1. Build the provider binary and write a `dev_overrides` CLI config (`TF_CLI_CONFIG_FILE`).
    2. Build the `ipam` CLI from `../ipam`.
    3. Start Azurite (npx) and the S3 emulator selected in todo 7.
    4. For each backend, run the same flow:
       - Create a pool with the CLI.
       - Run TWO separate Terraform working dirs, each with its own state and `count = 10` allocations from the same pool, applied concurrently (`&` + `wait`).
       - Assert with `ipam alloc list -o json` and `jq` that the 20 CIDRs are unique and non-overlapping, using a small Go checker `go run ./scripts/overlapcheck`.
       - `terraform destroy` both, then assert `ipam alloc list` is empty and `ipam pool delete` succeeds.
    5. Exit non-zero on any assertion.
    6. Kill the emulators on exit (`trap`).
  - **Must NOT do:**
    - Do not use real cloud accounts unless the `IPAM_TEST_S3_BUCKET` / `AZURE_STORAGE_ACCOUNT_URL` envs are set.
    - Do not leave background processes running.
  - **Closes:** GAP-1, GAP-5, GAP-6
  - **Parallelization:** Wave 5 | Blocked by: 7,8,10,16 | Blocks: F1-F4
  - **References:** todos 7, 8, 10, 15; Terraform `dev_overrides` docs (https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers).
  - **Acceptance criteria:** `bash scripts/e2e.sh` exits 0 and prints `E2E PASS azblob` and `E2E PASS s3`, or, if todo 7 recorded no working S3 emulator and no AWS env, prints `E2E SKIP s3 (reason)` and still passes azblob and local.
  - **QA scenarios:**
    - **Happy:** the full script run log.
    - **Failure:** running with `IPAM_E2E_FORCE_OVERLAP=1`, a test hook that pre-inserts an overlapping CIDR into the JSON doc by direct file write on the local backend, makes the overlap checker or decode validation fail with non-zero exit.
    - **Evidence:** `<attemptDir>/task-17-ipam-provider-design.log`
  - **Commit:** Y | `test: add cross-repo end-to-end script`
  - **Recommended task executor category:** `unspecified-high` (multi-process orchestration).

## Final verification wave
> Runs in parallel after ALL todos. ALL must APPROVE. Surface results and wait for the user's explicit okay before declaring complete.
- [ ] F1. Plan compliance audit
  - Every todo's acceptance command is re-run, and every Must NOT have is checked:
    - `grep -r gocloud.dev`, `preview`, `access_key` and `account_key` in both repos give no hits.
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
  - Check each IS-1..IS-8 row against the evidence in Success criteria, 1:1.
  - Any shortfall becomes new numbered todos.
  - **Recommended task executor category:** `unspecified-high`

## Commit strategy
- One commit per todo, using Conventional Commits with the message on each todo's Commit line.
- The client repo (`/Volumes/code/ipam`) gets commits from todos 1-11; the provider repo gets commits from todos 12-17.
- No pushes, tags or remotes: the user creates `github.com/appvia/ipam`, publishes v0.1.0, and then removes the provider's `replace` directive. That is a follow-up outside this plan.

## Success criteria
> One row per IS row. The plan is complete only when every IS row has a delivering todo and a proving QA scenario; F4 checks the delivered behavior against these rows 1:1, and a shortfall becomes new `- [ ] N.` rows, never a note.
| IS | Delivering todo(s) | Proving QA scenario | Evidence |
| --- | --- | --- | --- |
| IS-1 | 4,5,15,17 | 5 unique-key concurrency; 15 `count=20`; 17 two-state concurrent apply | task-5/15/17 |
| IS-2 | 5,15 | 5 same-key convergence; 15 recovery adopts existing + `key_reused` | task-5/15 |
| IS-3 | 14,15 | import-verify, out-of-band delete recreates, empty second plan | task-14/15, F3 |
| IS-4 | 5,14,15 | `pool_not_empty` guard; destroy releases (library check) | task-5/14/15 |
| IS-5 | 2,6,7,8,17 | contract suite on memory/local/S3/Azurite; e2e per backend | task-2/6/7/8/17 |
| IS-6 | 5,9,10,11 | CLI happy/failure on real binary; runnable examples | task-10/11 |
| IS-7 | 3,11 | golden round-trip; `schema_version 2` rejected | task-3 |
| IS-8 | 9,13,17 | e2e uses only a bucket/container + env credentials | task-17 |
