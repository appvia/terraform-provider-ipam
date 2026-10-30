// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	acc "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/appvia/ipam"
)

const testAccPoolDataSource = `
resource "ipam_pool" "parent" {
  id          = "parent"
  cidrs       = ["10.0.0.0/16", "10.2.0.0/16"]
  description = "platform"
  labels      = { env = "prod" }
}

resource "ipam_pool" "child" {
  id            = "child"
  parent_pool   = ipam_pool.parent.id
  prefix_length = 20
}

data "ipam_pool" "parent" {
  id = ipam_pool.parent.id
  depends_on = [ipam_pool.child]
}

data "ipam_pool" "child" {
  id = ipam_pool.child.id
}
`

func testAccCheckCarveOut(s *terraform.State) error {
	child, err := stateSet(s, "ipam_pool.child", "cidrs")
	if err != nil {
		return err
	}

	attrs := s.RootModule().Resources["data.ipam_pool.parent"].Primary.Attributes
	for key, value := range attrs {
		if strings.HasPrefix(key, "allocations.pool:child:") {
			if value != child[0] {
				return fmt.Errorf("carve-out %s = %s, want the child cidr %s", key, value, child[0])
			}

			return nil
		}
	}

	return fmt.Errorf("no pool:child carve-out in allocations: %v", attrs)
}

func TestAccPoolDataSource(t *testing.T) {
	dir := testAccBackend(t)
	allocationID := ipam.NewAllocationID()

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{Config: testAccConfig(dir, testAccPoolDataSource)},
			{
				PreConfig: func() {
					_, err := testAccClient(t, dir).Allocate(context.Background(), "parent",
						ipam.AllocationSpec{ID: allocationID, PrefixLength: 24})
					if err != nil {
						t.Fatalf("Allocate: %v", err)
					}
				},
				Config: testAccConfig(dir, testAccPoolDataSource),
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckResourceAttr("data.ipam_pool.parent", "id", "parent"),
					acc.TestCheckResourceAttr("data.ipam_pool.parent", "family", "4"),
					acc.TestCheckResourceAttr("data.ipam_pool.parent", "cidrs.#", "2"),
					acc.TestCheckTypeSetElemAttr("data.ipam_pool.parent", "cidrs.*", "10.0.0.0/16"),
					acc.TestCheckTypeSetElemAttr("data.ipam_pool.parent", "cidrs.*", "10.2.0.0/16"),
					acc.TestCheckResourceAttr("data.ipam_pool.parent", "description", "platform"),
					acc.TestCheckResourceAttr("data.ipam_pool.parent", "labels.env", "prod"),
					acc.TestCheckNoResourceAttr("data.ipam_pool.parent", "parent_pool"),
					acc.TestCheckResourceAttr("data.ipam_pool.parent", "allocations.%", "2"),
					acc.TestCheckResourceAttrSet("data.ipam_pool.parent", "allocations."+allocationID),
					testAccCheckCarveOut,
					acc.TestCheckResourceAttr("data.ipam_pool.child", "parent_pool", "parent"),
					acc.TestCheckResourceAttr("data.ipam_pool.child", "allocations.%", "0"),
					acc.TestCheckResourceAttrPair("data.ipam_pool.child", "cidrs.0", "ipam_pool.child", "cidrs.0"),
				),
			},
			{
				PreConfig: func() {
					if err := testAccClient(t, dir).Release(context.Background(), "parent", allocationID); err != nil {
						t.Fatalf("Release: %v", err)
					}
				},
				Config: testAccConfig(dir, testAccPoolDataSource),
				Check:  acc.TestCheckResourceAttr("data.ipam_pool.parent", "allocations.%", "1"),
			},
		},
	})
}

func TestAccPoolDataSourceMissing(t *testing.T) {
	dir := testAccBackend(t)

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		Steps: []acc.TestStep{
			{
				Config:      testAccConfig(dir, `data "ipam_pool" "x" { id = "missing" }`),
				ExpectError: regexp.MustCompile(`Read pool: not found`),
			},
		},
	})
}
