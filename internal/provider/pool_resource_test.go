// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	rschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
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
	"github.com/appvia/ipam/storage/memory"
)

func testAccFactories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){
		"ipam": providerserver.NewProtocol6WithError(New("test")()),
	}
}

// testAccBackend returns a fresh local backend directory and isolates the
// environment so the provider uses only the configured block.
func testAccBackend(t *testing.T) string {
	t.Helper()
	isolateEnv(t)

	return t.TempDir()
}

func testAccConfig(dir, body string) string {
	return fmt.Sprintf("provider \"ipam\" {\n  local = { path = %q }\n}\n\n%s\n", dir, body)
}

func testAccClient(t *testing.T, dir string) *ipam.Client {
	t.Helper()

	store, err := local.New(dir)
	if err != nil {
		t.Fatalf("local.New: %v", err)
	}

	return ipam.New(store)
}

func testAccCheckPoolsDestroyed(t *testing.T, dir string) func(*terraform.State) error {
	return func(s *terraform.State) error {
		client := testAccClient(t, dir)

		for _, rs := range s.RootModule().Resources {
			if rs.Type != "ipam_pool" {
				continue
			}

			_, err := client.GetPool(context.Background(), rs.Primary.ID)
			if !errors.Is(err, ipam.ErrNotFound) {
				return fmt.Errorf("pool %s still exists after destroy (err=%v)", rs.Primary.ID, err)
			}
		}

		return nil
	}
}

// stateSet returns the elements of a set or list attribute from state.
func stateSet(s *terraform.State, name, attr string) ([]string, error) {
	rs, ok := s.RootModule().Resources[name]
	if !ok {
		return nil, fmt.Errorf("%s not in state", name)
	}

	count, err := strconv.Atoi(rs.Primary.Attributes[attr+".#"])
	if err != nil {
		return nil, fmt.Errorf("%s.%s.#: %w", name, attr, err)
	}

	var values []string

	for key, value := range rs.Primary.Attributes {
		if strings.HasPrefix(key, attr+".") && key != attr+".#" {
			values = append(values, value)
		}
	}

	if len(values) != count {
		return nil, fmt.Errorf("%s.%s has %d values, count %d", name, attr, len(values), count)
	}

	return values, nil
}

func testAccCheckChildInside(child, parent string, bits int) acc.TestCheckFunc {
	return func(s *terraform.State) error {
		childCIDRs, err := stateSet(s, child, "cidrs")
		if err != nil {
			return err
		}

		parentCIDRs, err := stateSet(s, parent, "cidrs")
		if err != nil {
			return err
		}

		if len(childCIDRs) != 1 {
			return fmt.Errorf("child cidrs = %v, want exactly one", childCIDRs)
		}

		carved, err := netip.ParsePrefix(childCIDRs[0])
		if err != nil {
			return err
		}

		if carved.Bits() != bits {
			return fmt.Errorf("child cidr %s, want a /%d", carved, bits)
		}

		for _, raw := range parentCIDRs {
			outer := netip.MustParsePrefix(raw)
			if outer.Bits() <= carved.Bits() && outer.Contains(carved.Addr()) {
				return nil
			}
		}

		return fmt.Errorf("child cidr %s is not inside parent %v", carved, parentCIDRs)
	}
}

const testAccRootPool = `
resource "ipam_pool" "root" {
  id          = "root"
  cidrs       = ["10.0.0.0/16"]
  description = "root pool"
  labels      = { env = "test" }
}
`

func TestAccPoolResourceRoot(t *testing.T) {
	dir := testAccBackend(t)

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{
				Config: testAccConfig(dir, testAccRootPool),
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckResourceAttr("ipam_pool.root", "id", "root"),
					acc.TestCheckResourceAttr("ipam_pool.root", "family", "4"),
					acc.TestCheckResourceAttr("ipam_pool.root", "cidrs.#", "1"),
					acc.TestCheckTypeSetElemAttr("ipam_pool.root", "cidrs.*", "10.0.0.0/16"),
					acc.TestCheckResourceAttr("ipam_pool.root", "description", "root pool"),
					acc.TestCheckResourceAttr("ipam_pool.root", "labels.env", "test"),
					acc.TestCheckNoResourceAttr("ipam_pool.root", "parent_pool"),
					func(*terraform.State) error {
						pool, err := testAccClient(t, dir).GetPool(context.Background(), "root")
						if err != nil {
							return err
						}

						if pool.Description != "root pool" || pool.Labels["env"] != "test" ||
							len(pool.CIDRs) != 1 || pool.CIDRs[0].String() != "10.0.0.0/16" {
							return fmt.Errorf("stored pool = %+v", pool)
						}

						return nil
					},
				),
			},
			{
				ResourceName:                         "ipam_pool.root",
				ImportState:                          true,
				ImportStateId:                        "root",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "id",
			},
			{
				ResourceName:    "ipam_pool.root",
				ImportState:     true,
				ImportStateKind: acc.ImportBlockWithResourceIdentity,
			},
		},
	})
}

const testAccParentChild = `
resource "ipam_pool" "parent" {
  id    = "parent"
  cidrs = ["10.0.0.0/16"]
}

resource "ipam_pool" "child" {
  id            = "child"
  parent_pool   = ipam_pool.parent.id
  prefix_length = %d
  description   = %q
  labels        = %s
}

resource "ipam_pool" "explicit" {
  id          = "explicit"
  parent_pool = ipam_pool.parent.id
  cidr        = "10.0.128.0/20"
}
`

func parentChildConfig(dir string, bits int, description, labels string) string {
	return testAccConfig(dir, fmt.Sprintf(testAccParentChild, bits, description, labels))
}

func TestAccPoolResourceParentChild(t *testing.T) {
	dir := testAccBackend(t)

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{
				Config: parentChildConfig(dir, 20, "child", `{ team = "a" }`),
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckResourceAttr("ipam_pool.child", "parent_pool", "parent"),
					acc.TestCheckResourceAttr("ipam_pool.child", "prefix_length", "20"),
					acc.TestCheckResourceAttr("ipam_pool.child", "family", "4"),
					acc.TestCheckNoResourceAttr("ipam_pool.child", "cidr"),
					testAccCheckChildInside("ipam_pool.child", "ipam_pool.parent", 20),
					acc.TestCheckResourceAttr("ipam_pool.explicit", "cidrs.#", "1"),
					acc.TestCheckTypeSetElemAttr("ipam_pool.explicit", "cidrs.*", "10.0.128.0/20"),
					acc.TestCheckResourceAttr("ipam_pool.explicit", "cidr", "10.0.128.0/20"),
					func(*terraform.State) error {
						allocations, err := testAccClient(t, dir).ListAllocations(context.Background(), "parent")
						if err != nil {
							return err
						}

						if len(allocations) != 2 {
							return fmt.Errorf("parent carve-outs = %d, want 2", len(allocations))
						}

						return nil
					},
				),
			},
			{
				ResourceName:      "ipam_pool.child",
				ImportState:       true,
				ImportStateId:     "child",
				ImportStateVerify: true,
			},
			{
				ResourceName:      "ipam_pool.explicit",
				ImportState:       true,
				ImportStateId:     "explicit",
				ImportStateVerify: true,
			},
		},
	})
}

const testAccReplace = `
resource "ipam_pool" "parent" {
  id    = "parent"
  cidrs = [%q]
}

resource "ipam_pool" "child" {
  id            = "child"
  parent_pool   = ipam_pool.parent.id
  prefix_length = %d
}
%s`

const testAccReplaceExplicit = `
resource "ipam_pool" "explicit" {
  id          = "explicit"
  parent_pool = ipam_pool.parent.id
  cidr        = %q
}
`

func replaceConfig(dir, parent string, bits int, explicit string) string {
	extra := ""
	if explicit != "" {
		extra = fmt.Sprintf(testAccReplaceExplicit, explicit)
	}

	return testAccConfig(dir, fmt.Sprintf(testAccReplace, parent, bits, extra))
}

func TestAccPoolResourceReplace(t *testing.T) {
	dir := testAccBackend(t)
	childCIDRs := statecheck.CompareValue(compare.ValuesDiffer())

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{
				Config: replaceConfig(dir, "10.0.0.0/16", 20, ""),
				ConfigStateChecks: []statecheck.StateCheck{
					childCIDRs.AddStateValue("ipam_pool.child", tfjsonpath.New("cidrs")),
				},
			},
			{
				Config: replaceConfig(dir, "10.1.0.0/16", 21, ""),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipam_pool.parent", plancheck.ResourceActionReplace),
						plancheck.ExpectResourceAction("ipam_pool.child", plancheck.ResourceActionReplace),
						plancheck.ExpectUnknownValue("ipam_pool.child", tfjsonpath.New("cidrs")),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					childCIDRs.AddStateValue("ipam_pool.child", tfjsonpath.New("cidrs")),
				},
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckResourceAttr("ipam_pool.parent", "cidrs.#", "1"),
					acc.TestCheckTypeSetElemAttr("ipam_pool.parent", "cidrs.*", "10.1.0.0/16"),
					testAccCheckChildInside("ipam_pool.child", "ipam_pool.parent", 21),
				),
			},
			{Config: replaceConfig(dir, "10.1.0.0/16", 21, "10.1.128.0/20")},
			{
				Config: replaceConfig(dir, "10.1.0.0/16", 21, "10.1.64.0/20"),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipam_pool.explicit", plancheck.ResourceActionReplace),
						plancheck.ExpectResourceAction("ipam_pool.child", plancheck.ResourceActionNoop),
						plancheck.ExpectResourceAction("ipam_pool.parent", plancheck.ResourceActionNoop),
					},
				},
				Check: acc.TestCheckTypeSetElemAttr("ipam_pool.explicit", "cidrs.*", "10.1.64.0/20"),
			},
			{
				// A child moved to another parent is replaced.
				Config: replaceConfig(dir, "10.1.0.0/16", 21, "10.1.64.0/20") + `
resource "ipam_pool" "other" {
  id    = "other"
  cidrs = ["10.9.0.0/16"]
}
`,
			},
			{
				Config: strings.Replace(replaceConfig(dir, "10.1.0.0/16", 21, "10.1.64.0/20"),
					"parent_pool   = ipam_pool.parent.id", "parent_pool   = ipam_pool.other.id", 1) + `
resource "ipam_pool" "other" {
  id    = "other"
  cidrs = ["10.9.0.0/16"]
}
`,
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipam_pool.child", plancheck.ResourceActionReplace),
					},
				},
				Check: testAccCheckChildInside("ipam_pool.child", "ipam_pool.other", 21),
			},
		},
	})
}

func TestAccPoolResourceRenameReplaces(t *testing.T) {
	dir := testAccBackend(t)

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{Config: testAccConfig(dir, testAccRootPool)},
			{
				Config: testAccConfig(dir, strings.Replace(testAccRootPool, `id          = "root"`, `id          = "renamed"`, 1)),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipam_pool.root", plancheck.ResourceActionReplace),
					},
				},
				Check: func(*terraform.State) error {
					_, err := testAccClient(t, dir).GetPool(context.Background(), "root")
					if !errors.Is(err, ipam.ErrNotFound) {
						return fmt.Errorf("old pool still exists: %v", err)
					}

					return nil
				},
			},
		},
	})
}

func TestAccPoolResourceUpdateInPlace(t *testing.T) {
	dir := testAccBackend(t)
	sameCIDRs := statecheck.CompareValue(compare.ValuesSame())

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{
				Config: parentChildConfig(dir, 20, "child", `{ team = "a" }`),
				ConfigStateChecks: []statecheck.StateCheck{
					sameCIDRs.AddStateValue("ipam_pool.child", tfjsonpath.New("cidrs")),
				},
			},
			{
				Config: parentChildConfig(dir, 20, "renamed child", `{ team = "b", tier = "1" }`),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipam_pool.child", plancheck.ResourceActionUpdate),
						plancheck.ExpectKnownValue("ipam_pool.child", tfjsonpath.New("family"), knownvalue.Int64Exact(4)),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					sameCIDRs.AddStateValue("ipam_pool.child", tfjsonpath.New("cidrs")),
				},
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckResourceAttr("ipam_pool.child", "description", "renamed child"),
					acc.TestCheckResourceAttr("ipam_pool.child", "labels.%", "2"),
					func(s *terraform.State) error {
						pool, err := testAccClient(t, dir).GetPool(context.Background(), "child")
						if err != nil {
							return err
						}

						cidrs, err := stateSet(s, "ipam_pool.child", "cidrs")
						if err != nil {
							return err
						}

						if pool.Description != "renamed child" || pool.Labels["team"] != "b" ||
							pool.Labels["tier"] != "1" || len(cidrs) != 1 || pool.CIDRs[0].String() != cidrs[0] {
							return fmt.Errorf("stored pool %+v, state cidrs %v", pool, cidrs)
						}

						return nil
					},
				),
			},
			{
				// Removing labels and description is also in place.
				Config: testAccConfig(dir, `
resource "ipam_pool" "parent" {
  id    = "parent"
  cidrs = ["10.0.0.0/16"]
}

resource "ipam_pool" "child" {
  id            = "child"
  parent_pool   = ipam_pool.parent.id
  prefix_length = 20
}

resource "ipam_pool" "explicit" {
  id          = "explicit"
  parent_pool = ipam_pool.parent.id
  cidr        = "10.0.128.0/20"
}
`),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipam_pool.child", plancheck.ResourceActionUpdate),
					},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					sameCIDRs.AddStateValue("ipam_pool.child", tfjsonpath.New("cidrs")),
				},
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckNoResourceAttr("ipam_pool.child", "description"),
					acc.TestCheckNoResourceAttr("ipam_pool.child", "labels.%"),
				),
			},
		},
	})
}

const testAccIPv6 = `
resource "ipam_pool" "v6" {
  id     = "v6"
  cidrs  = ["fd00::/48"]
  labels = %s
}

resource "ipam_pool" "v6child" {
  id            = "v6child"
  parent_pool   = ipam_pool.v6.id
  prefix_length = 56
  labels        = {}
}

data "ipam_pool" "v6" {
  id         = ipam_pool.v6.id
  depends_on = [ipam_pool.v6child]
}
`

func TestAccPoolResourceIPv6EmptyLabels(t *testing.T) {
	dir := testAccBackend(t)
	config := func(labels string) string { return testAccConfig(dir, fmt.Sprintf(testAccIPv6, labels)) }
	emptyAfterApply := acc.ConfigPlanChecks{
		PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
	}

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{
				Config:           config("{}"),
				ConfigPlanChecks: emptyAfterApply,
				Check: acc.ComposeAggregateTestCheckFunc(
					acc.TestCheckResourceAttr("ipam_pool.v6", "family", "6"),
					acc.TestCheckResourceAttr("ipam_pool.v6child", "family", "6"),
					acc.TestCheckResourceAttr("data.ipam_pool.v6", "family", "6"),
					acc.TestCheckResourceAttr("ipam_pool.v6", "labels.%", "0"),
					acc.TestCheckResourceAttr("ipam_pool.v6child", "labels.%", "0"),
					testAccCheckChildInside("ipam_pool.v6child", "ipam_pool.v6", 56),
				),
			},
			{
				Config: config(`{ team = "net" }`),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipam_pool.v6", plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: acc.TestCheckResourceAttr("ipam_pool.v6", "labels.team", "net"),
			},
			{
				Config: config("{}"),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipam_pool.v6", plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: acc.TestCheckResourceAttr("ipam_pool.v6", "labels.%", "0"),
			},
		},
	})
}

func TestAccPoolResourceDisappears(t *testing.T) {
	dir := testAccBackend(t)
	deletePool := func() {
		if err := testAccClient(t, dir).DeletePool(context.Background(), "root"); err != nil {
			t.Fatalf("DeletePool: %v", err)
		}
	}

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{Config: testAccConfig(dir, testAccRootPool)},
			{
				PreConfig:          deletePool,
				Config:             testAccConfig(dir, testAccRootPool),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: testAccConfig(dir, testAccRootPool),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipam_pool.root", plancheck.ResourceActionCreate),
					},
				},
				Check: acc.TestCheckResourceAttr("ipam_pool.root", "id", "root"),
			},
			{
				// The pool is gone again before the destroy: destroying succeeds.
				PreConfig: deletePool,
				Config:    testAccConfig(dir, ""),
			},
		},
	})
}

func TestAccPoolResourceDeleteNotEmpty(t *testing.T) {
	dir := testAccBackend(t)
	allocationID := ipam.NewAllocationID()

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{
				Config: testAccConfig(dir, testAccRootPool),
				Check: func(*terraform.State) error {
					_, err := testAccClient(t, dir).Allocate(context.Background(), "root",
						ipam.AllocationSpec{ID: allocationID, PrefixLength: 24})

					return err
				},
			},
			{
				Config:      testAccConfig(dir, ""),
				ExpectError: regexp.MustCompile(`(?s)Delete pool: conflict.*pool_not_empty.*Release its allocations`),
			},
			{
				PreConfig: func() {
					if err := testAccClient(t, dir).Release(context.Background(), "root", allocationID); err != nil {
						t.Fatalf("Release: %v", err)
					}
				},
				Config: testAccConfig(dir, ""),
			},
		},
	})
}

func TestAccPoolResourceCreateConflict(t *testing.T) {
	dir := testAccBackend(t)

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		Steps: []acc.TestStep{
			{
				PreConfig: func() {
					_, err := testAccClient(t, dir).CreatePool(context.Background(), ipam.PoolSpec{
						ID: "root", CIDRs: []netip.Prefix{netip.MustParsePrefix("192.168.0.0/16")},
					})
					if err != nil {
						t.Fatalf("CreatePool: %v", err)
					}
				},
				Config:      testAccConfig(dir, testAccRootPool),
				ExpectError: regexp.MustCompile(`(?s)Create pool: conflict.*pool_exists`),
			},
		},
	})
}

// testAccCheckPolicy compares a pool's stored policy with want.
func testAccCheckPolicy(t *testing.T, dir, id string, want ipam.PoolPolicy) acc.TestCheckFunc {
	return func(*terraform.State) error {
		pool, err := testAccClient(t, dir).GetPool(context.Background(), id)
		if err != nil {
			return err
		}

		if pool.Policy != want {
			return fmt.Errorf("pool %s stored policy %+v, want %+v", id, pool.Policy, want)
		}

		return nil
	}
}

func TestAccPoolResourcePolicyDefault(t *testing.T) {
	dir := testAccBackend(t)

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{
				Config: testAccConfig(dir, testAccRootPool),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ipam_pool.root", tfjsonpath.New("strategy"), knownvalue.StringExact("best-fit")),
					statecheck.ExpectKnownValue("ipam_pool.root", tfjsonpath.New("cooldown"), knownvalue.Null()),
				},
				Check: testAccCheckPolicy(t, dir, "root", ipam.PoolPolicy{Strategy: ipam.StrategyBestFit}),
			},
			{
				Config: testAccConfig(dir, testAccRootPool),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

const testAccPolicyExplicit = `
resource "ipam_pool" "root" {
  id       = "root"
  cidrs    = ["10.0.0.0/16"]
  strategy = "lowest"
  cooldown = { mode = "strict", duration = %q }
}

resource "ipam_pool" "child" {
  id            = "child"
  parent_pool   = ipam_pool.root.id
  prefix_length = 24
  strategy      = "lowest"
  cooldown      = { mode = "strict", duration = "24h" }
}
`

const testAccPolicyPlain = `
resource "ipam_pool" "plain" {
  id       = "plain"
  cidrs    = ["192.168.0.0/16"]
  strategy = "lowest"
}
`

func TestAccPoolResourcePolicyExplicit(t *testing.T) {
	dir := testAccBackend(t)
	config := func(duration string) string {
		return testAccConfig(dir, fmt.Sprintf(testAccPolicyExplicit, duration))
	}
	strict24h := ipam.PoolPolicy{
		Strategy: ipam.StrategyLowest,
		Cooldown: ipam.Cooldown{Mode: ipam.CooldownStrict, Duration: 24 * time.Hour},
	}

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{
				Config: config("24h"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ipam_pool.root", tfjsonpath.New("strategy"), knownvalue.StringExact("lowest")),
					statecheck.ExpectKnownValue("ipam_pool.root",
						tfjsonpath.New("cooldown").AtMapKey("duration"), knownvalue.StringExact("24h")),
					statecheck.ExpectKnownValue("ipam_pool.child",
						tfjsonpath.New("cooldown").AtMapKey("mode"), knownvalue.StringExact("strict")),
				},
				Check: acc.ComposeAggregateTestCheckFunc(
					testAccCheckPolicy(t, dir, "root", strict24h),
					testAccCheckPolicy(t, dir, "child", strict24h),
				),
			},
			{
				// Import has no configured spelling, so the duration is the
				// canonical Go form.
				ResourceName:                         "ipam_pool.root",
				ImportState:                          true,
				ImportStateId:                        "root",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "id",
				ImportStateVerifyIgnore:              []string{"cooldown.duration"},
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("imported %d states, want 1", len(states))
					}

					attrs := states[0].Attributes
					if attrs["cooldown.duration"] != "24h0m0s" || attrs["cooldown.mode"] != "strict" ||
						attrs["strategy"] != "lowest" {
						return fmt.Errorf("imported attributes %v", attrs)
					}

					return nil
				},
			},
			{
				// A respelling of the same duration updates state only.
				Config: config("1440m"),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipam_pool.root", plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ipam_pool.root",
						tfjsonpath.New("cooldown").AtMapKey("duration"), knownvalue.StringExact("1440m")),
				},
				Check: testAccCheckPolicy(t, dir, "root", strict24h),
			},
			{
				Config: config("1440m") + testAccPolicyPlain,
				Check:  testAccCheckPolicy(t, dir, "plain", ipam.PoolPolicy{Strategy: ipam.StrategyLowest}),
			},
			{
				ResourceName:    "ipam_pool.plain",
				ImportState:     true,
				ImportStateKind: acc.ImportBlockWithResourceIdentity,
			},
		},
	})
}

func TestAccPoolResourcePolicyAdoptsLowestPool(t *testing.T) {
	dir := testAccBackend(t)
	lowest := ipam.PoolPolicy{Strategy: ipam.StrategyLowest}

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{
				PreConfig: func() {
					_, err := testAccClient(t, dir).CreatePool(context.Background(), ipam.PoolSpec{
						ID: "root", CIDRs: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/16")},
						Description: "root pool", Labels: map[string]string{"env": "test"}, Policy: lowest,
					})
					if err != nil {
						t.Fatalf("CreatePool: %v", err)
					}
				},
				Config: testAccConfig(dir, testAccRootPool),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ipam_pool.root", tfjsonpath.New("strategy"), knownvalue.StringExact("lowest")),
				},
				Check: testAccCheckPolicy(t, dir, "root", lowest),
			},
			{
				Config: testAccConfig(dir, testAccRootPool),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:                         "ipam_pool.root",
				ImportState:                          true,
				ImportStateId:                        "root",
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "id",
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 || states[0].Attributes["strategy"] != "lowest" {
						return fmt.Errorf("imported states %v, want one with strategy lowest", states)
					}

					return nil
				},
			},
		},
	})
}

func TestAccPoolResourceAdoptRequiresCooldown(t *testing.T) {
	dir := testAccBackend(t)
	withCooldown := strings.Replace(testAccRootPool, `labels      = { env = "test" }`,
		`labels      = { env = "test" }
  cooldown    = { mode = "strict", duration = "1h" }`, 1)

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{
				PreConfig: func() {
					_, err := testAccClient(t, dir).CreatePool(context.Background(), ipam.PoolSpec{
						ID: "root", CIDRs: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/16")},
						Description: "root pool", Labels: map[string]string{"env": "test"},
						Policy: ipam.PoolPolicy{Cooldown: ipam.Cooldown{Mode: ipam.CooldownStrict, Duration: time.Hour}},
					})
					if err != nil {
						t.Fatalf("CreatePool: %v", err)
					}
				},
				Config:      testAccConfig(dir, testAccRootPool),
				ExpectError: regexp.MustCompile(`(?s)Create pool: conflict.*pool_exists`),
			},
			{
				Config: testAccConfig(dir, withCooldown),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ipam_pool.root",
						tfjsonpath.New("cooldown").AtMapKey("duration"), knownvalue.StringExact("1h")),
				},
			},
			{
				Config: testAccConfig(dir, withCooldown),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

func TestAccPoolResourcePolicyUpdateInPlace(t *testing.T) {
	dir := testAccBackend(t)

	// The allocation makes a replacement fail with pool_not_empty, so every
	// policy change must apply in place.
	config := func(policy string) string {
		return testAccConfig(dir, fmt.Sprintf(`
resource "ipam_pool" "root" {
  id    = "root"
  cidrs = ["10.0.0.0/16"]
%s
}

resource "ipam_allocation" "a" {
  pool          = ipam_pool.root.id
  prefix_length = 24
}
`, policy))
	}
	inPlace := acc.ConfigPlanChecks{
		PreApply: []plancheck.PlanCheck{
			plancheck.ExpectResourceAction("ipam_pool.root", plancheck.ResourceActionUpdate),
			plancheck.ExpectResourceAction("ipam_allocation.a", plancheck.ResourceActionNoop),
		},
	}
	lowest := func(cooldown ipam.Cooldown) ipam.PoolPolicy {
		return ipam.PoolPolicy{Strategy: ipam.StrategyLowest, Cooldown: cooldown}
	}

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckPoolsDestroyed(t, dir),
		Steps: []acc.TestStep{
			{
				Config: config(""),
				Check:  testAccCheckPolicy(t, dir, "root", ipam.PoolPolicy{Strategy: ipam.StrategyBestFit}),
			},
			{
				Config:           config(`  strategy = "lowest"`),
				ConfigPlanChecks: inPlace,
				Check:            testAccCheckPolicy(t, dir, "root", lowest(ipam.Cooldown{})),
			},
			{
				Config: config(`  strategy = "lowest"
  cooldown = { mode = "fit-aware", duration = "1h" }`),
				ConfigPlanChecks: inPlace,
				Check: testAccCheckPolicy(t, dir, "root",
					lowest(ipam.Cooldown{Mode: ipam.CooldownFitAware, Duration: time.Hour})),
			},
			{
				Config: config(`  strategy = "lowest"
  cooldown = { mode = "strict", duration = "2h" }`),
				ConfigPlanChecks: inPlace,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ipam_pool.root",
						tfjsonpath.New("cooldown").AtMapKey("duration"), knownvalue.StringExact("2h")),
				},
				Check: testAccCheckPolicy(t, dir, "root",
					lowest(ipam.Cooldown{Mode: ipam.CooldownStrict, Duration: 2 * time.Hour})),
			},
			{
				Config:           config(`  strategy = "lowest"`),
				ConfigPlanChecks: inPlace,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ipam_pool.root", tfjsonpath.New("cooldown"), knownvalue.Null()),
				},
				Check: testAccCheckPolicy(t, dir, "root", lowest(ipam.Cooldown{Mode: ipam.CooldownNone})),
			},
			{
				// An omitted strategy keeps the stored one.
				Config: config(""),
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("ipam_pool.root", tfjsonpath.New("strategy"), knownvalue.StringExact("lowest")),
				},
				Check: testAccCheckPolicy(t, dir, "root", lowest(ipam.Cooldown{})),
			},
		},
	})
}

func TestAccPoolResourceValidation(t *testing.T) {
	dir := testAccBackend(t)

	cases := []struct {
		name string
		body string
		want string
	}{
		{"cidrs and parent", `
resource "ipam_pool" "p" {
  id          = "p"
  cidrs       = ["10.0.0.0/16"]
  parent_pool = "root"
  prefix_length = 24
}`, `cidrs cannot be configured together with parent_pool`},
		{"root without cidrs", `
resource "ipam_pool" "p" {
  id = "p"
}`, `cidrs must be configured for a root pool`},
		{"prefix_length without parent", `
resource "ipam_pool" "p" {
  id            = "p"
  cidrs         = ["10.0.0.0/16"]
  prefix_length = 24
}`, `prefix_length cannot be configured without parent_pool`},
		{"cidr without parent", `
resource "ipam_pool" "p" {
  id    = "p"
  cidrs = ["10.0.0.0/16"]
  cidr  = "10.0.0.0/24"
}`, `cidr cannot be configured without parent_pool`},
		{"child with both requests", `
resource "ipam_pool" "p" {
  id            = "p"
  parent_pool   = "root"
  prefix_length = 24
  cidr          = "10.0.0.0/24"
}`, `exactly one of prefix_length or cidr`},
		{"child with no request", `
resource "ipam_pool" "p" {
  id          = "p"
  parent_pool = "root"
}`, `exactly one of prefix_length or cidr`},
		{"non-canonical cidrs", `
resource "ipam_pool" "p" {
  id    = "p"
  cidrs = ["10.0.0.1/16"]
}`, `canonical CIDR`},
		{"non-canonical cidr", `
resource "ipam_pool" "p" {
  id          = "p"
  parent_pool = "root"
  cidr        = "10.0.0.1/24"
}`, `canonical CIDR`},
		{"invalid id", `
resource "ipam_pool" "p" {
  id    = "Bad_ID"
  cidrs = ["10.0.0.0/16"]
}`, `lowercase letters, digits and hyphens`},
		{"empty cidrs", `
resource "ipam_pool" "p" {
  id    = "p"
  cidrs = []
}`, `at least 1`},
		{"unknown strategy", `
resource "ipam_pool" "p" {
  id       = "p"
  cidrs    = ["10.0.0.0/16"]
  strategy = "first-fit"
}`, `"lowest" "best-fit"`},
		{"unknown cooldown mode", `
resource "ipam_pool" "p" {
  id       = "p"
  cidrs    = ["10.0.0.0/16"]
  cooldown = { mode = "soft", duration = "1h" }
}`, `"strict" "fit-aware"`},
		{"fractional cooldown", `
resource "ipam_pool" "p" {
  id       = "p"
  cidrs    = ["10.0.0.0/16"]
  cooldown = { mode = "strict", duration = "1.5s" }
}`, `8760h`},
		{"zero cooldown", `
resource "ipam_pool" "p" {
  id       = "p"
  cidrs    = ["10.0.0.0/16"]
  cooldown = { mode = "strict", duration = "0s" }
}`, `8760h`},
		{"cooldown too long", `
resource "ipam_pool" "p" {
  id       = "p"
  cidrs    = ["10.0.0.0/16"]
  cooldown = { mode = "fit-aware", duration = "8761h" }
}`, `8760h`},
		{"cooldown not a duration", `
resource "ipam_pool" "p" {
  id       = "p"
  cidrs    = ["10.0.0.0/16"]
  cooldown = { mode = "strict", duration = "tomorrow" }
}`, `Go duration`},
		{"cooldown without duration", `
resource "ipam_pool" "p" {
  id       = "p"
  cidrs    = ["10.0.0.0/16"]
  cooldown = { mode = "strict" }
}`, `attribute "duration" is`},
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

type storeCall struct {
	op, key               string
	deadline, propagation bool
}

// recordingStore records the context every storage call ran under.
type recordingStore struct {
	storage.Storage

	mu    sync.Mutex
	calls []storeCall
}

func (s *recordingStore) record(ctx context.Context, op, key string) {
	_, deadline := ctx.Deadline()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls = append(s.calls, storeCall{op, key, deadline, storage.AccessPropagationRetry(ctx)})
}

func (s *recordingStore) take() []storeCall {
	s.mu.Lock()
	defer s.mu.Unlock()

	calls := s.calls
	s.calls = nil

	return calls
}

func (s *recordingStore) Get(ctx context.Context, key string) ([]byte, storage.Version, error) {
	s.record(ctx, "get", key)

	return s.Storage.Get(ctx, key)
}

func (s *recordingStore) CreateIfAbsent(ctx context.Context, key string, data []byte) (storage.Version, error) {
	s.record(ctx, "create", key)

	return s.Storage.CreateIfAbsent(ctx, key, data)
}

func (s *recordingStore) PutIfMatch(
	ctx context.Context, key string, data []byte, version storage.Version,
) (storage.Version, error) {
	s.record(ctx, "put", key)

	return s.Storage.PutIfMatch(ctx, key, data, version)
}

func (s *recordingStore) DeleteIfMatch(ctx context.Context, key string, version storage.Version) error {
	s.record(ctx, "delete", key)

	return s.Storage.DeleteIfMatch(ctx, key, version)
}

func (s *recordingStore) List(ctx context.Context, prefix string) ([]string, error) {
	s.record(ctx, "list", prefix)

	return s.Storage.List(ctx, prefix)
}

func requireCalls(t *testing.T, phase string, calls []storeCall, mutation bool) {
	t.Helper()

	if len(calls) == 0 {
		t.Fatalf("%s: no storage calls recorded", phase)
	}

	for _, call := range calls {
		if call.deadline != mutation || call.propagation != mutation {
			t.Fatalf("%s: %s %s ran with deadline=%v propagation=%v, want both %v",
				phase, call.op, call.key, call.deadline, call.propagation, mutation)
		}
	}
}

func poolSchema(t *testing.T) rschema.Schema {
	t.Helper()

	var resp resource.SchemaResponse

	(&poolResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}

	return resp.Schema
}

func stringSet(values ...string) []tftypes.Value {
	elems := make([]tftypes.Value, 0, len(values))
	for _, v := range values {
		elems = append(elems, tftypes.NewValue(tftypes.String, v))
	}

	return elems
}

func stringMap(values map[string]string) map[string]tftypes.Value {
	elems := make(map[string]tftypes.Value, len(values))
	for k, v := range values {
		elems[k] = tftypes.NewValue(tftypes.String, v)
	}

	return elems
}

func TestPoolResourceMutationsUseMutationContext(t *testing.T) {
	ctx := context.Background()
	store := &recordingStore{Storage: memory.New()}
	r := &poolResource{data: &Data{Client: newClient(store), AccessPropagationTimeout: time.Minute}}
	s := poolSchema(t)
	typ := s.Type().TerraformType(ctx)
	null := tftypes.NewValue(typ, nil)

	planned := configValue(t, typ, map[string]any{
		"id": "root", "cidrs": stringSet("10.0.0.0/16"), "family": unknown{},
	})

	var created resource.CreateResponse

	created.State = tfsdk.State{Schema: s, Raw: null}
	r.Create(ctx, resource.CreateRequest{
		Plan:   tfsdk.Plan{Schema: s, Raw: planned},
		Config: tfsdk.Config{Schema: s, Raw: planned},
	}, &created)

	if created.Diagnostics.HasError() {
		t.Fatalf("Create: %v", created.Diagnostics)
	}

	requireCalls(t, "Create", store.take(), true)

	var read resource.ReadResponse

	read.State = created.State
	r.Read(ctx, resource.ReadRequest{State: created.State}, &read)

	if read.Diagnostics.HasError() || read.State.Raw.IsNull() {
		t.Fatalf("Read: %v (null=%v)", read.Diagnostics, read.State.Raw.IsNull())
	}

	requireCalls(t, "Read", store.take(), false)

	updated := configValue(t, typ, map[string]any{
		"id": "root", "cidrs": stringSet("10.0.0.0/16"), "family": int64(4),
		"description": "d", "labels": stringMap(map[string]string{"k": "v"}),
		"strategy": "lowest", "cooldown": map[string]any{"mode": "strict", "duration": "1h"},
	})

	var update resource.UpdateResponse

	update.State = read.State
	r.Update(ctx, resource.UpdateRequest{
		Plan:   tfsdk.Plan{Schema: s, Raw: updated},
		Config: tfsdk.Config{Schema: s, Raw: updated},
		State:  read.State,
	}, &update)

	if update.Diagnostics.HasError() {
		t.Fatalf("Update: %v", update.Diagnostics)
	}

	updateCalls := store.take()
	requireCalls(t, "Update", updateCalls, true)

	// Only UpdatePoolPolicy writes the reservations document on Update.
	if !slices.ContainsFunc(updateCalls, func(c storeCall) bool {
		return c.op == "put" && c.key == "reservations/root.json"
	}) {
		t.Fatalf("Update: no put of reservations/root.json in %+v", updateCalls)
	}

	var deleted resource.DeleteResponse

	r.Delete(ctx, resource.DeleteRequest{State: update.State}, &deleted)

	if deleted.Diagnostics.HasError() {
		t.Fatalf("Delete: %v", deleted.Diagnostics)
	}

	requireCalls(t, "Delete", store.take(), true)

	// Deleting a pool that is already gone succeeds.
	var again resource.DeleteResponse

	r.Delete(ctx, resource.DeleteRequest{State: update.State}, &again)

	if again.Diagnostics.HasError() {
		t.Fatalf("Delete of a missing pool: %v", again.Diagnostics)
	}

	// Read of a missing pool removes it from state.
	var gone resource.ReadResponse

	gone.State = update.State
	r.Read(ctx, resource.ReadRequest{State: update.State}, &gone)

	if gone.Diagnostics.HasError() || !gone.State.Raw.IsNull() {
		t.Fatalf("Read of a missing pool: %v (null=%v)", gone.Diagnostics, gone.State.Raw.IsNull())
	}
}

// TestPoolResourceUpdatePolicyGuards pins that Update reads the stored strategy
// when plan and state lack it (v0.1.x state applied with -refresh=false), keeps
// the planned null strategy in state, and skips UpdatePoolPolicy when the
// policy is unchanged.
func TestPoolResourceUpdatePolicyGuards(t *testing.T) {
	ctx := context.Background()
	store := &recordingStore{Storage: memory.New()}
	r := &poolResource{data: &Data{Client: newClient(store), AccessPropagationTimeout: time.Minute}}
	s := poolSchema(t)
	typ := s.Type().TerraformType(ctx)

	_, err := r.data.Client.CreatePool(ctx, ipam.PoolSpec{
		ID: "root", CIDRs: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/16")},
		Policy: ipam.PoolPolicy{Strategy: ipam.StrategyLowest},
	})
	if err != nil {
		t.Fatalf("CreatePool: %v", err)
	}

	update := func(prior, planned tftypes.Value) resource.UpdateResponse {
		t.Helper()

		store.take()

		state := tfsdk.State{Schema: s, Raw: prior}
		resp := resource.UpdateResponse{State: state}

		r.Update(ctx, resource.UpdateRequest{
			Plan:   tfsdk.Plan{Schema: s, Raw: planned},
			Config: tfsdk.Config{Schema: s, Raw: planned},
			State:  state,
		}, &resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("Update: %v", resp.Diagnostics)
		}

		return resp
	}
	base := map[string]any{"id": "root", "cidrs": stringSet("10.0.0.0/16"), "family": int64(4)}
	with := func(extra map[string]any) tftypes.Value {
		attrs := map[string]any{}
		for k, v := range base {
			attrs[k] = v
		}

		for k, v := range extra {
			attrs[k] = v
		}

		return configValue(t, typ, attrs)
	}
	strict := map[string]any{"mode": "strict", "duration": "1h"}

	resp := update(with(nil), with(map[string]any{"cooldown": strict}))

	pool, err := r.data.Client.GetPool(ctx, "root")
	if err != nil {
		t.Fatalf("GetPool: %v", err)
	}

	want := ipam.PoolPolicy{Strategy: ipam.StrategyLowest, Cooldown: ipam.Cooldown{Mode: ipam.CooldownStrict, Duration: time.Hour}}
	if pool.Policy != want {
		t.Fatalf("stored policy %+v, want %+v", pool.Policy, want)
	}

	var got poolResourceModel
	if diags := resp.State.Get(ctx, &got); diags.HasError() || !got.Strategy.IsNull() {
		t.Fatalf("state strategy %v (%v), want the planned null", got.Strategy, diags)
	}

	// A description-only edit never reaches UpdatePoolPolicy, which is the
	// only Update path that reads the reservations document.
	same := map[string]any{"strategy": "lowest", "cooldown": strict}
	planned := map[string]any{"strategy": "lowest", "cooldown": strict, "description": "d"}
	update(with(same), with(planned))

	for _, call := range store.take() {
		if call.key == "reservations/root.json" {
			t.Fatalf("metadata-only Update touched %s with %s", call.key, call.op)
		}
	}
}

// deniedStore fails every call with access denied.
type deniedStore struct{ storage.Storage }

func (deniedStore) Get(context.Context, string) ([]byte, storage.Version, error) {
	return nil, "", storage.ErrAccessDenied
}

func TestPoolResourceReadErrorIsDiagnostic(t *testing.T) {
	ctx := context.Background()
	r := &poolResource{data: &Data{Client: newClient(deniedStore{memory.New()}), AccessPropagationTimeout: time.Minute}}
	s := poolSchema(t)
	state := configValue(t, s.Type().TerraformType(ctx), map[string]any{
		"id": "root", "cidrs": stringSet("10.0.0.0/16"), "family": int64(4),
	})

	var resp resource.ReadResponse

	resp.State = tfsdk.State{Schema: s, Raw: state}
	r.Read(ctx, resource.ReadRequest{State: resp.State}, &resp)

	requireDiag(t, resp.Diagnostics, "Read pool: access denied")

	if resp.State.Raw.IsNull() {
		t.Fatal("Read removed the pool from state on access denied")
	}
}
