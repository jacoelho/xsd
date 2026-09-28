package validate

import (
	"errors"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
)

func TestIdentityScopeDispatchIndexUnregistersNestedReuseAndPartialAbort(t *testing.T) {
	t.Parallel()

	fixture := startedIdentityEvaluationForTest(t)
	evaluation := fixture.evaluation
	constraint := evaluation.selections[0].constraint
	constraints := evaluation.scopes[0].constraints
	rootName := xsdSchema.RuntimeName{Known: true, Name: fixture.elemName}
	hasScopeHit := func(hits []identitySelectorHit, scope int) bool {
		for _, hit := range hits {
			if hit.scope == scope && hit.constraint == constraint {
				return true
			}
		}
		return false
	}

	evaluation.scopes = append(evaluation.scopes, identityScope{depth: 2, constraints: constraints})
	if err := evaluation.registerIdentityScope(1); err != nil {
		t.Fatalf("register nested scope: %v", err)
	}
	evaluation.path = []xsdSchema.RuntimeName{rootName, rootName}
	hits := evaluation.appendIdentitySelectorHits(nil, evaluation.dispatch.index.SelfSelectors(), 2)
	if !hasScopeHit(hits, 1) {
		t.Fatalf("nested selector hits = %+v, want child scope", hits)
	}
	if _, err := evaluation.closeScopes(2, nil); err != nil {
		t.Fatalf("close nested scope: %v", err)
	}
	evaluation.path = evaluation.path[:1]
	hits = evaluation.appendIdentitySelectorHits(nil, evaluation.dispatch.index.SelfSelectors(), 1)
	if !hasScopeHit(hits, 0) {
		t.Fatalf("selector hits after nested close = %+v, want parent scope", hits)
	}

	// The popped index is reused by the next nested scope; cleanup must remove
	// only that new membership and leave the parent entry live.
	evaluation.scopes = append(evaluation.scopes, identityScope{depth: 2, constraints: constraints})
	if err := evaluation.registerIdentityScope(1); err != nil {
		t.Fatalf("register reused nested scope: %v", err)
	}
	evaluation.path = []xsdSchema.RuntimeName{rootName, rootName}
	hits = evaluation.appendIdentitySelectorHits(nil, evaluation.dispatch.index.SelfSelectors(), 2)
	if !hasScopeHit(hits, 1) {
		t.Fatalf("reused nested selector hits = %+v, want child scope", hits)
	}
	if _, err := evaluation.closeScopes(2, nil); err != nil {
		t.Fatalf("close reused nested scope: %v", err)
	}
	evaluation.path = evaluation.path[:1]
	hits = evaluation.appendIdentitySelectorHits(nil, evaluation.dispatch.index.SelfSelectors(), 1)
	if !hasScopeHit(hits, 0) {
		t.Fatalf("selector hits after reused close = %+v, want parent scope", hits)
	}

	if err := evaluation.beginStart(); err != nil {
		t.Fatalf("beginStart(partial scope): %v", err)
	}
	partial, ok := xsdSchema.ElementIdentityConstraintIDs([][]xsdSchema.IdentityConstraintID{{constraint, xsdSchema.NoIdentityConstraint}}, 0)
	if !ok {
		t.Fatal("constructing partial scope constraints failed")
	}
	evaluation.scopes = append(evaluation.scopes, identityScope{depth: 2, constraints: partial})
	if err := evaluation.registerIdentityScope(1); err == nil {
		t.Fatal("register partial scope succeeded, want metadata error")
	}
	evaluation.path = []xsdSchema.RuntimeName{rootName, rootName}
	hits = evaluation.appendIdentitySelectorHits(nil, evaluation.dispatch.index.SelfSelectors(), 2)
	if !hasScopeHit(hits, 1) {
		t.Fatalf("partial-registration selector hits = %+v, want child scope", hits)
	}
	evaluation.abortStart()
	if len(evaluation.scopes) != 1 {
		t.Fatalf("scopes after partial abort = %d, want parent only", len(evaluation.scopes))
	}
	hits = evaluation.appendIdentitySelectorHits(nil, evaluation.dispatch.index.SelfSelectors(), 1)
	if !hasScopeHit(hits, 0) {
		t.Fatalf("selector hits after partial abort = %+v, want parent scope", hits)
	}

	// A failed keyref report leaves the scope registered and dispatchable. The
	// next close, after recovery accepts the report, removes that scope.
	evaluation.scopes = append(evaluation.scopes, identityScope{
		depth:       2,
		constraints: constraints,
		refs: []identityTupleRef{{
			key:   "missing",
			path:  explicitRetainedPath("/root/ref"),
			line:  1,
			col:   1,
			refer: constraint,
		}},
	})
	if err := evaluation.registerIdentityScope(1); err != nil {
		t.Fatalf("register unresolved-ref scope: %v", err)
	}
	evaluation.path = []xsdSchema.RuntimeName{rootName, rootName}
	stop := errors.New("stop keyref recovery")
	if _, err := evaluation.closeScopes(2, func(error) error { return stop }); !errors.Is(err, stop) {
		t.Fatalf("close unresolved-ref scope = %v, want %v", err, stop)
	}
	hits = evaluation.appendIdentitySelectorHits(nil, evaluation.dispatch.index.SelfSelectors(), 2)
	if !hasScopeHit(hits, 1) {
		t.Fatalf("selector hits after failed close = %+v, want still-dispatchable child", hits)
	}
	if _, err := evaluation.closeScopes(2, func(error) error { return nil }); err != nil {
		t.Fatalf("close unresolved-ref scope after recovery: %v", err)
	}
	hits = evaluation.appendIdentitySelectorHits(nil, evaluation.dispatch.index.SelfSelectors(), 2)
	if hasScopeHit(hits, 1) {
		t.Fatalf("selector hits after successful close = %+v, want child removed", hits)
	}
}
