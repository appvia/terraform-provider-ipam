data "ipam_pool" "uks" {
  id = "uks"
}

output "uks_cidrs" {
  value = data.ipam_pool.uks.cidrs
}

# Allocation ID to CIDR, including child-pool carve-outs.
output "uks_allocations" {
  value = data.ipam_pool.uks.allocations
}
