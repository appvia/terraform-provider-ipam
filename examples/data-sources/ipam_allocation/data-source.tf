# Copyright 2026 Appvia Ltd.
# SPDX-License-Identifier: Apache-2.0

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
