// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/compare"
	acc "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	"github.com/appvia/ipam"
	"github.com/appvia/ipam/storage"
	"github.com/appvia/ipam/storage/local"
)

// testAccPool creates a pool outside Terraform, so allocation tests can check
// the pool's allocations after the configuration is destroyed.
func testAccPool(t *testing.T, dir, id string, cidrs ...string) {
	t.Helper()

	spec := ipam.PoolSpec{ID: id}
	for _, cidr := range cidrs {
		spec.CIDRs = append(spec.CIDRs, netip.MustParsePrefix(cidr))
	}

	if _, err := testAccClient(t, dir).CreatePool(context.Background(), spec); err != nil {
		t.Fatalf("CreatePool %s: %v", id, err)
	}
}

func testAccAllocations(t *testing.T, dir, pool string) []ipam.Allocation {
	t.Helper()

	allocations, err := testAccClient(t, dir).ListAllocations(context.Background(), pool)
	if err != nil {
		t.Fatalf("ListAllocations %s: %v", pool, err)
	}

	return allocations
}

// testAccCheckReleased asserts that destroy released every allocation.
func testAccCheckReleased(t *testing.T, dir string, pools ...string) func(*terraform.State) error {
	return func(*terraform.State) error {
		for _, pool := range pools {
			if allocations := testAccAllocations(t, dir, pool); len(allocations) != 0 {
				return fmt.Errorf("pool %s still has %d allocations after destroy: %+v", pool, len(allocations), allocations)
			}
		}

		return nil
	}
}

func testAccAllocationImportID(name string) acc.ImportStateIdFunc {
	return func(s *terraform.State) (string, error) {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return "", fmt.Errorf("%s not in state", name)
		}

		return rs.Primary.Attributes["pool"] + "/" + rs.Primary.Attributes["id"], nil
	}
}

// testAccCheckStored compares a resource in state with the stored allocation.
func testAccCheckStored(t *testing.T, dir, name string) acc.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("%s not in state", name)
		}

		attrs := rs.Primary.Attributes

		parsed, err := uuid.Parse(attrs["id"])
		if err != nil || parsed.Version() != 7 || parsed.String() != attrs["id"] {
			return fmt.Errorf("%s id %q is not a canonical UUIDv7 (err=%v)", name, attrs["id"], err)
		}

		stored, err := testAccClient(t, dir).GetAllocation(context.Background(), attrs["pool"], attrs["id"])
		if err != nil {
			return fmt.Errorf("%s: stored allocation: %w", name, err)
		}

		if stored.CIDR.String() != attrs["cidr"] {
			return fmt.Errorf("%s cidr %s, stored %s", name, attrs["cidr"], stored.CIDR)
		}

		if stored.Description != attrs["description"] {
			return fmt.Errorf("%s description %q, stored %q", name, attrs["description"], stored.Description)
		}

		if got := stored.CreatedAt.Format(time.RFC3339Nano); got != attrs["created_at"] {
			return fmt.Errorf("%s created_at %q, stored %q", name, attrs["created_at"], got)
		}

		labelCount := attrs["labels.%"]
		if labelCount == "" {
			labelCount = "0"
		}

		if labelCount != fmt.Sprint(len(stored.Labels)) {
			return fmt.Errorf("%s has %s labels, stored %v", name, labelCount, stored.Labels)
		}

		for key, value := range stored.Labels {
			if attrs["labels."+key] != value {
				return fmt.Errorf("%s label %s = %q, stored %q", name, key, attrs["labels."+key], value)
			}
		}

		return nil
	}
}

const testAccAllocationBasic = `
resource "ipam_allocation" "a" {
  pool          = "root"
  prefix_length = 24
  description   = "app"
  labels        = { team = "net" }
}
`

func TestAccAllocationResourceBasic(t *testing.T) {
	dir := testAccBackend(t)
	testAccPool(t, dir, "root", "10.0.0.0/16")

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckReleased(t, dir, "root"),
		Steps: []acc.TestStep{
			{
				Config: testAccConfig(dir, testAccAllocationBasic),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipam_allocation.a", plancheck.ResourceActionCreate),
						plancheck.ExpectUnknownValue("ipam_allocation.a", tfjsonpath.New("id")),
						plancheck.ExpectUnknownValue("ipam_allocation.a", tfjsonpath.New("cidr")),
						plancheck.ExpectUnknownValue("ipam_allocation.a", tfjsonpath.New("created_at")),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckResourceAttr("ipam_allocation.a", "pool", "root"),
					acc.TestCheckResourceAttr("ipam_allocation.a", "prefix_length", "24"),
					acc.TestCheckResourceAttr("ipam_allocation.a", "cidr", "10.0.0.0/24"),
					acc.TestCheckResourceAttr("ipam_allocation.a", "description", "app"),
					acc.TestCheckResourceAttr("ipam_allocation.a", "labels.%", "1"),
					acc.TestCheckResourceAttr("ipam_allocation.a", "labels.team", "net"),
					testAccCheckStored(t, dir, "ipam_allocation.a"),
					func(*terraform.State) error {
						if n := len(testAccAllocations(t, dir, "root")); n != 1 {
							return fmt.Errorf("pool has %d allocations, want 1", n)
						}

						return nil
					},
				),
			},
			{
				ResourceName:                         "ipam_allocation.a",
				ImportState:                          true,
				ImportStateIdFunc:                    testAccAllocationImportID("ipam_allocation.a"),
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "id",
			},
			{
				ResourceName:    "ipam_allocation.a",
				ImportState:     true,
				ImportStateKind: acc.ImportBlockWithResourceIdentity,
			},
		},
	})
}

const testAccAllocationMany = `
resource "ipam_allocation" "many" {
  count         = 20
  pool          = "root"
  prefix_length = 24
}

resource "ipam_allocation" "small" {
  count         = 5
  pool          = "root"
  prefix_length = 26
  labels        = { key = "k${count.index}" }
}
`

// disjointAllocations asserts every ipam_allocation in state has a distinct
// id and a CIDR that overlaps no other, and that state matches the store. It
// reads the JSON state directly.
type disjointAllocations struct {
	t    *testing.T
	dir  string
	want int
}

func testAccCheckDisjoint(t *testing.T, dir string, want int) statecheck.StateCheck {
	return disjointAllocations{t: t, dir: dir, want: want}
}

func (c disjointAllocations) CheckState(_ context.Context, req statecheck.CheckStateRequest, resp *statecheck.CheckStateResponse) {
	resp.Error = c.check(req.State)
}

func (c disjointAllocations) check(state *tfjson.State) error {
	if state == nil || state.Values == nil || state.Values.RootModule == nil {
		return errors.New("no state")
	}

	ids := map[string]string{}

	var prefixes []netip.Prefix

	for _, rs := range state.Values.RootModule.Resources {
		if rs.Type != "ipam_allocation" || rs.Mode != tfjson.ManagedResourceMode {
			continue
		}

		id, _ := rs.AttributeValues["id"].(string)
		if other, dup := ids[id]; dup {
			return fmt.Errorf("%s and %s share id %s", rs.Address, other, id)
		}

		ids[id] = rs.Address

		cidr, _ := rs.AttributeValues["cidr"].(string)

		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return fmt.Errorf("%s cidr: %w", rs.Address, err)
		}

		for _, other := range prefixes {
			if prefix.Overlaps(other) {
				return fmt.Errorf("%s cidr %s overlaps %s", rs.Address, prefix, other)
			}
		}

		prefixes = append(prefixes, prefix)
	}

	if len(ids) != c.want {
		return fmt.Errorf("%d allocations in state, want %d", len(ids), c.want)
	}

	stored := testAccAllocations(c.t, c.dir, "root")
	if len(stored) != c.want {
		return fmt.Errorf("%d allocations stored, want %d", len(stored), c.want)
	}

	for _, allocation := range stored {
		if _, ok := ids[allocation.ID]; !ok {
			return fmt.Errorf("stored allocation %s (%s) is not in state", allocation.ID, allocation.CIDR)
		}
	}

	return nil
}

func TestAccAllocationResourceConcurrent(t *testing.T) {
	dir := testAccBackend(t)
	testAccPool(t, dir, "root", "10.0.0.0/16")

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckReleased(t, dir, "root"),
		Steps: []acc.TestStep{
			{
				Config: testAccConfig(dir, testAccAllocationMany),
				ConfigStateChecks: []statecheck.StateCheck{
					testAccCheckDisjoint(t, dir, 25),
					statecheck.ExpectKnownValue("ipam_allocation.small[2]", tfjsonpath.New("labels"),
						knownvalue.MapExact(map[string]knownvalue.Check{"key": knownvalue.StringExact("k2")})),
					statecheck.ExpectKnownValue("ipam_allocation.small[2]", tfjsonpath.New("prefix_length"),
						knownvalue.Int64Exact(26)),
				},
			},
		},
	})
}

const testAccAllocationExplicit = `
resource "ipam_allocation" "fixed" {
  pool = "root"
  cidr = %q
}

resource "ipam_allocation" "next" {
  pool          = "root"
  prefix_length = 24
  depends_on    = [ipam_allocation.fixed]
}
`

func TestAccAllocationResourceExplicit(t *testing.T) {
	dir := testAccBackend(t)
	testAccPool(t, dir, "root", "10.0.0.0/16")

	config := func(cidr string) string { return testAccConfig(dir, fmt.Sprintf(testAccAllocationExplicit, cidr)) }
	fixedID := statecheck.CompareValue(compare.ValuesDiffer())
	nextID := statecheck.CompareValue(compare.ValuesSame())

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckReleased(t, dir, "root"),
		Steps: []acc.TestStep{
			{
				Config: config("10.0.0.0/24"),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectKnownValue("ipam_allocation.fixed", tfjsonpath.New("cidr"),
							knownvalue.StringExact("10.0.0.0/24")),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					fixedID.AddStateValue("ipam_allocation.fixed", tfjsonpath.New("id")),
					nextID.AddStateValue("ipam_allocation.next", tfjsonpath.New("id")),
					testAccCheckDisjoint(t, dir, 2),
				},
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckResourceAttr("ipam_allocation.fixed", "cidr", "10.0.0.0/24"),
					acc.TestCheckNoResourceAttr("ipam_allocation.fixed", "prefix_length"),
					acc.TestCheckResourceAttr("ipam_allocation.next", "cidr", "10.0.1.0/24"),
					testAccCheckStored(t, dir, "ipam_allocation.fixed"),
					testAccCheckStored(t, dir, "ipam_allocation.next"),
				),
			},
			{
				ResourceName:      "ipam_allocation.fixed",
				ImportState:       true,
				ImportStateIdFunc: testAccAllocationImportID("ipam_allocation.fixed"),
				ImportStateVerify: true,
			},
			{
				// Changing an explicit cidr replaces only that allocation.
				Config: config("10.0.128.0/24"),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipam_allocation.fixed", plancheck.ResourceActionReplace),
						plancheck.ExpectResourceAction("ipam_allocation.next", plancheck.ResourceActionNoop),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					fixedID.AddStateValue("ipam_allocation.fixed", tfjsonpath.New("id")),
					nextID.AddStateValue("ipam_allocation.next", tfjsonpath.New("id")),
					testAccCheckDisjoint(t, dir, 2),
				},
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckResourceAttr("ipam_allocation.fixed", "cidr", "10.0.128.0/24"),
					testAccCheckStored(t, dir, "ipam_allocation.fixed"),
				),
			},
		},
	})
}

const testAccAllocationReplace = `
resource "ipam_allocation" "a" {
  pool          = %q
  prefix_length = %d
  description   = "kept"
}
`

func TestAccAllocationResourceReplace(t *testing.T) {
	dir := testAccBackend(t)
	testAccPool(t, dir, "root", "10.0.0.0/16")
	testAccPool(t, dir, "other", "10.9.0.0/16")

	config := func(pool string, bits int) string {
		return testAccConfig(dir, fmt.Sprintf(testAccAllocationReplace, pool, bits))
	}
	ids := statecheck.CompareValue(compare.ValuesDiffer())
	replaced := acc.ConfigPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectResourceAction("ipam_allocation.a", plancheck.ResourceActionReplace),
			plancheck.ExpectUnknownValue("ipam_allocation.a", tfjsonpath.New("id")),
			plancheck.ExpectUnknownValue("ipam_allocation.a", tfjsonpath.New("cidr")),
			plancheck.ExpectUnknownValue("ipam_allocation.a", tfjsonpath.New("created_at")),
		},
		PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
	}

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckReleased(t, dir, "root", "other"),
		Steps: []acc.TestStep{
			{
				Config:            config("root", 24),
				ConfigStateChecks: []statecheck.StateCheck{ids.AddStateValue("ipam_allocation.a", tfjsonpath.New("id"))},
			},
			{
				Config:            config("root", 22),
				ConfigPlanChecks:  replaced,
				ConfigStateChecks: []statecheck.StateCheck{ids.AddStateValue("ipam_allocation.a", tfjsonpath.New("id"))},
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckResourceAttr("ipam_allocation.a", "prefix_length", "22"),
					acc.TestCheckResourceAttr("ipam_allocation.a", "cidr", "10.0.0.0/22"),
					testAccCheckStored(t, dir, "ipam_allocation.a"),
					func(*terraform.State) error {
						if n := len(testAccAllocations(t, dir, "root")); n != 1 {
							return fmt.Errorf("root has %d allocations after replace, want 1", n)
						}

						return nil
					},
				),
			},
			{
				Config:            config("other", 22),
				ConfigPlanChecks:  replaced,
				ConfigStateChecks: []statecheck.StateCheck{ids.AddStateValue("ipam_allocation.a", tfjsonpath.New("id"))},
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckResourceAttr("ipam_allocation.a", "cidr", "10.9.0.0/22"),
					testAccCheckStored(t, dir, "ipam_allocation.a"),
					testAccCheckReleased(t, dir, "root"),
				),
			},
		},
	})
}

const testAccAllocationMetadata = `
resource "ipam_allocation" "a" {
  pool          = "root"
  prefix_length = 24
%s}
`

func TestAccAllocationResourceUpdateInPlace(t *testing.T) {
	dir := testAccBackend(t)
	testAccPool(t, dir, "root", "10.0.0.0/16")

	config := func(attrs string) string { return testAccConfig(dir, fmt.Sprintf(testAccAllocationMetadata, attrs)) }
	sameID := statecheck.CompareValue(compare.ValuesSame())
	sameCIDR := statecheck.CompareValue(compare.ValuesSame())
	sameCreatedAt := statecheck.CompareValue(compare.ValuesSame())
	sameChecks := func() []statecheck.StateCheck {
		return []statecheck.StateCheck{
			sameID.AddStateValue("ipam_allocation.a", tfjsonpath.New("id")),
			sameCIDR.AddStateValue("ipam_allocation.a", tfjsonpath.New("cidr")),
			sameCreatedAt.AddStateValue("ipam_allocation.a", tfjsonpath.New("created_at")),
		}
	}
	inPlace := func() acc.ConfigPlanChecks {
		return acc.ConfigPlanChecks{
			PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction("ipam_allocation.a", plancheck.ResourceActionUpdate),
				plancheck.ExpectKnownValue("ipam_allocation.a", tfjsonpath.New("id"), knownvalue.NotNull()),
				plancheck.ExpectKnownValue("ipam_allocation.a", tfjsonpath.New("cidr"),
					knownvalue.StringExact("10.0.0.0/24")),
				plancheck.ExpectKnownValue("ipam_allocation.a", tfjsonpath.New("created_at"), knownvalue.NotNull()),
			},
			PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
		}
	}

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckReleased(t, dir, "root"),
		Steps: []acc.TestStep{
			{
				Config: config(`  description = "one"
  labels      = { team = "a" }
`),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				ConfigStateChecks: sameChecks(),
				Check:             testAccCheckStored(t, dir, "ipam_allocation.a"),
			},
			{
				Config: config(`  description = "two"
  labels      = { team = "b", tier = "1" }
`),
				ConfigPlanChecks:  inPlace(),
				ConfigStateChecks: sameChecks(),
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckResourceAttr("ipam_allocation.a", "description", "two"),
					acc.TestCheckResourceAttr("ipam_allocation.a", "labels.%", "2"),
					acc.TestCheckResourceAttr("ipam_allocation.a", "labels.tier", "1"),
					testAccCheckStored(t, dir, "ipam_allocation.a"),
				),
			},
			{
				// An empty labels map stays empty, not null.
				Config:           config("  labels = {}\n"),
				ConfigPlanChecks: inPlace(),
				ConfigStateChecks: append(sameChecks(),
					statecheck.ExpectKnownValue("ipam_allocation.a", tfjsonpath.New("labels"),
						knownvalue.MapExact(map[string]knownvalue.Check{})),
					statecheck.ExpectKnownValue("ipam_allocation.a", tfjsonpath.New("description"), knownvalue.Null())),
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckNoResourceAttr("ipam_allocation.a", "description"),
					acc.TestCheckResourceAttr("ipam_allocation.a", "labels.%", "0"),
					testAccCheckStored(t, dir, "ipam_allocation.a"),
				),
			},
			{
				// Removing labels altogether is null, and still in place.
				Config: config(`  description = ""
`),
				ConfigPlanChecks: inPlace(),
				ConfigStateChecks: append(sameChecks(),
					statecheck.ExpectKnownValue("ipam_allocation.a", tfjsonpath.New("labels"), knownvalue.Null()),
					statecheck.ExpectKnownValue("ipam_allocation.a", tfjsonpath.New("description"), knownvalue.StringExact(""))),
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckResourceAttr("ipam_allocation.a", "description", ""),
					acc.TestCheckNoResourceAttr("ipam_allocation.a", "labels.%"),
					testAccCheckStored(t, dir, "ipam_allocation.a"),
				),
			},
		},
	})
}

const testAccAllocationIPv6 = `
resource "ipam_pool" "v6" {
  id    = "v6"
  cidrs = ["fd00::/48"]
}

resource "ipam_allocation" "next" {
  pool          = ipam_pool.v6.id
  prefix_length = 64
  labels        = {}
}

resource "ipam_allocation" "fixed" {
  pool       = ipam_pool.v6.id
  cidr       = "fd00:0:0:ff::/64"
  depends_on = [ipam_allocation.next]
}
`

func TestAccAllocationResourceIPv6(t *testing.T) {
	dir := testAccBackend(t)

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{
				Config: testAccConfig(dir, testAccAllocationIPv6),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ipam_allocation.next", tfjsonpath.New("labels"),
						knownvalue.MapExact(map[string]knownvalue.Check{})),
					statecheck.ExpectKnownValue("ipam_allocation.fixed", tfjsonpath.New("labels"), knownvalue.Null()),
				},
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckResourceAttr("ipam_allocation.next", "cidr", "fd00::/64"),
					acc.TestCheckResourceAttr("ipam_allocation.next", "labels.%", "0"),
					acc.TestCheckResourceAttr("ipam_allocation.fixed", "cidr", "fd00:0:0:ff::/64"),
					testAccCheckStored(t, dir, "ipam_allocation.next"),
					testAccCheckStored(t, dir, "ipam_allocation.fixed"),
				),
			},
			{
				ResourceName:      "ipam_allocation.fixed",
				ImportState:       true,
				ImportStateIdFunc: testAccAllocationImportID("ipam_allocation.fixed"),
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccAllocationResourceDisappears(t *testing.T) {
	dir := testAccBackend(t)
	testAccPool(t, dir, "root", "10.0.0.0/16")

	ids := statecheck.CompareValue(compare.ValuesDiffer())
	config := testAccConfig(dir, testAccAllocationBasic)

	var lastID string

	captureID := func(s *terraform.State) error {
		lastID = s.RootModule().Resources["ipam_allocation.a"].Primary.ID

		return nil
	}
	release := func() {
		if err := testAccClient(t, dir).Release(context.Background(), "root", lastID); err != nil {
			t.Fatalf("Release: %v", err)
		}
	}

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		Steps: []acc.TestStep{
			{
				Config:            config,
				ConfigStateChecks: []statecheck.StateCheck{ids.AddStateValue("ipam_allocation.a", tfjsonpath.New("id"))},
				Check:             captureID,
			},
			{
				PreConfig:          release,
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: config,
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipam_allocation.a", plancheck.ResourceActionCreate),
						plancheck.ExpectUnknownValue("ipam_allocation.a", tfjsonpath.New("id")),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{ids.AddStateValue("ipam_allocation.a", tfjsonpath.New("id"))},
				Check:             acc.ComposeAggregateTestCheckFunc(captureID, testAccCheckStored(t, dir, "ipam_allocation.a")),
			},
			{
				// Released and its pool deleted before the destroy: destroying succeeds.
				PreConfig: func() {
					release()

					if err := testAccClient(t, dir).DeletePool(context.Background(), "root"); err != nil {
						t.Fatalf("DeletePool: %v", err)
					}
				},
				Config: testAccConfig(dir, ""),
			},
		},
	})
}

func TestAccAllocationResourceErrors(t *testing.T) {
	dir := testAccBackend(t)
	testAccPool(t, dir, "small", "10.0.0.0/24")
	testAccPool(t, dir, "root", "10.1.0.0/16")

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckReleased(t, dir, "small", "root"),
		Steps: []acc.TestStep{
			{
				Config: testAccConfig(dir, `
resource "ipam_allocation" "first" {
  pool          = "small"
  prefix_length = 24
}

resource "ipam_allocation" "second" {
  pool          = "small"
  prefix_length = 24
  depends_on    = [ipam_allocation.first]
}
`),
				ExpectError: regexp.MustCompile(`(?s)Create allocation: pool exhausted.*No free CIDR`),
			},
			{
				Config: testAccConfig(dir, `
resource "ipam_allocation" "first" {
  pool = "root"
  cidr = "10.1.0.0/24"
}

resource "ipam_allocation" "second" {
  pool       = "root"
  cidr       = "10.1.0.0/23"
  depends_on = [ipam_allocation.first]
}
`),
				ExpectError: regexp.MustCompile(`(?s)Create allocation: conflict.*Reason: overlap`),
			},
			{
				Config: testAccConfig(dir, `
resource "ipam_allocation" "first" {
  pool = "root"
  cidr = "10.9.0.0/24"
}
`),
				ExpectError: regexp.MustCompile(`Create allocation: invalid CIDR|outside`),
			},
			{
				Config: testAccConfig(dir, `
resource "ipam_allocation" "first" {
  pool          = "missing"
  prefix_length = 24
}
`),
				ExpectError: regexp.MustCompile(`(?s)Create allocation: not found.*pool missing`),
			},
			{
				Config: testAccConfig(dir, `
resource "ipam_allocation" "first" {
  pool          = "root"
  prefix_length = 8
}
`),
				ExpectError: regexp.MustCompile(`Create allocation: (pool exhausted|invalid CIDR)`),
			},
		},
	})

	// Failed creates leave nothing behind.
	if n := len(testAccAllocations(t, dir, "small")) + len(testAccAllocations(t, dir, "root")); n != 0 {
		t.Fatalf("%d allocations left after failed creates", n)
	}
}

func TestAccAllocationResourceValidation(t *testing.T) {
	dir := testAccBackend(t)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"both requests", `
resource "ipam_allocation" "a" {
  pool          = "root"
  prefix_length = 24
  cidr          = "10.0.0.0/24"
}`, `exactly one of prefix_length or cidr`},
		{"no request", `
resource "ipam_allocation" "a" {
  pool = "root"
}`, `exactly one of prefix_length or cidr`},
		{"non-canonical cidr", `
resource "ipam_allocation" "a" {
  pool = "root"
  cidr = "10.0.0.1/24"
}`, `canonical CIDR`},
		{"invalid pool", `
resource "ipam_allocation" "a" {
  pool          = "Bad_Pool"
  prefix_length = 24
}`, `lowercase letters, digits and hyphens`},
		{"prefix too long", `
resource "ipam_allocation" "a" {
  pool          = "root"
  prefix_length = 129
}`, `between 0 and 128`},
		{"id is computed", `
resource "ipam_allocation" "a" {
  id            = "018f22e2-1865-7be8-9f45-35d90607d0f7"
  pool          = "root"
  prefix_length = 24
}`, `Invalid Configuration for Read-Only Attribute`},
	}

	steps := make([]acc.TestStep, 0, len(cases))
	for _, tc := range cases {
		steps = append(steps, acc.TestStep{
			Config:      testAccConfig(dir, tc.body),
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(regexp.QuoteMeta(tc.want)),
		})
	}

	acc.Test(t, acc.TestCase{ProtoV6ProviderFactories: testAccFactories(), Steps: steps})
}

func TestAccAllocationResourceImportErrors(t *testing.T) {
	dir := testAccBackend(t)
	testAccPool(t, dir, "root", "10.0.0.0/16")

	missing := ipam.NewAllocationID()
	steps := []acc.TestStep{{Config: testAccConfig(dir, testAccAllocationBasic)}}

	for _, tc := range []struct{ id, want string }{
		{"root", `Expected an import ID of the form <pool>/<id>`},
		{"/" + missing, `Expected an import ID of the form <pool>/<id>`},
		{"root/", `Expected an import ID of the form <pool>/<id>`},
		{missing + "/root", `invalid argument`},
		{"root/" + missing, `Cannot import non-existent remote object`},
	} {
		steps = append(steps, acc.TestStep{
			ResourceName:  "ipam_allocation.a",
			ImportState:   true,
			ImportStateId: tc.id,
			ExpectError:   regexp.MustCompile(regexp.QuoteMeta(tc.want)),
		})
	}

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckReleased(t, dir, "root"),
		Steps:                    steps,
	})
}

func allocationSchema(t *testing.T) rschema.Schema {
	t.Helper()

	var resp resource.SchemaResponse

	(&allocationResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}

	return resp.Schema
}

// allocationHarness drives the resource's CRUD methods directly.
type allocationHarness struct {
	t      *testing.T
	r      *allocationResource
	schema rschema.Schema
	typ    tftypes.Type
}

func newAllocationHarness(t *testing.T, store storage.Storage) *allocationHarness {
	t.Helper()

	s := allocationSchema(t)

	return &allocationHarness{
		t:      t,
		r:      &allocationResource{data: &Data{Client: newClient(store), AccessPropagationTimeout: time.Minute}},
		schema: s,
		typ:    s.Type().TerraformType(context.Background()),
	}
}

func (h *allocationHarness) value(attrs map[string]any) tftypes.Value {
	return configValue(h.t, h.typ, attrs)
}

func (h *allocationHarness) create(attrs map[string]any) resource.CreateResponse {
	planned := map[string]any{"id": unknown{}, "created_at": unknown{}}
	if _, explicit := attrs["cidr"]; !explicit {
		planned["cidr"] = unknown{}
	}

	for k, v := range attrs {
		planned[k] = v
	}

	plan := h.value(planned)

	var resp resource.CreateResponse

	resp.State = tfsdk.State{Schema: h.schema, Raw: tftypes.NewValue(h.typ, nil)}
	h.r.Create(context.Background(), resource.CreateRequest{
		Plan:   tfsdk.Plan{Schema: h.schema, Raw: plan},
		Config: tfsdk.Config{Schema: h.schema, Raw: h.value(attrs)},
	}, &resp)

	return resp
}

func (h *allocationHarness) read(state tfsdk.State) resource.ReadResponse {
	var resp resource.ReadResponse

	resp.State = state
	h.r.Read(context.Background(), resource.ReadRequest{State: state}, &resp)

	return resp
}

func stateString(t *testing.T, state tfsdk.State, attr string) string {
	t.Helper()

	var value *string

	if diags := state.GetAttribute(context.Background(), path.Root(attr), &value); diags.HasError() {
		t.Fatalf("state %s: %v", attr, diags)
	}

	if value == nil {
		return ""
	}

	return *value
}

func TestAllocationResourceMutationsUseMutationContext(t *testing.T) {
	ctx := context.Background()
	store := &recordingStore{Storage: newLocalStore(t)}
	createPool(t, newClient(store))
	store.take()

	h := newAllocationHarness(t, store)
	created := h.create(map[string]any{"pool": "root", "prefix_length": int64(24)})

	if created.Diagnostics.HasError() {
		t.Fatalf("Create: %v", created.Diagnostics)
	}

	requireCalls(t, "Create", store.take(), true)

	read := h.read(created.State)
	if read.Diagnostics.HasError() || read.State.Raw.IsNull() {
		t.Fatalf("Read: %v (null=%v)", read.Diagnostics, read.State.Raw.IsNull())
	}

	requireCalls(t, "Read", store.take(), false)

	if !read.State.Raw.Equal(created.State.Raw) {
		t.Fatalf("Read changed the state Create wrote:\ncreate %v\nread   %v", created.State.Raw, read.State.Raw)
	}

	updated := h.value(map[string]any{
		"id": stateString(t, read.State, "id"), "pool": "root", "prefix_length": int64(24),
		"cidr": stateString(t, read.State, "cidr"), "created_at": stateString(t, read.State, "created_at"),
		"description": "d", "labels": stringMap(map[string]string{"k": "v"}),
	})

	var update resource.UpdateResponse

	update.State = read.State
	h.r.Update(ctx, resource.UpdateRequest{
		Plan:   tfsdk.Plan{Schema: h.schema, Raw: updated},
		Config: tfsdk.Config{Schema: h.schema, Raw: updated},
		State:  read.State,
	}, &update)

	if update.Diagnostics.HasError() {
		t.Fatalf("Update: %v", update.Diagnostics)
	}

	requireCalls(t, "Update", store.take(), true)

	stored, err := newClient(store).GetAllocation(ctx, "root", stateString(t, update.State, "id"))
	if err != nil || stored.Description != "d" || stored.Labels["k"] != "v" {
		t.Fatalf("stored after Update = %+v, %v", stored, err)
	}

	store.take()

	var deleted resource.DeleteResponse

	h.r.Delete(ctx, resource.DeleteRequest{State: update.State}, &deleted)

	if deleted.Diagnostics.HasError() {
		t.Fatalf("Delete: %v", deleted.Diagnostics)
	}

	requireCalls(t, "Delete", store.take(), true)

	// Releasing an allocation that is already released succeeds.
	var again resource.DeleteResponse

	h.r.Delete(ctx, resource.DeleteRequest{State: update.State}, &again)

	if again.Diagnostics.HasError() {
		t.Fatalf("Delete of a released allocation: %v", again.Diagnostics)
	}

	// Read of a released allocation removes it from state.
	gone := h.read(update.State)
	if gone.Diagnostics.HasError() || !gone.State.Raw.IsNull() {
		t.Fatalf("Read of a released allocation: %v (null=%v)", gone.Diagnostics, gone.State.Raw.IsNull())
	}

	// Destroy after the pool itself is gone succeeds; Read removes it too.
	if err := newClient(store).DeletePool(ctx, "root"); err != nil {
		t.Fatalf("DeletePool: %v", err)
	}

	var poolGone resource.DeleteResponse

	h.r.Delete(ctx, resource.DeleteRequest{State: update.State}, &poolGone)

	if poolGone.Diagnostics.HasError() {
		t.Fatalf("Delete after the pool was deleted: %v", poolGone.Diagnostics)
	}

	if gone := h.read(update.State); gone.Diagnostics.HasError() || !gone.State.Raw.IsNull() {
		t.Fatalf("Read after the pool was deleted: %v (null=%v)", gone.Diagnostics, gone.State.Raw.IsNull())
	}
}

func TestAllocationResourceReadErrorIsDiagnostic(t *testing.T) {
	store := newLocalStore(t)
	createPool(t, newClient(store))

	h := newAllocationHarness(t, store)
	created := h.create(map[string]any{"pool": "root", "prefix_length": int64(24)})

	if created.Diagnostics.HasError() {
		t.Fatalf("Create: %v", created.Diagnostics)
	}

	h.r.data.Client = newClient(deniedStore{store})
	resp := h.read(created.State)

	requireDiag(t, resp.Diagnostics, "Read allocation: access denied")

	if resp.State.Raw.IsNull() {
		t.Fatal("Read removed the allocation from state on access denied")
	}
}

// lostResponseStore commits matching PutIfMatch calls and then reports an
// unknown outcome, as when the response to a write that landed is lost.
type lostResponseStore struct {
	storage.Storage

	lose     atomic.Int64 // number of committed puts to report as unknown
	denyGets atomic.Int64 // number of reads after a lost response to deny
	puts     atomic.Int64
}

func (s *lostResponseStore) PutIfMatch(
	ctx context.Context, key string, data []byte, version storage.Version,
) (storage.Version, error) {
	s.puts.Add(1)

	next, err := s.Storage.PutIfMatch(ctx, key, data, version)
	if err != nil || s.lose.Add(-1) < 0 {
		return next, err
	}

	return "", fmt.Errorf("lost response: %w", storage.ErrUnknownOutcome)
}

func (s *lostResponseStore) Get(ctx context.Context, key string) ([]byte, storage.Version, error) {
	if s.lose.Load() <= 0 && s.puts.Load() > 0 && s.denyGets.Add(-1) >= 0 {
		return nil, "", fmt.Errorf("injected: %w", storage.ErrAccessDenied)
	}

	return s.Storage.Get(ctx, key)
}

func newLocalStore(t *testing.T) storage.Storage {
	t.Helper()

	store, err := local.New(t.TempDir())
	if err != nil {
		t.Fatalf("local.New: %v", err)
	}

	return store
}

func TestLostResponseCreateIsIdempotent(t *testing.T) {
	store := &lostResponseStore{Storage: newLocalStore(t)}
	createPool(t, newClient(store.Storage))
	store.lose.Store(1)

	h := newAllocationHarness(t, store)
	created := h.create(map[string]any{"pool": "root", "prefix_length": int64(24)})

	if created.Diagnostics.HasError() {
		t.Fatalf("Create after a lost response: %v", created.Diagnostics)
	}

	allocations, err := newClient(store.Storage).ListAllocations(context.Background(), "root")
	if err != nil {
		t.Fatal(err)
	}

	if len(allocations) != 1 {
		t.Fatalf("%d allocations after a lost response, want exactly 1: %+v", len(allocations), allocations)
	}

	if id := stateString(t, created.State, "id"); allocations[0].ID != id {
		t.Fatalf("state id %s, stored id %s", id, allocations[0].ID)
	}

	if cidr := stateString(t, created.State, "cidr"); allocations[0].CIDR.String() != cidr {
		t.Fatalf("state cidr %s, stored cidr %s", cidr, allocations[0].CIDR)
	}

	if store.puts.Load() != 1 {
		t.Fatalf("%d puts, want 1: the retry must find the committed allocation", store.puts.Load())
	}
}

// TestLostResponseUncertainCreateKeepsID covers a Create that cannot learn
// whether its write landed: the generated id goes into state with the error,
// so Terraform keeps the allocation (tainted) instead of leaking it, and the
// next refresh and destroy reach the one committed allocation.
func TestLostResponseUncertainCreateKeepsID(t *testing.T) {
	ctx := context.Background()
	store := &lostResponseStore{Storage: newLocalStore(t)}
	createPool(t, newClient(store.Storage))
	store.lose.Store(1)
	store.denyGets.Store(1)

	h := newAllocationHarness(t, store)
	created := h.create(map[string]any{
		"pool": "root", "prefix_length": int64(24), "description": "d",
		"labels": stringMap(map[string]string{"k": "v"}),
	})

	requireDiag(t, created.Diagnostics, "Create allocation: access denied")

	if created.State.Raw.IsNull() {
		t.Fatal("Create dropped the generated id after an uncertain outcome")
	}

	id := stateString(t, created.State, "id")

	allocations, err := newClient(store.Storage).ListAllocations(ctx, "root")
	if err != nil {
		t.Fatal(err)
	}

	if len(allocations) != 1 || allocations[0].ID != id {
		t.Fatalf("stored %+v, want exactly the allocation %s", allocations, id)
	}

	if !created.State.Raw.IsFullyKnown() {
		t.Fatalf("state has unknown values: %v", created.State.Raw)
	}

	read := h.read(created.State)
	if read.Diagnostics.HasError() || stateString(t, read.State, "cidr") != allocations[0].CIDR.String() {
		t.Fatalf("Read after an uncertain create: %v, cidr %q", read.Diagnostics, stateString(t, read.State, "cidr"))
	}

	var deleted resource.DeleteResponse

	h.r.Delete(ctx, resource.DeleteRequest{State: read.State}, &deleted)

	if deleted.Diagnostics.HasError() {
		t.Fatalf("Delete: %v", deleted.Diagnostics)
	}

	if remaining, _ := newClient(store.Storage).ListAllocations(ctx, "root"); len(remaining) != 0 {
		t.Fatalf("%d allocations left after Delete", len(remaining))
	}
}

// A definite failure (nothing written) leaves no state behind.
func TestAllocationResourceDefiniteFailureLeavesNoState(t *testing.T) {
	store := newLocalStore(t)
	createPool(t, newClient(store))

	h := newAllocationHarness(t, store)
	created := h.create(map[string]any{"pool": "root", "prefix_length": int64(8)})

	requireDiag(t, created.Diagnostics, "Create allocation:")

	if !created.State.Raw.IsNull() {
		t.Fatalf("state written after a definite failure: %v", created.State.Raw)
	}

	missing := h.create(map[string]any{"pool": "missing", "prefix_length": int64(24)})
	requireDiag(t, missing.Diagnostics, "Create allocation: not found")

	if !missing.State.Raw.IsNull() {
		t.Fatalf("state written for a missing pool: %v", missing.State.Raw)
	}
}

// Separate clients (separate Terraform states or processes) allocating from
// one pool at once get distinct, non-overlapping CIDRs.
func TestAllocationResourceConcurrentClients(t *testing.T) {
	dir := t.TempDir()
	createPool(t, testAccClient(t, dir))

	const workers = 10

	var wg sync.WaitGroup

	results := make([]resource.CreateResponse, workers)
	for i := range workers {
		h := newAllocationHarness(t, testAccStore(t, dir))

		wg.Go(func() {
			results[i] = h.create(map[string]any{"pool": "root", "prefix_length": int64(24)})
		})
	}

	wg.Wait()

	stored := map[string]string{}
	for _, allocation := range testAccAllocations(t, dir, "root") {
		stored[allocation.ID] = allocation.CIDR.String()
	}

	if len(stored) != workers {
		t.Fatalf("%d allocations stored, want %d", len(stored), workers)
	}

	var prefixes []netip.Prefix

	for i, resp := range results {
		if resp.Diagnostics.HasError() {
			t.Fatalf("worker %d: %v", i, resp.Diagnostics)
		}

		id, cidr := stateString(t, resp.State, "id"), stateString(t, resp.State, "cidr")
		if stored[id] != cidr {
			t.Fatalf("worker %d: state %s=%s, stored %q", i, id, cidr, stored[id])
		}

		prefix := netip.MustParsePrefix(cidr)
		for _, other := range prefixes {
			if prefix.Overlaps(other) {
				t.Fatalf("worker %d: %s overlaps %s", i, prefix, other)
			}
		}

		prefixes = append(prefixes, prefix)
	}
}

func testAccStore(t *testing.T, dir string) storage.Storage {
	t.Helper()

	store, err := local.New(dir)
	if err != nil {
		t.Fatalf("local.New: %v", err)
	}

	return store
}

func TestAllocationImportIDParsing(t *testing.T) {
	id := ipam.NewAllocationID()

	for _, tc := range []struct {
		raw, pool, id string
		ok            bool
	}{
		{"prod/" + id, "prod", id, true},
		{"prod/a/b", "prod", "a/b", true},
		{"prod", "", "", false},
		{"/" + id, "", "", false},
		{"prod/", "", "", false},
		{"", "", "", false},
	} {
		pool, got, err := parseAllocationImportID(tc.raw)
		if (err == nil) != tc.ok || pool != tc.pool || got != tc.id {
			t.Errorf("parseAllocationImportID(%q) = %q, %q, %v; want %q, %q, ok=%v",
				tc.raw, pool, got, err, tc.pool, tc.id, tc.ok)
		}

		if err != nil && !strings.Contains(err.Error(), "<pool>/<id>") {
			t.Errorf("parseAllocationImportID(%q) error %q does not name the format", tc.raw, err)
		}
	}
}
