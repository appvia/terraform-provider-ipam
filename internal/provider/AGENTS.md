# internal/provider

## OVERVIEW
Whole provider in one package: provider config + `ipam_pool` / `ipam_allocation` resources and data sources over `*ipam.Client`. Earned its file: score ~17 (all code, 30+ symbols, highest reference centrality).

## WHERE TO LOOK
| Task | File | Notes |
|------|------|-------|
| Backend blocks, auth, redaction | provider.go | `openS3`, `openAzure`, `azureAccountURL`, `redact`, `boolPointer` |
| Error kinds, hints | errors.go | `errorKinds`, `reasonHints` (by `OpError.Reason`), `residueHint` |
| Pool CRUD, carve-outs, import | pool_resource.go | also hosts shared `providerData`, `alreadyGone`, `canonicalCIDR`, `poolIDValidators` |
| Allocation CRUD, uncertain create | allocation_resource.go | `mayHaveCommitted`, `unknownToNull`, `parseAllocationImportID` |
| Data sources | pool_data_source.go, allocation_data_source.go | reuse `poolResourceModel.setPool` |
| Test harness | pool_resource_test.go, provider_test.go, allocation_resource_test.go | `testAccFactories`, `testAccBackend`, `isolateEnv`, `recordingStore`, `lostResponseStore`, `newAllocationHarness` |

## CONVENTIONS
- Resources/data sources hold `data *Data` (set via `providerData`), never a bare `*ipam.Client`.
- Create/Update/Delete: `mctx, cancel := r.data.MutationContext(ctx); defer cancel()` and pass `mctx` to the client. Read and both data sources use plain `ctx` so access denied fails fast. Pinned by `Test{Pool,Allocation}ResourceMutationsUseMutationContext` (recordingStore checks ctx per call).
- Client built only by `newClient`: `ipam.WithMaxAttempts(0)` = retry conflicts until ctx ends; MutationContext supplies the deadline (`access_propagation_timeout`).
- Every client error -> `errorDiagnostic("<Verb> <noun>", err)`; operation strings are matched (`residueOperations` = only `"Create pool"`, `"Delete pool"` get `residueHint` on `DeadlineExceeded`). Keep strings exact when adding ops.
- New error kind/reason: add to `errorKinds` / `reasonHints` (includes `cooldown` for strict refusals), test in errors_test.go / errors_hint_test.go.
- Resource Read: plain `ipam.ErrNotFound` -> `RemoveResource`; data source Read: not found is an error diagnostic.
- Delete ignores not-found only via `alreadyGone` (reason-less); `pool_incomplete` not-found still errors.
- Allocation ID: `ipam.NewAllocationID()` generated ONCE in Create, written to `plan.ID` before the call. If `mayHaveCommitted(err)`: `unknownToNull()` + save state (Terraform marks it tainted) so refresh/destroy can reach it. Definite failures (exhausted, not found, invalid, unsupported schema, reason `overlap`/`id_reused`) leave no state.
- Plan modifiers: user-set identity/shape attrs `RequiresReplace` (`id`, `parent_pool`, `prefix_length`, pool `cidr`, `cidrs`, alloc `pool`); computed `UseStateForUnknown` (`family`, alloc `id`, `created_at`, `cidrs`); alloc `cidr` is Optional+Computed with `RequiresReplaceIfConfigured`. Only `description`, `labels`, `strategy`, `cooldown` update in place.
- Pool policy: `strategy` is Optional+Computed with `UseStateForUnknown` (omitted = keep stored; new pools get `best-fit`). `cooldown.duration` is a plain string; `setPool`/`setCooldown` keep the config's spelling when it parses to the stored duration (import/data source get Go canonical `24h0m0s`). Create refuses adoption (`pool_exists`) of a pool with a stored cooldown when config omits `cooldown`. Update calls `UpdatePoolPolicy` (with `mctx`) only when the policy changed.
- Null vs empty: `setPool`/`setAllocation` keep null `description`/`labels` when the store is empty and config omitted them; empty map labels from config stay empty (`TestAccPoolResourceIPv6EmptyLabels`). Labels to client: null -> `nil` map.
- CIDRs must be canonical (`canonicalCIDR`: masked, lowercase, no 4-in-6) so state == config.
- Identity: pool `{id}`, allocation `{pool,id}`; import string `<pool>/<id>` split on first `/`.
- Pool `ValidateConfig`: root needs `cidrs`; child needs `parent_pool` + exactly one of `prefix_length`/`cidr`. Return early on unknowns.

## ANTI-PATTERNS
- Never pass `ctx` (not `mctx`) to a mutating client call, or `mctx` to Read.
- Never regenerate the allocation ID on retry or drop state on an uncertain create (leaks a committed allocation).
- Never add `err.Error()` from Azure/backend setup to a diagnostic without `redact`; tests `requireNoLeak` assert this.
- Never call the network in `Configure`.
- Acc tests: call `testAccBackend(t)` (isolates env, local temp dir); never read developer creds. `for_each` configs are not covered here (plugin-testing); rely on scripts/e2e.sh.

## TESTS
`go test ./internal/provider -run 'TestProvider|TestPool|TestAllocation|TestLostResponse|TestErrorDiagnostic' -count=1` (unit); `TF_ACC=1 go test ./internal/provider -run TestAcc -count=1` (needs `terraform` on PATH).
