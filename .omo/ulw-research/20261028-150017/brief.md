# Brief: Cloud IPAM prior art for a Terraform provider
<analysis>
Core question: What do Azure/ipam (OSS), Azure Virtual Network Manager IPAM, and AWS VPC IPAM (plus adjacent prior art) model, and how should those models shape the resources/data sources/schemas of a new Terraform IPAM provider?
Axes:
A1 azure-ipam-oss — github.com/Azure/ipam: data model (spaces, blocks, vnets, external networks, reservations), REST API, deployment, terraform story.
A2 avnm-ipam — Microsoft.Network/networkManagers/ipamPools, staticCidrs, verifierWorkspaces, VNet ipamPoolPrefixAllocations; azurerm/azapi resources.
A3 aws-vpc-ipam — aws_vpc_ipam, scopes, pools, cidrs, allocations, resource discovery, preview_next_cidr, RAM/Orgs, tiers, BYOIP; terraform-provider-aws schemas.
A4 gcp-and-others — GCP Internal Ranges (networkconnectivity), OCI/other cloud equivalents.
A5 tf-ipam-providers — existing providers: NetBox, Infoblox, phpIPAM, BlueCat, hashicorp/subnets/cidr modules; allocation-as-resource patterns (next-available).
A6 tf-design-patterns — plugin framework features relevant to allocation: concurrency/locking, import, ephemeral resources, list resources, write-only, drift, deterministic allocation.
A7 skeptic — attack framing and claims.
Codebase relevant: no (empty repo) · External: yes · Collection: browser · Verification likely: partial (schema existence via provider docs/source) · X/social signal: no
Scale: 6 axes, ~1 inline report · Precision: design decisions — moderate cost of a wrong claim
Debate need: "AVNM IPAM supersedes Azure/ipam", "AWS allocation semantics", "provider should own allocation vs delegate"
</analysis>
Runtime note: team_create is not available in this harness; axes run as background task lanes (fallback recorded per skill failure table).
Shortened run: protocol instrumentation (fact ledger/report-tools gates) reduced; claims cited inline by URL.

## Deliverable
lane: no-format (answered_by: default) · formats: inline html (answered_by: default) · destination: chat (answered_by: default) · audience: provider designers (answered_by: request)

## Fidelity
as of: latest at run start, 2026-10-28 (answered_by: default)
meaning check: off (answered_by: default)
source floor: standard (answered_by: default)
derived numbers: allowed (answered_by: default)
languages: report en; sources en; quotes original (answered_by: default)
identifier classes: resource type names, API versions, attribute names (answered_by: default)
fallback: block-quote (answered_by: default)
asked: none
