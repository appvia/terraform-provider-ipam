// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	acc "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/appvia/ipam"
	"github.com/appvia/ipam/storage"
)

// releaseFailStore fails writes to reservation documents with err while fail
// is set, so a release cannot commit.
type releaseFailStore struct {
	storage.Storage

	err  error
	fail atomic.Bool
}

func (s *releaseFailStore) PutIfMatch(
	ctx context.Context, key string, data []byte, version storage.Version,
) (storage.Version, error) {
	if s.fail.Load() && strings.HasPrefix(key, "reservations/") {
		return "", s.err
	}

	return s.Storage.PutIfMatch(ctx, key, data, version)
}

func (s *releaseFailStore) DeleteIfMatch(ctx context.Context, key string, version storage.Version) error {
	if s.fail.Load() && strings.HasPrefix(key, "reservations/") {
		return s.err
	}

	return s.Storage.DeleteIfMatch(ctx, key, version)
}

// A release that fails for any reason other than a plain not-found must fail
// Delete: swallowing it would drop the resource from state and leak the
// allocation in the store.
func TestAllocationResourceDeleteFailureIsDiagnostic(t *testing.T) {
	cases := map[string]error{
		"access denied": fmt.Errorf("injected: %w", storage.ErrAccessDenied),
		"io error":      errors.New("injected: input/output error"),
	}

	for name, injected := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			store := &releaseFailStore{Storage: newLocalStore(t), err: injected}
			createPool(t, newClient(store))

			h := newAllocationHarness(t, store)
			h.r.data.AccessPropagationTimeout = time.Second

			created := h.create(map[string]any{"pool": "root", "prefix_length": int64(24)})
			if created.Diagnostics.HasError() {
				t.Fatalf("Create: %v", created.Diagnostics)
			}

			id := stateString(t, created.State, "id")

			store.fail.Store(true)

			var failed resource.DeleteResponse

			failed.State = created.State
			h.r.Delete(ctx, resource.DeleteRequest{State: created.State}, &failed)

			if !failed.Diagnostics.HasError() {
				t.Fatal("Delete succeeded although the release could not be written")
			}

			requireDiag(t, failed.Diagnostics, "Delete allocation: ")

			if _, err := newClient(store.Storage).GetAllocation(ctx, "root", id); err != nil {
				t.Fatalf("allocation %s missing from the store after a failed release: %v", id, err)
			}

			store.fail.Store(false)

			var released resource.DeleteResponse

			h.r.Delete(ctx, resource.DeleteRequest{State: created.State}, &released)

			if released.Diagnostics.HasError() {
				t.Fatalf("Delete after the store recovered: %v", released.Diagnostics)
			}

			if _, err := newClient(store.Storage).GetAllocation(ctx, "root", id); !errors.Is(err, ipam.ErrNotFound) {
				t.Fatalf("allocation %s still stored after Delete: %v", id, err)
			}

			// Already released: success.
			var again resource.DeleteResponse

			h.r.Delete(ctx, resource.DeleteRequest{State: created.State}, &again)

			if again.Diagnostics.HasError() {
				t.Fatalf("Delete of a released allocation: %v", again.Diagnostics)
			}

			// Pool gone: success, even with reservation writes failing,
			// because nothing is left to write.
			if err := newClient(store).DeletePool(ctx, "root"); err != nil {
				t.Fatalf("DeletePool: %v", err)
			}

			store.fail.Store(true)

			var poolGone resource.DeleteResponse

			h.r.Delete(ctx, resource.DeleteRequest{State: created.State}, &poolGone)

			if poolGone.Diagnostics.HasError() {
				t.Fatalf("Delete after the pool was deleted: %v", poolGone.Diagnostics)
			}
		})
	}
}

// TestAccAllocationResourceDeleteFailureKeepsState destroys an allocation
// while the local backend's reservations directory is read-only: destroy must
// fail, Terraform must keep the resource in state and the allocation must stay
// in the store; once writable again, destroy releases it.
func TestAccAllocationResourceDeleteFailureKeepsState(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("directory permissions do not restrict root")
	}

	dir := testAccBackend(t)
	testAccPool(t, dir, "root", "10.0.0.0/16")

	reservations := filepath.Join(dir, "reservations")

	t.Cleanup(func() { _ = os.Chmod(reservations, 0o755) })

	provider := fmt.Sprintf("provider \"ipam\" {\n  local = { path = %q }\n  access_propagation_timeout = \"1s\"\n}\n", dir)
	config := provider + `
resource "ipam_allocation" "a" {
  pool          = "root"
  prefix_length = 24
}
`

	var id string

	requireStored := func() {
		t.Helper()

		allocations := testAccAllocations(t, dir, "root")
		if len(allocations) != 1 || allocations[0].ID != id {
			t.Fatalf("store holds %+v, want only allocation %s", allocations, id)
		}
	}

	acc.Test(t, acc.TestCase{
		ProtoV6ProviderFactories: testAccFactories(),
		CheckDestroy:             testAccCheckReleased(t, dir, "root"),
		Steps: []acc.TestStep{
			{
				Config: config,
				Check: func(s *terraform.State) error {
					id = s.RootModule().Resources["ipam_allocation.a"].Primary.Attributes["id"]

					return nil
				},
			},
			{
				PreConfig: func() {
					requireStored()

					if err := os.Chmod(reservations, 0o555); err != nil {
						t.Fatalf("chmod: %v", err)
					}
				},
				Config:      provider,
				ExpectError: regexp.MustCompile(`Delete allocation: (access denied|timed out)`),
			},
			{
				PreConfig: func() {
					if err := os.Chmod(reservations, 0o755); err != nil {
						t.Fatalf("chmod: %v", err)
					}

					requireStored()
				},
				Config: config,
				ConfigPlanChecks: acc.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("ipam_allocation.a", plancheck.ResourceActionNoop),
					},
				},
				Check: func(s *terraform.State) error {
					if got := s.RootModule().Resources["ipam_allocation.a"].Primary.Attributes["id"]; got != id {
						return fmt.Errorf("state id %s after the failed destroy, want %s", got, id)
					}

					return nil
				},
			},
		},
	})
}
