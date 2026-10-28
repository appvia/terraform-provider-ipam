# Verify: AVNM allocation of 80 addresses returns /26 + /28 (multi-prefix)
Source checked: sources/S-avnm-vnet-ipam-example.json (azure-rest-api-specs main, examples/2026-03-01/VirtualNetworkCreateWithIpamPool.json), fetched 2026-10-28T15:07:42.167Z
Observation: subnet response allocatedAddressPrefixes = ["10.0.0.0/26","10.0.0.64/28"] for numberOfIpAddresses "80" — present.
But the same example is internally inconsistent: VNet request numberOfIpAddresses "65536", response echoes "800" with allocatedAddressPrefixes ["10.0.0.0/22"] (1024 addresses); request subnet name test-1 vs response subnet1; request location eastus vs response eastus2euap.
Learn how-to only shows round counts (256) and says "allocates a CIDR".
VERDICT: PARTIAL. allocatedAddressPrefixes is schema-typed as a list (confirmed), so a provider must model it as a list. Whether the service actually splits non-power-of-two counts into multiple prefixes is NOT proven — the only evidence is a hand-written, inconsistent spec example. Needs a live API test to confirm.
