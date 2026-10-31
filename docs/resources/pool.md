---
page_title: "ipam_pool Resource - ipam"
subcategory: ""
description: |-
  A pool of IP ranges. A root pool lists its cidrs; a child pool is carved from parent_pool with either prefix_length or an explicit cidr. Only description, labels, strategy and cooldown update in place; every other change replaces the pool. A pool that still holds allocations or child pools cannot be destroyed.
---

# ipam_pool (Resource)

A pool of IP ranges. A root pool lists its `cidrs`; a child pool is carved from `parent_pool` with either `prefix_length` or an explicit `cidr`. Only `description`, `labels`, `strategy` and `cooldown` update in place; every other change replaces the pool. A pool that still holds allocations or child pools cannot be destroyed.

## Changes and replacement

`description`, `labels`, `strategy` and `cooldown` update in place. Changing `id`, `cidrs`, `parent_pool`, `prefix_length` or `cidr` replaces the pool.

## Allocation strategy and cooldown

`strategy` decides where a `next` request (an allocation or child pool with `prefix_length`) lands:

- `best-fit` uses the smallest free block that fits, lowest address first on a tie. It keeps large blocks whole for large requests.
- `lowest` uses the lowest free address that fits. It's predictable, but released holes get split up by requests of other sizes.

New pools default to `best-fit`. An existing or adopted pool keeps its stored strategy while `strategy` is omitted, and removing `strategy` from the configuration keeps the pool's current strategy. A pool re-created by replacement gets `best-fit` unless `strategy` is set. A strategy change affects only later allocations; existing allocations don't move.

`cooldown` holds released CIDRs back from `next` requests for `duration`, so a stale reference such as a DNS record or firewall rule doesn't reach a new owner straight away. The duration is a Go duration in whole seconds between `1s` and `8760h`, for example `24h`. There are two modes:

- `strict` never hands out cooling space to a `next` request, so a request can be refused while the pool still has free space. The refusal fails with `pool exhausted`, reason `cooldown`, and names the time when the cooling space becomes usable. A child pool carved with `prefix_length` from a strict-cooldown parent can be refused the same way.
- `fit-aware` avoids cooling space only when an equally tight free block exists. It costs nothing in fragmentation, but when the only good fit is cooling, it's reused.

Explicit `cidr` requests, for allocations and child pools, bypass cooldown in both modes. Removing `cooldown` from the configuration turns cooldown off and drops the pool's release records.

### Upgrading other ipam clients

A pool with `best-fit` or a cooldown is stored as a v2 document. The ipam CLI and library before v0.3.0 and terraform-provider-ipam v0.1.x can't read it, and an old client that deletes a child of a v2 parent leaks the child's carve-out in the parent. Reclaim leaked space with a current CLI:

```text
ipam alloc release <parent> pool:<child>:<incarnation>
```

Upgrade every client that uses the bucket first. Until then, set `strategy = "lowest"` explicitly and leave out `cooldown` to keep v1 documents.

## Destroying and replacing a parent pool

A pool that still holds allocations or child pools can't be deleted. Destroy fails with `pool_not_empty`.

Terraform destroys dependants before their parent only when they're replaced too. If you replace a parent pool (for example by changing its `cidrs`) while its child pools stay unchanged, the parent's delete fails with `pool_not_empty`. Add `replace_triggered_by` to each child so it's replaced with its parent:

```terraform
resource "ipam_pool" "uks" {
  id          = "uks"
  parent_pool = ipam_pool.root.id
  cidr        = "10.1.0.0/16"

  lifecycle {
    replace_triggered_by = [ipam_pool.root.cidrs]
  }
}
```

Reference the parent as `ipam_pool.root.id` rather than a literal, or add `depends_on = [ipam_pool.root]`, so Terraform knows the order. Allocations in other states have to be released before the platform state can replace the pool.

## Existing pools

Create adopts a pool that already exists with an identical definition (same CIDRs, or same parent and request, description and labels) instead of failing. Two workspaces that declare the same pool therefore both own it, and either one's destroy deletes it. Keep each pool in exactly one state. A different pool with the same ID fails with `pool_exists`; import it or choose another ID.

For adoption, an omitted `strategy` matches any stored strategy, and an omitted `cooldown` matches only a pool without one. Any other policy difference fails with `pool_exists`: configure the same cooldown, or import the pool.

If an earlier create of the same pool was interrupted, Create waits out `access_propagation_timeout` and then fails with `timed out`. To recover with the CLI:

1. `ipam pool get <id>` reports such a partially created pool as not found. For a child pool, the leftover shows in `ipam alloc list <parent>` as a `pool:<child>:<incarnation>` entry.
2. Run `ipam pool delete <id>` to clear the pool's own documents. When the pool's reservations document is the missing one, this also releases the parent's reservation.
3. If `ipam alloc list <parent>` still lists the `pool:<child>:<incarnation>` entry, release it by its full ID with `ipam alloc release <parent> pool:<child>:<incarnation>`, but only once no create of that child is running anywhere.
4. Apply again.

## Example Usage

```terraform
# Copyright 2026 Appvia Ltd.
# SPDX-License-Identifier: MPL-2.0

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

  # Pack allocations tightly and hold released CIDRs back for a day.
  strategy = "best-fit"
  cooldown = {
    mode     = "strict"
    duration = "24h"
  }

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
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `id` (String) Pool ID: 1-63 lowercase letters, digits and hyphens.

### Optional

- `cidr` (String) Carve exactly this canonical CIDR from `parent_pool`.
- `cidrs` (Set of String) Canonical CIDRs of a root pool (one address family, no overlaps). For a child pool this is computed: the single CIDR carved from the parent.
- `cooldown` (Attributes) Holds released CIDRs back from `next` requests. Omit for no cooldown. Updated in place. (see [below for nested schema](#nestedatt--cooldown))
- `description` (String) Free-text description. Updated in place.
- `labels` (Map of String) Free-form labels. Updated in place.
- `parent_pool` (String) ID of the pool to carve this child pool from.
- `prefix_length` (Number) Carve the next free CIDR of this prefix length from `parent_pool`.
- `strategy` (String) Allocation strategy for `next` requests: `best-fit` (smallest free block that fits) or `lowest` (lowest free address). Defaults to `best-fit` for new pools; an existing or adopted pool keeps its stored strategy while `strategy` is omitted, and a pool re-created by replacement gets `best-fit` unless `strategy` is set. Updated in place. A pool with `best-fit` or a cooldown is stored as a v2 document that the ipam CLI and library before v0.3.0 and terraform-provider-ipam v0.1.x cannot read.

### Read-Only

- `family` (Number) Address family: 4 or 6.

<a id="nestedatt--cooldown"></a>
### Nested Schema for `cooldown`

Required:

- `duration` (String) How long a released CIDR cools, as a Go duration in whole seconds between 1s and 8760h, for example `24h`.
- `mode` (String) `strict` refuses a `next` request while only cooling space fits it; `fit-aware` avoids cooling space only when an equally tight free block exists. Explicit `cidr` requests bypass cooldown.

## Import

Import a pool by its ID.

```shell
# Copyright 2026 Appvia Ltd.
# SPDX-License-Identifier: MPL-2.0

# Import a pool by its ID.
terraform import ipam_pool.root platform
```

Import records the cooldown `duration` in Go canonical form, such as `24h0m0s`. If the configuration writes it differently (`24h`), the first plan after import shows an in-place update that changes nothing in the store.

With Terraform 1.12 or later you can also import by identity:

```terraform
# Copyright 2026 Appvia Ltd.
# SPDX-License-Identifier: MPL-2.0

import {
  to = ipam_pool.root
  identity = {
    id = "platform"
  }
}
```
