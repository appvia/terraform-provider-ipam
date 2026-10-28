st_01a18308 [completed] transcript via event-log:
assistant: I read this as exhaustive prior-art research on Azure Virtual Network Manager IPAM for provider design - I’ll verify Microsoft, ARM/REST, AzureRM, and AzAPI behavior, then deliver a fully cited synthesis. I’ll stop right away when all six requested areas, claim risks, expansion leads, and dead ends are documented.
tool: todo
assistant: I’ve structured the research around primary-source collection, mechanism synthesis, and a citation-complete report toward AVNM IPAM prior art for the new Terraform provider. You need: 0/9 tasks complete; no blockers. Now: Read browser instructions and verify attached engine. Next: Collect Microsoft IPAM concepts and operational guidance.
tool: read
tool: read
tool: eval
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
tool: todo
tool: eval
tool: eval
tool: eval
tool: eval
tool: eval
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: eval
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
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
tool: eval
tool: eval
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: eval
tool: eval
tool: eval
tool: eval
tool: eval
tool: eval
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: webfetch
tool: todo
tool: webfetch
tool: webfetch
tool: webfetch
tool: web_search
tool: web_search
tool: webfetch
tool: webfetch
tool: web_search
tool: web_search
tool: todo
assistant: I’ve completed primary-source collection toward the AVNM IPAM prior-art report. You need: 5/9 tasks complete; the attached browser was unavailable (`no-daemon`), so I used direct Microsoft Learn, GitHub REST specs, AzureRM source/history, and AzAPI docs as instructed. Now: Map ARM resources properties operations and hierarchy. Next: Map Terraform arguments lifecycle drift and allocations.
tool: todo
tool: todo
tool: todo
assistant: I’ve completed the mechanism and provider-behavior synthesis toward the AVNM IPAM prior-art report. You need: 8/9 tasks complete; the key finding is that Azure models allocation as a size request plus a server-selected *list* of prefixes, and AzureRM uses computed state plus targeted diff suppression to reconcile that asymmetry. Now: Deliver cited findings claims leads and dead ends. Next: none.
tool: todo
assistant: Ask: Exhaustively research AVNM IPAM as prior art for a Terraform IPAM provider.

For you: 9/9 research tasks completed. The attached browser reported `no-daemon`, so I used Microsoft Learn, Azure REST specifications, AzureRM source/history, and AzAPI documentation directly.

Now: no task remains.

Next: no task remains.

# Azure Virtual Network Manager IPAM prior art

## Executive conclusions

1. **Azure’s core allocation contract is “request capacity, receive server-selected prefixes,” not “choose a CIDR.”** A VNet or subnet submits a pool ID and `numberOfIpAddresses`; Azure returns `allocatedAddressPrefixes`. The returned value is a list because one request can produce multiple CIDRs: Microsoft’s REST example represents an 80-address subnet allocation as `/26 + /28`. [REST example](https://raw.githubusercontent.com/Azure/azure-rest-api-specs/main/specification/network/resource-manager/Microsoft.Network/Network/Network/examples/2026-03-01/VirtualNetworkCreateWithIpamPool.json)

2. **Allocation intent and allocation result are separate state.** AzureRM models `id` and `number_of_ip_addresses` as configuration and `allocated_ip_address_prefixes` as computed output. It suppresses diffs on ordinary `address_space`/`address_prefixes` because Azure populates those from IPAM after creation. [VNet source](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/internal/services/network/virtual_network_resource.go) [Subnet source](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/internal/services/network/subnet_resource.go)

3. **The hierarchy is logical, not an ARM child-resource hierarchy.** Every IPAM pool is an ARM child of the Network Manager; a child pool points to another pool by `parentPoolName`. Static CIDRs are actual ARM children of a pool. [Pool reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/ipampools) [Static CIDR reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/ipampools/staticcidrs)

4. **The service moved from preview to GA during 2025.** Microsoft Q&A still described it as preview on January 22, 2025; Microsoft documentation dated June 25, 2025 documented regional GA, and current 2026 documentation says it is GA everywhere AVNM is available except six named regions. I found no standalone Azure Updates announcement that proves a more precise worldwide launch timestamp, so **June 25, 2025 is the earliest verified official-document GA date, not necessarily the internal launch instant**. [January 2025 preview status](https://learn.microsoft.com/en-us/answers/questions/2150338/azure-ip-address-management-ipam-when-will-be-avai) [June 2025 documented GA page](https://learn.microsoft.com/zh-cn/azure/virtual-network-manager/concept-ip-address-management) [Current availability](https://learn.microsoft.com/en-us/azure/virtual-network-manager/how-to-manage-ip-addresses-network-manager)

---

## 1. ARM resources, API versions, properties, and operations

### Resource types

| ARM type | Role | Important properties |
|---|---|---|
| `Microsoft.Network/networkManagers/ipamPools` | Tracked, regional IPAM pool under a Network Manager | Required `addressPrefixes`; optional `description`, `displayName`, `parentPoolName`; newer APIs add `minAllocationSize` and `maxAllocationSize`; responses also carry inferred `ipAddressType`, `provisioningState`, and `etag`. [ARM reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/ipampools) [REST create](https://learn.microsoft.com/en-us/rest/api/virtualnetwork/ipam-pools/create?view=rest-virtualnetwork-2026-01-01) |
| `Microsoft.Network/networkManagers/ipamPools/staticCidrs` | Reservation/allocation under a pool | `addressPrefixes`, `description`, or `numberOfIPAddressesToAllocate`; the latter asks Azure to select available prefix space. [ARM reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/ipampools/staticcidrs) |
| `Microsoft.Network/networkManagers/verifierWorkspaces` | Sibling Network Manager feature, unrelated to allocation but useful provider-design precedent | Regional tracked resource with optional `description` and tags. [ARM reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/verifierworkspaces) |
| `Microsoft.Network/virtualNetworks` and `.../subnets` | Consumers of IPAM allocations | `ipamPoolPrefixAllocations[]`, each containing `pool.id`, requested `numberOfIpAddresses`, and response-only `allocatedAddressPrefixes`. [VNet reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/virtualnetworks) [REST example](https://raw.githubusercontent.com/Azure/azure-rest-api-specs/main/specification/network/resource-manager/Microsoft.Network/Network/Network/examples/2026-03-01/VirtualNetworkCreateWithIpamPool.json) |

### API-version timeline

- IPAM pools, static CIDRs, and verifier workspaces first appear in `2024-01-01-preview`; stable APIs begin at `2024-05-01`. The published sequence then includes `2024-07-01`, `2024-10-01`, `2025-01-01`, `2025-03-01`, `2025-05-01`, `2025-07-01`, `2025-09-01`, `2026-01-01`, and `2026-03-01`. [Pool versions](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/ipampools) [Static CIDR versions](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/ipampools/staticcidrs) [Verifier versions](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/verifierworkspaces)
- `ipamPoolPrefixAllocations` was added to the VNet deployment schema in stable API `2024-05-01`. [VNet API change log](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/change-log/virtualnetworks)
- Current AzureRM source uses `Microsoft.Network@2025-07-01`; current ARM references expose `2026-03-01`, while generated REST pages currently default to `2026-01-01`. [AzureRM pool source](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/internal/services/networkmanager/network_manager_ipam_pool_resource.go) [ARM pool reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/ipampools) [REST create](https://learn.microsoft.com/en-us/rest/api/virtualnetwork/ipam-pools/create?view=rest-virtualnetwork-2026-01-01)

### Pool properties

- `addressPrefixes: string[]` is the pool’s managed space and is required by the ARM deployment schema. [ARM reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/ipampools)
- `parentPoolName` contains the parent pool’s **name**, not its ARM ID; an empty value identifies a root pool. [ARM reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/ipampools)
- `displayName` is a friendly label and `description` is free-form metadata. [REST create](https://learn.microsoft.com/en-us/rest/api/virtualnetwork/ipam-pools/create?view=rest-virtualnetwork-2026-01-01)
- `minAllocationSize` and `maxAllocationSize`, available in newer APIs, express compliance bounds for allocations; they do not themselves allocate a prefix. Both are strings because IP counts, particularly IPv6 counts, need not fit a conventional 32-bit integer. [REST create](https://learn.microsoft.com/en-us/rest/api/virtualnetwork/ipam-pools/create?view=rest-virtualnetwork-2026-01-01)
- `ipAddressType` is inferred by Azure and reports `IPv4` or `IPv6`; `provisioningState` and `etag` are service-managed response fields. [REST create](https://learn.microsoft.com/en-us/rest/api/virtualnetwork/ipam-pools/create?view=rest-virtualnetwork-2026-01-01)

### Pool operations

The TypeSpec defines normal `GET`, async `PUT`, synchronous `PATCH`, async `DELETE`, and list-by-Network-Manager operations. Create/update/delete accept optional `If-Match`, so the service exposes optimistic-concurrency protection even though AzureRM does not surface the ETag to users. [TypeSpec](https://github.com/Azure/azure-rest-api-specs/blob/main/specification/network/resource-manager/Microsoft.Network/Network/Network/IpamPool.tsp)

Two action-style reads provide inventory and telemetry:

- `POST .../ipamPools/{pool}/getPoolUsage` returns:
  - pool `addressPrefixes`;
  - `allocatedAddressPrefixes`;
  - `availableAddressPrefixes`;
  - `reservedAddressPrefixes`;
  - child pools with IDs and prefixes;
  - allocated, available, reserved, and total address counts.  
  [REST operation](https://learn.microsoft.com/en-us/rest/api/virtualnetwork/ipam-pools/get-pool-usage?view=rest-virtualnetwork-2026-01-01)

- `POST .../ipamPools/{pool}/listAssociatedResources` is paginated and returns each association’s resource ID, pool ID, assigned prefixes, reserved prefixes, creation and expiry times, and assigned/reserved counts. [REST operation](https://learn.microsoft.com/en-us/rest/api/virtualnetwork/ipam-pools/list-associated-resources?view=rest-virtualnetwork-2026-01-01)

These operational views are not ordinary pool properties and should be modeled as data sources or read actions, not folded into the mutable pool resource.

---

## 2. VNet/subnet consumption: request size, receive prefixes

A VNet requests an allocation under:

```json
{
  "properties": {
    "addressSpace": {
      "ipamPoolPrefixAllocations": [{
        "pool": {
          "id": "/subscriptions/.../networkManagers/nm/ipamPools/pool"
        },
        "numberOfIpAddresses": "256"
      }]
    }
  }
}
```

A subnet uses the same shape under `subnets[].properties.ipamPoolPrefixAllocations`. [How-to template](https://learn.microsoft.com/en-us/azure/virtual-network-manager/how-to-manage-ip-addresses-network-manager) [Bicep examples](https://learn.microsoft.com/en-us/azure/virtual-network-manager/deploy-ip-address-management-pools-bicep)

Azure responds with both ordinary `addressPrefixes` and allocation metadata:

```json
{
  "addressPrefixes": ["10.0.0.0/26", "10.0.0.64/28"],
  "ipamPoolPrefixAllocations": [{
    "pool": {"id": "..."},
    "numberOfIpAddresses": "80",
    "allocatedAddressPrefixes": [
      "10.0.0.0/26",
      "10.0.0.64/28"
    ]
  }]
}
```

The REST example proves three important semantics:

1. The requested value is a **count**, not a CIDR prefix length.
2. The allocated result is server-selected and must be treated as computed.
3. The result can contain multiple CIDRs: 80 addresses become 64 + 16.  
   [REST example](https://raw.githubusercontent.com/Azure/azure-rest-api-specs/main/specification/network/resource-manager/Microsoft.Network/Network/Network/examples/2026-03-01/VirtualNetworkCreateWithIpamPool.json)

The portal allows at most one IPv4 pool and one IPv6 pool on a single VNet, matching AzureRM’s maximum of two `ip_address_pool` blocks. A subnet has at most one pool block in AzureRM. [Microsoft how-to](https://learn.microsoft.com/en-us/azure/virtual-network-manager/how-to-manage-ip-addresses-network-manager) [AzureRM VNet docs](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/website/docs/r/virtual_network.html.markdown) [AzureRM subnet docs](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/website/docs/r/subnet.html.markdown)

A pool can be associated with VNets in other Azure regions, so consumers must not assume pool and resource location equality. [Concept documentation](https://learn.microsoft.com/en-us/azure/virtual-network-manager/concept-ip-address-management)

---

## 3. Hierarchy, static CIDRs, IPv6, limits, and lifecycle status

### Hierarchy

- Root pools have no parent.
- Child pools divide the space of a root or another child.
- Azure supports **up to seven pool layers**. [Concept documentation](https://learn.microsoft.com/en-us/azure/virtual-network-manager/concept-ip-address-management)
- In ARM, child pools remain `networkManagers/ipamPools` resources and refer to the logical parent through `parentPoolName`; they are not ARM resources nested beneath the parent pool. [Pool reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/ipampools)

### Static CIDRs

Static CIDRs reserve space for networks that IPAM cannot directly associate with, including on-premises 
...[elided 8601 chars]...
and requested count.
2. **Azure owns result:** allocated prefixes.
3. **Refresh copies the result into a computed list:** `allocated_ip_address_prefixes`.
4. **The ordinary network address field is not treated as conflicting user input when IPAM mode is configured:** AzureRM suppresses diffs on `address_space` or `address_prefixes`.
5. **Mode transitions are detected from raw configuration:** suppression is disabled when the user actually changes from IPAM allocation to explicit prefixes.
6. **Unsafe shrinking is rejected during planning/apply logic:** existing and requested decimal counts are parsed as arbitrary-precision integers and compared before update.  
   [VNet source](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/internal/services/network/virtual_network_resource.go) [Subnet source](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/internal/services/network/subnet_resource.go)

For static CIDRs, AzureRM uses a less clean but effective variant:

- Explicit-prefix mode tracks configured prefixes normally.
- Count mode accepts only the count.
- Azure returns the selected prefix through `addressPrefixes`.
- The provider stores it but suppresses the config/state difference while count mode remains configured.
- Switching modes clears the no-longer-applicable ARM property explicitly.  
  [Static CIDR source](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/internal/services/networkmanager/network_manager_ipam_pool_static_cidr_resource.go)

This avoids perpetual plans, but a purpose-built provider should prefer a distinct computed `allocated_prefixes` field instead of overloading an optional input field.

---

## 6. AzAPI usage

Before native AzureRM support, or for still-unexposed fields such as allocation bounds, AzAPI can directly manage these ARM types:

```hcl
resource "azapi_resource" "pool" {
  type      = "Microsoft.Network/networkManagers/ipamPools@2026-03-01"
  name      = var.name
  parent_id = var.network_manager_id
  location  = var.location

  body = {
    properties = {
      addressPrefixes  = var.address_prefixes
      parentPoolName   = var.parent_pool_name
      minAllocationSize = var.min_allocation_size
      maxAllocationSize = var.max_allocation_size
    }
  }
}
```

Microsoft publishes equivalent generated AzAPI shapes for pools, static CIDRs, and verifier workspaces. [Pool AzAPI reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/ipampools) [Static CIDR AzAPI reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/ipampools/staticcidrs) [Verifier AzAPI reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/verifierworkspaces)

For a VNet allocation, an `azapi_resource` uses `properties.addressSpace.ipamPoolPrefixAllocations`. To expose Azure’s selected prefixes, set:

```hcl
response_export_values = [
  "properties.addressSpace.ipamPoolPrefixAllocations",
  "properties.addressSpace.addressPrefixes",
]
```

AzAPI then places those service responses in its computed `output` object. Its `body` is reconciled against the read response, and `response_export_values` controls which computed paths are exported. [AzAPI resource documentation](https://github.com/Azure/terraform-provider-azapi/blob/main/docs/resources/resource.md) [VNet ARM shape](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/virtualnetworks)

Using `azapi_update_resource` or a second `azapi_resource` to partially manage a VNet already owned by `azurerm_virtual_network` risks two Terraform resources competing over the same ARM document. Prefer either native AzureRM IPAM blocks or full AzAPI ownership of the VNet.

---

## 7. Design implications for a new Terraform IPAM provider

### Recommended resource model

1. **Pool resource**
   - Inputs: parent/container ID, name, prefixes, logical parent reference, display metadata, allocation bounds.
   - Computed: IP family, provisioning state, ETag.
   - Make identity/container/location changes replacement-only.
   - Treat prefix removal or shrinking according to backend capability rather than promising mutable lists.

2. **Allocation resource**
   - Inputs: pool ID, requested address count, consumer identity or reservation key.
   - Computed: `allocated_prefixes`, actual allocated count, status.
   - Never require callers to predict the selected CIDR.
   - Model output as a list, not a scalar.
   - Preserve an allocation identity so normal refresh cannot silently select a different prefix.

3. **Static reservation resource**
   - Use two explicit mutually exclusive modes:
     - `prefixes`;
     - `requested_address_count`.
   - Export `allocated_prefixes` in both modes.
   - Do not overload an input field with server-generated state as AzureRM does.

4. **Usage and association data sources**
   - Keep inventory and utilization separate from the mutable pool resource.
   - Support pagination and expose total/allocated/available/reserved counts plus associated resource IDs.
   - The Azure split between `getPoolUsage` and `listAssociatedResources` is a strong precedent. [Usage API](https://learn.microsoft.com/en-us/rest/api/virtualnetwork/ipam-pools/get-pool-usage?view=rest-virtualnetwork-2026-01-01) [Associations API](https://learn.microsoft.com/en-us/rest/api/virtualnetwork/ipam-pools/list-associated-resources?view=rest-virtualnetwork-2026-01-01)

### State and planning rules

- Represent address counts as decimal strings or arbitrary-precision integers; IPv6 makes fixed-width integer assumptions unsafe. [Pool ARM reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/ipampools)
- Treat server-selected prefixes as computed and stable after allocation.
- Suppress differences only on fields demonstrably synthesized by the API, and inspect raw configuration so a real mode transition is not hidden.
- Normalize CIDRs and ARM-style IDs before comparing state.
- Preserve list/set identity by pool ID or allocation ID rather than list position.
- Make shrink and release semantics explicit: Azure’s current inability to remove IPAM-managed spaces shows that deletion and reduction are not ordinary updates. [Azure limitations](https://learn.microsoft.com/en-us/azure/virtual-network-manager/concept-limitations)
- Use optimistic concurrency when the backend supplies revisions or ETags; Azure exposes `If-Match` for pool mutation. [REST create](https://learn.microsoft.com/en-us/rest/api/virtualnetwork/ipam-pools/create?view=rest-virtualnetwork-2026-01-01)
- Poll asynchronous create/delete operations and wait for terminal provisioning state before writing final computed allocation state. [IPAM TypeSpec](https://github.com/Azure/azure-rest-api-specs/blob/main/specification/network/resource-manager/Microsoft.Network/Network/Network/IpamPool.tsp)
- Expose import for pools, reservations, and allocations from the beginning; AzureRM’s resources all support ARM-ID import. [AzureRM pool docs](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/website/docs/r/network_manager_ipam_pool.html.markdown)

### Most important lesson

Do not design the provider as a deterministic CIDR calculator whose chosen prefix is configuration. Design it as a durable allocator:

```text
desired capacity + pool/policy
             |
             v
    allocation transaction
             |
             v
server-selected prefix list + stable allocation identity
```

That separation is what lets Terraform plan on intent, refresh actual allocation, detect true drift, and avoid reallocating address space merely because the backend represents a requested size with a different CIDR decomposition.

## CLAIMS

- CLAIM: AVNM allocation is size-based and server-selected, with a list of returned prefixes rather than a guaranteed single CIDR. — RISK: normal — SOURCES: [REST allocation example](https://raw.githubusercontent.com/Azure/azure-rest-api-specs/main/specification/network/resource-manager/Microsoft.Network/Network/Network/examples/2026-03-01/VirtualNetworkCreateWithIpamPool.json), [Microsoft how-to](https://learn.microsoft.com/en-us/azure/virtual-network-manager/how-to-manage-ip-addresses-network-manager) — COUNTER: Portal wording often says “a CIDR,” but the REST response schema and 80-address example prove the result can be multiple prefixes. — PRIMARY: [Azure REST specification](https://raw.githubusercontent.com/Azure/azure-rest-api-specs/main/specification/network/resource-manager/Microsoft.Network/Network/Network/examples/2026-03-01/VirtualNetworkCreateWithIpamPool.json)

- CLAIM: AzureRM separates allocation intent from computed allocated prefixes and suppresses drift on Azure-populated address fields. — RISK: normal — SOURCES: [VNet source](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/internal/services/network/virtual_network_resource.go), [Subnet source](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/internal/services/network/subnet_resource.go) — COUNTER: Static CIDRs use an older mixed input/state technique instead of a clean computed field. — PRIMARY: [AzureRM VNet implementation](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/internal/services/network/virtual_network_resource.go)

- CLAIM: Pool hierarchy is represented through `parentPoolName`, with a supported maximum depth of seven layers. — RISK: normal — SOURCES: [Concept documentation](https://learn.microsoft.com/en-us/azure/virtual-network-manager/concept-ip-address-management), [ARM pool reference](https://learn.microsoft.com/en-us/azure/templates/microsoft.network/networkmanagers/ipampools) — COUNTER: The parent is logical; ARM still places every pool directly under the Network Manager. — PRIMARY: [Microsoft concept documentation](https://learn.microsoft.com/en-us/azure/virtual-network-manager/concept-ip-address-management)

- CLAIM: AVNM IPAM supports IPv4 and IPv6 pools. — RISK: normal — SOURCES: [Concept documentation](https://learn.microsoft.com/en-us/azure/virtual-network-manager/concept-ip-address-management), [REST create definitions](https://learn.microsoft.com/en-us/rest/api/virtualnetwork/ipam-pools/create?view=rest-virtualnetwork-2026-01-01) — COUNTER: Individual consumer surfaces can impose stricter family/cardinality rules, such as one IPv4 and one IPv6 pool per VNet. — PRIMARY: [Microsoft concept documentation](https://learn.microsoft.com/en-us/azure/virtual-network-manager/concept-ip-address-management)

- CLAIM: June 25, 2025 is the earliest official-document date verified here that labels AVNM IPAM generally available in regions. — RISK: high — SOURCES: [June 2025 Microsoft Learn page](https://learn.microsoft.com/zh-cn/azure/virtual-network-manager/concept-ip-address-management), [January 2025 preview status](https://learn.microsoft.com/en-us/answers/questions/2150338/azure-ip-address-management-ipam-when-will-be-avai), [current GA status](https://learn.microsoft.com/en-us/azure/virtual-network-manager/how-to-manage-ip-addresses-network-manager) — COUNTER: No standalone Azure Updates GA announcement was located, so this is the documented transition date rather than a proven single global launch timestamp. — PRIMARY: [Microsoft Learn](https://learn.microsoft.com/zh-cn/azure/virtual-network-manager/concept-ip-address-management)

- CLAIM: As of current 2026 documentation, AVNM IPAM is GA in AVNM regions except Chile Central, Jio India West, Malaysia West, Qatar Central, South Africa West, and West India. — RISK: normal — SOURCES: [Current management how-to](https://learn.microsoft.com/en-us/azure/virtual-network-manager/how-to-manage-ip-addresses-network-manager), [current Bicep guide](https://learn.microsoft.com/en-us/azure/virtual-network-manager/deploy-ip-address-management-pools-bicep) — COUNTER: Azure regional availability changes over time and should be checked dynamically. — PRIMARY: [Microsoft Learn](https://learn.microsoft.com/en-us/azure/virtual-network-manager/how-to-manage-ip-addresses-network-manager)

- CLAIM: The purpose-built provider should model request count and allocated prefixes as separate fields and preserve a stable allocation identity. — RISK: normal — SOURCES: [REST allocation example](https://raw.githubusercontent.com/Azure/azure-rest-api-specs/main/specification/network/resource-manager/Microsoft.Network/Network/Network/examples/2026-03-01/VirtualNetworkCreateWithIpamPool.json), [AzureRM VNet source](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/internal/services/network/virtual_network_resource.go) — COUNTER: A deterministic local allocator could compute prefixes, but doing so would no longer match Azure’s server-owned allocation contract or concurrent-consumer behavior. — PRIMARY: [Azure REST specification](https://raw.githubusercontent.com/Azure/azure-rest-api-specs/main/specification/network/resource-manager/Microsoft.Network/Network/Network/examples/2026-03-01/VirtualNetworkCreateWithIpamPool.json)

## EXPAND

- LEAD: Investigate whether the new provider should expose allocations as first-class Terraform resources or embed them inside consumer resources. — WHY: Azure embeds allocation requests in VNet/subnet documents, but a generic IPAM provider likely needs an independent allocation identity to support arbitrary consumers and safe lifecycle management. — ANGLE: Compare Azure’s association inventory with AWS VPC IPAM allocation IDs and Infoblox/NetBox reservation models.

- LEAD: Test backend behavior for non-power-of-two requests, growth, failed shrink, delete, and concurrent allocations. — WHY: Microsoft’s 80-address example produces two CIDRs, while provider logic prohibits decreases; these are central state and replacement semantics. — ANGLE: Build contract tests around exact allocation identity, prefix stability, and transactional locking.

- LEAD: Expose pool usage and association inventory as paginated data sources. — WHY: Azure separates operational telemetry from mutable pool state, avoiding refresh cost and volatile plans. — ANGLE: Mirror `getPoolUsage` and `listAssociatedResources`, including reserved/available counts and prefix lists.

- LEAD: Add allocation policy bounds and ETag-aware mutation from the outset. — WHY: ARM now exposes minimum/maximum allocation sizes and optimistic concurrency, while AzureRM does not yet surface either fully. — ANGLE: Make policy fields normal pool arguments and use revision/ETag preconditions to prevent lost updates.

- DEAD END: No authoritative fixed maximum number of pools or static CIDRs per Network Manager was found; Microsoft’s current IPAM limitation page documents hierarchy depth and non-removability instead.

- DEAD END: No standalone Azure Updates item establishing a more precise one-day worldwide GA launch was found; June 25, 2025 is the earliest official-document GA date verified, with current 2026 docs confirming broad GA.