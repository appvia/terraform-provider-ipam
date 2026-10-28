[Research complete] toward identifying cloud-IPAM prior art for a new Terraform provider. You need: ledger 8/8 done; findings below cover GCP, OCI, IBM, Alibaba, DigitalOcean, Kubernetes, and five commercial platforms, with design implications and claim-risk notes. The attached browser was unavailable because the BrowserSkill daemon was not running (`no-daemon`), so research used direct web retrieval from primary documentation and provider repositories. Now: no task remains. Next: no task remains.

## Executive assessment

GCP and Alibaba are the strongest non-AWS/Azure cloud-provider precedents:

- **GCP Internal Ranges** is a relatively flat reservation object tied to a VPC, but it has unusually rich allocation algorithms, overlap exceptions, usage semantics, and consumer references. [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges)
- **Alibaba Cloud VPC IPAM** follows the hierarchical model familiar from AWS IPAM and commercial DDI: IPAM instance → scope → pool → pool CIDR → allocation, plus resource discovery. [Alibaba Terraform IPAM](https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam.html.markdown)
- **OCI IP Address Insights** is principally discovery, utilization, conflict detection, and alerting rather than a pool allocator. [OCI IP Address Insights](https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/ip_inventory.htm)
- **IBM Cloud VPC and DigitalOcean VPC** expose address prefixes, subnets, reservations, or user-selected VPC ranges, but I found no documented standalone first-party IPAM hierarchy comparable to GCP, Alibaba, AWS, or Azure. This negative finding is less certain than the positive findings and is marked high-risk below. [IBM VPC addressing](https://cloud.ibm.com/docs/vpc?topic=vpc-managing-ip-addresses) [DigitalOcean VPC features](https://docs.digitalocean.com/products/networking/vpc/details/features/)

## 1. Google Cloud Network Connectivity Internal Ranges

### Resource and field model

`projects.locations.internalRanges` represents a private address reservation within a VPC, with networking resources able to identify themselves as users of that range. The resource is global under the project/location hierarchy. [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges)

Important fields:

- `ipCidrRange`: an explicit CIDR. IPv6 requires an explicit range and is restricted to `usage=EXTERNAL_TO_VPC` and `peering=FOR_SELF`. [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges)
- `prefixLength`: a size request for automatically allocated IPv4 space. If supplied with `ipCidrRange`, their sizes must agree. It can also express an expansion by decreasing the prefix length. [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges)
- `targetCidrRange[]`: candidate parent spaces in which automatic allocation searches. The API reference says the fallback is `10.0.0.0/8`; the operational guide documents broader defaults depending on custom- versus auto-mode VPCs. [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges) [GCP internal-range guide](https://cloud.google.com/vpc/docs/create-use-internal-ranges)
- `network`: immutable VPC reference. A network cannot be deleted while reserved ranges refer to it. [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges)
- `users[]`: output-only references to resources consuming the range; these references prevent deletion. [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges)
- `excludeCidrRanges[]`: blocks omitted from auto-allocation without creating separate reservations for them. Google’s guide says this exclusion list cannot be changed after creation. [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges) [GCP internal-range guide](https://cloud.google.com/vpc/docs/create-use-internal-ranges)
- `immutable`: when true, only labels and description are mutable according to the REST reference; the operational guide specifically describes protection of CIDR and overlap configuration and says immutability itself cannot later be changed. [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges) [GCP internal-range guide](https://cloud.google.com/vpc/docs/create-use-internal-ranges)

### Usage

- `FOR_VPC`: VPC resources can associate themselves with the reserved range.
- `EXTERNAL_TO_VPC`: reserves space that cannot be associated with VPC resources, such as on-premises address space announced through Interconnect.
- `FOR_MIGRATION`: locks a range between source and target subnets and requires migration metadata; peering must be or default to `FOR_SELF`.  
  [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges)

This is a useful distinction between **allocatable space**, **externally occupied space**, and **temporarily locked migration space**.

### Peering semantics

- `FOR_SELF`: default; usable by the owning VPC and visible to peers, but peers cannot consume it.
- `FOR_PEER`: donated for consumption by a peer rather than by the owning VPC.
- `NOT_SHARED`: local to the owning VPC and reusable by peers under corresponding isolation conditions.  
  [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges)

This is more expressive than a simple pool/VPC association: allocation authority and route visibility are separate policy dimensions.

### Overlap policy

By default, Google blocks overlapping internal ranges and resources in the same VPC. Explicit exceptions are:

- `OVERLAP_ROUTE_RANGE`: permits more-specific static routes.
- `OVERLAP_EXISTING_SUBNET_RANGE`: permits overlap with existing subnets.  
  [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges) [GCP internal-range guide](https://cloud.google.com/vpc/docs/create-use-internal-ranges)

Overlap is therefore an explicit, typed exception rather than a single permissive boolean.

### Allocation options

Automatic allocation is selected by supplying `prefixLength` without `ipCidrRange`; only then can `allocationOptions` be used. Strategies are:

- `RANDOM`: concurrency-friendly legacy behavior, but fragments space.
- `FIRST_AVAILABLE`: deterministic and predictable.
- `RANDOM_FIRST_N_AVAILABLE`: randomly selects from the first N available ranges; intended to balance concurrent allocation with lower fragmentation. `firstAvailableRangesLookupSize` represents expected parallelism.
- `FIRST_SMALLEST_FITTING`: deterministic best-fit strategy that minimizes fragmentation.  
  [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges)

This is the richest cloud-native allocation-strategy vocabulary found in this axis.

### Subnetwork consumption

A subnet references an internal range through Compute’s `reservedInternalRange` field. The reference must use the `networkconnectivity.googleapis.com/projects/.../locations/global/internalRanges/...` form. The subnet can consume either the whole reservation or a specified `range` subset; secondary subnet ranges can also reference internal ranges. [GCP internal-range guide](https://cloud.google.com/vpc/docs/create-use-internal-ranges)

Terraform exposes this as:

```hcl
resource "google_compute_subnetwork" "example" {
  network                 = google_compute_network.example.id
  region                  = "us-central1"
  reserved_internal_range = "networkconnectivity.googleapis.com/${google_network_connectivity_internal_range.example.id}"
}
```

The same `reserved_internal_range` property exists inside `secondary_ip_range`; `ip_cidr_range` becomes optional when a reserved range is supplied. [Google provider subnetwork documentation](https://github.com/hashicorp/terraform-provider-google/blob/main/website/docs/r/compute_subnetwork.html.markdown)

### Terraform `google_network_connectivity_internal_range`

The Google provider mirrors nearly the entire API surface:

- Required: `name`, `network`, `usage`, `peering`.
- Optional allocation inputs: `ip_cidr_range`, `prefix_length`, `target_cidr_range`, `exclude_cidr_ranges`, `allocation_options`.
- Optional policy inputs: `overlaps`, `migration`, `immutable`, labels and description.
- Computed: `users`, provider/effective labels, and canonical ID.
- `allocation_options.allocation_strategy` supports all four API strategies.
- The resource also has `deletion_policy` values `DELETE`, `PREVENT`, and `ABANDON`, separating Terraform lifecycle intent from API immutability.  
  [Google provider internal-range documentation](https://github.com/hashicorp/terraform-provider-google/blob/main/website/docs/r/network_connectivity_internal_range.html.markdown)

## 2. Other cloud providers

### Oracle Cloud Infrastructure

OCI’s native IPAM-named capability is **IP Address Insights**. It provides tenancy-wide hierarchical visibility across VCNs, subnets, and network resources, including:

- used-address inventory;
- CIDR utilization;
- IP conflict review;
- utilization alarms.  
  [OCI IP Address Insights](https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/ip_inventory.htm)

The documented IAM verb is read-oriented (`read ipam`), and the capability’s stated limitations concern periodically computed utilization metrics. This supports treating it as an **observability/reconciliation IPAM**, not as evidence of a user-managed pool/allocation hierarchy. [OCI IP Address Insights](https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/ip_inventory.htm)

### IBM Cloud

IBM VPC assigns each zone a default address prefix, allows prefixes to be customized, and creates subnets as CIDR subsets of those prefixes. IBM also exposes reserved IPs and public address ranges as VPC networking primitives. [IBM VPC networking](https://cloud.ibm.com/docs/vpc?topic=vpc-about-networking-for-vpc) [IBM IP addresses](https://cloud.ibm.com/docs/vpc?topic=vpc-managing-ip-addresses)

I found no standalone IBM Cloud service in the examined first-party VPC documentation with independent IPAM instances, scopes, hierarchical pools, allocation strategies, or cross-cloud discovery. The native pattern is therefore **VPC-local address-prefix management**, not an enterprise IPAM control plane. [IBM VPC networking](https://cloud.ibm.com/docs/vpc?topic=vpc-about-networking-for-vpc)

### Alibaba Cloud

Alibaba now has a first-party **VPC IPAM** service with an extensive Terraform surface:

- `alicloud_vpc_ipam_ipam`: top-level IPAM with operating regions and automatically created private/public default scopes. [Terraform IPAM](https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam.html.markdown)
- `alicloud_vpc_ipam_ipam_scope`: custom scope, currently documented as supporting private scope type. [Terraform scope](https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam_scope.html.markdown)
- `alicloud_vpc_ipam_ipam_pool`: hierarchical pools through `source_ipam_pool_id`; supports region, IPv4/IPv6, auto-import, and minimum/default/maximum allocation masks. [Terraform pool](https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam_pool.html.markdown)
- `alicloud_vpc_ipam_ipam_pool_cidr`: provisions parent space into a pool either explicitly by CIDR or, for supported cases, by `netmask_length`; pool CIDRs are effectively immutable. [Terraform pool CIDR](https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam_pool_cidr.html.markdown)
- `alicloud_vpc_ipam_ipam_pool_allocation`: allocates or reserves from a pool using either explicit `cidr` or size-oriented `cidr_mask`. [Terraform allocation](https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam_pool_allocation.html.markdown)
- `alicloud_vpc_ipam_ipam_resource_discovery`: discovers resources across selected operating regions. [Terraform resource discovery](https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam_resource_discovery.html.markdown)

This is strong precedent for separating **pool capacity** from **allocations**, supporting both explicit CIDRs and size requests, and representing discovery as its own resource.

### DigitalOcean

DigitalOcean VPCs allow users to choose a VPC IP range and, in public preview, add subnets and manage routes. The documented product surface centers on VPC networks, subnets, peering, routes, and NAT rather than an independent IPAM hierarchy. [DigitalOcean VPC features](https://docs.digitalocean.com/products/networking/vpc/details/features/)

I found no documented DigitalOcean-native IPAM product with pools, scoped allocation policies, cross-region inventory, or next-available CIDR resources. The closest native pattern is direct CIDR selection on VPC/subnet resources. [DigitalOcean VPC features](https://docs.digitalocean.com/products/networking/vpc/details/features/)

## 3. Kubernetes and Cilium patterns

Kubernetes requires non-overlapping Pod, Service, and Node ranges, but delegates Pod addressing and advanced IPAM behavior to the network plugin rather than defining a universal cluster-level IPAM API. [Kubernetes cluster networking](https://kubernetes.io/docs/concepts/cluster-administration/networking/)

Cilium provides useful implementation patterns:

- Cluster-pool mode allocates per-node PodCIDRs from cluster CIDRs through `CiliumNode`; existing pool entries and mask size should not be changed, and capacity is expanded by appending CIDRs. [Cilium cluster-pool IPAM](https://docs.cilium.io/en/stable/network/concepts/ipam/cluster-pool/)
- Multi-pool mode represents pools as `CiliumPodIPPool`, each with one or more parent CIDRs and a per-node `maskSize`. Workloads select pools by explicit annotation or pod/namespace selectors. [Cilium multi-pool IPAM](https://docs.cilium.io/en/stable/network/concepts/ipam/multi-pool/)
- Pools cannot overlap. A workload matching multiple pools fails allocation rather than using ambiguous precedence. [Cilium multi-pool IPAM](https://docs.cilium.io/en/stable/network/concepts/ipam/multi-pool/)
- In-use CIDRs cannot safely be removed; mask size is immutable; CIDRs can be appended at runtime. [Cilium multi-pool IPAM](https://docs.cilium.io/en/stable/network/concepts/ipam/multi-pool/)
- Node agents express requested capacity while the operator records allocated CIDRs, separating demand from allocation result. [Cilium multi-pool IPAM](https://docs.cilium.io/en/stable/network/concepts/ipam/multi-pool/)

For a Terraform provider, the valuable pattern is not Pod-level address management but **append-only pool capacity, immutable allocation granularity, explicit selectors, and fail-closed ambiguity handling**.

## 4. Commercial multi-cloud IPAM and Terraform

### Infoblox Universal DDI / BloxOne

Universal DDI centralizes DNS, DHCP, IPAM, and asset visibility through a cloud control plane and API across hybrid and multi-cloud networks. It supports policy-based allocation and discovery, including assigning unique address blocks into cloud environments. [Infoblox Universal DDI](https://www.infoblox.com/products/universal-ddi/)

Terraform options:

- The BloxOne DDI provider manages BloxOne resources such as IP spaces, address blocks, subnets, ranges, fixed addresses, and individual addresses. [BloxOne provider](https://github.com/infobloxopen/terraform-provider-b1ddi) [BloxOne resource list](https://github.com/infobloxopen/terraform-provider-b1ddi/tree/master/docs/resources)
- A newer `infobloxopen/infoblox` provider, currently documented as beta with GA planned for January 2027, targets both NIOS and Universal DDI and lists IPAM, IPAM Federation, cloud, and cloud-discovery object groups. [Infoblox provider](https://github.com/infobloxopen/terraform-provider-infoblox)
- These providers primarily manage Infoblox’s DDI source of truth; cloud-network creation still requires a cloud provider or orchestration linking the allocated CIDR into that provider.

### BlueCat Integrity

Integrity is an API-first DDI source of truth with discovery and reconciliation across on-premises infrastructure, AWS, Azure, and GCP. BlueCat explicitly describes Terraform integration for IP provisioning and DDI automation. [BlueCat Integrity](https://bluecatnetworks.com/products/integrity/)

The BlueCat Terraform provider exposes explicit and next-available IPv4 blocks and networks. Its examples support a `parent_block` plus requested `size`, as well as explicit addresses/CIDRs and individual address allocation. [BlueCat Terraform provider](https://github.com/bluecatlabs/terraform-provider-bluecat)

The important prior art is a dual Terraform surface: **explicit CIDR resources and next-available size requests**.

### BlueCat Micetro

Micetro is an API-driven DDI overlay that manages existing DNS/DHCP/IPAM across on-premises, cloud, and branch environments, including AWS, Azure, and Google Cloud discovery and orchestration. [BlueCat Micetro](https://bluecatnetworks.com/products/micetro/)

Its Men&Mice/Micetro Terraform provider exposes ranges plus an `ipam_record` that can either claim an explicit address or atomically find a free address from a named range. The `free_ip` block supports exclusions, search starting point, ping verification, and a temporary claim time to prevent concurrent reuse. [Micetro provider](https://github.com/menandmice/terraform-provider-menandmice) [Micetro IPAM record](https://github.com/menandmice/terraform-provider-menandmice/blob/master/docs/resources/ipam_record.md)

That temporary-claim mechanism is notable prior art for concurrent allocation.

### EfficientIP SOLIDserver

SOLIDserver provides unified DDI across on-premises and AWS, Azure, and GCP, including multi-vendor DNS/DHCP integration. [SOLIDserver product](https://www.efficientip.com/products/solidserver-ddi/)

The `EfficientIP-Labs/solidserver` Terraform provider operates through the SOLIDserver REST API and manages IP spaces, IPv4/IPv6 subnets, pools, addresses, VLANs, and related DNS/DHCP objects. [SOLIDserver provider](https://github.com/EfficientIP-Labs/terraform-provider-solidserver)

As with Infoblox and BlueCat, Terraform manages the centralized DDI database; a separate cloud-provider resource consumes the resulting assignment.

### NetBox Cloud

NetBox Cloud is the hosted SaaS form of NetBox, retaining NetBox’s APIs and automation model. [NetBox Cloud](https://netboxlabs.com/netbox-cloud/)

NetBox IPAM models aggregates, nested prefixes, ranges, addresses, VRFs, utilization, and optionally overlapping space isolated by VRF. VRFs can independently allow or prohibit duplicate IP objects. [NetBox IPAM](https://netboxlabs.com/docs/netbox/en/stable/features/ipam/)

The community-maintained `e-breuninger/netbox` Terraform provider provides full lifecycle management of NetBox resources and focuses on VM management and IPAM. [NetBox Terraform provider](https://github.com/e-breuninger/terraform-provider-netbox)

Its allocation resources include:

- `netbox_available_prefix`: requests a `prefix_length` beneath a `parent_prefix_id`, returning the selected prefix. [Available prefix](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/docs/resources/available_prefix.md)
- `netbox_available_ip_address`: obtains the next address from a prefix or IP range. [Available IP address](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/docs/resources/available_ip_address.md)
- `netbox_prefix`: manages an explicit prefix and its VRF, tenant, location, status, role, and other metadata. [Explicit prefix](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/docs/resources/prefix.md)

This is strong precedent for distinct **explicit-prefix** and **allocated-prefix** resource types.

## 5. Design implications for a new Terraform IPAM provider

### A. Support explicit CIDR and size-request allocation as distinct modes

Both are established patterns:

- GCP: `ip_cidr_range` versus `prefix_length`. [GCP Terraform resource](https://github.com/hashicorp/terraform-provider-google/blob/main/website/docs/r/network_connectivity_internal_range.html.markdown)
- Alibaba: `cidr` versus `cidr_mask`. [Alibaba allocation](https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam_pool_allocation.html.markdown)
- NetBox: `netbox_prefix` versus `netbox_available_prefix`. [NetBox explicit prefix](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/docs/resources/prefix.md) [NetBox available prefix](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/docs/resources/available_prefix.md)
- BlueCat: explicit address/CIDR versus parent plus requested size. [BlueCat provider](https://github.com/bluecatlabs/terraform-provider-bluecat)

Recommendation: model these either as mutually exclusive arguments with exactly-one validation, or as separate resources if their lifecycle differs materially. Separate resources make replacement and drift behavior clearer.

### B. Treat the allocated CIDR as server-selected identity

For size requests, the chosen CIDR must be computed and persisted in Terraform state. Changes to parent pool, requested size, address family, or allocation strategy should normally force replacement unless the backend can perform safe in-place expansion. This follows the force-new behavior in Alibaba allocation resources and immutable pool-CIDR semantics. [Alibaba allocation](https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam_pool_allocation.html.markdown) [Alibaba pool CIDR](https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam_pool_cidr.html.markdown)

### C. Make allocation strategy explicit and stable

A useful initial vocabulary is:

- `first_available`;
- `random`;
- `random_first_n`;
- `smallest_fitting`.  
  [GCP allocation strategies](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges)

The provider should not silently change algorithms between versions. If backend allocation is implementation-defined, expose `default` explicitly and document that reproducibility is weaker.

### D. Default to rejecting overlap

GCP defaults to no overlap and exposes typed exceptions; Cilium rejects overlapping pools entirely; NetBox isolates intentional overlap by VRF. [GCP internal ranges](https://cloud.google.com/vpc/docs/create-use-internal-ranges) [Cilium multi-pool](https://docs.cilium.io/en/stable/network/concepts/ipam/multi-pool/) [NetBox IPAM](https://netboxlabs.com/docs/netbox/en/stable/features/ipam/)

Recommendation:

- reject overlap by default;
- scope uniqueness by an explicit routing domain/VRF;
- expose narrow overlap reasons rather than `allow_overlap=true`;
- fail when allocation matches multiple candidate pools unless the caller supplies deterministic precedence.

### E. Separate external reservations, active allocations, and migration locks

GCP’s `FOR_VPC`, `EXTERNAL_TO_VPC`, and `FOR_MIGRATION` are useful lifecycle states, while NetBox’s status metadata distinguishes active, reserved, deprecated, and container prefixes. [GCP usage](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges) [NetBox prefix resource](https://github.com/e-breuninger/terraform-provider-netbox/blob/master/docs/resources/prefix.md)

A provider should not overload one boolean such as `reserved`; use a typed purpose/status enum.

### F. Prefer append-only capacity and immutable allocation granularity

Cilium documents that in-use CIDRs should not be removed and pool mask size is immutable; GCP immutable ranges restrict updates, while mutable ranges can only expand to a supernet containing the prior allocation. [Cilium multi-pool](https://docs.cilium.io/en/stable/network/concepts/ipam/multi-pool/) [GCP internal-range guide](https://cloud.google.com/vpc/docs/create-use-internal-ranges)

Recommendation:

- make parent scope, family, granularity, and possibly strategy immutable;
- allow adding capacity blocks;
- permit expansion only with an explicit containment invariant;
- force replacement for shrink, relocation, or family change.

### G. Exclusions belong to allocation policy, not child reservations

GCP’s `excludeCidrRanges` prevents selection without filling the model with dummy reservations, but the list is immutable after creation. [GCP internal-range guide](https://cloud.google.com/vpc/docs/create-use-internal-ranges)

Recommendation: support exclusion lists on pools or allocation requests, canonicalize them, validate containment, and make post-allocation changes replacement-triggering unless the backend can prove they do not invalidate the result.

### H. Track consumers and protect deletion

GCP records `users[]` and blocks deletion while consumers remain; the Google provider separately offers deletion policy controls. [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges) [GCP Terraform resource](https://github.com/hashicorp/terraform-provider-google/blob/main/website/docs/r/network_connectivity_internal_range.html.markdown)

Recommendation: expose computed consumer references and a lifecycle policy such as `delete`, `prevent`, or `abandon`. API referential integrity and Terraform lifecycle protection solve different problems and should remain separate.

### I. Concurrency needs a first-class contract

GCP’s random-first-N strategy explicitly addresses concurrent allocation; Micetro supports temporary claims to prevent competing consumers from taking an address. [GCP REST resource](https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges) [Micetro IPAM record](https://github.com/menandmice/terraform-provider-menandmice/blob/master/docs/resources/ipam_record.md)

Recommendation: ensure the backend performs atomic selection-and-reservation. Client-side “read free ranges, choose one, then create” is unsafe. If allocations have a workflow before final consumption, add lease/claim expiry rather than relying on Terraform apply serialization.

## CLAIMS

- CLAIM: GCP Internal Ranges supports explicit CIDRs, size-based automatic allocation, search targets, exclusions, four allocation strategies, typed overlaps, peering policy, migration locks, and immutable reservations. — RISK: normal — SOURCES: https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges ; https://cloud.google.com/vpc/docs/create-use-internal-ranges — COUNTER: The REST reference and operational guide differ on the documented default target ranges; provider behavior should follow the API actually used and be acceptance-tested. — PRIMARY: https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges
- CLAIM: GCP subnetworks consume an Internal Range through `reservedInternalRange`, including primary or secondary ranges and optionally a subset of the reservation. — RISK: normal — SOURCES: https://cloud.google.com/vpc/docs/create-use-internal-ranges ; https://github.com/hashicorp/terraform-provider-google/blob/main/website/docs/r/compute_subnetwork.html.markdown — COUNTER: Some Terraform examples still identify the field as beta/provider-version-sensitive; exact GA availability should be checked against the provider version selected by the project. — PRIMARY: https://cloud.google.com/vpc/docs/create-use-internal-ranges
- CLAIM: Alibaba Cloud provides a full first-party VPC IPAM hierarchy and corresponding Terraform resources for instances, scopes, pools, pool CIDRs, allocations, and discovery. — RISK: normal — SOURCES: https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam.html.markdown ; https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam_pool.html.markdown ; https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam_pool_allocation.html.markdown — COUNTER: Availability is provider-version- and region-dependent; the IPAM resources begin at documented versions 1.234.0–1.243.0. — PRIMARY: https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam.html.markdown
- CLAIM: OCI IP Address Insights is a tenancy-wide inventory, utilization, conflict, and alerting capability, not documented as a user-managed CIDR pool allocator. — RISK: normal — SOURCES: https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/ip_inventory.htm — COUNTER: Oracle may add or separately document allocation APIs beyond the retrieved IP Address Insights surface. — PRIMARY: https://docs.oracle.com/en-us/iaas/Content/Network/Concepts/ip_inventory.htm
- CLAIM: IBM Cloud VPC’s native model centers on zone address prefixes, subnets, and IP reservations rather than a standalone cross-cloud IPAM hierarchy. — RISK: high — SOURCES: https://cloud.ibm.com/docs/vpc?topic=vpc-about-networking-for-vpc ; https://cloud.ibm.com/docs/vpc?topic=vpc-managing-ip-addresses — COUNTER: Absence claims are difficult to prove; IBM may expose an IPAM-branded capability outside the VPC documentation set or through a partner product. — PRIMARY: https://cloud.ibm.com/docs/vpc?topic=vpc-about-networking-for-vpc
- CLAIM: DigitalOcean documents direct VPC/subnet CIDR management but no standalone native IPAM service with pools and allocations. — RISK: high — SOURCES: https://docs.digitalocean.com/products/networking/vpc/details/features/ — COUNTER: Subnet functionality was in public preview and the product catalog can evolve; re-check before treating this as a permanent platform limitation. — PRIMARY: https://docs.digitalocean.com/products/networking/vpc/details/features/
- CLAIM: Cilium’s multi-pool design makes allocation granularity immutable, allows pool capacity extension, rejects overlapping pools, and fails ambiguous multi-pool matches. — RISK: normal — SOURCES: https://docs.cilium.io/en/stable/network/concepts/ipam/multi-pool/ — COUNTER: These are cluster dataplane constraints and should inform, not dictate, a general-purpose multi-cloud IPAM data model. — PRIMARY: https://docs.cilium.io/en/stable/network/concepts/ipam/multi-pool/
- CLAIM: Commercial IPAM providers expose Terraform mainly by managing the DDI/IPAM source of truth; cloud-provider resources then consume the selected addresses or CIDRs. — RISK: normal — SOURCES: https://github.com/infobloxopen/terraform-provider-b1ddi ; https://github.com/bluecatlabs/terraform-provider-bluecat ; https://github.com/menandmice/terraform-provider-menandmice ; https://github.com/EfficientIP-Labs/terraform-provider-solidserver ; https://github.com/e-breuninger/terraform-provider-netbox — COUNTER: Vendor-specific modules or integrations may combine allocation and cloud resource creation into a single higher-level workflow. — PRIMARY: https://github.com/infobloxopen/terraform-provider-b1ddi
- CLAIM: Both explicit-CIDR and size-request allocation are recurring Terraform patterns and should be first-class in a new provider. — RISK: normal — SOURCES: https://github.com/hashicorp/terraform-provider-google/blob/main/website/docs/r/network_connectivity_internal_range.html.markdown ; https://github.com/aliyun/terraform-provider-alicloud/blob/master/website/docs/r/vpc_ipam_ipam_pool_allocation.html.markdown ; https://github.com/e-breuninger/terraform-provider-netbox/blob/master/docs/resources/available_prefix.md ; https://github.com/bluecatlabs/terraform-provider-bluecat — COUNTER: One resource with mutually exclusive arguments is simpler, while separate explicit and allocated resource types can produce clearer lifecycle behavior; the best choice depends on this provider’s backend API. — PRIMARY: https://github.com/hashicorp/terraform-provider-google/blob/main/website/docs/r/network_connectivity_internal_range.html.markdown
- CLAIM: Safe concurrent allocation requires an atomic backend reservation operation or a temporary claim/lease mechanism. — RISK: normal — SOURCES: https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges ; https://github.com/menandmice/terraform-provider-menandmice/blob/master/docs/resources/ipam_record.md — COUNTER: Terraform state locking reduces same-state concurrency but does not protect separate workspaces, external clients, or multiple automation systems. — PRIMARY: https://cloud.google.com/network-connectivity/docs/reference/networkconnectivity/rest/v1/projects.locations.internalRanges

## EXPAND

- LEAD: Verify the live Google provider version in the target project and whether `reserved_internal_range` remains beta-gated. — WHY: The provider documentation includes beta examples even though the API guide presents the capability directly. — ANGLE: Inspect the selected provider schema and run a minimal plan against `google` and `google-beta`.
- LEAD: Compare Alibaba VPC IPAM’s backend API concurrency and allocation-order guarantees with GCP’s named strategies. — WHY: Alibaba clearly supports size requests but its Terraform documentation does not state how an available CIDR is selected. — ANGLE: Inspect `CreateIpamPoolAllocation` API semantics and concurrent-allocation behavior.
- LEAD: Decide whether the new provider should use one allocation resource or separate explicit/automatic resource types. — WHY: Google and Alibaba use mutually exclusive fields; NetBox separates them into different resources. — ANGLE: Model update, import, drift, and replacement behavior for each design before freezing schema.
- LEAD: Investigate a typed routing-domain/VRF abstraction. — WHY: NetBox permits overlapping address space safely by VRF, while GCP uses narrower overlap exceptions and Cilium forbids overlap entirely. — ANGLE: Define uniqueness as `(address family, routing domain, CIDR)` rather than global CIDR uniqueness.
- LEAD: Prototype atomic claim semantics for cross-workspace concurrency. — WHY: Terraform state locking alone is insufficient; GCP and Micetro both expose concurrency-aware mechanisms. — ANGLE: Backend transaction, idempotency key, expiring claim, and final consumer binding.
- LEAD: Add consumer references and abandonment semantics to the provider design. — WHY: GCP’s `users[]`, deletion blocking, and separate Terraform `ABANDON/PREVENT/DELETE` policy form a robust lifecycle model. — ANGLE: Distinguish API dependency protection from Terraform state-removal behavior.
- DEAD END: General web search was unavailable because configured search backends hit authorization, challenge, or rate-limit failures; direct primary URLs and provider repositories were used instead.
- DEAD END: The attached browser was not ready (`no-daemon`), so no signed-in or interactive browsing path was used.
- DEAD END: Several guessed legacy Alibaba CEN-IPAM URLs and resource names returned 404; the current product and provider namespace is `vpc_ipam`, confirmed from the official provider repository tree and generated documentation.