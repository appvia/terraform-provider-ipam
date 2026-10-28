st_01a1830e [completed] transcript via event-log:
assistant: I read this as an exhaustive red-team of the prior-art evidence and design framing - I’ll audit every lane claim against primary sources, identify overreach and missing alternatives, then deliver claim-by-claim verdicts plus the required CLAIMS/EXPAND ledger. I’ll stop right away when all nine claims and the abstraction framing have specific counter-evidence, calibrated replacement wording, and verified URLs.
tool: todo
assistant: Evidence collection is underway toward a defensible red-team report on the proposed IPAM design. You need: 0/16 tasks done; the lane journals are the first evidence set, and no verdict has been accepted yet. Now: Read all lane journals and evidence. Next: Spot-check C1 atomic allocation primary sources.
tool: tool_search
tool: eval
tool: read
tool: read
tool: read
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
tool: todo
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
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
tool: todo
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: read
tool: eval
tool: eval
tool: read
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
tool: todo
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: todo
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: todo
tool: webfetch
tool: webfetch
tool: todo
assistant: The nine evidence claims are audited toward a corrected report rather than a defense of the lane conclusions. You need: 10/16 tasks done; C4’s date is already materially weakened (regional GA was documented in source history by June 12, not June 25), C6 omits deterministic allocation, and C7 overstates what the AWS bugs prove. Now: Identify missing prior-art products and approaches. Next: Argue strongest case against backend abstraction.
tool: tool_search
tool: tool_search
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
tool: mcp_codegraph_codegraph_explore
tool: eval
tool: todo
tool: read
tool: read
tool: read
tool: read
tool: read
tool: read
tool: todo
tool: todo
tool: todo
tool: todo
assistant: The attack surface is complete toward a final verdict set and design recommendation. You need: 14/17 tasks done; the strongest corrections are now evidence-backed, and the framing audit found three material omissions: state-only allocation, OpenStack Neutron subnet pools/address scopes, and controller/component orchestration. Now: Cross-check independence and source strength. Next: Write exhaustive verdict and replacement wording.
tool: todo
tool: todo
tool: todo
assistant: Red-team complete toward a corrected prior-art basis for the new Terraform IPAM provider. You need: 17/17 tasks done; C4’s date is wrong, C6’s topology is incomplete, C7 misattributes causation, and the backend-agnostic provider case does not survive scrutiny. Now: no task remains. Next: no task remains.

## Bottom line

Only C5 and C9 substantially stand as written; C3’s schema facts and release versions stand, but its implied proof of real multi-prefix allocation does not. C1 captures the right safety objective but overstates the required implementation, while C2, C4, C6, C7, and C8 all need material qualification.

**Recommendation — Large effort:** do not build a backend-agnostic provider. If new work is justified, target **Azure/ipam v4 specifically**, preferably by hardening or forking the existing Xtratus provider, and first add an upstream idempotency/recovery contract for reservation creation.

## Action plan

1. Replace C1–C9 with the calibrated wording below.
2. Run one live AVNM test for a non-power-of-two request before claiming actual multi-prefix decomposition.
3. Require an immutable client allocation key and lookup-by-key before treating Azure/ipam allocation as retry-safe.
4. Put any cross-cloud abstraction in a module, Pulumi component, or Crossplane composition over backend-specific resources—not in one provider resource schema.

# Claim attacks

## C1 — Allocation must be an atomic resource Create

**Verdict: weaken to “authoritative allocation must be an apply-time managed write.”**

### Attack and counter-evidence

- AWS explicitly distinguishes a real allocation from `PreviewNextCidr`; only the allocation action carries the strong-consistency guarantee. [AWS AllocateIpamPoolCidr](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_AllocateIpamPoolCidr.html)
- The AWS preview implementation calls the API only during Create, stores `<cidr>_<pool>` in state, has a no-op Delete, and on Read merely decodes that state. It therefore suffers from **staleness and lack of ownership**, not refresh churn. [AWS preview source](https://raw.githubusercontent.com/hashicorp/terraform-provider-aws/main/internal/service/ec2/vpc_ipam_preview_next_cidr.go)
- Parallel previews returning the same CIDR are directly reported in [AWS provider issue #25958](https://github.com/hashicorp/terraform-provider-aws/issues/25958).
- BloxOne issue #294 explicitly calls its preview informational and non-reserving, but this is a feature proposal/user analysis—not independent proof that every relevant BloxOne server operation is atomic. [BloxOne #294](https://github.com/infobloxopen/terraform-provider-bloxone/issues/294)
- Terraform says data sources perform reads and are refreshed during planning. Azure/ipam’s official `external` example violates that contract by executing a script that POSTs a reservation, so repeated reads can create remote mutations. [Terraform data sources](https://developer.hashicorp.com/terraform/language/data-sources), [Azure example](https://raw.githubusercontent.com/Azure/ipam/main/examples/ipam-terraform/main.tf), [reservation script](https://raw.githubusercontent.com/Azure/ipam/main/examples/ipam-terraform/scripts/new-ipam-reservation.sh)
- A standalone allocation resource is not universally mandatory:
  - AWS VPC and AVNM VNet/subnet creation allocate inside consumer Create.
  - Deterministic/state-only allocators can be correct within one locked state, without any server-side reservation. [static-ipam](https://github.com/gimletlabs/terraform-provider-static-ipam), [`cidrsubnet`](https://developer.hashicorp.com/terraform/language/functions/cidrsubnet)
- State-only allocation is not shared IPAM: `static-ipam` explicitly warns that separate resources or states cannot coordinate or detect overlap.

### Final-report wording

> An authoritative remote allocation must occur as an apply-time managed write that atomically chooses and reserves the result—either in allocation-resource Create or consumer-resource Create. Preview/data-source results are advisory only. Deterministic allocation within one locked Terraform state is a separate valid topology, but it cannot coordinate other states or external allocators.

---

## C2 — AWS consistency and the 32-minute timeout

**Verdict: weaken; the consistency guarantee stands, but the timeout causality is overstated.**

### Attack and counter-evidence

- AWS’s exact guarantee is narrow: `AllocateIpamPoolCidr` creates an allocation with strong consistency and the returned CIDR will not overlap another allocation **from the same pool**. It does not promise linearizable listing, release, or cross-pool behavior. [AllocateIpamPoolCidr](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_AllocateIpamPoolCidr.html)
- `ReleaseIpamPoolAllocation` is only for **manual allocations** and carries the generic EC2 eventual-consistency warning. Integrated allocations are removed through consumer deletion or monitoring changes. [ReleaseIpamPoolAllocation](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ReleaseIpamPoolAllocation.html)
- `GetIpamPoolAllocations` explicitly warns that reads following both allocation and release are eventually consistent. [GetIpamPoolAllocations](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_GetIpamPoolAllocations.html)
- The 32-minute timeout belongs to `aws_vpc_ipam_pool_cidr`, which is **provisioned pool capacity**, not the manual allocation resource. It waits for VPC/subnet allocations to disappear before calling `DeprovisionIpamPoolCidr`. [pool-CIDR source](https://raw.githubusercontent.com/hashicorp/terraform-provider-aws/main/internal/service/ec2/vpc_ipam_pool_cidr.go)
- By contrast, `aws_vpc_ipam_pool_cidr_allocation` calls `ReleaseIpamPoolAllocation` and returns without a deletion waiter or a 32-minute timeout. [manual-allocation source](https://raw.githubusercontent.com/hashicorp/terraform-provider-aws/main/internal/service/ec2/vpc_ipam_pool_cidr_allocation.go)
- “20–30 minutes” comes from provider comments and observed user reports, not an AWS service-level maximum. [AWS provider #46324](https://github.com/hashicorp/terraform-provider-aws/issues/46324)

### Final-report wording

> AWS guarantees non-overlap for the `AllocateIpamPoolCidr` allocation transaction within one pool. Subsequent listing and release visibility are eventually consistent. Provider and user evidence shows integrated VPC/subnet allocations can remain visible for roughly 20–30 minutes, but that is not an AWS SLA. The AWS provider’s 32-minute timeout applies to deprovisioning a pool CIDR after dependent allocations disappear, not to manual allocation release itself.

---

## C3 — AVNM count requests, computed prefixes, and AzureRM versions

**Verdict: weaken only the implied runtime multi-prefix proof; the schema and version claims stand.**

### Attack and counter-evidence

- ARM and AzureRM clearly model:
  - `numberOfIpAddresses` as a decimal string;
  - `allocatedAddressPrefixes` as a list;
  - `id` and requested count as inputs;
  - allocated prefixes as computed output.  
  [AzureRM VNet source](https://raw.githubusercontent.com/hashicorp/terraform-provider-azurerm/main/internal/services/network/virtual_network_resource.go), [AzureRM subnet source](https://raw.githubusercontent.com/hashicorp/terraform-provider-azurerm/main/internal/services/network/subnet_resource.go)
- AzureRM rejects decreases in `number_of_ip_addresses` client-side for a matching pool ID. This is provider behavior; it should not be attributed directly to an ARM service contract.
- Diff suppression for `address_space`/`address_prefixes` is conditional: it suppresses only while raw configuration still contains `ip_address_pool`, so an actual transition back to explicit prefixes remains visible.
- The sole apparent proof that 80 addresses become `/26 + /28` is an internally inconsistent specification example:
  - VNet request says `65536`;
  - response echoes `800`;
  - returned `/22` contains 1024 addresses;
  - subnet and location names also change between request and response.  
  [REST example](https://raw.githubusercontent.com/Azure/azure-rest-api-specs/main/specification/network/resource-manager/Microsoft.Network/Network/Network/examples/2026-03-01/VirtualNetworkCreateWithIpamPool.json)
- Therefore, the list type is proven, but actual multi-prefix decomposition for arbitrary counts still needs a live API test.
- Release versions are confirmed:
  - pool resource: [v4.23.0](https://github.com/hashicorp/terraform-provider-azurerm/releases/tag/v4.23.0)
  - VNet block: [v4.32.0](https://github.com/hashicorp/terraform-provider-azurerm/releases/tag/v4.32.0)
  - subnet block: [v4.38.0](https://github.com/hashicorp/terraform-provider-azurerm/releases/tag/v4.38.0)
  - static CIDR: [v4.41.0](https://github.com/hashicorp/terraform-provider-azurerm/releases/tag/v4.41.0)
- The static-CIDR resource is also a counterexample to a universal request/result split: it overloads `address_prefixes` and uses targeted diff suppression rather than a distinct computed field. [static-CIDR source](https://raw.githubusercontent.com/hashicorp/terraform-provider-azurerm/main/internal/services/networkmanager/network_manager_ipam_pool_static_cidr_resource.go)

### Final-report wording

> AVNM VNet and subnet consumers submit a decimal-string address count and ARM exposes the server result as `allocatedAddressPrefixes[]`. AzureRM models the count as configured intent and the prefix list as computed state, conditionally suppresses Azure-populated address-field diffs, and rejects count decreases client-side. The list shape is authoritative; actual decomposition of a non-power-of-two request into multiple prefixes remains unverified because the only published example is internally inconsistent. Native support arrived in AzureRM 4.23.0, 4.32.0, 4.38.0, and 4.41.0 for pool, VNet, subnet, and static CIDR respectively.

---

## C4 — AVNM GA date and Azure/ipam succession

**Verdict: refuted as written on the date; weaken the succession statement to a bounded search finding.**

### Attack and counter-evidence

- A January 22, 2025 Microsoft Q&A response still called the feature preview. [Microsoft Q&A](https://learn.microsoft.com/en-us/answers/questions/2150338/azure-ip-address-management-ipam-when-will-be-avai)
- Microsoft Docs repository history changed the shared availability include from preview to regional GA on **June 12, 2025**, not June 25:
  - [June 12 GA version](https://raw.githubusercontent.com/MicrosoftDocs/azure-docs/89c42cd36a9928b34e556c2388dc1499a3868744/includes/virtual-network-manager-ipam.md)
  - [preceding preview version](https://raw.githubusercontent.com/MicrosoftDocs/azure-docs/04893cb4a0795a90dee76e382e96e606f7d51205/includes/virtual-network-manager-ipam.md)
  - [source-history commit](https://github.com/MicrosoftDocs/azure-docs/commit/89c42cd36a9928b34e556c2388dc1499a3868744)
- This was **regional GA**, with other regions still preview—not a single worldwide GA transition.
- Current documentation reports broad GA except six regions. [Current availability](https://learn.microsoft.com/en-us/azure/virtual-network-manager/how-to-manage-ip-addresses-network-manager)
- No standalone Azure Updates GA announcement was fo
...[elided 13249 chars]...
 with a routing table.
   - Its official Terraform provider is still beta, so it is product-model prior art, not yet strong provider-quality evidence.  
   [Nautobot namespace model](https://docs.nautobot.com/projects/core/en/stable/user-guide/core-data-model/ipam/namespace/), [Nautobot provider](https://github.com/nautobot/terraform-provider-nautobot)

5. **Crossplane controller semantics**
   - Compositions can own allocation and consumer together.
   - Creation annotations explicitly address ambiguous create/leaked-resource risk, a problem more important than basic overlap selection.
   - Generated Upjet providers can inherit Terraform schema defects; changing orchestration engines does not erase backend/schema problems.  
   [Crossplane managed resources](https://docs.crossplane.io/latest/managed-resources/managed-resources/), [IPAM refresh bug #879](https://github.com/crossplane-contrib/provider-upjet-aws/issues/879)

6. **Pulumi and OpenTofu are delivery surfaces, not new IPAM semantics**
   - Pulumi’s AWS and GCP providers are Terraform-bridged; they largely inherit the same resource contracts. Its meaningful alternative is a component that composes backend-specific resources. [Pulumi provider model](https://www.pulumi.com/docs/iac/concepts/providers/)
   - OpenTofu uses the same provider model, but current feature parity is not complete: Terraform 1.14 list resources/actions should remain optional until OpenTofu documents compatible surfaces. [OpenTofu provider requirements](https://opentofu.org/docs/language/providers/requirements/)

7. **Ambiguous-create recovery**
   - Atomic non-overlap is insufficient. A response can be lost after the server commits.
   - AWS provides `ClientToken`; Azure/ipam does not.
   - Xtratus’s audited client has a fixed timeout, no stable allocation key, and confirmed timeout-after-commit risk. This should be a top-level requirement, not a footnote.

## Strongest case against a backend-agnostic provider

A common provider schema would need to erase or union incompatible contracts:

| Dimension | Backend divergence |
|---|---|
| Request | prefix length, exact CIDR, decimal address count, named reservation, or no request at all |
| Result | one CIDR, multiple prefixes, one IP, consumer association, or pure calculated value |
| Identity | allocation UUID, reservation ID, consumer resource ID, CIDR, or only Terraform state |
| Ownership | standalone allocation, consumer-owned allocation, historical reservation, or inventory record |
| Release | manual API call, consumer deletion, eventual discovery, successful no-op, or state removal |
| Scope | AWS scope/locale/account, GCP VPC/peering, NetBox VRF, Nautobot namespace, Azure Space |
| Updates | immutable, grow-only, count increases, mutable exact prefix, or replacement |
| Concurrency | client token, transaction, temporary lease, optimistic Git write, or Terraform state lock |
| Import | stable remote ID, composite context, unrecoverable original intent, or impossible external adoption |

The result would be either:

- a lowest-common-denominator resource that omits each backend’s important safety features; or
- a union schema full of mutually exclusive backend-specific fields and conditional lifecycle behavior.

Neither is portable. Changing `backend = "aws"` to `backend = "avnm"` cannot preserve identity, cardinality, release semantics, or consumer ownership and would necessarily be destructive.

## Concrete target recommendation

**Target Azure/ipam v4 specifically, not a backend-agnostic façade.**

- AWS, AVNM, GCP, and OpenStack allocation belongs in their existing cloud providers because consumer-integrated allocation is strongest there.
- NetBox, Infoblox, BlueCat, phpIPAM, and other DDI products already have backend-specific providers.
- Azure/ipam is the clearest gap, but it is not greenfield:
  - Xtratus already has a six-resource Plugin Framework provider;
  - that provider has import, timeout, Content-Type, drift, and retry weaknesses;
  - Azure/ipam v4 is still an evolving unreleased contract.

**Implementation posture:**

1. Add a client-supplied, immutable reservation key to Azure/ipam and support lookup by that key.
2. Contribute to or fork Xtratus rather than creating a second overlapping provider without a migration story.
3. Use composite reservation import IDs such as `space/block/id`.
4. Handle 409, 401, 403, 429, and v4’s staging/schema 503 distinctly.
5. Make normal CRUD/import work on both Terraform and OpenTofu; keep Terraform-only list-resource support optional.
6. Put cross-cloud portability in modules/components that expose a small output contract such as `allocated_prefixes`, `allocation_id`, and `scope`, while retaining backend-specific resources underneath.

## Evidence-risk check

- **Strongest:** AWS API guarantees; AzureRM source and release notes; Azure/ipam pinned source; GCP REST API; OpenStack documentation.
- **Weakest:** actual AVNM multi-prefix runtime behavior; precise worldwide GA instant; absence of a Microsoft successor statement; AWS’s observed 20–30-minute release duration; claims that any topology is “dominant.”
- **Independence failures:** several Terraform conclusions rely on provider source plus issues from that provider’s users; BloxOne #294 is a proposal, not a service guarantee.
- **Unverified live behavior:** no AVNM subscription was available for a real 80-address allocation test, and Azure/ipam v4 was not acceptance-tested against Xtratus v2.

## CLAIMS
- CLAIM: Authoritative remote allocation must be an apply-time managed write, but it need not be a standalone allocation resource and state-only deterministic allocation is a separate valid topology. — RISK: high — SOURCES: [AWS allocation API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_AllocateIpamPoolCidr.html), [AWS preview source](https://raw.githubusercontent.com/hashicorp/terraform-provider-aws/main/internal/service/ec2/vpc_ipam_preview_next_cidr.go), [static-ipam](https://github.com/gimletlabs/terraform-provider-static-ipam) — COUNTER: Single-state deterministic allocation needs no remote transaction, while consumer Create can own the transaction. — PRIMARY: [AWS API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_AllocateIpamPoolCidr.html)
- CLAIM: AWS strongly guarantees only the allocation transaction’s non-overlap within one pool; release/listing are eventually consistent, and the provider’s 32-minute timeout is for pool-CIDR deprovision rather than manual release. — RISK: high — SOURCES: [Allocate](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_AllocateIpamPoolCidr.html), [Release](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ReleaseIpamPoolAllocation.html), [pool-CIDR source](https://raw.githubusercontent.com/hashicorp/terraform-provider-aws/main/internal/service/ec2/vpc_ipam_pool_cidr.go) — COUNTER: 20–30 minutes is observed implementation behavior, not an AWS SLA. — PRIMARY: [AWS API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_AllocateIpamPoolCidr.html)
- CLAIM: AVNM’s request-count and allocated-prefix-list schema is proven, but real multi-prefix decomposition for non-power-of-two requests is not. — RISK: high — SOURCES: [AzureRM VNet source](https://raw.githubusercontent.com/hashicorp/terraform-provider-azurerm/main/internal/services/network/virtual_network_resource.go), [REST example](https://raw.githubusercontent.com/Azure/azure-rest-api-specs/main/specification/network/resource-manager/Microsoft.Network/Network/Network/examples/2026-03-01/VirtualNetworkCreateWithIpamPool.json) — COUNTER: The sole multi-prefix example has multiple contradictory request/response values. — PRIMARY: [ARM example/schema](https://raw.githubusercontent.com/Azure/azure-rest-api-specs/main/specification/network/resource-manager/Microsoft.Network/Network/Network/examples/2026-03-01/VirtualNetworkCreateWithIpamPool.json)
- CLAIM: AVNM IPAM entered phased regional GA in 2025, with source-history evidence by June 12 rather than June 25; no reviewed source formally names it Azure/ipam’s successor. — RISK: high — SOURCES: [GA include](https://raw.githubusercontent.com/MicrosoftDocs/azure-docs/89c42cd36a9928b34e556c2388dc1499a3868744/includes/virtual-network-manager-ipam.md), [prior preview include](https://raw.githubusercontent.com/MicrosoftDocs/azure-docs/04893cb4a0795a90dee76e382e96e606f7d51205/includes/virtual-network-manager-ipam.md), [current status](https://learn.microsoft.com/en-us/azure/virtual-network-manager/how-to-manage-ip-addresses-network-manager) — COUNTER: No standalone launch announcement was found, and absence of successor wording is not proof of future strategy. — PRIMARY: [MicrosoftDocs history](https://github.com/MicrosoftDocs/azure-docs/commit/89c42cd36a9928b34e556c2388dc1499a3868744)
- CLAIM: Azure/ipam schedules tag reconciliation every minute, but fulfilled-reservation deletion is a successful no-op and does not release the associated network. — RISK: normal — SOURCES: [reconciler](https://github.com/Azure/ipam/blob/12e41f4b93a7e5f3c61a859428c32137440037b2/engine/app/routers/azure.py#L860-L942), [delete](https://github.com/Azure/ipam/blob/12e41f4b93a7e5f3c61a859428c32137440037b2/engine/app/routers/space.py#L3191-L3241) — COUNTER: The minute is scheduling cadence, not a reconciliation-latency guarantee. — PRIMARY: [Azure/ipam pinned source](https://github.com/Azure/ipam/commit/12e41f4b93a7e5f3c61a859428c32137440037b2)
- CLAIM: Consumer-integrated and detached allocations are two remote-backend patterns, not the complete topology set. — RISK: high — SOURCES: [Terraform CIDR module](https://github.com/hashicorp/terraform-cidr-subnets), [Crossplane Compositions](https://docs.crossplane.io/latest/composition/compositions/), [OpenStack subnet pools](https://docs.openstack.org/neutron/latest/admin/config-subnet-pools.html) — COUNTER: Deterministic state-owned allocation and composite/controller ownership are materially distinct. — PRIMARY: [OpenStack documentation](https://docs.openstack.org/neutron/latest/admin/config-subnet-pools.html)
- CLAIM: Request/result separation is preferred when semantics differ, while Optional+Computed remains valid when explicit and server-selected modes share one logical field. — RISK: high — SOURCES: [AWS #45844](https://github.com/hashicorp/terraform-provider-aws/issues/45844), [AWS #46795](https://github.com/hashicorp/terraform-provider-aws/issues/46795), [schema behavior](https://developer.hashicorp.com/terraform/plugin/sdkv2/schemas/schema-behaviors) — COUNTER: #45844 asks to add Computed; #46795 is faulty update logic, so neither proves the schema combination invalid. — PRIMARY: [current AWS provider source](https://raw.githubusercontent.com/hashicorp/terraform-provider-aws/main/internal/service/ec2/vpc_.go)
- CLAIM: Overlap policy should use an explicit backend-native collision/visibility domain, not global uniqueness or a presumed universal routing-domain abstraction. — RISK: normal — SOURCES: [AWS scopes](https://docs.aws.amazon.com/vpc/latest/ipam/how-it-works-ipam.html), [NetBox VRFs](https://netboxlabs.com/docs/netbox/en/stable/models/ipam/vrf/), [OpenStack address scopes](https://docs.openstack.org/neutron/latest/admin/config-address-scopes.html) — COUNTER: NetBox can allow duplicates within one VRF, Azure Spaces are merely logical groups, and peering/import rules can connect nominally separate domains. — PRIMARY: [OpenStack address scopes](https://docs.openstack.org/neutron/latest/admin/config-address-scopes.html)
- CLAIM: GCP exposes four named strategies only for automatically allocated IPv4 Internal Ranges. — RISK: normal — SOURCES: [GCP REST API](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges), [Google provider](https://raw.githubusercontent.com/hashicorp/terraform-provider-google/main/website/docs/r/network_connectivity_internal_range.html.markdown) — COUNTER: Explicit CIDRs and IPv6 do not use them, and documentation differs on whether omission specifically means RANDOM. — PRIMARY: [GCP REST API](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges)
- CLAIM: A single backend-agnostic allocation resource would be a lossy lowest-common-denominator or an unsafe union schema; backend-specific providers plus higher-level composition are the safer boundary. — RISK: high — SOURCES: [Pulumi provider model](https://www.pulumi.com/docs/iac/concepts/providers/), [Crossplane Compositions](https://docs.crossplane.io/latest/composition/compositions/), [OpenStack subnet pools](https://docs.openstack.org/neutron/latest/admin/config-subnet-pools.html) — COUNTER: A common module-level output contract is feasible, but backend switching cannot preserve allocation identity or lifecycle semantics. — PRIMARY: multiple backend API contracts above.

## EXPAND
- LEAD: Execute a live AVNM VNet/subnet allocation requesting 80 addresses. — WHY: It is the only way to establish whether the service really returns `/26 + /28` rather than the behavior existing only in an inconsistent example. — ANGLE: Capture request, terminal ARM response, refresh response, growth attempt, and decrease rejection.
- LEAD: Add an Azure/ipam reservation idempotency contract before provider implementation. — WHY: Atomic non-overlap does not recover a timeout after the server commits. — ANGLE: Immutable client key, uniqueness scope, conflict behavior for changed size, and GET-by-key reconciliation.
- LEAD: Acceptance-test Xtratus v2 against the released Azure/ipam v4 API. — WHY: Source review predicts Content-Type, nullable-response, 409, 503, import, and timeout failures. — ANGLE: Non-admin reads, exhaustion, ambiguous create, settled delete, and service staging.
- LEAD: Add OpenStack Neutron and a state-only allocator to the design comparison. — WHY: They invalidate the current two-topology framing and sharpen address-scope, quota, and single-state tradeoffs. — ANGLE: Compare integrated subnet creation with one-resource pool/reservation state ownership.
- LEAD: Define a Terraform/OpenTofu compatibility floor. — WHY: Core CRUD/import is portable, but Terraform 1.14 list resources/actions are not yet a safe shared baseline. — ANGLE: Dual-CLI acceptance matrix with optional Terraform-only discovery support.
- DEAD END: Do not claim an exact worldwide AVNM GA day from mutable Learn article dates; the defensible finding is phased regional GA documented in source history.
- DEAD END: Do not infer Microsoft successor strategy from the absence of a migration statement or from one contributor’s GitHub comment.
- DEAD END: Do not use preview-next-CIDR, provider-local mutexes, or plan-time availability as ownership mechanisms.
- DEAD END: Do not pursue one provider resource with `backend = aws|avnm|gcp|netbox`; its identity, release, cardinality, and scope semantics cannot be made safely interchangeable.