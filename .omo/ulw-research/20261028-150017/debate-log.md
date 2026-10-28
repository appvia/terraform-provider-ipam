# Debate log
| claim | attack | defense | verdict | graph change |
|---|---|---|---|---|
| AVNM returns multi-prefix for non-power-of-two counts (a2 lane) | Lead (self-attack via primary fetch): sole evidence is a synthetic REST example whose other values are inconsistent (65536 req vs 800 resp vs /22) | Schema types allocatedAddressPrefixes as array | PARTIAL: list-typing confirmed, split behavior unproven | C-avnm-multiprefix -> partial |
| C1 allocation must be atomic resource Create | static-ipam/cidrsubnet state-only allocators; consumer Create (aws_vpc, azurerm ip_address_pool) also allocates | AWS preview source + #25958; Azure/ipam external example mutates on read | WEAKENED: "apply-time managed write (allocation or consumer Create)"; state-only is separate topology | C1 -> supported (reworded) |
| C2 AWS consistency + 32-min timeout | 32-min timeout belongs to aws_vpc_ipam_pool_cidr deprovision, not manual release; 20-30 min is observed not SLA | AWS API docs | WEAKENED | C2 -> supported (reworded) |
| C3 AVNM count->prefix list, azurerm versions | multi-prefix only in inconsistent example; decrease-rejection is provider logic | release tags confirmed | PARTIAL on multi-prefix; versions supported | C3 -> supported, C3b -> unresolved |
| C4 AVNM GA June 25 2025, not successor | Docs git history shows regional GA on June 12 2025; maintainer comment author marked CONTRIBUTOR | Q&A Jan 2025 preview | REFUTED date -> June 12 2025 regional GA; successor claim bounded | C4 -> supported (corrected) |
| C5 X-IPAM-RES-ID reconciliation | 1-min is scheduler cadence not SLA; also vHubs, lowercase tag | pinned source | STANDS w/ precision | C5 supported |
| C6 two topologies only | state-only, composite (Crossplane/Pulumi/modules), OpenStack subnet pools | — | REFUTED as exhaustive | C6 -> partial |
| C7 Optional+Computed caused AWS issues | #45844 is missing Computed; #46795 is update-branch bug | AzureRM VNet split works | WEAKENED: split when concepts differ | C7 -> partial |
| C8 routing-domain scope | NetBox VRF allows dupes by default; Azure/ipam spaces not routing domains; OpenStack address scopes | AWS scopes | WEAKENED: backend-native collision domain + overlap policy | C8 -> supported (reworded) |
| C9 GCP strategies | IPv4 auto only; default strategy doc tension | REST API | STANDS scoped | C9 supported |
| Framing: backend-agnostic provider | contracts incompatible on request/result/identity/release/scope | — | Accepted: abstraction belongs in modules over backend-specific resources | rec R1 |
