st_01a1830d [completed] transcript via event-log:
assistant: I read this as an exhaustive, source-pinned audit of both Azure IPAM leads - I’ll clone and inspect both repositories, compare the v4 PR head against v3.6.0, verify issue evidence, and stop when every requested finding is cited with permalinks plus CLAIMS and EXPAND sections.
tool: todo
assistant: [Research plan initialized] toward an exhaustive, SHA-pinned audit of the Terraform provider and Azure/ipam v4 contract. You need: ledger 0/11 done; no blockers. Now: Load research and git operating instructions. Next: Clone provider repository and pin audited SHA.
tool: read
tool: read
tool: eval
tool: eval
tool: eval
tool: todo
tool: todo
tool: eval
tool: todo
assistant: [Both source repositories are pinned] toward the provider audit and v4 contract comparison. You need: ledger 3/11 done; provider v2.0.0 resolves to commit `efef90b56051f82fc50060c026da0e32fb46a4b9`, Azure/ipam baseline is `12e41f4...`, and PR head is `a54d58f3...`; no blockers. Now: Inventory provider resources and data sources. Next: Document schemas imports and reservation lifecycle.
tool: eval
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: todo
tool: eval
tool: read
tool: read
tool: read
tool: eval
tool: read
tool: read
tool: read
tool: todo
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
assistant: [Provider v2 audit is complete] toward a full Terraform-facing comparison. You need: ledger 6/11 done; the provider has 6 resources and 10 data sources, uses Plugin Framework, has no client retries/idempotency key, and open issues confirm timeout-after-create, import drift, response-shape, and auth/API pain; no blockers. Now: Diff v4 routes against v3 baseline. Next: Compare models auth and IPv6 behavior.
tool: eval
tool: webfetch
tool: eval
tool[error]: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: eval
tool: eval
tool: eval
tool: eval
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: eval
tool: read
tool: todo
tool: eval
tool: read
tool: read
tool: read
tool: read
tool: todo
tool: todo
assistant: [API-contract comparison is complete] toward a cited design brief for the new provider. You need: ledger 9/11 done; existing provider routes remain, four operational routes are added, v4 introduces nullable/redacted reservation identities, granular grants, JWKS caching, staging/incompatible 503 modes, and still excludes IPv6. Now: Verify permalink line citations and claims. Next: Deliver findings claims expansions and dead ends.
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
assistant: [Exhaustive audit complete] toward designing a new Terraform provider for Azure IPAM. You need: ledger 11/11 complete; both repositories were shallow-cloned and source claims were checked against pinned revisions. Provider v2.0.0 resolves to commit `efef90b56051f82fc50060c026da0e32fb46a4b9`; Azure/ipam v3.6.0 is `12e41f4b93a7e5f3c61a859428c32137440037b2`; PR 377 head is `a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8`.

## Lead 1 — `XtratusCloud/azureipam` v2.0.0

### SDK and provider configuration

This is **Terraform Plugin Framework**, not SDKv2: `terraform-plugin-framework v1.11.0` is a direct dependency; SDKv2 appears only indirectly through testing/tooling dependencies ([go.mod:5-12](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/go.mod#L5-L12)).

Provider configuration:

- `api_url`: optional, fallback `AZUREIPAM_API_URL`.
- `token`: optional, sensitive, fallback `AZUREIPAM_TOKEN`.
- `skip_cert_verification`: optional, default false.
- Token handling is static bearer-token injection; there is no Azure credential acquisition or token refresh ([provider.go:50-73](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/provider.go#L50-L73), [provider.go:108-161](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/provider.go#L108-L161)).

### Complete inventory

The provider registers **6 resources** and **10 data sources** ([provider.go:176-211](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/provider.go#L176-L211)).

Resources:

1. `azureipam_space`
2. `azureipam_block`
3. `azureipam_external`
4. `azureipam_reservation`
5. `azureipam_reservation_cidr`
6. `azureipam_block_network`

Data sources:

1. `azureipam_space`
2. `azureipam_spaces`
3. `azureipam_block`
4. `azureipam_blocks`
5. `azureipam_external`
6. `azureipam_externals`
7. `azureipam_reservation`
8. `azureipam_reservations`
9. `azureipam_block_networks`
10. `azureipam_block_networks_availables`

### Resource schemas

`RR` means Plugin Framework `RequiresReplace`, equivalent to SDKv2 `ForceNew`.

| Resource | Required | Optional | Computed | RR / ForceNew |
|---|---|---|---|---|
| `space` | `name`, `description` | — | — | none |
| `block` | `space`, `name`, `cidr` | — | — | `space` |
| `external` | `space`, `block`, `name`, `description`, `cidr` | — | — | `space`, `block` |
| `reservation` | `space`, `blocks`, `size` | `description`, `reverse_search`, `smallest_cidr` | `id`, `block`, `cidr`, `created_by`, `created_on`, `settled_by`, `settled_on`, `status`, `tags` | `space`, `blocks`, `size`, `reverse_search`, `smallest_cidr` |
| `reservation_cidr` | `space`, `block` | `specific_cidr`, `description` | `id`, `cidr`, `created_by`, `created_on`, `settled_by`, `settled_on`, `status`, `tags` | `space`, `block`, `specific_cidr` |
| `block_network` | `space`, `block`, `id` | — | `name`, `prefixes`, `resource_group`, `subscription_id`, `tenant_id` | `space`, `block`, `id` |

Schema sources:

- Space: [space_resource.go:39-58](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/space_resource.go#L39-L58)
- Block: [block_resource.go:45-75](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/block_resource.go#L45-L75)
- External: [external_resource.go:48-85](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/external_resource.go#L48-L85)
- Reservation: [reservation_resource.go:64-166](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/reservation_resource.go#L64-L166)
- Fixed-CIDR reservation: [reservation_resource_cidr.go:57-142](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/reservation_resource_cidr.go#L57-L142)
- Block network: [block_network_resource.go:52-104](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/block_network_resource.go#L52-L104)

Important schema/implementation mismatches:

- `reservation_cidr.specific_cidr` is optional in Terraform, but the client rejects creation when neither `size` nor `specific_cidr` is supplied. Since this resource never supplies `size`, it is operationally required ([reservation_resource_cidr.go:72-79](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/reservation_resource_cidr.go#L72-L79), [reservation.go:85-92](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/ipamclient/reservation.go#L85-L92)).
- Both reservation Update implementations merely copy the plan to state and do not call the API. Thus `description`, although not replacement-triggering, is not remotely updated ([reservation_resource.go:244-255](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/reservation_resource.go#L244-L255), [reservation_resource_cidr.go:221-232](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/reservation_resource_cidr.go#L221-L232)).
- Read methods generally return a diagnostic for a missing remote object instead of calling `RemoveResource`, so out-of-band deletion becomes a refresh failure rather than normal drift recovery.

### Data-source schemas

| Data source | Inputs | Computed results |
|---|---|---|
| `space` | `name` required; `expand`, `append_utilization` optional | `description`, nested `blocks`, `size`, `used` |
| `spaces` | `expand`, `append_utilization` optional | nested `spaces` |
| `block` | `space`, `name` required; `expand`, `append_utilization` optional | `cidr`, nested `vnets`, `externals`, `reservations`, `size`, `used` |
| `blocks` | `space` required; `expand`, `append_utilization` optional | nested `blocks` |
| `external` | `space`, `block`, `name` required | `description`, `cidr` |
| `externals` | `space`, `block` required | nested `externals` |
| `reservation` | `space`, `block`, `id` required | `cidr`, description/audit/status fields, `tags` |
| `reservations` | `space`, `block` required; `include_settled` optional | nested `reservations` |
| `block_networks` | `space`, `block` required | nested `networks` |
| `block_networks_availables` | `space`, `block` required | `ids` |

Representative schema sources: [reservation_data_source.go:48-104](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/reservation_data_source.go#L48-L104), [reservations_data_source.go:61-126](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/reservations_data_source.go#L61-L126), [block_data_source.go:56-178](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/block_data_source.go#L56-L178), [block_networks_availables_data_source.go:47-71](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/block_networks_availables_data_source.go#L47-L71).

### Import formats

| Resource | Import ID |
|---|---|
| `space` | `<space-name>` |
| `block` | `<space-name>/<block-name>` |
| `external` | `<space-name>/<block-name>/<external-name>` |
| `reservation` | `<reservation-id>` |
| `reservation_cidr` | `<reservation-id>` |
| `block_network` | `<space-name>/<block-name>/<full-Azure-resource-ID>` |

Sources: [space_resource.go:188-191](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/space_resource.go#L188-L191), [block_resource.go:207-233](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/block_resource.go#L207-L233), [external_resource.go:204-231](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/external_resource.go#L204-L231), [reservation_resource.go:287-290](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/reservation_resource.go#L287-L290), [block_network_resource.go:196-224](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/block_network_resource.go#L196-L224).

The block/external import regexes allow only alphanumerics, although Azure IPAM permits `.`, `_`, and `-`. Open PR [#19](https://github.com/XtratusCloud/terraform-provider-azureipam/pull/19) documents this mismatch.

Reservation import is especially weak: importing only sets `id`; Read then scans all Spaces/Blocks to rediscover `space` and `block`, while replacement-required request attributes such as `blocks`, `reverse_search`, and `smallest_cidr` cannot be reconstructed reliably. This matches open drift report [#18](https://github.com/XtratusCloud/terraform-provider-azureipam/issues/18).

### Reservation lifecycle

**Create**

- By size with one block: `POST /api/spaces/{space}/blocks/{block}/reservations`.
- By exact CIDR: same endpoint with `{cidr, desc}`.
- By size across multiple blocks: `POST /api/spaces/{space}/reservations`.
- The API returns a generated ID, assigned block/CIDR, timestamps, status, and `X-IPAM-RES-ID` tag ([reservation.go:85-174](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/ipamclient/reservation.go#L85-L174)).

**Read**

`FindReservationById` first fetches all Spaces and scans every nested Block reservation, then performs a block-specific list with `settled=true` to return the full record ([reservation.go:43-81](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/ipamclient/reservation.go#L43-L81)).

**Fulfillment**

Azure IPAM’s reconciliation loop sees a VNet carrying the reservation tag, verifies the CIDR, associates the VNet with the Block, and changes the reservation to `status="fulfilled"` with `settledOn`/`settledBy`. It does **not** delete the reservation audit record ([azure.py:875-939](https://github.com/Azure/ipam/blob/12e41f4b93a7e5f3c61a859428c32137440037b2/engine/app/routers/azure.py#L875-L939)).

The provider consequently keeps a fulfilled reservation in state and refreshes its computed status/timestamps.

**Delete/destroy**

The provider sends `DELETE /api/spaces/{space}/blocks/{block}/reservations` with a JSON array containing the ID ([reservation.go:177-204](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/ipamclient/reservation.go#L177-L204)).

The API does not physically delete a pending reservation. It marks it settled with:

- `status = "cancelledByUser"`
- `settledOn = now`
- `settledBy = caller`

For already settled/fulfilled reservations, delete is a successful no-op. Terraform therefore removes the resource from state while Azure IPAM retains its historical reservation record ([space.py:3067-3140](https://github.com/Azure/ipam/blob/12e41f4b93a7e5f3c61a859428c32137440037b2/engine/app/routers/space.py#L3067-L3140), [space.py:3191-3241](https://github.com/Azure/ipam/blob/12e41f4b93a7e5f3c61a859428c32137440037b2/engine/app/routers/space.py#L3191-L3241)).

### Retry and idempotency behavior

- The client has a hard-coded **10-second timeout** and no request retry layer ([ipamclient.go:20-31](https://github.com/X
...[elided 2021 chars]...
/issues/21).
- **Successful remote mutation followed by client decoding failure:** [#20](https://github.com/XtratusCloud/terraform-provider-azureipam/issues/20).
- **Request-contract drift / missing JSON header:** [#25](https://github.com/XtratusCloud/terraform-provider-azureipam/issues/25).
- **Auth UX:** upstream Azure/ipam [#99](https://github.com/Azure/ipam/issues/99) reports Swagger failing to attach the Authorization header. PR 377’s explicit `HTTPBearer` OpenAPI security scheme addresses this class of problem.
- No open provider issue explicitly titled “duplicate reservation” or provider-side token refresh/auth failure was found; duplicate risk is inferred from confirmed timeout-after-commit behavior and the absence of idempotency.

## Lead 2 — Azure/ipam PR 377 versus v3.6.0

### Route contract

A mechanical FastAPI decorator comparison found **74 routes in v3.6.0 and 78 at the PR head**, with **four additions and no removals**:

- `GET /api/azure/nic` — standalone network interfaces ([azure.py:836-852](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/routers/azure.py#L836-L852)).
- `GET /api/health` — authenticated dependency/schema health report ([health.py:117-138](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/routers/health.py#L117-L138)).
- `GET /api/notifications` ([notifications.py:26-47](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/routers/notifications.py#L26-L47)).
- `POST /api/notifications/{notification_id}/resolve` returning 202 ([notifications.py:50-80](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/routers/notifications.py#L50-L80)).

Existing Space, Block, reservation, external, and association paths remain stable.

Notable changes on existing routes:

- `GET /spaces`, `/spaces/{space}`, and Block equivalents no longer advertise separate `SpaceBasic`/`BlockBasic` response types for non-admin users. They return the normal structural models but apply field-level reservation redaction.
- `GET /spaces/{space}/blocks/{block}/available` adds `include_blocked`. With `expand=true`, it can return `blocked_by` entries identifying overlapping externals/reservations ([space.py:1312-1399](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/routers/space.py#L1312-L1399)).
- Unavailable CIDR allocation now generally returns **409 Conflict** rather than 500, allowing a provider to distinguish exhaustion/overlap from server faults ([space.py:964-979](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/routers/space.py#L964-L979)).
- A new service-mode gate returns **503** for nearly all API calls while a slot is staging or schema-incompatible; `/api/status` and `/api/health` remain available ([main.py:52-60](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/main.py#L52-L60), [main.py:315-350](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/main.py#L315-L350)).

### Model changes

Core request fields for `SpaceReq`, `BlockReq`, `SpaceCIDRReq`, and `BlockCIDRReq` are unchanged ([models.py:337-382](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/models.py#L337-L382)).

Terraform-relevant response changes:

- `Reservation.id` and `ReservationExpand.id` change from required string to `Optional[str]` because another user’s pending reservation is visible for occupancy but its ID and description are redacted ([models.py:223-255](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/models.py#L223-L255), [space.py:76-98](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/routers/space.py#L76-L98)).
- `SpaceBasic`, `SpaceBasicUtil`, `BlockBasic`, and `BlockBasicUtil` are removed.
- New `BlockedBy` and `NetworkExpandBlocked` models support the `available?include_blocked=true` response ([models.py:183-208](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/models.py#L183-L208)).
- Utilization responses now use `ExtNetUtil`/`ExtSubnetUtil`, adding `size` and `used` to nested external networks/subnets ([models.py:118-155](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/models.py#L118-L155), [models.py:287-313](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/models.py#L287-L313)).
- `VWanHub.parent_space` and `parent_block` become a list of `parent_containers`, allowing one network to belong to multiple Space/Block pairs ([models.py:464-482](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/models.py#L464-L482)).
- `Status` expands from version/container to include `stack`, `environment`, `start_time`, `mode`, and optional container details ([models.py:590-609](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/models.py#L590-L609)).

### Authentication and authorization

The bearer-wire contract remains `Authorization: Bearer <token>`, but implementation and authorization semantics change materially:

- Authentication is now declared as a real OpenAPI `HTTPBearer` security scheme, fixing Swagger’s failure to attach a token ([dependencies.py:15-47](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/dependencies.py#L15-L47)).
- JWKS retrieval is asynchronous, cached, lock-protected, retried for transient failures, refreshed hourly with jitter, and permitted to remain stale for up to 24 hours ([dependencies.py:61-169](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/dependencies.py#L61-L169)).
- Tokens still must be Entra v2 RS256 tokens with the engine client ID as audience and tenant-specific v2 issuer; v1 tokens are rejected ([dependencies.py:190-249](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/dependencies.py#L190-L249)).
- Authorization is centralized into explicit permissions. Admin receives all permissions; ordinary User receives association/external reads, can create and manage their own reservations, but cannot mutate Spaces/Blocks/associations/externals or inspect/delete everyone’s reservations ([permissions.py:17-49](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/permissions.py#L17-L49), [permissions.py:96-155](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/permissions.py#L96-L155)).
- Azure-resource visibility now distinguishes caller/OBO scope from IPAM’s tenant-wide engine scope, particularly for occupancy checks.

Provider impact: credentials should be tested separately for authentication and operation-specific authorization. A token valid for reservation creation may still return 403 for Space, Block, external, or association resources.

### IPv6

PR 377 does **not** add an IPv6 API contract:

- Public CIDR request models remain `IPv4Network`/`IPv4Address`, with IPv4 examples and naming ([models.py:17-99](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/models.py#L17-L99)).
- Azure network fixup explicitly retains IPv4 prefixes; IPv6 extraction and `prefixes_v6` remain commented out ([helper.py:86-125](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/routers/common/helper.py#L86-L125)).
- Blocks and reservation models still expose one string `cidr`, not an address-family-discriminated shape.

A Terraform provider should therefore validate IPv4 explicitly and avoid claiming IPv6 support merely because the underlying parsing library can parse IPv6 strings.

### Recommended Terraform schema/behavior changes for v4

1. Model reservation `id` and `description` as nullable in list/nested data sources; direct managed reservation resources should still require a non-null ID after Create.
2. Keep Space/Block/Reservation write schemas stable; no v4 rename is needed.
3. Add optional `include_blocked` and a computed `blocked_by` nested object if exposing the available-network endpoint.
4. Add external/subnet `size` and `used` computed fields to utilization data sources.
5. Treat 409 as a deterministic allocation conflict, 401 as token invalid/expired, 403 as insufficient grant, and 503 as service staging/schema-incompatible.
6. Send `Content-Type: application/json` on every JSON request.
7. Support configurable timeouts and safe 503/429 backoff, but do not replay POST creates without reconciliation.
8. Prefer stable composite import IDs for reservations, e.g. `space/block/id`, avoiding an O(all Spaces × Blocks × reservations) discovery scan.
9. On Read 404/invalid-ID responses, remove the Terraform resource from state rather than returning permanent drift errors.
10. Do not expose IPv6 schema fields until the API returns a real address-family-aware contract.

## CLAIMS

- CLAIM: Provider v2.0.0 uses Terraform Plugin Framework, with six resources and ten data sources — RISK: normal — SOURCES: [go.mod](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/go.mod#L5-L12), [provider.go](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/internal/provider/provider.go#L176-L211) — COUNTER: direct SDKv2 imports were checked; SDKv2 is only indirect — PRIMARY: pinned provider source.
- CLAIM: Destroying a pending reservation cancels it, while destroying a fulfilled reservation is a successful no-op that leaves the audit record in IPAM — RISK: high — SOURCES: [provider client](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/ipamclient/reservation.go#L177-L204), [v3 API bulk delete](https://github.com/Azure/ipam/blob/12e41f4b93a7e5f3c61a859428c32137440037b2/engine/app/routers/space.py#L3067-L3140), [single delete](https://github.com/Azure/ipam/blob/12e41f4b93a7e5f3c61a859428c32137440037b2/engine/app/routers/space.py#L3191-L3241) — COUNTER: searched for physical deletion of reservation records; deletion code is commented out — PRIMARY: pinned client and server source.
- CLAIM: Reservation creation is not idempotent and a timeout can leave an untracked remote reservation, creating duplicate logical allocations on retry — RISK: high — SOURCES: [client timeout](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/ipamclient/ipamclient.go#L20-L31), [create request](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/ipamclient/reservation.go#L85-L174), [issue 16](https://github.com/XtratusCloud/terraform-provider-azureipam/issues/16) — COUNTER: searched for idempotency keys, deterministic request IDs, retries, and post-error reconciliation; none found — PRIMARY: pinned provider source plus reporter’s observed timeout-after-commit.
- CLAIM: PR 377 preserves all existing routes and adds four operational routes — RISK: normal — SOURCES: [health.py](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/routers/health.py#L117-L138), [notifications.py](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/routers/notifications.py#L26-L80), [azure.py](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/routers/azure.py#L836-L852) — COUNTER: all FastAPI decorators were mechanically compared at both SHAs; zero removals — PRIMARY: pinned PR source and executed AST comparison.
- CLAIM: v4 makes reservation identities nullable for records redacted from non-admin callers — RISK: high — SOURCES: [models.py](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/models.py#L223-L255), [space.py](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/routers/space.py#L76-L98) — COUNTER: direct owner/admin reservation reads retain IDs; nullability applies to shared list/nested responses — PRIMARY: pinned PR source.
- CLAIM: PR 377 does not add IPv6 support — RISK: normal — SOURCES: [models.py](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/models.py#L17-L99), [helper.py](https://github.com/Azure/ipam/blob/a54d58f337a99ee4531ea8a9175a7bd5cea3bfd8/engine/app/routers/common/helper.py#L86-L125) — COUNTER: searched the PR source for active IPv6 fields and routes; only commented-out extraction remains — PRIMARY: pinned PR source.
- CLAIM: Provider v2.0.0’s missing JSON Content-Type is a concrete v4 compatibility blocker — RISK: high — SOURCES: [client headers](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/efef90b56051f82fc50060c026da0e32fb46a4b9/ipamclient/ipamclient.go#L43-L48), [issue 25](https://github.com/XtratusCloud/terraform-provider-azureipam/issues/25) — COUNTER: request bodies are valid JSON, but no centralized or per-request Content-Type assignment exists — PRIMARY: pinned client source and reproduced API error in the open issue.

## EXPAND

- LEAD: Run provider v2.0.0 acceptance tests against a container built from PR head `a54d58f3` — WHY: confirms the reported Content-Type 422 and nullable-redaction behavior end-to-end — ANGLE: exercise reservation POST, non-admin list reads, 409 exhaustion, and staging 503.
- LEAD: Derive a versioned Terraform schema from `/api/openapi.json` — WHY: PR 377 now has explicit bearer security and permission annotations, making schema-drift detection practical — ANGLE: snapshot v3.6.0 and v4 OpenAPI, then gate provider releases on incompatible changes.
- LEAD: Design a reservation create-token extension upstream — WHY: the current API offers no safe recovery from timeout-after-commit — ANGLE: client-supplied idempotency key persisted with the reservation and queryable after ambiguous transport failures.
- LEAD: Audit schema-convergence and service-mode durations — WHY: this determines whether 503 should be retried during Terraform operations and for how long — ANGLE: update-slot workflow, `Retry-After`, `/api/health`, and `/api/status`.
- DEAD END: No IPv6 provider contract exists in PR 377; active code continues to filter to IPv4.
- DEAD END: No existing Space/Block/reservation route was removed between the pinned v3.6.0 baseline and PR head.
- DEAD END: No open provider issue explicitly reports a provider token-refresh implementation bug; the provider does not implement token refresh at all.
- DEAD END: No explicit “duplicate reservation” issue was found; the duplicate-logical-allocation risk is established by issue #16’s timeout-after-success evidence plus the source-confirmed absence of idempotency.

Now: no task remains.  
Next: no task remains.