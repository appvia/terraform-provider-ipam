// Copyright 2026 Appvia Ltd.
// SPDX-License-Identifier: Apache-2.0

package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/diag"

	"github.com/appvia/ipam"
	"github.com/appvia/ipam/storage"
)

var errorKinds = []struct {
	err     error
	summary string
	hint    string
}{
	{ipam.ErrNotFound, "not found", ""},
	{ipam.ErrConflict, "conflict", ""},
	{ipam.ErrPoolExhausted, "pool exhausted", "No free CIDR of the requested size is left; use a smaller prefix or another pool."},
	{ipam.ErrInvalidCIDR, "invalid CIDR", ""},
	{ipam.ErrInvalidArgument, "invalid argument", ""},
	{ipam.ErrUnsupportedSchema, "unsupported document schema", "The document was written by a newer ipam version; upgrade the provider. It is never overwritten."},
	{storage.ErrAccessDenied, "access denied", "Check the identity's IAM policy or Azure role assignments. New assignments can take minutes to propagate; Create, Update and Delete retry for access_propagation_timeout."},
	{storage.ErrUnknownOutcome, "unknown outcome", "The write may or may not have been applied. Run the operation again; it is idempotent."},
	{context.DeadlineExceeded, "timed out", "The operation did not finish within access_propagation_timeout."},
	{context.Canceled, "cancelled", ""},
}

// residueHint explains the usual cause of a pool Create or Delete that only
// times out: documents left by an interrupted create or delete, which the CLI
// reports at once as pool_incomplete. Allocations never time out on residue;
// they fail at once with not found, so the hint is for pool operations only.
// Each command named here was checked against the CLI on both residue shapes.
const residueHint = "If an earlier apply was interrupted while creating or deleting this pool, the pool may be " +
	"partially created (pool_incomplete). `ipam pool get <id>` reports such a pool as not found; for a child " +
	"pool, `ipam alloc list <parent>` still shows its carve-out `pool:<child>:<incarnation>`. Run " +
	"`ipam pool delete <id>` to clear the pool's own documents. If `ipam alloc list <parent>` still shows the " +
	"carve-out afterwards, release it by its full ID with " +
	"`ipam alloc release <parent> pool:<child>:<incarnation>`, but only once no create of that child is " +
	"running anywhere. Then apply again."

// residueOperations are the operations a pool's residue makes time out.
var residueOperations = map[string]bool{"Create pool": true, "Delete pool": true}

var reasonHints = map[string]string{
	"pool_not_empty": "Release its allocations and delete its child pools first; Terraform destroys dependants " +
		"before their parent only when they are in the same configuration.",
	"pool_exists": "A different pool with this ID already exists. Import it, or choose another ID.",
	"overlap":     "The requested CIDR overlaps an existing allocation or child pool in this pool; choose a free CIDR.",
	"pool_incomplete": "An interrupted create or delete left partial documents for this pool. Run " +
		"`ipam pool delete <id>` to clear them, then retry.",
	"cooldown": "Strict cooldown holds recently released CIDRs back from next requests; the error names when " +
		"cooling space becomes usable. Wait until then, request an explicit cidr (explicit requests bypass " +
		"cooldown), or switch the pool's cooldown mode to fit-aware or remove it.",
}

// errorDiagnostic maps an ipam error onto a diagnostic whose summary names the
// error kind and whose detail carries the error, the OpError Reason and a hint.
func errorDiagnostic(operation string, err error) diag.Diagnostic {
	summary, hint := "failed", ""

	for _, kind := range errorKinds {
		if errors.Is(err, kind.err) {
			summary, hint = kind.summary, kind.hint

			break
		}
	}

	detail := err.Error()

	var opErr *ipam.OpError
	if errors.As(err, &opErr) && opErr.Reason != "" {
		detail += "\nReason: " + opErr.Reason

		if reasonHint, ok := reasonHints[opErr.Reason]; ok {
			hint = reasonHint
		}
	}

	if residueOperations[operation] && errors.Is(err, context.DeadlineExceeded) {
		hint += " " + residueHint
	}

	if hint != "" {
		detail += "\n\n" + hint
	}

	return diag.NewErrorDiagnostic(operation+": "+summary, detail)
}
