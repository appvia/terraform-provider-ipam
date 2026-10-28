# Claim graph
## verified-claims
- C1: Authoritative remote allocation must be an apply-time managed write (allocation-resource or consumer Create); preview/data-source next-available is advisory.
- C2: AWS AllocateIpamPoolCidr guarantees non-overlap within one pool; listing/release eventually consistent; 32-min timeout is pool-CIDR deprovision.
- C3: AVNM consumers request a decimal-string count and receive allocatedAddressPrefixes[]; azurerm 4.23/4.32/4.38/4.41.
- C4: AVNM IPAM regional GA first verified June 12 2025; no formal successor statement for Azure/ipam.
- C5: Azure/ipam reconciles via X-IPAM-RES-ID tag on a 1-min schedule; deleting a fulfilled reservation is a no-op.
- C8: Uniqueness should be scoped by a backend-native collision domain plus explicit overlap policy.
- C9: GCP auto IPv4 internal ranges expose 4 allocation strategies.
- C10: XtratusCloud/azureipam v2.0.0 (Plugin Framework, 6 res/10 ds) has no create idempotency; 10s timeout; missing Content-Type breaks v4 (#16, #25).
- C11: Azure/ipam v4 PR 377 adds 4 routes, removes none, nullable reservation ids for redacted rows, 409 on conflict, 503 service mode, still IPv4-only.

| claim_id | statement | risk tier | status | fact_ids | sources |
|---|---|---|---|---|---|
| C1 | apply-time managed write for allocation | high | supported | - | AWS API, aws preview source, #25958, Azure/ipam example |
| C2 | AWS consistency scope + timeout attribution | high | supported | - | AWS API docs, aws provider source |
| C3 | AVNM count->prefix list; azurerm versions | normal | supported | - | azurerm source/releases, ARM ref |
| C3b | AVNM splits non-power-of-two counts into multiple prefixes | high | unresolved | - | inconsistent REST example only |
| C4 | AVNM regional GA June 12 2025 | high | supported | - | MicrosoftDocs git history, Q&A |
| C5 | Azure/ipam tag reconciliation | normal | supported | - | pinned source |
| C6 | two topologies exhaustive | normal | refuted | - | skeptic counterexamples |
| C7 | Optional+Computed caused AWS bugs | normal | refuted | - | #45844 #46795 |
| C8 | collision-domain scope | normal | supported | - | AWS, NetBox, GCP, OpenStack docs |
| C9 | GCP strategies | normal | supported | - | GCP REST |
| C10 | Xtratus provider weaknesses | high | supported | - | pinned source, issues |
| C11 | Azure/ipam v4 deltas | high | supported | - | PR 377 head source |
