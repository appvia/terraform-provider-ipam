variable "allocation_id" {
  type = string
}

data "ipam_allocation" "vnet" {
  pool = "uks"
  id   = var.allocation_id
}

output "vnet_cidr" {
  value = data.ipam_allocation.vnet.cidr
}
