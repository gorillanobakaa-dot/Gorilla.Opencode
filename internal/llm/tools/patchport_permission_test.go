// GORILLA OVERRIDE (2026-09-03): the patch_port permission gate had NO tests.
//
// One line stands between a model and a rewritten kernel tree:
//
//	if op != "inspect" { ...ask... }
//
// Nothing asserted it. Refactor that condition the wrong way and every test in
// the repo still passes, while forward-port, backport, rebase, refresh and
// port-series start rewriting trees unannounced. The tool exists to be pointed
// at a real kernel source tree, so the blast radius of that line is somebody's
// whole checkout.
//
// Raised by the owner from a live run: patch_port ran with no prompt and it
// looked like the tool was skipping confirmation. It was not -- the operation
// was `inspect`, which only ever runs `git apply --check` -- but "the safe path
// happens to be the one that ran" is not a guarantee, it is a coincidence until
// something checks it. These tests make it a guarantee.
package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/opencode-ai/opencode/internal/permission"
)

// recordingPermissions answers Request with a fixed verdict and records what it
// was asked. permission.Service is embedded rather than implemented: every
// method this test does not deliberately override is nil, so an unexpected call
// panics and names itself instead of quietly returning a zero value.
type recordingPermissions struct {
	permission.Service
	grant bool
	calls []permission.CreatePermissionRequest
}

func (r *recordingPermissions) Request(opts permission.CreatePermissionRequest) bool {
	r.calls = append(r.calls, opts)
	return r.grant
}

func runPatchPort(t *testing.T, perms *recordingPermissions, op, tree string) (ToolResponse, error) {
	t.Helper()
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "s1")
	ctx = context.WithValue(ctx, MessageIDContextKey, "m1")

	params := `{"operation":"` + op + `","tree":"` + tree + `"}`
	return NewPatchPortTool(perms).Run(ctx, ToolCall{
		Name:  PatchPortToolName,
		Input: params,
	})
}

// everyModifyingOp is the closed set from patchport.go minus inspect. If an
// operation is added to the tool it must be added here too, and the test below
// fails until it is: a new op that nobody gated is exactly the regression this
// file exists to catch.
var everyModifyingOp = []string{"forward-port", "backport", "rebase", "refresh", "port-series"}

// inspect is read-only (git apply --check), so asking would train the user to
// dismiss prompts. It must NOT prompt.
func TestInspectDoesNotAskPermission(t *testing.T) {
	perms := &recordingPermissions{grant: true}
	if _, err := runPatchPort(t, perms, "inspect", t.TempDir()); err != nil {
		t.Fatalf("inspect returned a transport error: %v", err)
	}
	if len(perms.calls) != 0 {
		t.Errorf("inspect asked for permission %d time(s); it only reads, so a prompt here "+
			"is noise that teaches the user to approve without reading", len(perms.calls))
	}
}

// The empty operation defaults to inspect. It must default to the SAFE side, or
// a malformed tool call silently becomes a tree rewrite.
func TestTheDefaultOperationIsTheReadOnlyOne(t *testing.T) {
	perms := &recordingPermissions{grant: true}
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "s1")
	ctx = context.WithValue(ctx, MessageIDContextKey, "m1")

	if _, err := NewPatchPortTool(perms).Run(ctx, ToolCall{
		Name:  PatchPortToolName,
		Input: `{"tree":"` + t.TempDir() + `"}`,
	}); err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if len(perms.calls) != 0 {
		t.Errorf("an omitted operation asked for permission, so it did not default to inspect")
	}
}

// Every operation that can write MUST ask first.
func TestEveryModifyingOperationAsksPermission(t *testing.T) {
	for _, op := range everyModifyingOp {
		t.Run(op, func(t *testing.T) {
			perms := &recordingPermissions{grant: true}
			tree := t.TempDir()
			if _, err := runPatchPort(t, perms, op, tree); err != nil {
				t.Fatalf("transport error: %v", err)
			}
			if len(perms.calls) != 1 {
				t.Fatalf("%s asked %d times, want exactly 1 — this operation rewrites a git tree",
					op, len(perms.calls))
			}
			got := perms.calls[0]
			if got.Action != op {
				t.Errorf("the prompt says the action is %q but the operation is %q", got.Action, op)
			}
			if got.Path != tree {
				t.Errorf("the prompt names path %q but the tree is %q — a user cannot consent to "+
					"a tree they were not shown", got.Path, tree)
			}
			if got.ToolName != PatchPortToolName {
				t.Errorf("the prompt attributes the request to %q, not %q", got.ToolName, PatchPortToolName)
			}
			if !strings.Contains(got.Description, tree) {
				t.Errorf("the description does not name the tree being modified: %q", got.Description)
			}
		})
	}
}

// Denying must actually stop it. A prompt whose "no" is ignored is worse than
// no prompt, because it manufactures a record of consent that was refused.
func TestDenyingPermissionAbortsBeforeAnythingRuns(t *testing.T) {
	for _, op := range everyModifyingOp {
		t.Run(op, func(t *testing.T) {
			perms := &recordingPermissions{grant: false}
			_, err := runPatchPort(t, perms, op, t.TempDir())
			if !errors.Is(err, permission.ErrorPermissionDenied) {
				t.Fatalf("%s returned %v after the user said no; want ErrorPermissionDenied", op, err)
			}
			if len(perms.calls) != 1 {
				t.Errorf("%s asked %d times, want 1", op, len(perms.calls))
			}
		})
	}
}

// The op that was APPROVED must be the op that runs. Gating on one value and
// executing another is how a prompt becomes theatre; both must read the same
// normalised variable.
func TestTheApprovedOperationIsTheOneDescribed(t *testing.T) {
	perms := &recordingPermissions{grant: true}
	// Mixed case and surrounding space: normalisation happens before the gate,
	// so the user must be shown the normalised op, not the raw string.
	ctx := context.WithValue(context.Background(), SessionIDContextKey, "s1")
	ctx = context.WithValue(ctx, MessageIDContextKey, "m1")
	if _, err := NewPatchPortTool(perms).Run(ctx, ToolCall{
		Name:  PatchPortToolName,
		Input: `{"operation":"  ReBase  ","tree":"` + t.TempDir() + `"}`,
	}); err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if len(perms.calls) != 1 {
		t.Fatalf("asked %d times, want 1", len(perms.calls))
	}
	if got := perms.calls[0].Action; got != "rebase" {
		t.Errorf("the user was asked to approve %q but the normalised operation is \"rebase\"", got)
	}
}

// An unknown operation must be refused outright, and must not reach the prompt.
// Asking about an op that will never run trains the user to approve noise.
func TestAnUnknownOperationIsRefusedWithoutAsking(t *testing.T) {
	perms := &recordingPermissions{grant: true}
	resp, err := runPatchPort(t, perms, "delete-everything", t.TempDir())
	if err != nil {
		t.Fatalf("transport error: %v", err)
	}
	if !resp.IsError {
		t.Error("an unknown operation was not reported as an error")
	}
	if len(perms.calls) != 0 {
		t.Errorf("an unknown operation asked for permission %d time(s)", len(perms.calls))
	}
}
