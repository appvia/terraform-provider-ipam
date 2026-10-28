st_01a1830b [completed] transcript via event-log:
assistant: I read this as exhaustive comparative research on Terraform IPAM provider and module allocation patterns - I’ll inspect primary schemas and documentation, trace recurring lifecycle risks, and synthesize cited design guidance. I’ll stop right away when every requested ecosystem is covered, each material claim has a URL, and the report ends with the required CLAIMS and EXPAND sections.
tool: todo
assistant: [Research plan established] toward an exhaustive, source-backed comparison of Terraform IPAM allocation designs. You need: ledger 0/12 done; all requested providers, functions, cloud ecosystems, and lifecycle risks are explicitly tracked. Now: Read browser workflow and verify attached engine. Next: Locate provider repositories and registry documentation.
tool: read
tool: read
tool: eval
tool: todo
tool: web_search
tool: web_search
tool: web_search
tool: web_search
tool: web_search
tool: web_search
tool: web_search
tool: web_search
tool: eval
tool: eval
tool: eval
tool: eval
tool: eval
tool: eval
tool: eval
tool: eval
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
tool: eval
tool: eval
tool: eval
tool: eval
tool: eval
tool: eval
tool: eval
tool: eval
tool: eval
tool: eval
tool: eval
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
tool: webfetch
assistant: [Primary-source survey complete] toward a defensible comparison of Terraform IPAM allocation patterns. You need: ledger 3/12 done; the attached browser was unavailable (`no-daemon`), so evidence comes from GitHub API/raw sources, Terraform Registry APIs, and official documentation; canonical schemas are collected for every named provider. Now: Analyze NetBox allocation lifecycle and drift patterns. Next: Analyze Infoblox and BloxOne allocation schemas.
tool: todo
tool: webfetch
tool: todo
tool: webfetch
tool: todo
tool: webfetch
tool: todo
tool: webfetch
tool: webfetch
tool: todo
tool: webfetch
tool: webfetch
tool: todo
tool: webfetch
tool: webfetch
tool: todo
tool: webfetch
assistant: [Analysis complete] toward an exhaustive design reference for Terraform IPAM. You need: ledger 10/12 done; all provider families and recurring risks are now traced to schemas, CRUD paths, official docs, registry metadata, and issue evidence; no code or workspace files were changed. Now: Write exhaustive citation-backed comparative findings. Next: Produce required claims and expansion appendix.
tool: webfetch
tool: todo
tool: webfetch
tool: todo
tool: webfetch
assistant: Ask: Exhaustive research axis A5 — existing Terraform IPAM providers/modules and their resource/schema patterns.

For you:

Now: no task remains; ledger 12/12 complete.  
Next: no task remains.

The attached browser was unavailable because BrowserSkill reported `no-daemon`, so I used GitHub repository/code search, raw primary sources, Terraform Registry APIs, and official HashiCorp documentation as requested.

## Executive findings

The strongest recurring design is:

```text
resource allocation_request {
  parent_id / parent_cidr / pool_id = ...
  prefix_length / netmask_length    = ...
  allocation_key / keepers          = ...
  # optional metadata
}

computed:
  cidr
  remote_allocation_id
```

The allocation must be a **resource write**, not a data source lookup. Parent/pool and requested size normally require replacement; the assigned CIDR is computed and retained in state. The remote IPAM must make the choose-and-reserve step atomic, while the provider must read thereafter by stable remote ID instead of asking for “next available” again. This pattern appears in NetBox, BloxOne, AWS VPC IPAM, Azure IPAM, BlueCat, Micetro, and phpIPAM resources. ([NetBox source](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_available_prefix.go#L15-L39), [BloxOne source](https://github.com/infobloxopen/terraform-provider-bloxone/blob/master/internal/service/ipam/model_ipamsvc_subnet.go#L138-L175), [AWS source](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_pool_cidr_allocation.go#L36-L110), [Azure IPAM source](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/main/internal/provider/reservation_resource.go#L65-L150))

Data-source-based “first free” or preview allocation is intrinsically advisory: Terraform data sources are read-only and refresh during planning, so their result can change and is not reserved. ([Terraform data-source behavior](https://developer.hashicorp.com/terraform/language/data-sources), [BloxOne preview source](https://github.com/infobloxopen/terraform-provider-bloxone/blob/master/internal/service/ipam/api_next_available_subnet_data_source.go#L61-L97), [BloxOne race discussion](https://github.com/infobloxopen/terraform-provider-bloxone/issues/294), [phpIPAM documentation](https://github.com/lord-kyron/terraform-provider-phpipam/blob/master/docs/data-sources/first_free_subnet.md#L1-L40))

Pure CIDR calculation avoids remote races but is not shared-state IPAM: it cannot see allocations made outside the same deterministic address plan. Its main risk is renumbering caused by changing positional inputs. ([`cidrsubnet`](https://developer.hashicorp.com/terraform/language/functions/cidrsubnet), [`cidrsubnets`](https://developer.hashicorp.com/terraform/language/functions/cidrsubnets), [HashiCorp module change rules](https://github.com/hashicorp/terraform-cidr-subnets/blob/master/README.md#L71-L157))

---

## 1. NetBox: `e-breuninger/netbox`

### Static resources

- `netbox_prefix` models a specific CIDR as a normal resource: `prefix` is required and validated as CIDR, but is not declared `ForceNew`; status and metadata are managed alongside it. ([source](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_prefix.go#L14-L39))
- `netbox_ip_range` similarly takes required `start_address` and `end_address`; size is computed. These fields are not marked `ForceNew` in the schema. ([source](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_ip_range.go#L14-L33), [computed fields](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_ip_range.go#L55-L73))

### `netbox_available_prefix`

This is the clean “parent plus requested prefix length” pattern:

- `parent_prefix_id`: required, `ForceNew`.
- `prefix_length`: required, `ForceNew`.
- `prefix`: computed.
- The create path POSTs directly to NetBox’s `available-prefixes` allocation endpoint, saves the returned object ID and prefix, then manages metadata through the ordinary prefix update/read/delete paths. ([schema](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_available_prefix.go#L15-L39), [create](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_available_prefix.go#L133-L155))
- Import requires all otherwise-unrecoverable allocation context: `parent_prefix_id prefix_id prefix_length`. ([import parser](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_available_prefix.go#L114-L130))

The similarly named `netbox_available_prefix` **data source** does not allocate. It reads a computed list of available prefixes under a prefix ID and therefore should be treated only as discovery/preview. ([source](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/data_source_netbox_available_prefix.go#L9-L38))

### `netbox_available_ip_address`

- Exactly one of `prefix_id` or `ip_range_id` is accepted; both are `ForceNew`.
- `ip_address` is computed.
- Create POSTs to the corresponding NetBox next-available endpoint and stores the returned remote ID and address.
- Read thereafter uses the remote IP-address ID; a 404 removes it from Terraform state rather than allocating a replacement during refresh. ([schema](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_available_ip_address.go#L14-L51), [create](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_available_ip_address.go#L116-L153), [read/404 behavior](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_available_ip_address.go#L156-L168))

Historically, parallel resources received duplicate addresses because an older implementation performed “list free, pick first, create” as separate operations; the reported workaround was Terraform serialization. The current source uses NetBox’s atomic available-IP POST instead. ([duplicate-allocation issue](https://github.com/e-breuninger/terraform-provider-netbox/issues/59), [current atomic call](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_available_ip_address.go#L116-L153))

### Drift and import weakness

NetBox does not expose which prefix or range originally supplied an IP address. Importing an existing `netbox_available_ip_address` therefore cannot reconstruct its `ForceNew` allocation input; state gets `0`, configuration supplies a real ID, and Terraform proposes destroy/recreate—potentially silently renumbering a live host. ([issue #861](https://github.com/e-breuninger/terraform-provider-netbox/issues/861), [proposed fix #960](https://github.com/e-breuninger/terraform-provider-netbox/pull/960))

NetBox’s allocation endpoint is concurrency-safe, but it lacks a caller-supplied idempotency/allocation key. A timeout after successful allocation can therefore leave the caller unable to distinguish success from failure; retrying can allocate a second object. The upstream proposal explicitly documents advisory locking, retry ambiguity, missing provenance, and the Terraform import problem. ([NetBox proposal #23411](https://github.com/netbox-community/netbox/issues/23411))

**Pattern to reuse:** resource-based atomic allocation, computed result, stable remote ID, and replacement on a known move between parent pools. Improve it with a stable allocation key stored remotely.

---

## 2. Infoblox NIOS and BloxOne

### Infoblox NIOS `infoblox_ipv4_network` — provider 2.13 schema

The requested `infoblox_ipv4_network` design is present in the 2.x provider line; the Registry now also exposes a 3.0 beta with a substantially regenerated model. ([v2.13 source](https://github.com/infobloxopen/terraform-provider-infoblox/blob/v2.13.0/infoblox/resource_infoblox_network.go), [Registry metadata](https://registry.terraform.io/v1/providers?namespace=infobloxopen&name=infoblox))

The 2.13 resource supports three modes:

1. Static: explicit `cidr`.
2. Dynamic from a known container: `parent_cidr + allocate_prefix_len`.
3. Dynamic by extensible-attribute selection: `filter_params + allocate_prefix_len`, with `object` selecting `network` or `networkcontainer`. ([documentation](https://github.com/infobloxopen/terraform-provider-infoblox/blob/v2.13.0/docs/resources/infoblox_ipv4_network.md#L1-L19), [examples](https://github.com/infobloxopen/terraform-provider-infoblox/blob/v2.13.0/docs/resources/infoblox_ipv4_network.md#L26-L71))

Schema details:

- `parent_cidr` and `filter_params` are optional allocation selectors.
- `allocate_prefix_len` is optional, default `0`.
- `cidr` is `Optional + Computed`.
- `ext_attrs` is an optional JSON string.
- `internal_id` and NIOS `ref` are computed identity fields. ([source](https://github.com/infobloxopen/terraform-provider-infoblox/blob/v2.13.0/infoblox/resource_infoblox_network.go#L45-L138))

Create calls either `AllocateNetwork`, `AllocateNetworkByEA`, or `CreateNetwork`; the allocated CIDR is written into state. ([source](https://github.com/infobloxopen/terraform-provider-infoblox/blob/v2.13.0/infoblox/resource_infoblox_network.go#L142-L233))

This provider did not express all allocation changes as schema `ForceNew`. Instead, update returns errors for changes to immutable fields including `cidr`, `network_view`, allocation object, and reservation settings. That is less Terraform-native than plan-time replacement because failure occurs at apply/update rather than producing a replacement plan. ([source](https://github.com/infobloxopen/terraform-provider-infoblox/blob/v2.13.0/infoblox/resource_infoblox_network.go#L371-L425))

Read refreshes the actual `cidr` and extensible attributes; delete resolves and deletes the NIOS object and clears state. Import is supported and reconstructs the current CIDR and metadata. ([read](https://github.com/infobloxopen/terraform-provider-infoblox/blob/v2.13.0/infoblox/resource_infoblox_network.go#L286-L361), [delete](https://github.com/infobloxopen/terraform-provider-infoblox/blob/v2.13.0/infoblox/resource_infoblox_network.go#L493-L540), [import](https://github.com/infobloxopen/terraform-provider-infoblox/blob/v2.13.0/infoblox/resource_infoblox_network.go#L586-L642))

The related IP-allocation resource exposes `allocated_ipv4_addr`/`allocated_ipv6_addr` as computed results, supports next-available selection by CIDR or extensible-attribute filters, and explicitly avoids re-running dynamic allocation on unrelated updates by blanking unchanged CIDR arguments before updating. Delete releases the backing host record. ([schema](https://github.com/infobloxopen/terraform-provider-infoblox/blob/v2.13.0/infoblox/resource_infoblox_ip_allocation.go#L37-L176), [allocation](https://github.com/infobloxopen/terraform-provider-infoblox/blob/v2.13.0/infoblox/resource_infoblox_ip_allocation.go#L249-L336), [anti-renewal logic](https://github.com/infobloxopen/terraform-provider-infoblox/blob/v2.13.0/infoblox/resource_infoblox_ip_allocation.go#L591-L606), [release](https://github.com/infobloxopen/terraform-provider-infoblox/blob/v2.13.0/infoblox/resource_infoblox_ip_allocation.go#L768-L809))

### BloxOne `bloxone_ipam_subnet`

BloxOne modernizes the pattern:

- Exactly one of `address` or `next_available_id`.
- `address` is `Optional + Computed`, uses prior state when unknown, and requires replacement when explicitly configured.
- `cidr` and `space` are required.
- `next_available_id` identifies the parent address block, requires replacement when configured, and uses prior state when unknown.
- On create, `next_available_id` is transformed into the BloxOne `/nextavailablesubnet` allocation pseudo-address. ([model and schema](https://github.com/infobloxopen/terraform-provider-bloxone/blob/master/internal/service/ipam/model_ipamsvc_subnet.go#L30-L150), [parent and replacement behavior](https://github.com/infobloxopen/terraform-provider-bloxone/blob/master/internal/service/ipam/model_ipamsvc_subnet.go#L418-L479), [expansion](https://github.com/infobloxopen/terraform-provider-bloxone/blob/master/internal/service/ipam/model_ipamsvc_subnet.go#L522-L530))

The provider takes a keyed mutex around creation usi
...[elided 16067 chars]...
104))
- `azurerm_network_manager_ipam_pool_static_cidr`: exactly one of explicit `address_prefixes` or `number_of_ip_addresses_to_allocate`, under a required pool ID. Server-assigned prefixes are read back; diff suppression prevents the computed prefix from conflicting with size-based configuration. ([source](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/internal/services/networkmanager/network_manager_ipam_pool_static_cidr_resource.go#L22-L99), [create](https://github.com/hashicorp/terraform-provider-azurerm/blob/main/internal/services/networkmanager/network_manager_ipam_pool_static_cidr_resource.go#L101-L147))

### Azure/IPAM solution provider

`XtratusCloud/azureipam` is a Registry provider targeting the separate Azure IPAM solution. Its reservation resource takes:

- `space`;
- ordered candidate `blocks`;
- requested CIDR size;
- optional reverse-search and smallest-block strategies.

All allocation-defining inputs require replacement, while remote reservation ID, selected block, CIDR, status, and required VNet tags are computed. Delete releases the reservation; import passes through its reservation ID. ([Registry entry](https://registry.terraform.io/v1/providers?namespace=XtratusCloud&name=azureipam), [schema](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/main/internal/provider/reservation_resource.go#L65-L150), [create](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/main/internal/provider/reservation_resource.go#L153-L193), [delete/import](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/main/internal/provider/reservation_resource.go#L253-L301))

Its documentation warns that Azure IPAM does not retain creation-time `blocks`, `reverse_search`, and `smallest_cidr`, so imports cannot reconstruct them and require manual reconciliation to prevent replacement. ([documentation](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/main/docs/resources/reservation.md#L73-L86))

A separate `reservation_cidr` resource reserves an explicit CIDR under a space/block; its allocation fields require replacement and assigned CIDR is computed. ([source](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/main/internal/provider/reservation_resource_cidr.go#L57-L124))

### GCP custom IPAM

`openx/terraform-provider-gcp-ipam-autopilot` models an IP range with required `name` and `range_size`, optional parent/domain, and `Optional + Computed + ForceNew` CIDR. Omitting CIDR allows the remote Cloud Run service to allocate under a parent; create stores the returned CIDR and ID. ([repository](https://github.com/openx/terraform-provider-gcp-ipam-autopilot), [schema and create](https://github.com/openx/terraform-provider-gcp-ipam-autopilot/blob/main/ipam/resource_ip_range.go#L27-L127), [documentation](https://github.com/openx/terraform-provider-gcp-ipam-autopilot/blob/main/docs/resources/ip_range.md#L12-L40))

---

## 7. Recurring lifecycle and correctness issues

### Concurrency

1. A read-only preview followed by create is a TOCTOU race; BloxOne explicitly documents that multiple plans can preview the same CIDR. ([issue](https://github.com/infobloxopen/terraform-provider-bloxone/issues/294))
2. The choose-and-reserve operation must be one remote atomic write or protected by a remote temporary claim. NetBox uses allocation POST endpoints; Micetro exposes temporary claims. ([NetBox create](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_available_prefix.go#L133-L155), [Micetro hold](https://github.com/menandmice/terraform-provider-menandmice/blob/develop/menandmice/resource_range.go#L101-L113))
3. Provider-process mutexes protect only concurrent operations inside that process; they do not replace server-side atomicity. ([BloxOne mutex](https://github.com/infobloxopen/terraform-provider-bloxone/blob/master/internal/service/ipam/api_subnet_resource.go#L70-L82))
4. Terraform backend locking protects a shared Terraform state, not allocations coordinated through separate states or non-Terraform clients. ([state locking](https://developer.hashicorp.com/terraform/language/state/locking))

### Churn and drift

1. Allocation selectors belong in resource configuration and the chosen CIDR belongs in computed state.
2. Refresh must read the existing allocation by stable remote ID; it must not call “next available” again.
3. Parent/pool/size changes should normally require replacement; `UseStateForUnknown` prevents unknown plan values from erasing a prior assigned CIDR.
4. Intentional reallocation should use a stable explicit trigger such as `keepers`, not a volatile timestamp unless perpetual reconsideration is genuinely intended. ([BloxOne state modifiers](https://github.com/infobloxopen/terraform-provider-bloxone/blob/master/internal/service/ipam/model_ipamsvc_subnet.go#L138-L150), [BlueCat keepers](https://github.com/umich-vci/terraform-provider-bluecat/blob/master/internal/provider/resource_ip4_available_network.go#L66-L100))

### Import and adoption

Import is safe only if the provider can reconstruct both remote identity and allocation intent. When the API omits origin pool, requested strategy, or requested size, blindly marking those fields `ForceNew` causes immediate destructive replacement. NetBox and Azure IPAM both exhibit this failure mode. ([NetBox issue](https://github.com/e-breuninger/terraform-provider-netbox/issues/861), [Azure IPAM warning](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/main/docs/resources/reservation.md#L73-L86))

Useful designs are:

- composite import IDs containing parent/pool and requested size;
- adoption-aware diff logic for unknown historical inputs;
- a durable remote allocation key and `allocated_from` provenance;
- explicit import-only fields when the remote API cannot recover original intent. ([NetBox prefix import](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_available_prefix.go#L114-L130), [NetBox idempotency proposal](https://github.com/netbox-community/netbox/issues/23411))

### Delete and release

- API-backed allocation resources should release/delete the remote reservation and tolerate already-missing objects.
- Parent pool deletion must wait for child releases where the remote system is eventually consistent.
- No-op delete is correct only for pure calculation or selector resources that created no remote reservation. ([BloxOne delete](https://github.com/infobloxopen/terraform-provider-bloxone/blob/master/internal/service/ipam/api_subnet_resource.go#L154-L175), [AWS eventual release](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_pool_cidr.go#L177-L225), [pure-provider no-op](https://github.com/finext-networks/terraform-provider-cidrblock/blob/main/internal/provider/resource_pool.go#L601-L606))

## Recommended schema for this provider

```hcl
resource "ipam_prefix_allocation" "example" {
  parent_id    = "pool-123"   # RequiresReplace
  prefix_length = 24          # RequiresReplace
  allocation_key = "app/prod/web" # RequiresReplace; remote uniqueness

  metadata = {
    owner = "network-team"
  }
}

# Computed and UseStateForUnknown:
# id
# cidr
# allocated_from
```

Required behavioral contract:

1. Create sends parent, requested prefix length, and allocation key in one atomic remote call.
2. A repeated create with the same key returns the same allocation.
3. Read uses allocation ID/key and never performs new allocation.
4. Parent or prefix-length changes replace the resource.
5. Delete releases the exact allocation and is idempotent.
6. Import can recover the key, parent, size, and CIDR—or uses a documented composite import ID.
7. Data sources may preview capacity but must be documented as non-reserving.
8. Pure layout functions remain separate and side-effect-free. These requirements directly address the failures documented by NetBox imports/idempotency, BloxOne previews, and positional `cidrsubnets` churn. ([NetBox proposal](https://github.com/netbox-community/netbox/issues/23411), [BloxOne issue](https://github.com/infobloxopen/terraform-provider-bloxone/issues/294), [`cidrsubnets` warning](https://developer.hashicorp.com/terraform/language/functions/cidrsubnets))

## CLAIMS
- CLAIM: Authoritative next-available allocation should be a resource whose create operation atomically chooses and reserves the CIDR; a data source is only a non-reserving preview. — RISK: high — SOURCES: [Terraform data sources](https://developer.hashicorp.com/terraform/language/data-sources), [BloxOne race](https://github.com/infobloxopen/terraform-provider-bloxone/issues/294), [phpIPAM resource](https://github.com/lord-kyron/terraform-provider-phpipam/blob/master/plugin/providers/phpipam/resource_phpipam_first_free_subnet.go#L31-L68) — COUNTER: A data source is acceptable when the result is informational and never used as an ownership claim. — PRIMARY: [BloxOne issue #294](https://github.com/infobloxopen/terraform-provider-bloxone/issues/294)
- CLAIM: The dominant interoperable schema is parent/pool reference plus requested prefix length, with computed CIDR and replacement on allocation-input changes. — RISK: normal — SOURCES: [NetBox](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_available_prefix.go#L15-L39), [BloxOne](https://github.com/infobloxopen/terraform-provider-bloxone/blob/master/internal/service/ipam/model_ipamsvc_subnet.go#L138-L175), [AWS](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_pool_cidr_allocation.go#L47-L94), [Azure IPAM](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/main/internal/provider/reservation_resource.go#L65-L121) — COUNTER: Static resources may accept an explicit CIDR instead, and pure planners can derive CIDRs entirely from configuration. — PRIMARY: [NetBox available-prefix schema](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/netbox/resource_netbox_available_prefix.go#L15-L39)
- CLAIM: Stable allocation identity must survive retries and imports; remote allocation keys and provenance are stronger than relying solely on Terraform state. — RISK: high — SOURCES: [NetBox idempotency proposal](https://github.com/netbox-community/netbox/issues/23411), [NetBox import failure](https://github.com/e-breuninger/terraform-provider-netbox/issues/861), [Azure IPAM import warning](https://github.com/XtratusCloud/terraform-provider-azureipam/blob/main/docs/resources/reservation.md#L73-L86) — COUNTER: Terraform state alone is sufficient where one locked state is the sole allocator and state loss/adoption are excluded operationally. — PRIMARY: [NetBox issue #23411](https://github.com/netbox-community/netbox/issues/23411)
- CLAIM: Pure CIDR functions eliminate mutable-allocation races but cannot coordinate shared inventory and can renumber infrastructure when positional inputs change. — RISK: normal — SOURCES: [`cidrsubnets`](https://developer.hashicorp.com/terraform/language/functions/cidrsubnets), [HashiCorp module](https://github.com/hashicorp/terraform-cidr-subnets/blob/master/README.md#L71-L157), [provider-function purity contract](https://developer.hashicorp.com/terraform/plugin/framework/functions/concepts) — COUNTER: Stable explicit `netnum` assignments or tombstoned list positions make pure planning highly reliable for centrally owned address plans. — PRIMARY: [Terraform `cidrsubnets` documentation](https://developer.hashicorp.com/terraform/language/functions/cidrsubnets)
- CLAIM: Provider-local locking is insufficient for multi-workspace allocation; correctness ultimately requires remote atomicity or a remote temporary claim. — RISK: high — SOURCES: [BloxOne keyed mutex](https://github.com/infobloxopen/terraform-provider-bloxone/blob/master/internal/service/ipam/api_subnet_resource.go#L70-L82), [Micetro temporary claim](https://github.com/menandmice/terraform-provider-menandmice/blob/develop/menandmice/resource_range.go#L101-L113), [Terraform state locking](https://developer.hashicorp.com/terraform/language/state/locking) — COUNTER: A single provider process and single locked state can serialize a narrowly scoped pool, but that is an operational restriction rather than an IPAM guarantee. — PRIMARY: [Micetro range schema](https://github.com/menandmice/terraform-provider-menandmice/blob/develop/menandmice/resource_range.go#L50-L115)

## EXPAND
- LEAD: Investigate allocation-key semantics and a database uniqueness constraint for this provider’s core resource. — WHY: It closes retry ambiguity, import provenance, cross-workspace duplication, and silent renumbering in one mechanism. — ANGLE: Model `allocation_key` as required and immutable within a parent pool; repeated create returns the same remote allocation, while conflicting size returns a typed conflict. Start from the concrete NetBox proposal’s `allocation_key + allocated_from` contract: [netbox-community/netbox#23411](https://github.com/netbox-community/netbox/issues/23411).
- LEAD: Compare plan-modifier behavior against BloxOne and AWS before finalizing schema. — WHY: Both demonstrate `Optional + Computed` assigned CIDRs, replacement of allocation inputs, and state preservation for unknown plan values. — ANGLE: Build a schema matrix for create, import, unknown parent, parent move, size change, remote deletion, and external CIDR drift using [BloxOne’s model](https://github.com/infobloxopen/terraform-provider-bloxone/blob/master/internal/service/ipam/model_ipamsvc_subnet.go#L138-L175) and [AWS allocation schema](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_pool_cidr_allocation.go#L47-L110).
- LEAD: Define explicit release states and eventual-consistency handling. — WHY: Cloud IPAM deletion may lag dependent-resource deletion by many minutes, and immediate pool reuse can otherwise fail unpredictably. — ANGLE: Separate “release requested,” “release pending,” and “released/not found”; use bounded provider timeouts modeled on [AWS VPC IPAM](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_pool_cidr.go#L177-L225).
- DEAD END: Do not build authoritative allocation around a `first_free` data source, a plan-time candidate CIDR, `timestamp()` as identity, or a provider-only mutex; each fails to establish durable cross-process ownership. ([phpIPAM preview](https://github.com/lord-kyron/terraform-provider-phpipam/blob/master/plugin/providers/phpipam/data_source_phpipam_first_free_subnet.go#L9-L42), [BloxOne race](https://github.com/infobloxopen/terraform-provider-bloxone/issues/294), [BlueCat timestamp example](https://github.com/bluecatlabs/terraform-provider-bluecat/blob/master/docs/resources/bluecat_ipv4network.md), [BloxOne mutex](https://github.com/infobloxopen/terraform-provider-bloxone/blob/master/internal/service/ipam/api_subnet_resource.go#L70-L82))