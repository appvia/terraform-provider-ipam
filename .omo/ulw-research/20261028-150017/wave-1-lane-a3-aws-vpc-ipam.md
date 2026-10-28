st_01a18309 [completed] transcript via event-log:
assistant: I read this as exhaustive prior-art research on AWS VPC IPAM for a new Terraform provider - I’ll inspect the required browser setup, gather primary AWS and Terraform evidence, synthesize implementation implications, and stop when the cited report plus CLAIMS/EXPAND ledger is complete.
tool: todo
assistant: AWS VPC IPAM research is now structured end-to-end. You need: 0/12 done; required browser instructions are first, then AWS, Terraform, issue evidence, and synthesis. Now: Read browser research operating instructions. Next: Verify attached browser engine readiness.
tool: read
tool: todo
tool: read
tool: eval
tool: todo
assistant: The source path is decided: attached BrowserSkill is unavailable (`no-daemon`), so the requested fallback is webfetch against primary sources. You need: 2/12 done; no browser-authenticated evidence will be used. Now: Document AWS IPAM object and tier model. Next: Trace allocation and VPC consumption semantics.
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
tool: todo
tool: todo
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
tool: todo
assistant: The AWS model and evolution are mapped, including current 2026 capabilities: policies, prefix-list automation, external IPAM authority, allocation tags, CloudFront BYOIP, and BGP/RPKI protection. You need: 6/12 done; primary AWS docs and announcements support the service analysis. Now: Extract Terraform resource schema lifecycle details. Next: Investigate preview CIDR behavior and pitfalls.
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
assistant: Terraform behavior and failure modes are now evidenced from current provider source and issue history. You need: 9/12 done; key risks are preview-without-reservation, eventual-consistency teardown, mutable/computed schema drift, and cross-account/Region coupling. Now: Derive new provider design implications. Next: Write fully cited prior-art research report.
tool: todo
tool: todo
tool: todo
assistant: Research complete toward designing a new Terraform IPAM provider from AWS prior art. You need:
- **12/12 research tasks complete.**
- BrowserSkill was checked first but reported `no-daemon`; per the assignment, research used `webfetch` against AWS documentation, AWS announcements, Terraform AWS provider source, and GitHub issues.
- No repository files were changed.
- Now: no task remains.
- Next: no task remains.

# AWS VPC IPAM prior-art report

## 1. Service model and object hierarchy

AWS models IPAM as:

```text
IPAM
└── Scope: public or private
    └── Pool: optionally nested, up to quota depth
        └── Provisioned CIDR
            └── Allocation: child pool, VPC/subnet, integrated resource, or reservation
```

- Creating an IPAM automatically creates one default private scope and one default public scope. A scope is an independent routing/security domain, allowing the same addresses to exist in disconnected networks without being considered conflicting. Pools contain contiguous ranges; allocations assign part of a pool to another pool or resource. [AWS: How IPAM works](https://docs.aws.amazon.com/vpc/latest/ipam/how-it-works-ipam.html)
- AWS distinguishes **provisioning**—adding address space to a pool—from **allocation**—assigning that space to a child pool or resource. Default hierarchy depth is ten pools. [AWS: Plan for IP address provisioning](https://docs.aws.amazon.com/vpc/latest/ipam/planning-ipam.html)
- Pools may be nested using `SourceIpamPoolId`. A child pool’s provisioned CIDR must be available within its parent. Pools have an immutable address family, and can have a locale, public address source, AWS service restriction, resource-planning source, and allocation rules. [EC2 API: CreateIpamPool](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateIpamPool.html)
- A pool locale is the Region—or for supported public IPv4 pools, a Local Zone network border group—where allocations may be consumed. A VPC can allocate only from a pool whose locale matches the VPC Region. Once chosen, locale cannot be modified. [AWS: Create a Regional pool](https://docs.aws.amazon.com/vpc/latest/ipam/create-reg-ipam.html)
- Pool allocation rules include minimum, maximum, and default netmask lengths; required resource tags; locale compliance; and `auto_import`. Rules apply to managed resources directly in the pool, not recursively to resources in child pools. [AWS: Create a top-level pool](https://docs.aws.amazon.com/vpc/latest/ipam/create-top-ipam.html)
- `auto_import` continuously imports discovered resources inside the pool range. It may import noncompliant resources, chooses the largest of overlapping CIDRs, randomly chooses one resource when multiple resources have the same CIDR, requires a locale, and cannot import an allocation that conflicts with an existing one. [EC2 API: CreateIpamPool](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateIpamPool.html)

### Free versus Advanced tier

- **Free Tier** targets a single Region/account and includes BYOIPv4/BYOIPv6 management, Amazon-provided contiguous IPv6, and Public IP Insights. **Advanced Tier** adds multi-Region/multi-account management, private IPv4 management, and historical auditing; it is billed per active address or attached prefix. [Amazon VPC pricing](https://aws.amazon.com/vpc/pricing/)
- Free Tier pools can use only the IPAM home Region as locale. Downgrading from Advanced requires removing private-scope pools, non-default private scopes, non-home-Region pools, non-default resource-discovery associations, and cross-account allocations. [AWS: Modify IPAM tier](https://docs.aws.amazon.com/vpc/latest/ipam/mod-ipam-tier.html)
- Current IPAM creation also supports `metered_account` (`ipam-owner` or `resource-owner`) and `enable_private_gua`, allowing customer GUA ranges to be used as private IPv6 space. [EC2 API: CreateIpam](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateIpam.html)

## 2. Allocation semantics

### Provisioned CIDRs

- `ProvisionIpamPoolCidr` accepts either an explicit `Cidr` or a `NetmaskLength`. Netmask-based provisioning is supported for child pools and Amazon-provided IPv6 top-level pools, but not for top-level BYOIP CIDRs. Public BYOIP can require an authorization context or external verification token. [EC2 API: ProvisionIpamPoolCidr](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ProvisionIpamPoolCidr.html)
- AWS supports DNS-token verification in addition to signed `remarks-x509` authorization. External verification tokens are AWS-generated values that prove control of external resources such as public ranges. [EC2 API: CreateIpamExternalResourceVerificationToken](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateIpamExternalResourceVerificationToken.html)

### Manual allocation/reservation

- `AllocateIpamPoolCidr` accepts either an explicit `cidr` or `netmask_length`. If the pool has no default netmask, one is required; an explicitly supplied value overrides the pool default. The API guarantees strong consistency for allocation, so the returned CIDR does not overlap another allocation from the same pool. [EC2 API: AllocateIpamPoolCidr](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_AllocateIpamPoolCidr.html)
- `allowed_cidrs` and `disallowed_cidrs` constrain automatic selection and are valid only with `netmask_length`, not with an explicit CIDR. `PreviewNextCidr=true` computes a candidate but does not reserve it. [EC2 API: AllocateIpamPoolCidr](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_AllocateIpamPoolCidr.html)
- `ReleaseIpamPoolAllocation` releases only manual allocations. Integrated resource allocations should normally disappear when the resource is deleted; alternatively their monitored state can be disabled. Release and subsequent reads use EC2 eventual consistency. [EC2 API: ReleaseIpamPoolAllocation](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ReleaseIpamPoolAllocation.html)
- Allocation listing is locale-sensitive and eventually consistent after allocation or release. [EC2 API: GetIpamPoolAllocations](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_GetIpamPoolAllocations.html)

### VPC consumption

- `CreateVpc` accepts `Ipv4IpamPoolId` plus `Ipv4NetmaskLength`; AWS chooses and allocates the concrete CIDR atomically as part of VPC creation. Equivalent IPv6 parameters are available. [EC2 API: CreateVpc](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateVpc.html)
- In Terraform, `aws_vpc.ipv4_ipam_pool_id` and `ipv4_netmask_length` are therefore preferable to previewing a CIDR and passing it through `cidr_block`: the VPC operation performs the real allocation rather than relying on a stale candidate. [Terraform AWS: `aws_vpc`](https://github.com/hashicorp/terraform-provider-aws/blob/main/website/docs/r/vpc.html.markdown)
- A manually reserved CIDR is not automatically “adopted” by later VPC creation. Reserving a range and then asking the VPC API to allocate from the same pool can make the desired space unavailable; this is represented in issue #39046’s “allocation size too big” workflow. [terraform-provider-aws #39046](https://github.com/hashicorp/terraform-provider-aws/issues/39046)

## 3. Organizations, sharing, and discovery

- An Organizations management account delegates an ordinary member account—not the management account—as IPAM administrator. IPAM creates `AWSServiceRoleForIPAM` across member accounts, discovers their resources, and imports their CIDRs for monitoring. Without delegation, discovery is limited to the IPAM owner account. [AWS: Integrate IPAM with Organizations](https://docs.aws.amazon.com/vpc/latest/ipam/enable-integ-ipam.html)
- Pools are shared through AWS RAM. A share can target accounts, IAM roles, or OUs; creation occurs in the IPAM home Region. Standard permission allows viewing and allocating/releasing CIDRs, while a separate permission permits importing existing BYOIP ranges. [AWS: Share an IPAM pool](https://docs.aws.amazon.com/vpc/latest/ipam/share-pool-ipam.html)
- Each IPAM has a default resource discovery. Additional discoveries can be created independently, shared, and associated with another account’s IPAM to monitor accounts outside the IPAM owner’s organization. A discovery owned by an Organizations delegated administrator covers that organization’s members. [AWS: Work with resource discoveries](https://docs.aws.amazon.com/vpc/latest/ipam/res-disc-work-with.html)
- Resource discoveries have their own operating Regions; associations explicitly bind an IPAM to a discovery. [EC2 API: CreateIpamResourceDiscovery](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateIpamResourceDiscovery.html), [EC2 API: AssociateIpamResourceDiscovery](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_AssociateIpamResourceDiscovery.html)
- BYOIP managed through IPAM can be onboarded once and distributed across Regions/accounts, whereas non-IPAM BYOIP must be onboarded per Region. [AWS: Bring your IP addresses to IPAM](https://docs.aws.amazon.com/vpc/latest/ipam/tutorials-byoip-ipam.html)

## 4. Significant 2024–2026 evolution

### 2024

- AWS introduced Amazon-provided contiguous public IPv4 blocks, provisioned into public regional IPAM pools and consumed as EIPs by EC2, NLB, and NAT Gateway. All addresses in the block are billed, not only addresses in use. [AWS announcement: contiguous IPv4](https://aws.amazon.com/about-aws/whats-new/2024/08/amazon-provided-contiguous-ipv4-blocks/), [Amazon VPC pricing](https://aws.amazon.com/vpc/pricing/)
- IPAM added OU exclusions, allowing discovery/monitoring to omit selected parts of an organization. [AWS announcement: OU-scoped IPAM](https://aws.amazon.com/about-aws/whats-new/2024/11/amazon-vpc-ipam-organizational-units-aws-organizations/)
- BYOIP and BYOASN expanded to all AWS Local Zones. [AWS announcement: BYOIP/BYOASN Local Zones](https://aws.amazon.com/about-aws/whats-new/2024/10/amazon-virtual-private-cloud-byoip-byoasn-local-zones/)

### 2025

- **Prefix List Resolver** began synchronizing managed prefix lists from rules over VPCs, subnets, and IPAM pools, removing manual security-group and route-table prefix maintenance. [AWS announcement: prefix-list automation](https://aws.amazon.com/about-aws/whats-new/2025/10/amazon-vpc-ipam-prefix-list-automation/), [AWS: Automate prefix-list updates](https://docs.aws.amazon.com/vpc/latest/ipam/automate-prefix-list-updates.html)
- **IPAM policies** added centrally enforced public IPv4 allocation rules, initially for EIPs and regional NAT Gateways. Policies target accounts/OUs/the organization and have explicit precedence; Advanced Tier is required for cross-account application. [AWS announcement: IPAM policies](https://aws.amazon.com/about-aws/whats-new/2025/11/amazon-vpc-ipam-policies-ip-allocation-strategy/), [AWS: IPAM policies](https://docs.aws.amazon.com/vpc/latest/ipam/ipam-policies.html)
- Policy allocation is deliberately fail-open: an unavailable/inaccessible/exhausted pool can fall back to Amazon-provided addresses. An explicitly selected pool or EIP allocation overrides policy. [AWS: Public IPv4 allocation policies](https://docs.aws.amazon.com/vpc/latest/ipam/define-public-ipv4-allocation-strategy-with-ipam-policies.html)
- AWS added Infoblox Universal IPAM as an external authority for top-level address acquisition and added CloudFront Anycast Static IP BYOIPv4 integration. [AWS announcement: Infoblox](https://aws.amazon.com/about-aws/whats-new/2025/11/amazon-vpc-ipam-automates-ip-from-infoblox/), [AWS announcement: CloudFront BYOIP](https://aws.amazon.com/about-aws/whats-new/2025/11/vpc-ipam-cloudfront-byoip)

### 2026 through October 28

- Policies expanded to RDS and ALB, alongside EIPs and regional NAT Gateway. [AWS announcement: RDS/ALB policies](https://aws.amazon.com/about-aws/whats-new/2026/01/amazon-vpc-ipam-policies-rds-alb/)
- CloudFront IPAM integration added BYOIPv6 `/48` in
...[elided 3633 chars]...
imeout; this is a concrete implementation defect to avoid copying. [Provider source](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_pool_cidr.go)

### `aws_vpc_ipam_pool_cidr_allocation`

- `cidr`, description, `disallowed_cidrs`, pool ID, and netmask are ForceNew. The allocation ID and resource metadata are computed; tags are mutable. The Terraform resource composes state identity from allocation ID plus pool ID. [Provider source](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_pool_cidr_allocation.go), [resource documentation](https://github.com/hashicorp/terraform-provider-aws/blob/main/website/docs/r/vpc_ipam_pool_cidr_allocation.html.markdown)

### Discovery and Organizations resources

- `aws_vpc_ipam_resource_discovery` updates description, operating Regions, and OU exclusions in place; ARN, home Region, default status, and owner are computed. It retries newly created OU paths because Organizations discovery propagates asynchronously. [Provider source](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_resource_discovery.go)
- `aws_vpc_ipam_resource_discovery_association` exposes required IPAM/discovery IDs plus computed association metadata. The IDs are not marked ForceNew despite no update path beyond tags—another schema inconsistency. [Provider source](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_resource_discovery_association.go)
- `aws_vpc_ipam_organization_admin_account.delegated_admin_account_id` is ForceNew; Organizations ARN, email, name, and service principal are computed. [Provider source](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_organization_admin_account.go)

### Data sources

- `aws_vpc_ipam_pool` performs singular lookup by pool ID and/or EC2 filters and exports allocation rules, locality, hierarchy, source resource, and state. [Provider source](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_pool_data_source.go)
- `aws_vpc_ipam_pools` returns a computed set of matching pools. [Provider source](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_pools_data_source.go)
- `aws_vpc_ipam_pool_cidrs` lists provisioned CIDRs and their states for one pool. [Provider source](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_pool_cidrs_data_source.go)

## 6. `preview_next_cidr`: behavior and pitfalls

- Despite its resource-shaped Terraform interface, preview makes one `AllocateIpamPoolCidr(PreviewNextCidr=true)` call during Create, stores `<cidr>_<pool-id>` as state ID, performs a no-op Delete, and on Read merely decodes state without querying AWS. It therefore neither reserves the CIDR nor detects that the candidate has since become unavailable. [Provider source](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_preview_next_cidr.go)
- Parallel preview instances can all receive the same candidate because no preview reserves space; issue #25958 demonstrates repeated `count` instances returning the same CIDR. [terraform-provider-aws #25958](https://github.com/hashicorp/terraform-provider-aws/issues/25958)
- Its apparent stability is state persistence, not authoritative drift detection. Replacement/taint can select a different CIDR, while a normal refresh retains a stale one. This makes preview unsuitable as an allocation primitive or cross-stack coordination mechanism. [Provider source](https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_preview_next_cidr.go), [EC2 API: AllocateIpamPoolCidr](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_AllocateIpamPoolCidr.html)
- Correct patterns are either: let the consumer allocate directly using pool ID plus netmask, or create a real manual allocation and pass its concrete CIDR to downstream consumers that accept an explicit range. [AWS: Allocate CIDRs](https://docs.aws.amazon.com/vpc/latest/ipam/allocate-cidrs-ipam.html)

## 7. Terraform pain points observed in issue history

- Preview produces duplicate candidates under parallel evaluation. [#25958](https://github.com/hashicorp/terraform-provider-aws/issues/25958)
- CIDR deprovision can fail until integrated-resource allocations disappear; this drove configurable 32-minute deletion timeouts. [#30165](https://github.com/hashicorp/terraform-provider-aws/issues/30165)
- Reserving a CIDR separately and then asking VPC creation to allocate can produce capacity/allocation errors. [#39046](https://github.com/hashicorp/terraform-provider-aws/issues/39046)
- The plural pools data source has been reported to collapse multiple API results to one Terraform set entry. [#40314](https://github.com/hashicorp/terraform-provider-aws/issues/40314)
- IPAM-managed IPv6 VPC netmask state has produced perpetual diffs because computed semantics were missing. [#45844](https://github.com/hashicorp/terraform-provider-aws/issues/45844)
- Updating an IPAM-backed subnet has been reported to disassociate its computed IPv6 CIDR. [#46795](https://github.com/hashicorp/terraform-provider-aws/issues/46795)
- Subnet deletion historically returned before IPAM released its allocation, leaving 20–30-minute reuse failures; later provider work aligned it with VPC teardown. [#46324](https://github.com/hashicorp/terraform-provider-aws/issues/46324)
- Cross-account resource-planning pools initially attempted to read the VPC from the IPAM administrator account and failed; this was fixed by removing invalid local-account validation. [#46256](https://github.com/hashicorp/terraform-provider-aws/issues/46256), [fix PR #46483](https://github.com/hashicorp/terraform-provider-aws/pull/46483)
- Terraform support lags new AWS surface area: IPAM policy resource support remains requested, and external Infoblox authority configuration is not exposed by `aws_vpc_ipam_scope`. [#45160](https://github.com/hashicorp/terraform-provider-aws/issues/45160), [#50158](https://github.com/hashicorp/terraform-provider-aws/issues/50158)
- End-to-end public IPv4/BYOIP workflows have required resources outside the IPAM pool model, prompting a long-standing EC2 IPv4-pool feature request. [#33194](https://github.com/hashicorp/terraform-provider-aws/issues/33194)

## 8. Implications for a new Terraform IPAM provider

1. **Make allocation transactional.** Do not expose “next CIDR” as though it were a durable resource. Provide an atomic allocation resource with an idempotency key and authoritative allocation ID; if preview exists, label it explicitly non-reserving and plan-time advisory. [AWS allocation API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_AllocateIpamPoolCidr.html), [#25958](https://github.com/hashicorp/terraform-provider-aws/issues/25958)
2. **Separate address-space provisioning from consumption.** Model pool CIDRs, reservations, and resource allocations as different resource types with different ownership and deletion semantics. [AWS planning terminology](https://docs.aws.amazon.com/vpc/latest/ipam/planning-ipam.html)
3. **Encode immutable relationships correctly.** Parent pool, scope, address family, locale, public source, and external authority should be replacement-only unless the backend genuinely supports moving them. Never leave an argument mutable in schema when Update ignores it. [AWS CreateIpamPool API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_CreateIpamPool.html)
4. **Preserve requested-versus-observed state.** For server-selected CIDRs, store both requested netmask and allocated CIDR. Optional+Computed fields need explicit read semantics so refresh does not create perpetual diffs or accidentally delete service-computed associations. [#45844](https://github.com/hashicorp/terraform-provider-aws/issues/45844), [#46795](https://github.com/hashicorp/terraform-provider-aws/issues/46795)
5. **Treat release as an asynchronous state machine.** Delete should wait for allocation disappearance, use locale-aware clients, expose operation-specific timeouts, and distinguish manual release from resource-owned release. [AWS release API](https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ReleaseIpamPoolAllocation.html), [#30165](https://github.com/hashicorp/terraform-provider-aws/issues/30165)
6. **Make account and Region context first-class.** Every pool/allocation should carry owner, management Region, locale, and discovery authority. Never validate a cross-account resource using only the provider account’s direct lookup APIs. [AWS resource discovery](https://docs.aws.amazon.com/vpc/latest/ipam/res-disc-work-with.html), [#46256](https://github.com/hashicorp/terraform-provider-aws/issues/46256)
7. **Use stable collection identity.** Return pools/allocations as maps keyed by backend ID, or sets whose hash is only the immutable ID—not a hash of mutable/computed nested fields. [#40314](https://github.com/hashicorp/terraform-provider-aws/issues/40314)
8. **Model policy fallback explicitly.** AWS IPAM policies may silently fall back to Amazon addresses when a pool is inaccessible or exhausted. A new provider should expose enforcement/fallback status so “policy applied” is not confused with “address came from desired pool.” [AWS policy allocation behavior](https://docs.aws.amazon.com/vpc/latest/ipam/define-public-ipv4-allocation-strategy-with-ipam-policies.html)
9. **Design for extension points.** The 2024–2026 cadence added external authorities, policies, prefix-list resolvers, allocation tags, global-service pools, and RPKI objects. Prefer typed subresources and versioned capability discovery over a monolithic pool resource. [AWS IPAM policies](https://docs.aws.amazon.com/vpc/latest/ipam/ipam-policies.html), [AWS BGP protection](https://docs.aws.amazon.com/vpc/latest/ipam/monitor-bgp-route-security.html)
10. **Expose provenance and compliance separately from desired state.** Imported resources may be noncompliant, and policies can fall back. Read models should expose allocation source, compliance violations, import status, and authoritative owner rather than flattening these into one “managed” boolean. [AWS pool auto-import behavior](https://docs.aws.amazon.com/vpc/latest/ipam/create-top-ipam.html)

## CLAIMS
- CLAIM: Preview-next-CIDR is not a reservation and cannot safely coordinate parallel Terraform allocations. — RISK: high — SOURCES: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_AllocateIpamPoolCidr.html, https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_preview_next_cidr.go, https://github.com/hashicorp/terraform-provider-aws/issues/25958 — COUNTER: Terraform state makes a single preview appear stable until replacement, but does not protect it from external allocation. — PRIMARY: https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_preview_next_cidr.go
- CLAIM: Integrated resource deletion and IPAM allocation release are separate, eventually consistent transitions that can take 20–30 minutes. — RISK: high — SOURCES: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ReleaseIpamPoolAllocation.html, https://github.com/hashicorp/terraform-provider-aws/issues/30165, https://github.com/hashicorp/terraform-provider-aws/issues/46324 — COUNTER: Recent AWS provider versions include longer waits for several paths, but consumers still need asynchronous deletion semantics. — PRIMARY: https://docs.aws.amazon.com/AWSEC2/latest/APIReference/API_ReleaseIpamPoolAllocation.html
- CLAIM: AWS IPAM has evolved beyond pools and CIDRs into organization policies, external authority integration, prefix-list automation, allocation governance, global services, and BGP/RPKI protection. — RISK: normal — SOURCES: https://docs.aws.amazon.com/vpc/latest/ipam/ipam-policies.html, https://aws.amazon.com/about-aws/whats-new/2025/10/amazon-vpc-ipam-prefix-list-automation/, https://aws.amazon.com/about-aws/whats-new/2025/11/amazon-vpc-ipam-automates-ip-from-infoblox/, https://aws.amazon.com/about-aws/whats-new/2026/08/amazon-vpc-ipam-bgp-rpki-byoip/ — COUNTER: Some capabilities remain service- or tier-specific and should not all be forced into the provider’s initial core model. — PRIMARY: https://docs.aws.amazon.com/vpc/latest/ipam/ipam-policies.html
- CLAIM: Cross-account and cross-Region identity must be explicit in any new provider’s resource model. — RISK: high — SOURCES: https://docs.aws.amazon.com/vpc/latest/ipam/enable-integ-ipam.html, https://docs.aws.amazon.com/vpc/latest/ipam/share-pool-ipam.html, https://github.com/hashicorp/terraform-provider-aws/issues/46256 — COUNTER: A single-account provider can initially reduce scope, but its state IDs and APIs should not preclude future federation. — PRIMARY: https://docs.aws.amazon.com/vpc/latest/ipam/enable-integ-ipam.html
- CLAIM: Backend-selected address values require separate requested and observed schema fields to avoid perpetual drift and destructive updates. — RISK: high — SOURCES: https://github.com/hashicorp/terraform-provider-aws/issues/45844, https://github.com/hashicorp/terraform-provider-aws/issues/46795, https://github.com/hashicorp/terraform-provider-aws/blob/main/internal/service/ec2/vpc_ipam_pool_cidr.go — COUNTER: Careful Optional+Computed handling can work, but a split request/result schema is easier to reason about. — PRIMARY: https://github.com/hashicorp/terraform-provider-aws/issues/46795

## EXPAND
- LEAD: Compare AWS’s strong-consistency allocation contract with the target provider’s backend transaction and locking primitives. — WHY: This determines whether one allocation resource can guarantee non-overlap under concurrent applies. — ANGLE: idempotency keys, compare-and-swap, leases, and allocation ownership.
- LEAD: Build a lifecycle matrix for every proposed resource before implementation. — WHY: The AWS provider’s missing ForceNew/computed flags caused drift and destructive updates. — ANGLE: create/read/update/delete support, immutable identifiers, requested versus observed fields, import identity.
- LEAD: Design an explicit deletion/reconciliation protocol. — WHY: IPAM teardown is dominated by asynchronous release and discovery lag rather than the initial delete API call. — ANGLE: terminal states, bounded waiters, retryable errors, orphan detection, and safe timeout recovery.
- LEAD: Decide whether governance objects belong in the new provider’s first-class API. — WHY: AWS’s current direction is policy-based automation rather than only direct allocation. — ANGLE: policies, compliance findings, external authorities, resolver outputs, and fail-open versus fail-closed behavior.
- DEAD END: Treating preview-next-CIDR as a plan-time allocator or producing several previews with `count`; it neither reserves space nor guarantees distinct results.