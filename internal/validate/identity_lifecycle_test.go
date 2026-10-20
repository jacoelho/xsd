package validate

import (
	"errors"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/xsderrors"
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
			key:   "conflict",
			path:  explicitRetainedPath("/root/ref"),
			line:  1,
			col:   1,
			refer: constraint,
		}},
	})
	for index := range 2 {
		child := identityScope{depth: 3}
		selection := identitySelection{constraint: constraint, node: uint64(index + 10), depth: 4}
		if err := evaluation.publishIdentityKey(&child, selection, "conflict", identityLimits{}, identityTestContext("/root/branch/row", 1, 1)); err != nil {
			t.Fatal(err)
		}
		if index == 0 {
			selection.node = 20
			if err := evaluation.publishIdentityKey(&child, selection, "kept", identityLimits{}, identityTestContext("/root/branch/other", 1, 1)); err != nil {
				t.Fatal(err)
			}
		}
		mergeIdentityTables(&evaluation.scopes[1], &child)
	}
	entriesBeforeClose := evaluation.entries
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
	entry, exists := evaluation.scopes[1].tables[constraint]["conflict"]
	if !exists || !entry.conflict {
		t.Fatal("failed close completed the still-open conflict")
	}
	if _, exists := evaluation.scopes[0].tables[constraint]["kept"]; exists {
		t.Fatal("failed close propagated child entries")
	}
	reported := 0
	if _, err := evaluation.closeScopes(2, func(error) error { reported++; return nil }); err != nil {
		t.Fatalf("close unresolved-ref scope after recovery: %v", err)
	}
	if reported != 1 || evaluation.entries != entriesBeforeClose {
		t.Fatalf("completed close reports = %d, entries = %d; want 1 and %d", reported, evaluation.entries, entriesBeforeClose)
	}
	hits = evaluation.appendIdentitySelectorHits(nil, evaluation.dispatch.index.SelfSelectors(), 2)
	if hasScopeHit(hits, 1) {
		t.Fatalf("selector hits after successful close = %+v, want child removed", hits)
	}
	other := identityScope{depth: 2}
	if err := evaluation.publishIdentityKey(&other, identitySelection{constraint: constraint, node: 30, depth: 3}, "conflict", identityLimits{}, identityTestContext("/root/other/row", 1, 1)); err != nil {
		t.Fatal(err)
	}
	mergeIdentityTables(&evaluation.scopes[0], &other)
	parent := &evaluation.scopes[0]
	parent.refs = []identityTupleRef{{refer: constraint, key: "conflict"}, {refer: constraint, key: "kept"}}
	if err := validateIdentityScopeRefs(parent, failIdentityReport(t)); err != nil {
		t.Fatalf("parent references after completed-child removal: %v", err)
	}
}

func TestIdentityInheritedFramesPreserveScopeAllowanceAfterStartAbort(t *testing.T) {
	t.Parallel()

	fixture := startedIdentityEvaluationForTest(t)
	evaluation := fixture.evaluation
	constraint := evaluation.selections[0].constraint
	constraints := evaluation.scopes[0].constraints
	child := identityScope{depth: 3}
	if err := evaluation.publishIdentityKey(&child, identitySelection{constraint: constraint, node: 10}, "kept", identityLimits{}, identityTestContext("/root/wrap/row", 1, 1)); err != nil {
		t.Fatal(err)
	}
	evaluation.scopes = append(evaluation.scopes, child)
	if _, err := evaluation.closeScopes(3, failIdentityReport(t)); err != nil {
		t.Fatal(err)
	}
	if err := evaluation.beginStart(); err != nil {
		t.Fatal(err)
	}
	ctx := identityTestContext("/root/wrap/child", 1, 1)
	if err := evaluation.startScope(constraints, 3, 2, ctx); err != nil {
		t.Fatalf("inherited frame consumed declared-scope allowance: %v", err)
	}
	if err := evaluation.registerIdentityScope(len(evaluation.scopes) - 1); err != nil {
		t.Fatal(err)
	}
	evaluation.abortStart()
	if err := evaluation.startScope(constraints, 3, 1, ctx); err == nil {
		t.Fatal("start abort lost the active declared scope from the allowance")
	} else {
		expectXSDCode(t, err, xsderrors.CodeValidationLimit)
	}
	if err := evaluation.startScope(constraints, 3, 2, ctx); err != nil {
		t.Fatalf("start abort retained a declared-scope charge: %v", err)
	}
	if _, err := evaluation.closeScopes(3, failIdentityReport(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := evaluation.closeScopes(2, failIdentityReport(t)); err != nil {
		t.Fatal(err)
	}
	parent := &evaluation.scopes[0]
	parent.refs = []identityTupleRef{{refer: constraint, key: "kept"}}
	if err := validateIdentityScopeRefs(parent, failIdentityReport(t)); err != nil {
		t.Fatalf("start abort lost inherited rows: %v", err)
	}
}

func TestIdentityScopeRemovalAndLimitsSurviveSessionReuse(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Tree"><xs:choice minOccurs="0" maxOccurs="unbounded">
    <xs:element ref="root"/><xs:element name="wrap" type="Tree"/>
    <xs:element name="row"><xs:complexType><xs:attribute name="v" type="xs:integer" use="required"/></xs:complexType></xs:element>
    <xs:element name="ref"><xs:complexType><xs:attribute name="v" type="xs:integer" use="required"/></xs:complexType></xs:element>
  </xs:choice></xs:complexType>
  <xs:element name="root" type="Tree">
    <xs:key name="K"><xs:selector xpath="row"/><xs:field xpath="@v"/></xs:key>
    <xs:keyref name="R" refer="K"><xs:selector xpath="ref"/><xs:field xpath="@v"/></xs:keyref>
  </xs:element>
</xs:schema>`)
	session, err := newSessionForTest(rt, Options{MaxIdentityScopes: 2})
	if err != nil {
		t.Fatal(err)
	}
	const good = `<root><wrap><root><row v="1"/></root><root><row v="+001"/></root></wrap><root><row v="1"/></root><ref v="1"/></root>`
	for _, tt := range []struct {
		name string
		doc  string
		code xsderrors.Code
	}{
		{"unresolved conflict", `<root><wrap><root><row v="1"/></root><root><row v="+001"/></root></wrap><ref v="1"/></root>`, xsderrors.CodeValidationIdentity},
		{"scope limit after inherited frame", `<root><wrap><root><row v="1"/></root><root><root/></root></wrap></root>`, xsderrors.CodeValidationLimit},
	} {
		t.Run(tt.name, func(t *testing.T) {
			requireCode(t, session.Validate(strings.NewReader(tt.doc)), tt.code)
			if err := session.Validate(strings.NewReader(good)); err != nil {
				t.Fatalf("valid table removal after failure: %v", err)
			}
		})
	}
}

func TestIdentityCompletedTablesExcludeConflictsAcrossMergeSizes(t *testing.T) {
	t.Parallel()
	const keyID xsdSchema.IdentityConstraintID = 1
	for _, shape := range []string{"larger parent", "larger child"} {
		t.Run(shape, func(t *testing.T) {
			t.Parallel()
			var evaluation identityEvaluation
			evaluation.scopes = []identityScope{{depth: 1}, {depth: 2}}
			publish := func(scope *identityScope, id xsdSchema.IdentityConstraintID, key string, node uint64) {
				t.Helper()
				if err := evaluation.publishIdentityKey(scope, identitySelection{constraint: id, node: node}, key, identityLimits{}, identityTestContext("/row", 1, 1)); err != nil {
					t.Fatal(err)
				}
			}
			middle := &evaluation.scopes[1]
			if shape == "larger parent" {
				for id := xsdSchema.IdentityConstraintID(2); id < 5; id++ {
					publish(middle, id, "local", uint64(id))
				}
			}
			for node := uint64(10); node < 12; node++ {
				child := identityScope{depth: 3}
				publish(&child, keyID, "conflict", node)
				if node == 10 {
					publish(&child, keyID, "kept", 20)
				}
				mergeIdentityTables(middle, &child)
			}
			child := identityScope{depth: 3}
			publish(&child, keyID, "other", 30)
			if shape == "larger child" {
				publish(&child, keyID, "other2", 31)
				publish(&child, keyID, "other3", 32)
				for id := xsdSchema.IdentityConstraintID(2); id < 5; id++ {
					publish(&child, id, "local", uint64(id))
				}
			}
			mergeIdentityTables(middle, &child)
			entries := evaluation.entries
			if _, err := evaluation.closeScopes(2, failIdentityReport(t)); err != nil {
				t.Fatal(err)
			}
			if evaluation.entries != entries {
				t.Fatalf("table completion changed cumulative entry accounting: %d, want %d", evaluation.entries, entries)
			}
			fresh := identityScope{depth: 2}
			publish(&fresh, keyID, "conflict", 40)
			mergeIdentityTables(&evaluation.scopes[0], &fresh)
			parent := &evaluation.scopes[0]
			parent.refs = []identityTupleRef{{refer: keyID, key: "conflict"}, {refer: keyID, key: "kept"}, {refer: keyID, key: "other"}}
			for id := xsdSchema.IdentityConstraintID(2); id < 5; id++ {
				parent.refs = append(parent.refs, identityTupleRef{refer: id, key: "local"})
			}
			if err := validateIdentityScopeRefs(parent, failIdentityReport(t)); err != nil {
				t.Fatalf("completed table references: %v", err)
			}
		})
	}
}
