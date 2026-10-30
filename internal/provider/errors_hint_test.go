// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/appvia/ipam"
	"github.com/appvia/ipam/storage"
)

func TestErrorDiagnosticTimeoutHintsAtResidue(t *testing.T) {
	timeouts := map[string]error{
		"wrapped deadline":  fmt.Errorf("create pool kid: %w", context.DeadlineExceeded),
		"op error deadline": &ipam.OpError{Op: "delete pool", Pool: "kid", Err: context.DeadlineExceeded},
	}

	for _, operation := range []string{"Create pool", "Delete pool"} {
		for name, err := range timeouts {
			t.Run(operation+"/"+name, func(t *testing.T) {
				d := errorDiagnostic(operation, err)

				if d.Summary() != operation+": timed out" {
					t.Fatalf("summary = %q", d.Summary())
				}

				for _, want := range []string{
					"access_propagation_timeout",
					"pool_incomplete",
					"not found",
					"ipam alloc list <parent>",
					"ipam pool delete <id>",
					"ipam alloc release <parent> pool:<child>:<incarnation>",
				} {
					if !strings.Contains(d.Detail(), want) {
						t.Fatalf("detail %q lacks %q", d.Detail(), want)
					}
				}
			})
		}
	}
}

func TestErrorDiagnosticResidueHintOnlyForPoolTimeouts(t *testing.T) {
	deadline := fmt.Errorf("x: %w", context.DeadlineExceeded)

	cases := map[string]struct {
		operation string
		err       error
	}{
		"cancelled":  {"Create pool", context.Canceled},
		"exhausted":  {"Create pool", ipam.ErrPoolExhausted},
		"other":      {"Create pool", errors.New("boom")},
		"conflict":   {"Create pool", &ipam.OpError{Op: "create pool", Pool: "kid", Reason: "pool_exists", Err: ipam.ErrConflict}},
		"not found":  {"Delete pool", &ipam.OpError{Op: "delete pool", Pool: "kid", Err: ipam.ErrNotFound}},
		"denied":     {"Create pool", fmt.Errorf("x: %w", storage.ErrAccessDenied)},
		"alloc wait": {"Create allocation", deadline},
		"pool read":  {"Read pool", deadline},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			d := errorDiagnostic(tc.operation, tc.err)
			if strings.Contains(d.Detail(), "pool_incomplete") || strings.Contains(d.Detail(), "pool:<child>") {
				t.Fatalf("detail %q carries the residue hint", d.Detail())
			}
		})
	}
}
