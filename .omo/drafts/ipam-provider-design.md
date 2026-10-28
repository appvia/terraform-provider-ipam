---
slug: ipam-provider-design
status: awaiting-approval
intent: clear
review_required: true
plan_path: .omo/plans/ipam-provider-design.md
plan_sha256: null
review_round_id: null
review_round_limit: 5
pending-action: write and review .omo/plans/ipam-provider-design.md
review:
  plan_reviewer:
    status: pending
    workspace_root: null
    runtime_home: null
    target: .omo/plans/ipam-provider-design.md
    round_id: null
    plan_sha256: null
    launch_id: null
    session: null
    result: null
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
- D2 (user) Client repo github.com/appvia/ipam at /Volumes/code/ipam: pkg/ipam, pkg/storage/{s3,azblob,memory}, cmd/ipam cobra CLI built in this plan. Provider stays appvia/terraform-provider-ipam, registry appvia/ipam. User creates GitHub remote.
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
