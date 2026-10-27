package validate

import (
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
)

func TestIdentityTupleRefIdleStorageBoundsAndTransfer(t *testing.T) {
	t.Parallel()
	for _, capacity := range []int{4096, 4097} {
		refs := make([]identityTupleRef, capacity)
		refs[capacity-1] = identityTupleRef{key: "old", path: explicitRetainedPath("/old"), line: 2, col: 3, refer: 4}
		var state identityState
		state.retainScopeRefs(refs, 4096)
		if capacity == 4097 {
			if state.idleRefs != nil {
				t.Fatal("oversized reference storage retained")
			}
			continue
		}
		if len(state.idleRefs) != 0 || cap(state.idleRefs) != 4096 {
			t.Fatalf("idle refs len/cap = %d/%d, want 0/4096", len(state.idleRefs), cap(state.idleRefs))
		}
		for _, ref := range state.idleRefs[:cap(state.idleRefs)] {
			if ref != (identityTupleRef{}) {
				t.Fatal("retained backing holds document references")
			}
		}
		var scope identityScope
		ctx := identityTestContext("/new", 5, 6)
		if err := state.publishIdentityKeyRef(&scope, 7, identitySelection{depth: 1}, "new", identityLimits{}, ctx); err != nil {
			t.Fatal(err)
		}
		if state.idleRefs != nil || len(scope.refs) != 1 || &scope.refs[0] != &refs[0] {
			t.Fatal("borrow did not transfer the one idle backing to the active scope")
		}
		state.scopes = []identityScope{scope}
		state.reset(4096, 4096)
		if len(state.scopes) != 0 || len(state.idleRefs) != 0 || cap(state.idleRefs) != 4096 || refs[0] != (identityTupleRef{}) {
			t.Fatal("reset did not reclaim and clear the active reference storage")
		}
		evaluation := identityEvaluation{identityState: state}
		evaluation.discard()
		if evaluation.idleRefs != nil {
			t.Fatal("semantic discard retained idle reference storage")
		}
	}
}

func TestIdentityTupleRefIdleStoragePrefersLargerAndPreservesAdmission(t *testing.T) {
	t.Parallel()
	var state identityState
	larger := make([]identityTupleRef, 1, 8)
	larger[0] = identityTupleRef{key: "outer"}
	state.retainScopeRefs(make([]identityTupleRef, 1, 2), 4096)
	state.retainScopeRefs(larger, 4096)
	state.retainScopeRefs(make([]identityTupleRef, 1, 4), 4096)
	if cap(state.idleRefs) != 8 || &state.idleRefs[:8][0] != &larger[0] {
		t.Fatal("smaller closed scope displaced larger idle backing")
	}
	state.entries = 1
	var scope identityScope
	if err := state.publishIdentityKeyRef(&scope, xsdSchema.IdentityConstraintID(1), identitySelection{}, "rejected", identityLimits{Entries: 1}, identityTestContext("/rejected", 1, 1)); err == nil {
		t.Fatal("identity admission exceeded configured limit")
	}
	if scope.refs != nil || cap(state.idleRefs) != 8 {
		t.Fatal("failed admission borrowed idle storage")
	}
}
