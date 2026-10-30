// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"regexp"
	"testing"

	acc "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	"github.com/appvia/ipam"
)

const testAccAllocationDataSource = `
resource "ipam_pool" "root" {
  id    = "root"
  cidrs = ["10.0.0.0/16"]
}

resource "ipam_allocation" "next" {
  pool          = ipam_pool.root.id
  prefix_length = 24
  description   = "app"
  labels        = { team = "net" }
}

resource "ipam_allocation" "fixed" {
  pool = ipam_pool.root.id
  cidr = "10.0.128.0/20"
}

data "ipam_allocation" "next" {
  pool = ipam_allocation.next.pool
  id   = ipam_allocation.next.id
}

data "ipam_allocation" "fixed" {
  pool = ipam_allocation.fixed.pool
  id   = ipam_allocation.fixed.id
}
`

func TestAccAllocationDataSource(t *testing.T) {
	dir := testAccBackend(t)

	pair := func(attr string) acc.TestCheckFunc {
		return acc.TestCheckResourceAttrPair("data.ipam_allocation.next", attr, "ipam_allocation.next", attr)
	}

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{
				Config: testAccConfig(dir, testAccAllocationDataSource),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("data.ipam_allocation.fixed", tfjsonpath.New("labels"),
						knownvalue.MapExact(map[string]knownvalue.Check{})),
					statecheck.ExpectKnownValue("ipam_allocation.fixed", tfjsonpath.New("labels"), knownvalue.Null()),
				},
				Check: acc.ComposeAggregateTestCheckFunc(
					pair("id"), pair("pool"), pair("cidr"), pair("prefix_length"), pair("description"),
					pair("created_at"), pair("labels.%"), pair("labels.team"),
					acc.TestCheckResourceAttr("data.ipam_allocation.next", "cidr", "10.0.0.0/24"),
					acc.TestCheckResourceAttr("data.ipam_allocation.next", "labels.team", "net"),
					acc.TestCheckResourceAttr("data.ipam_allocation.fixed", "cidr", "10.0.128.0/20"),
					acc.TestCheckResourceAttr("data.ipam_allocation.fixed", "prefix_length", "20"),
					acc.TestCheckResourceAttr("data.ipam_allocation.fixed", "description", ""),
					acc.TestCheckResourceAttr("data.ipam_allocation.fixed", "labels.%", "0"),
					acc.TestCheckResourceAttrPair("data.ipam_allocation.fixed", "created_at",
						"ipam_allocation.fixed", "created_at"),
				),
			},
		},
	})
}

func TestAccAllocationDataSourceErrors(t *testing.T) {
	dir := testAccBackend(t)
	testAccPool(t, dir, "root", "10.0.0.0/16")

	lookup := func(pool, id string) string {
		return testAccConfig(dir, fmt.Sprintf("data \"ipam_allocation\" \"x\" {\n  pool = %q\n  id   = %q\n}\n", pool, id))
	}

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		Steps: []acc.TestStep{
			{
				Config:      lookup("root", ipam.NewAllocationID()),
				ExpectError: regexp.MustCompile(`Read allocation: not found`),
			},
			{
				Config:      lookup("missing", ipam.NewAllocationID()),
				ExpectError: regexp.MustCompile(`(?s)Read allocation: not found.*pool missing`),
			},
			{
				Config:      lookup("root", "not-a-uuid"),
				ExpectError: regexp.MustCompile(`Read allocation: invalid argument`),
			},
		},
	})
}
