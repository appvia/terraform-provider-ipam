st_01a1830c [completed] transcript via event-log:
assistant: I read this as an exhaustive, source-grounded assessment of Terraform/OpenTofu framework capabilities for IPAM allocation resources - I’ll research every named axis, reconcile late-2026 status and divergences, and stop when the cited report with CLAIMS and EXPAND sections is delivered.
tool: todo
assistant: [Research plan initialized] toward a late-2026 capability assessment for an IPAM allocation provider. You need: 0/14 tasks done; sources will be limited to HashiCorp framework/Terraform material and OpenTofu documentation, with attached-browser fallback disclosed. Now: Read browser research workflow instructions. Next: Confirm attached browser retrieval readiness.
tool: read
tool: todo
tool: read
tool: eval
assistant: [Source path established] toward the requested official-docs research. You need: 2/14 tasks done once this check is recorded; the attached engine is unavailable because BrowserSkill reports `no-daemon`, so I am using the required `webfetch`/official-web fallback. Now: Research allocation schema and plan modifiers. Next: Research ephemeral and write-only capability relevance.
tool: todo
tool: web_search
tool: web_search
tool: web_search
tool: web_search
tool: web_search
tool: web_search
tool: web_search
tool: web_search
tool: web_search
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: todo
tool: todo
tool: todo
tool: todo
tool: todo
tool: todo
tool: todo
tool: todo
assistant: [Capability research complete] toward the IPAM provider design report. You need: 11/14 tasks done once the divergence item is recorded; Terraform’s capability set is well sourced, while OpenTofu’s current docs confirm identity/functions/ephemerality but do not document Terraform 1.14-style query/list resources or actions, so those will be treated as optional compatibility extensions rather than baseline behavior. Now: Cross-check late-2026 feature status and versions. Next: Draft cited recommendations for IPAM provider.
tool: todo
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: todo
tool: todo
tool: todo
assistant: [Research delivered] toward a late-2026 design basis for Terraform/OpenTofu IPAM allocation resources. You need: 14/14 research tasks completed; official framework, Terraform 1.8–1.14 changelogs, and current OpenTofu documentation were cross-checked. The attached browser was unavailable (`no-daemon`), so the required official-site `webfetch` fallback was used. Now: none. Next: none.

# Axis A6: Provider-framework capabilities for IPAM allocations

## Executive recommendation

Model an allocation as an ordinary managed resource:

- Configured allocation intent: parent pool, requested prefix length, address family, scope/tenant, description, tags, and—critically—a stable caller-supplied allocation key where the API supports idempotency.
- Computed results: immutable allocation ID, allocated CIDR, timestamps/status, and other server-selected values.
- Treat the allocated CIDR as durable state, not ephemeral or write-only.
- Make changes that require a different allocation replace the resource.
- Expose resource identity and legacy import IDs.
- Add Terraform 1.14 list-resource support for discovering and bulk-importing existing allocations, but keep it optional because OpenTofu does not yet document equivalent query/list-resource support.
- Restrict provider-defined functions to pure, offline CIDR math.
- Do not use actions for normal release or “preview next CIDR”: actions have neither referenceable output nor state-management authority.
- Require correctness from the IPAM API’s atomic allocation and idempotency contract. A provider mutex is only a local optimization.

## Capability timeline

| Capability | Terraform status | OpenTofu status relevant at cutoff |
|---|---|---|
| Provider-defined functions | Introduced in Terraform 1.8; invoked as `provider::<name>::<function>()` ([1.8 changelog](https://raw.githubusercontent.com/hashicorp/terraform/v1.8.0/CHANGELOG.md), [framework functions](https://developer.hashicorp.com/terraform/plugin/framework/functions)) | Supported since OpenTofu 1.7, including provider-alias-qualified calls ([OpenTofu functions](https://opentofu.org/docs/language/functions/)) |
| Ephemeral resources | Terraform 1.10 ([1.10 changelog](https://raw.githubusercontent.com/hashicorp/terraform/v1.10.0/CHANGELOG.md), [framework guide](https://developer.hashicorp.com/terraform/plugin/framework/ephemeral-resources)) | OpenTofu 1.11 ([1.11 release](https://opentofu.org/blog/opentofu-1-11-0/), [ephemeral resources](https://opentofu.org/docs/language/ephemerality/ephemeral-resources/)) |
| Write-only attributes | Terraform 1.11 ([1.11 changelog](https://raw.githubusercontent.com/hashicorp/terraform/v1.11.0/CHANGELOG.md), [framework guide](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments)) | OpenTofu 1.11 ([1.11 release](https://opentofu.org/blog/opentofu-1-11-0/), [write-only attributes](https://opentofu.org/docs/language/ephemerality/write-only-attributes/)) |
| Resource identity/import by identity | Terraform 1.12; Terraform 1.13 fixed identity-loss cases ([1.12 changelog](https://raw.githubusercontent.com/hashicorp/terraform/v1.12.0/CHANGELOG.md), [1.13 changelog](https://raw.githubusercontent.com/hashicorp/terraform/v1.13.0/CHANGELOG.md), [identity guide](https://developer.hashicorp.com/terraform/plugin/framework/resources/identity)) | Current OpenTofu import blocks support `identity` and legacy `id` ([OpenTofu import blocks](https://opentofu.org/docs/language/import/)) |
| List resources / `terraform query` / bulk import | Terraform 1.14 ([1.14 changelog](https://raw.githubusercontent.com/hashicorp/terraform/v1.14.0/CHANGELOG.md), [list-resource framework guide](https://developer.hashicorp.com/terraform/plugin/framework/list-resources), [bulk import](https://developer.hashicorp.com/terraform/language/import/bulk)) | No equivalent is documented in the reviewed OpenTofu 1.13 release or current CLI documentation; treat it as Terraform-only until tested and officially documented ([OpenTofu 1.13 release](https://opentofu.org/blog/opentofu-1-13-0/), [OpenTofu sitemap](https://opentofu.org/sitemap.xml)) |
| Provider actions | Terraform 1.14 ([1.14 changelog](https://raw.githubusercontent.com/hashicorp/terraform/v1.14.0/CHANGELOG.md), [actions overview](https://developer.hashicorp.com/terraform/plugin/framework/actions)) | No equivalent is documented in the reviewed OpenTofu 1.13 material; do not make actions necessary for core provider operation ([OpenTofu 1.13 release](https://opentofu.org/blog/opentofu-1-13-0/), [OpenTofu sitemap](https://opentofu.org/sitemap.xml)) |

## 1. Allocation schema and plan modifiers

For a server-selected allocation, define `cidr` as `Computed: true`. Terraform’s resource creation contract explicitly permits providers to replace unknown planned values with known API results, and HashiCorp’s example uses a computed server-generated ID with `UseStateForUnknown()` ([create lifecycle](https://developer.hashicorp.com/terraform/plugin/framework/resources/create)).

Use the modifiers as follows:

- **`UseStateForUnknown()` on `cidr` and `allocation_id`:** It copies prior state into an otherwise unknown plan and is intended for computed attributes known not to change over the object’s lifetime. This prevents noisy `(known after apply)` output on unrelated updates ([plan modification](https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification#common-use-case-attribute-plan-modifiers)).
- **`RequiresReplace()` on immutable allocation inputs:** Parent pool, requested prefix length, address family, and immutable scope/tenant identifiers should force replacement when the API cannot reallocate them in place. The modifier marks a changed attribute for replacement rather than update ([plan modification](https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification#requiresreplace)).
- **`RequiresReplaceIf()` for conditional immutability:** Use this only where replacement depends on multiple values or backend capability—for example, changing a requested prefix length may be updatable while pending but replacement-only after activation. It accepts provider-defined conditional logic ([plan modification](https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification#requiresreplaceif)).
- **Do not use `UseStateForUnknown()` for values that legitimately change on update.** The framework warns that final update-state values must agree with planned values; stale state substituted into the plan can otherwise cause an inconsistent-result error ([plan modification](https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification)).

Recommended shape:

```go
"allocation_id": schema.StringAttribute{
    Computed: true,
    PlanModifiers: []planmodifier.String{
        stringplanmodifier.UseStateForUnknown(),
    },
},
"cidr": schema.StringAttribute{
    Computed: true,
    PlanModifiers: []planmodifier.String{
        stringplanmodifier.UseStateForUnknown(),
    },
},
"pool_id": schema.StringAttribute{
    Required: true,
    PlanModifiers: []planmodifier.String{
        stringplanmodifier.RequiresReplace(),
    },
},
"prefix_length": schema.Int64Attribute{
    Required: true,
    PlanModifiers: []planmodifier.Int64{
        int64planmodifier.RequiresReplace(),
    },
},
```

`Read` should refresh all observable values, remove the resource from state on definitive not-found, and preserve prior values only when semantically equal. HashiCorp specifically recommends these behaviors to expose drift while avoiding cosmetic diffs ([read lifecycle](https://developer.hashicorp.com/terraform/plugin/framework/resources/read)).

## 2. Ephemeral resources and write-only attributes

Neither capability fits the allocated CIDR:

- Ephemeral resources are opened for a Terraform phase and closed afterward; their values are not stored in plan or state and can only flow into restricted ephemeral contexts ([Terraform ephemeral resources](https://developer.hashicorp.com/terraform/plugin/framework/ephemeral-resources), [OpenTofu ephemeral resources](https://opentofu.org/docs/language/ephemerality/ephemeral-resources/)).
- A CIDR allocation is a durable remote object whose address must remain available to downstream resources, refresh, drift detection, import, and destroy. Making it ephemeral would discard the very association the provider must manage.
- Write-only attributes are always null in plan and state, cannot themselves produce a normal diff, and cannot be `Computed`. Their intended use is transient secrets such as passwords or API keys ([Terraform write-only arguments](https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments)).
- OpenTofu likewise states that write-only values exist only in configuration, remain null in plan/state, and generally need a separate version/trigger attribute to signal updates ([OpenTofu write-only attributes](https://opentofu.org/docs/language/ephemerality/write-only-attributes/)).

Relevant IPAM uses are limited to ancillary secrets: for example, a one-operation bootstrap token accepted by an allocation API. Provider authentication belongs in provider configuration, potentially fed by an ephemeral credential resource, rather than on every allocation.

## 3. List resources, `terraform query`, and bulk import

Terraform 1.14 list resources are highly relevant to IPAM discovery. A list implementation searches remote objects of an existing managed-resource type, streams identities and optional resource data, and is invoked through `terraform query` using `.tfquery.hcl` configuration ([list-resource overview](https://developer.hashicorp.com/terraform/plugin/framework/list-resources), [listing implementation](https://developer.hashicorp.com/terraform/plugin/framework/list-resources/list)).

Recommended list filters:

- pool ID or pool identity;
- tenant/account/site/VRF;
- address family;
- CIDR containment;
- status and tags;
- creation-owner or external reference.

Return:

- immutable allocation identity;
- a human-readable display name such as `10.42.16.0/20 (prod-eu)`;
- full state only when `IncludeResource` is true.

The framework exposes `IncludeResource` so the provider can avoid expensive per-allocation reads when Terraform needs only identities. `terraform query -generate-config-out=generated.tf` requests sufficient data to generate resource and import blocks ([list implementation](https://developer.hashicorp.com/terraform/plugin/framework/list-resources/list), [bulk import](https://developer.hashicorp.com/terraform/language/import/bulk)).

This is better than an ordinary “all allocations” data source for adoption workflows because list results are explicitly connected to managed-resource identity and configuration generation. Keep a conventional filtered data source only if normal module expressions need online lookup.

## 4. Import blocks and resource identity

Terraform resource identity is a provider-defined object stored alongside state. It must identify at most one remote object per provider, permit existence lookup during import, and remain stable throughout the object’s lifecycle ([resource identity](https://developer.hashicorp.com/terraform/plugin/framework/resources/identity)).

For an allocation:

1. Prefer an immutable backend allocation UUID as the required identity.
2. Add scope/account/tenant only when the UUID is not globally unique within a provider configuration.
3. Avoid using CIDR alone: address ranges can be released and reused, and identity is required to stay associated with the same remote object for its lifecycle ([resource identity](https://developer.hashicorp.com/terraform/plugin/framework/resources/identity)).
4. Set identity during `Create`, `Read`, and `Update`; the framework validates unexpected identity changes ([resource identity](https://developer.hashicorp.com/terraform/plugin/framework/resources/identity#writing-identity)).
5. Continue supporting a legacy string import ID unless the provider intentionally requires Terraform 1.12+. HashiCorp recommends accepting identity or `id`, and provides `ImportStatePassthro
...[elided 2467 chars]...
/actions), [action implementation](https://developer.hashicorp.com/terraform/plugin/framework/actions/implementation), [invocation](https://developer.hashicorp.com/terraform/language/invoke-actions)).

Consequences:

- **“Preview next CIDR” is a poor action.** The action cannot return a CIDR for use elsewhere, and “next” can become stale immediately. Prefer a clearly documented data source if an advisory preview is genuinely needed.
- **“Release allocation” should normally be `Delete`.** An action that releases a managed allocation cannot update/remove its Terraform state, leaving a stale resource until refresh. The managed-resource delete path already treats remote not-found as success and removes state when no error is returned ([delete lifecycle](https://developer.hashicorp.com/terraform/plugin/framework/resources/delete)).
- A separate administrative action may be defensible for releasing an allocation that is deliberately *not* managed by the current state, but it should require an explicit immutable allocation ID and carry strong warnings.

Actions should remain an optional Terraform-only extension, never a dependency of the cross-engine allocation lifecycle.

## 7. Concurrency, idempotency, and eventual consistency

Terraform walks independent graph nodes concurrently, processing up to ten by default unless `-parallelism` changes the limit. HashiCorp explicitly notes that providers generally handle API limiting below Terraform through backoff and retry ([Terraform graph walking](https://developer.hashicorp.com/terraform/internals/graph#walking-the-graph)). OpenTofu also defaults `-parallelism` to ten ([OpenTofu plan](https://opentofu.org/docs/cli/commands/plan/#other-options)).

An IPAM provider must therefore assume concurrent allocations against the same pool.

### Correctness hierarchy

1. **Server-side atomic allocation is mandatory.** The authoritative IPAM service must serialize or transactionally compare-and-allocate within a pool. A provider cannot safely implement “list free ranges, choose one, then create” under concurrent Terraform runs.
2. **Use a stable idempotency key/client token for `Create`.** Prefer an explicit immutable `allocation_key`, external reference, or similarly unique input accepted by the API. It must map retries to the same allocation rather than allocate another CIDR.
3. **Recover ambiguous creates by that key.** If the request times out after the server commits, query by the idempotency key before retrying creation.
4. **A provider-side keyed mutex is only an optimization.** It can reduce races among goroutines in one provider process, but cannot coordinate separate Terraform processes, workspaces, hosts, or non-Terraform clients. Terraform starts providers as separate binaries, while graph operations can run concurrently ([framework overview](https://developer.hashicorp.com/terraform/plugin/framework), [graph walking](https://developer.hashicorp.com/terraform/internals/graph#walking-the-graph)).
5. **Handle eventual consistency explicitly.** After create, poll the exact allocation ID/key until readable; after delete, tolerate not-found; on `Read`, definitive not-found removes state. HashiCorp’s provider guidance discusses retry/state-change handling for replicated and eventually consistent APIs, while the framework’s read/delete guidance defines not-found behavior ([retry guidance](https://developer.hashicorp.com/terraform/plugin/sdkv2/resources/retries-and-customizable-timeouts), [read lifecycle](https://developer.hashicorp.com/terraform/plugin/framework/resources/read), [delete lifecycle](https://developer.hashicorp.com/terraform/plugin/framework/resources/delete)).

Avoid making a random token generated only immediately before the HTTP request the sole recovery key: if the response is lost before anything is persisted, a later Terraform run cannot reconstruct it. A stable configured key or API-supported lookup contract closes that failure window.

## 8. Private state

Framework private state is provider-owned byte data stored in Terraform state but hidden from plans. HashiCorp cites ETags and timeout machinery as examples; providers may read or write it during specific lifecycle phases ([private state](https://developer.hashicorp.com/terraform/plugin/framework/resources/private-state)).

Do **not** make private state the only home of the allocation ID:

- Import, identity, list resources, diagnostics, and operators all need a durable semantic identifier.
- Store the allocation ID in resource identity and usually as a computed `allocation_id` attribute for older clients and provider usability.
- Use private state for opaque implementation metadata: ETags, continuation/operation handles, API revision tokens, or hashes used to detect changes to write-only inputs.
- Private state is still part of Terraform state; “private” means hidden from plans and configuration, not secret storage ([private state](https://developer.hashicorp.com/terraform/plugin/framework/resources/private-state)).

## 9. OpenTofu divergences

The portable baseline is strong:

- Managed-resource CRUD, plan modifiers, private state, and protocol-level provider operation remain the provider’s core.
- Provider-defined functions are supported by OpenTofu from 1.7, with protocol compatibility explicitly documented ([OpenTofu functions](https://opentofu.org/docs/language/functions/)).
- Ephemeral resources and write-only attributes arrived in OpenTofu 1.11 ([OpenTofu 1.11](https://opentofu.org/blog/opentofu-1-11-0/)).
- Current OpenTofu import blocks support provider-defined identity and legacy IDs ([OpenTofu import](https://opentofu.org/docs/language/import/)).

Material divergences:

- OpenTofu’s reviewed 1.13 docs do not document Terraform 1.14 list resources, `terraform query`, or provider actions. Ship those behind optional framework interfaces and maintain ordinary CRUD/import paths for OpenTofu ([OpenTofu 1.13](https://opentofu.org/blog/opentofu-1-13-0/), [OpenTofu sitemap](https://opentofu.org/sitemap.xml)).
- OpenTofu supports provider-alias-qualified functions, an extension beyond the basic Terraform provider-function form ([OpenTofu functions](https://opentofu.org/docs/language/functions/)).
- OpenTofu 1.12 introduced `lifecycle { destroy = false }`, which forgets state without deleting the remote object. For IPAM, that intentionally leaves the CIDR allocated; provider documentation should call this out because users may otherwise expect address release ([OpenTofu 1.12 release](https://opentofu.org/blog/opentofu-1-12-0/), [resource behavior](https://opentofu.org/docs/language/resources/behavior/)).
- OpenTofu 1.13 adds `convert`/`assume...` planning hints and experimental symbol libraries, but neither replaces authoritative server allocation or managed-resource state ([OpenTofu 1.13 release](https://opentofu.org/blog/opentofu-1-13-0/)).

## Recommended implementation priority

1. Managed allocation resource with atomic/idempotent create, computed stable CIDR, identity, import, and robust consistency handling.
2. Acceptance tests covering concurrent creates in one pool, ambiguous create recovery, eventual read visibility, delete-not-found, and import by ID/identity.
3. Pure provider functions only where built-in CIDR functions are insufficient.
4. Terraform 1.14 list-resource discovery and bulk import.
5. Actions only for genuinely administrative, non-stateful operations.
6. Ephemeral/write-only support only for ancillary secrets, not addresses or allocation identity.

## CLAIMS
- CLAIM: A server-allocated CIDR should be `Computed` and use `UseStateForUnknown` only when immutable for the allocation lifetime. — RISK: normal — SOURCES: https://developer.hashicorp.com/terraform/plugin/framework/resources/create, https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification — COUNTER: If the backend can move an allocation in place, retaining prior state in the plan can misrepresent the result and should be omitted. — PRIMARY: https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification
- CLAIM: Allocation-defining inputs should use `RequiresReplace` or conditional `RequiresReplaceIf` when the API cannot safely update them in place. — RISK: normal — SOURCES: https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification — COUNTER: Mutable labels, descriptions, and tags should remain ordinary updates. — PRIMARY: https://developer.hashicorp.com/terraform/plugin/framework/resources/plan-modification
- CLAIM: CIDRs and allocation IDs are inappropriate for ephemeral resources or write-only attributes because they are durable lifecycle state, not transient secrets. — RISK: normal — SOURCES: https://developer.hashicorp.com/terraform/plugin/framework/ephemeral-resources, https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments, https://opentofu.org/docs/language/ephemerality/ — COUNTER: Temporary authentication material used to call the IPAM API can legitimately be ephemeral or write-only. — PRIMARY: https://developer.hashicorp.com/terraform/plugin/framework/resources/write-only-arguments
- CLAIM: Terraform 1.14 list resources are the correct provider surface for discovering existing allocations and generating bulk-import configuration. — RISK: normal — SOURCES: https://raw.githubusercontent.com/hashicorp/terraform/v1.14.0/CHANGELOG.md, https://developer.hashicorp.com/terraform/plugin/framework/list-resources, https://developer.hashicorp.com/terraform/language/import/bulk — COUNTER: A conventional data source is still needed when lookup results must participate directly in ordinary expressions. — PRIMARY: https://developer.hashicorp.com/terraform/plugin/framework/list-resources
- CLAIM: Resource identity should use an immutable allocation UUID, optionally combined with scope, rather than CIDR alone. — RISK: high — SOURCES: https://developer.hashicorp.com/terraform/plugin/framework/resources/identity — COUNTER: CIDR can be part of identity only if the backend contract guarantees it never changes or identifies a different object during the managed object’s lifetime. — PRIMARY: https://developer.hashicorp.com/terraform/plugin/framework/resources/identity
- CLAIM: Provider-defined IPAM functions must remain pure and offline; querying “next available CIDR” violates the function contract. — RISK: normal — SOURCES: https://developer.hashicorp.com/terraform/plugin/framework/functions/concepts, https://opentofu.org/docs/language/functions/ — COUNTER: Deterministic subnet calculation from explicit inputs is appropriate and already partly covered by built-ins. — PRIMARY: https://developer.hashicorp.com/terraform/plugin/framework/functions/concepts
- CLAIM: Terraform actions are unsuitable for authoritative “preview next CIDR” or normal allocation release because actions expose no consumable output and cannot modify resource state. — RISK: normal — SOURCES: https://developer.hashicorp.com/terraform/plugin/framework/actions, https://developer.hashicorp.com/terraform/plugin/framework/actions/implementation, https://developer.hashicorp.com/terraform/language/invoke-actions — COUNTER: An explicitly administrative action against an unmanaged allocation may be valid if its imperative and stateless nature is intentional. — PRIMARY: https://developer.hashicorp.com/terraform/plugin/framework/actions/implementation
- CLAIM: Allocation correctness must rely on server-side atomicity and stable idempotency/recovery keys, not provider-process mutexes. — RISK: high — SOURCES: https://developer.hashicorp.com/terraform/internals/graph#walking-the-graph, https://developer.hashicorp.com/terraform/plugin/framework/create, https://developer.hashicorp.com/terraform/plugin/sdkv2/resources/retries-and-customizable-timeouts — COUNTER: A keyed mutex can still reduce avoidable contention within one provider process, but it cannot coordinate other processes or workspaces. — PRIMARY: https://developer.hashicorp.com/terraform/internals/graph#walking-the-graph
- CLAIM: The primary allocation ID belongs in resource identity and visible computed state, while private state should hold opaque implementation metadata. — RISK: normal — SOURCES: https://developer.hashicorp.com/terraform/plugin/framework/resources/identity, https://developer.hashicorp.com/terraform/plugin/framework/resources/private-state — COUNTER: A genuinely opaque API token can live only in private state if it is not the resource’s semantic identity and import does not require it. — PRIMARY: https://developer.hashicorp.com/terraform/plugin/framework/resources/private-state
- CLAIM: Terraform 1.14 list resources and actions should be optional extensions because current OpenTofu 1.13 documentation does not expose equivalent provider surfaces. — RISK: high — SOURCES: https://opentofu.org/blog/opentofu-1-13-0/, https://opentofu.org/sitemap.xml, https://raw.githubusercontent.com/hashicorp/terraform/v1.14.0/CHANGELOG.md — COUNTER: OpenTofu may add compatible surfaces later; retest against its provider protocol and release documentation before permanently classifying them as unsupported. — PRIMARY: https://opentofu.org/blog/opentofu-1-13-0/

## EXPAND
- LEAD: Validate the target IPAM API’s exact atomic-allocation and idempotency semantics. — WHY: This is the highest-risk dependency; provider logic cannot eliminate duplicate or overlapping allocations if the backend exposes only a non-atomic list-then-create workflow. — ANGLE: Determine whether allocation accepts a client token or external key, how ambiguous timeouts are recovered, uniqueness scope, token retention period, and behavior of concurrent identical requests.
- LEAD: Prototype resource identity against both Terraform and OpenTofu acceptance-test matrices. — WHY: Both currently document identity imports, but version negotiation and framework behavior should be proven before making identity the only import path. — ANGLE: Test create/read/import by identity, legacy ID import, identity persistence across refresh, and older-client degradation.
- LEAD: Design the Terraform 1.14 list-resource schema around cheap identity-only enumeration. — WHY: Bulk discovery may involve thousands of allocations, and the framework intentionally distinguishes identity-only results from full resource data. — ANGLE: Measure pagination, filtering, `IncludeResource`, generated configuration, duplicate-workspace detection, and API rate limits.
- LEAD: Define lifecycle documentation for OpenTofu `destroy = false`. — WHY: Forgetting an allocation without releasing it consumes address space and can look like a leak unless explicitly documented. — ANGLE: Show retain/forget, later identity import, and deliberate release workflows.
- DEAD END: Do not pursue ephemeral CIDRs, write-only allocation identities, provider-function-backed online allocation, or action-based normal release; their framework semantics conflict with durable managed allocation state.