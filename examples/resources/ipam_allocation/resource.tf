# The next free /24 in a pool.
resource "ipam_allocation" "vnet" {
  pool          = "uks"
  prefix_length = 24
  description   = "Hub VNet"
  labels = {
    team = "network"
  }
}

# Exactly this CIDR (a reservation).
resource "ipam_allocation" "gateway" {
  pool = "uks"
  cidr = "10.1.255.0/24"
}

# Several allocations with count.
resource "ipam_allocation" "subnet" {
  count = 3

  pool          = "uks"
  prefix_length = 26
  description   = "subnet ${count.index}"
}

# One allocation per app with for_each.
resource "ipam_allocation" "app" {
  for_each = toset(["api", "web"])

  pool          = "uks"
  prefix_length = 24
  labels = {
    app = each.key
  }
}

# IPv6.
resource "ipam_allocation" "v6" {
  pool          = "platform-v6"
  prefix_length = 48
}

output "vnet_cidr" {
  value = ipam_allocation.vnet.cidr
}
