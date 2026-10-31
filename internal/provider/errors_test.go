// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/appvia/ipam"
	"github.com/appvia/ipam/storage"
)

func TestProviderErrorDiagnostic(t *testing.T) {
	tests := map[string]struct {
		err         error
		wantSummary string
		wantDetail  []string
	}{
		"not found": {
			err:         &ipam.OpError{Op: "get pool", Pool: "p", Err: ipam.ErrNotFound},
			wantSummary: "Create pool: not found",
		},
		"conflict with reason": {
			err:         &ipam.OpError{Op: "allocate", Pool: "p", Reason: "id_reused", Err: ipam.ErrConflict},
			wantSummary: "Create pool: conflict",
			wantDetail:  []string{"Reason: id_reused"},
		},
		"wrapped pool not empty": {
			err:         fmt.Errorf("outer: %w", &ipam.OpError{Op: "delete pool", Reason: "pool_not_empty", Err: ipam.ErrConflict}),
			wantSummary: "Create pool: conflict",
			wantDetail:  []string{"Reason: pool_not_empty"},
		},
		"exhausted":    {err: ipam.ErrPoolExhausted, wantSummary: "Create pool: pool exhausted", wantDetail: []string{"smaller prefix"}},
		"invalid cidr": {err: ipam.ErrInvalidCIDR, wantSummary: "Create pool: invalid CIDR"},
		"invalid arg":  {err: ipam.ErrInvalidArgument, wantSummary: "Create pool: invalid argument"},
		"schema":       {err: ipam.ErrUnsupportedSchema, wantSummary: "Create pool: unsupported document schema", wantDetail: []string{"never overwritten"}},
		"denied":       {err: fmt.Errorf("x: %w", storage.ErrAccessDenied), wantSummary: "Create pool: access denied", wantDetail: []string{"access_propagation_timeout"}},
		"unknown":      {err: storage.ErrUnknownOutcome, wantSummary: "Create pool: unknown outcome", wantDetail: []string{"idempotent"}},
		"deadline":     {err: fmt.Errorf("x: %w", context.DeadlineExceeded), wantSummary: "Create pool: timed out"},
		"cancelled":    {err: context.Canceled, wantSummary: "Create pool: cancelled"},
		"other":        {err: errors.New("boom"), wantSummary: "Create pool: failed", wantDetail: []string{"boom"}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			d := errorDiagnostic("Create pool", tc.err)

			if d.Severity() != diag.SeverityError {
				t.Fatalf("severity = %v", d.Severity())
			}

			if d.Summary() != tc.wantSummary {
				t.Fatalf("summary = %q, want %q", d.Summary(), tc.wantSummary)
			}

			if !strings.HasPrefix(d.Detail(), tc.err.Error()) {
				t.Fatalf("detail %q does not start with the error %q", d.Detail(), tc.err.Error())
			}

			for _, want := range tc.wantDetail {
				if !strings.Contains(d.Detail(), want) {
					t.Fatalf("detail %q lacks %q", d.Detail(), want)
				}
			}

			if !strings.Contains(d.Detail(), "Reason:") && len(tc.wantDetail) > 0 &&
				strings.HasPrefix(tc.wantDetail[0], "Reason:") {
				t.Fatalf("detail %q lacks the reason", d.Detail())
			}
		})
	}
}
