---
slug: ipam-provider-design
status: plan-approved-by-reviewer
intent: clear
review_required: true
plan_path: .omo/plans/ipam-provider-design.md
plan_sha256: 4a2eec17c555cb2e1079267c65f87b3372a0bd1a6ea7fde9cb7b9ed2abd51104
review_round_id: r4-20261029
review_round_limit: 5
pending-action: execute via /ulw-execute ipam-provider-design
review:
  plan_reviewer:
    status: approved
    workspace_root: /Volumes/code/terraform-provider-ipam
    runtime_home: null
    target: .omo/plans/ipam-provider-design.md
    round_id: r4-20261029
    plan_sha256: 4a2eec17c555cb2e1079267c65f87b3372a0bd1a6ea7fde9cb7b9ed2abd51104
    launch_id: l4-20261029
    session: st_01a187c7
    result: "OKAY r4 (notes only, reviewer: none needs another round): Azurite versioning No vs WARN (set delete retention in test setup); matrix parallel columns for 18/19/20 vs 11/16 (Depends-on column authoritative); internal Allocate/Release path for pool: IDs + CLI must reject pool: IDs; task 2 no hand-written retry loops wording; task 8 ErrInvalidArgument must be a storage/backend sentinel; TL;DR builtin-role exception."
approach: Two Go repos. (1) client library (module TBD) owning pools/allocations, CIDR allocator on net/netip, Storage interface with S3 + Azure Blob + memory impls using ETag CAS (one JSON doc per pool), plus CLI. (2) terraform-provider-ipam (plugin framework) importing the client: ipam_pool, ipam_allocation resources, ipam_pool data source, idempotent allocation keys.
classification: architecture
stance: one-by-one (cold start, CLEAR)
---

# Draft: ipam-provider-design

## Affected user and ideal state
<!-- Who this output touches (a customer, another programmer, a program or agent consuming it - often more than one) and how each uses it today and will use it after. -->
<!-- IS-n | property of the ideal state: nothing snags, feels odd, regresses, or degrades for them | reason -->
<!-- GAP-n | difference between IS-n and today | reason | closed by todo(s) -->
Users: (U1) platform engineers writing Terraform that needs non-overlapping CIDRs across many states/accounts/clouds; (U2) Go programmers building the CLI / future web front end on the client library; (U3) operators who inspect/repair state in the bucket.
- IS-1 | Two independent `terraform apply` runs (different states) never receive overlapping CIDRs from the same pool | core IPAM promise; static-ipam/cidrsubnet fail this (research C1)
- IS-2 | Re-applying / retrying after a crash returns the same allocation, never a duplicate | idempotency gap in Xtratus provider (C10)
- IS-3 | Allocation result is known after apply, stable across plans, drift visible on refresh, importable | TF framework guidance (lane a6)
- IS-4 | Destroy releases the CIDR; pool deletion refused while allocations exist | no orphaned/leaked space
- IS-5 | Same behaviour on S3 and Azure Blob; backend chosen by provider config only | user requirement: Go interface abstraction
- IS-6 | Client library usable without Terraform (CLI, future UI) with a small stable API | user requirement: two repos
- IS-7 | State is human-readable JSON in the bucket, versioned schema | operator repair/inspection, simplicity
- IS-8 | Works with no server to run: only a bucket + credentials | 'simple' requirement
- GAP-1..8: nothing exists today (repo has only research, commit 94b4c59); every IS row is a gap.

## Components (topology ledger)
<!-- Lock the SHAPE before depth. One row per top-level component that can succeed or fail independently. -->
<!-- id | outcome (one line) | status: active|deferred | evidence path -->

## Open assumptions (announced defaults)
<!-- Record any default you adopt instead of asking, so the user can veto it at the gate. -->
<!-- assumption | adopted default | rationale | reversible? -->

## Findings (cited - path:lines)
- Repo empty except .omo research; remote github.com/appvia/terraform-provider-ipam; go1.27.3 local.
- Research SYNTHESIS.md/claim-graph.md: C1 apply-time managed write; idempotent create w/ allocation key; C8 collision-domain scope; preview/next-available data source advisory only; backend-agnostic provider over cloud IPAMs rejected (debate-log framing) - N/A here because our library IS the authority.
- Lane a6: cidr Computed+UseStateForUnknown; pool/prefix RequiresReplace; resource identity (TF1.12), list resources TF1.14-only optional; provider functions for pure CIDR math only; no ephemeral/write-only for CIDRs.
- architect advisory lane unavailable (no model) - skipped.

## Decisions (with rationale)
- D1 (user) Data model: pools (static CIDRs OR carved from parent_pool) + allocations. No scope/space concept; a root pool tree = collision domain.
- D2 (user) Client repo github.com/appvia/ipam at /Volumes/code/ipam, cobra CLI at cmd/ipam built in this plan.
- D15 (user, 2026-10-28 "Drop the pkg prefix... root of the package can be the client entrypoint?") Layout: no pkg/. Module root package `ipam` = client entrypoint (New, Open, OpenStorage, public types, errors). `storage` + `storage/{memory,local,s3,azblob,storagetest}` stay PUBLIC (interface is in ipam.New signature; provider is a separate module and cannot import internal/). `internal/alloc` (pure netip allocator) and `internal/document` (v1 JSON codec) are internal. URL factory moved from storage/config to root `open.go`. Provider stays appvia/terraform-provider-ipam, registry appvia/ipam. User creates GitHub remote.
- D16 (user, 2026-10-29 "I think I am sold on the parent/child situation for pool managers and reservations") Storage split: pools/<id>.json (platform-written definitions) + reservations/<id>.json (allocations; written by per-environment deploy identity and platform). Pools are per location/environment, not team. Access enforced by S3 IAM object ARNs / Azure blob-path role-assignment conditions (flat namespace only); documented, not managed by code in this plan.
- D17 (user, 2026-10-29 "Accepted on ID being in modifyPlan. Add this to plan.") Allocation identity = generated UUIDv7, set in ipam_allocation ModifyPlan on create; idempotent by ID (same ID+request returns existing; different request -> id_reused). No caller key, no owner. Deterministic-from-config IDs rejected (count/cross-state collisions; provider cannot see resource address).
- D18 (user, 2026-10-29 answers: "Azure + AWS (Recommended)", "Separate repos (Recommended)", "Add ipam doctor (Recommended)") Provisioning modules terraform-azurerm-ipam and terraform-aws-ipam in separate repos under /Volumes/code from the Appvia module template (GPL-3.0 licence inherited, flagged); storage hardening + platform/per-environment access rules; no pools. CLI gets read-only `ipam doctor` (non-mutating write probe with same bytes + wrong version).
- D19 (user, 2026-10-29 "For azure I want us to use https://github.com/Azure/entrauth/aztfauth") Azure auth via aztfauth.NewCredential in provider (azure_blob block mirrors azapi auth attrs + ARM_* env) and CLI/ipam.Open (azblob.CredentialFromEnv). Library azblob.Config takes azcore.TokenCredential. Pin pseudo-version as azapi does (no tags). Lazy token fetch keeps Configure offline. Replaces earlier "SDK default chains only / no credentials in config" rule: sensitive auth attrs allowed (provider config not stored in state).
- D20 (user idea 2026-10-29 "Ipam pool creation can be in same apply as storage creation?" + "storage data plane ... retryfunc built in") Single-apply supported: provider Configure offline; module outputs built from inputs (known at plan); access-denied retry during role propagation via azcore RetryOptions.ShouldRetry (Azure) / per-call S3 retryer, gated by storage.WithAccessPropagationRetry context flag set only in provider Create/Update/Delete with access_propagation_timeout (amended after review r1 B2) (default 5m). Docs warn about data sources in first apply.
- D21 (user, 2026-10-29 answer "Generate in Create (Recommended)") Supersedes the plan-time half of D17: ModifyPlan ID generation is impossible (Terraform re-plans during apply and requires identical known values). Allocation id is unknown at plan and generated once in Create before the CAS loop; no key/owner. Saved-plan crash adoption dropped; a hard crash can leak one allocation (safe, recoverable).
- D22 (user, 2026-10-29 answer "Toggle, default custom (Recommended)") Azure module input environment_role = custom (default; data-actions-only custom role + ABAC) | builtin (Storage Blob Data Contributor + ABAC write-own-paths, never delete/add/move; container deletion via management actions remains possible; documented).
- Plan-consultant (st_01a18776, 2026-10-29) findings folded: env grants without delete + data-actions-only Azure custom role; unconditional ListBucket + GetBucketVersioning; AtMostOneOf + IPAM_BACKEND fallback; Azurite-only connection string; incarnation token vs ETag reuse (ABA); crash-safe DeletePool order + parent scan; delete treats NotFound as success; in-place description/labels; dependency matrix fixes; module/provider single-apply contract test; doctor WARN semantics; emulator preflight task 21; falsifiable acceptance (no-network transport, checked waits, overlapcheck unit tests, golden condition, schema/AST-based F1, F4 covers IS-1..11 with UNVERIFIED marking); single AuthConfig mapping; constraints defaults (scale, $0, TF>=1.12, public cloud, Windows CI).
- D3 (user) Backends now: S3 + Azure Blob + memory; integration via Azurite and MinIO/LocalStack (MinIO If-Match support to be verified as a task).
- D4 (user) TDD for allocator + storage contract suite; provider acceptance tests on memory backend; licences MPL-2.0 provider, Apache-2.0 client.
- D5 (default) Storage: one JSON object per pool tree? -> one object per pool, CAS via ETag If-Match; create via If-None-Match:*. Child pool creation = CAS on parent doc (allocation of kind pool) then create child doc.
- D6 (default) IPv4 + IPv6 via net/netip; lowest-aligned-first allocation.
- D7 (default) Idempotency: allocation `name` (key) unique per pool; same key+same request returns existing.
- D8 (verified) S3 conditional writes If-Match/If-None-Match documented (docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html); azblob 1.8.2 current (2026-10-01).
- D9 (default, ultrabrain lane) Allocator: net/netip + go4.org/netipx IPSet, per-parent free-set, lowest aligned candidate; pools single-family, multiple disjoint CIDRs; explicit CIDR allocation supported; allocation request descriptor stored for idempotency; CAS loop max 8 attempts full jitter 10ms..250ms; typed errors ErrNotFound/ErrPreconditionFailed/ErrConflict/ErrPoolExhausted/ErrInvalidCIDR/ErrUnsupportedSchema.
- D10 (default) No separate reservations concept: an explicit-CIDR allocation is a reservation (simplicity).
- D11 (default) Child pool: CAS allocation in parent with key `pool:<child-id>` then CreateIfAbsent child doc; retry converges by key. Delete child = delete empty child doc then release parent key.
- D12 (default) Add `local` filesystem backend (flock + sha256 ETag) for CLI dev and TF acceptance tests; memory backend for unit tests.
- D13 (default) Provider config: one of `s3 {bucket,prefix,region,endpoint}`, `azure_blob {account_url,container,prefix}`, `local {path}`; auth via SDK default credential chains only.
- D14 (default) Terraform surface: resources ipam_pool, ipam_allocation; data sources ipam_pool, ipam_allocation; import IDs `<pool>` and `<pool>/<key>`; no preview/next-available data source.

## Scope IN

## Scope OUT (Must NOT have)

## Open questions

## Approval gate
status: awaiting-approval
next-action: on approval write and review .omo/plans/ipam-provider-design.md
<!-- When exploration is exhausted and unknowns are answered, set status: awaiting-approval. -->
<!-- That durable record is the loop guard: on a later turn read it and resume at the gate instead of re-running exploration. -->

## Review ledger
- Round 1 (r1-20261029, st_01a1879f, sha aa35f576...): REJECT. Accepted blockers: B1 (lost-response IS-2 proof), B2 (Update access-propagation vs guardrail), B3 (task 5 depends on task 6). Notes folded: N1 incarnation_mismatch retry, N2 filesystem_mirror for dev provider in terraform test, N3 Azure role name includes storage account, N4 KMS permissions. Ledger frozen after round 1.
- Round 2 (r2-20261029, st_01a187a8, sha 9ebb7d06...): OKAY with notes. B1-B3 confirmed closed. Notes folded; plan change invalidated r2 approval.
- Round 3 (r3-20261029, st_01a187b3, sha 283dbf13...): OKAY with notes, no blockers. Live plan hash re-validated equal to approved digest. Convergence: r3 raised no new blocker -> no further round runs automatically; post-approval note patches would require a fresh round.
- User chose (2026-10-29 "Patch + round 4 (Recommended)") to patch r3 notes and run confirming round 4.
- Round 4 (r4-20261029, st_01a187c7, sha 4a2eec17...): OKAY with notes; live hash re-validated equal to approved digest. Review complete. Notes left for the executor (plan unchanged to preserve the approved digest). Pending user option: storage-account CanNotDelete lock (not added).
- Fixes for r1: B1 -> storage.ErrUnknownOutcome sentinel + CAS re-read on it (tasks 2,5,7,8,15); B2 -> propagation retry allowed on Create/Update/Delete, guardrail + task 13 aligned; B3 -> task 5 blocked by 6, matrix/waves updated, ABA premise reworded + negative control.
