# ULW-Research Synthesis: Cloud IPAM prior art for a Terraform provider
Members + lanes: 8 (6 axis lanes, 1 expansion, 1 skeptic) · Waves: 2 · Excursions: 0 · Sources: 397 URLs (23 domains) · Verifications: 1 (verify-avnm-multi-prefix.md) · Debate rounds: 9 claims + framing
See report.html for the full cited synthesis; claim-graph.md and debate-log.md for verdicts.
## Executive summary
- Azure/ipam: self-hosted Space→Block→Reservation; reservations reconciled via X-IPAM-RES-ID; existing XtratusCloud/azureipam provider lacks create idempotency and breaks on v4 Content-Type.
- AVNM IPAM: first-party ARM pools (≤7 levels), count-based requests returning prefix lists; azurerm 4.23/4.32/4.38/4.41; regional GA first verified 2025-06-12.
- AWS VPC IPAM: IPAM→scope→pool→pool CIDR→allocation; strongly consistent allocation within one pool; preview_next_cidr is the anti-pattern.
- Provider shape: allocation as apply-time managed write; separate request/result fields; separate capacity/allocation/binding resources; realm-scoped uniqueness; idempotent create with allocation_key; no backend-agnostic abstraction.
## Gaps
- AVNM multi-prefix decomposition unverified (C3b); Xtratus v2 vs v4 not live-tested; IBM/DO absence claims single-sourced.
