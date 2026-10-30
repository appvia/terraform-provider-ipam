# A root pool lists its CIDRs.
resource "ipam_pool" "root" {
  id          = "platform"
  cidrs       = ["10.0.0.0/8"]
  description = "All private IPv4 space"
  labels = {
    owner = "platform"
  }
}

# A child pool carved at an explicit CIDR.
resource "ipam_pool" "uks" {
  id          = "uks"
  parent_pool = ipam_pool.root.id
  cidr        = "10.1.0.0/16"

  # Replacing the parent (for example changing its cidrs) fails with
  # pool_not_empty while this child still exists. replace_triggered_by
  # replaces the child too, and Terraform destroys it first.
  lifecycle {
    replace_triggered_by = [ipam_pool.root.cidrs]
  }
}

# Child pools carved as the next free /20, one per environment.
resource "ipam_pool" "env" {
  for_each = toset(["dev", "test"])

  id            = "uks-${each.key}"
  parent_pool   = ipam_pool.uks.id
  prefix_length = 20
  labels = {
    environment = each.key
  }

  lifecycle {
    replace_triggered_by = [ipam_pool.uks.cidrs]
  }
}

# An IPv6 root pool.
resource "ipam_pool" "v6" {
  id    = "platform-v6"
  cidrs = ["2001:db8::/32"]
}
