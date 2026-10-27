package validate

import (
	"errors"
	"strconv"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
)

func TestIdentityRootTableReuseAdmissionAndCleanup(t *testing.T) {
	t.Parallel()
	const id xsdSchema.IdentityConstraintID = 1
	for _, size := range []int{4096, 4097} {
		table := make(map[string]identityTableEntry, size)
		for i := range size {
			table[strconv.Itoa(i)] = identityTableEntry{path: explicitRetainedPath("/old"), originDepth: 1}
		}
		var state identityState
		state.scopes = []identityScope{{depth: 1, tables: map[xsdSchema.IdentityConstraintID]map[string]identityTableEntry{id: table}}}
		state.propagateClosedIdentityScope()
		if size > 4096 {
			if state.idleTable != nil {
				t.Fatal("oversized root table retained")
			}
			continue
		}
		if state.idleTable == nil || len(table) != 0 {
			t.Fatal("consumed root table did not clear and retain storage")
		}
		state.entries = 1
		var scope identityScope
		selection := identitySelection{constraint: id, node: 2, depth: 1}
		ctx := identityTestContext("/new", 2, 3)
		if err := state.publishIdentityKey(nil, &scope, selection, "new", identityLimits{Entries: 1}, ctx); err == nil {
			t.Fatal("publication exceeded admission limit")
		}
		if state.idleTable == nil || scope.tables != nil {
			t.Fatal("rejected publication borrowed or created a table")
		}
		state.entries = 0
		if err := state.publishIdentityKey(nil, &scope, selection, "new", identityLimits{}, ctx); err != nil {
			t.Fatal(err)
		}
		if state.idleTable != nil || table["new"].node != 2 || len(table) != 1 {
			t.Fatal("idle table ownership was not transferred exclusively")
		}
		if err := state.publishIdentityKey(nil, &scope, selection, "new", identityLimits{}, ctx); err == nil || state.entries != 1 || table["new"].node != 2 {
			t.Fatal("duplicate local publication changed table or admission count")
		}
	}
}

func TestIdentityRootTableReuseRejectsMergedPrunedStorage(t *testing.T) {
	t.Parallel()
	const id xsdSchema.IdentityConstraintID = 1
	root := identityScope{depth: 1}
	for node := uint64(2); node < 4; node++ {
		table := make(map[string]identityTableEntry, 4097)
		for i := range 4097 {
			table[strconv.Itoa(i)] = identityTableEntry{node: node, originDepth: 2}
		}
		child := identityScope{depth: 2, tables: map[xsdSchema.IdentityConstraintID]map[string]identityTableEntry{id: table}}
		mergeIdentityTables(&root, &child)
	}
	root.tables[id]["kept"] = identityTableEntry{node: 1, originDepth: 1}
	root.completeTables()
	if len(root.tables[id]) != 1 {
		t.Fatalf("completed root cardinality = %d, want 1", len(root.tables[id]))
	}
	state := identityState{scopes: []identityScope{root}}
	state.propagateClosedIdentityScope()
	if state.idleTable != nil {
		t.Fatal("merged table was retained after conflict pruning hid its growth")
	}
}

func TestIdentityRootTableReuseResetBoundsAndOwnership(t *testing.T) {
	t.Parallel()
	const id xsdSchema.IdentityConstraintID = 1
	rootTable := map[string]identityTableEntry{"root": {path: explicitRetainedPath("/root")}}
	childTable := map[string]identityTableEntry{"child": {path: explicitRetainedPath("/root/child")}}
	state := identityState{scopes: []identityScope{
		{depth: 1, tables: map[xsdSchema.IdentityConstraintID]map[string]identityTableEntry{id: rootTable}},
		{depth: 2, tables: map[xsdSchema.IdentityConstraintID]map[string]identityTableEntry{id: childTable}},
	}}
	state.reset(4096, 4096)
	if state.idleTable == nil || len(rootTable) != 0 || len(childTable) != 1 {
		t.Fatal("reset reclaimed outside the eligible root owner")
	}
	state.scopes = []identityScope{{depth: 1, tables: map[xsdSchema.IdentityConstraintID]map[string]identityTableEntry{id: childTable}}}
	state.reset(4095, 4096)
	if state.idleTable != nil || len(childTable) != 1 {
		t.Fatal("lower reset bound retained or reclaimed fixed-envelope storage")
	}
	state.idleTable = rootTable
	evaluation := identityEvaluation{identityState: state}
	evaluation.discard()
	if evaluation.idleTable != nil {
		t.Fatal("discard retained idle map")
	}
}

func TestIdentityRootTableReuseSkipsMultiplePopulatedTables(t *testing.T) {
	t.Parallel()
	one := map[string]identityTableEntry{"one": {}}
	three := map[string]identityTableEntry{"a": {}, "b": {}, "c": {}}
	scope := identityScope{depth: 1, tables: map[xsdSchema.IdentityConstraintID]map[string]identityTableEntry{1: one, 2: three}}
	var state identityState
	state.retainRootTable(&scope)
	if state.idleTable != nil || len(three) != 3 || len(one) != 1 {
		t.Fatal("multiple populated tables were reclaimed by the single-table retention path")
	}
}

func TestIdentityRootTableFailedClosePreservesActiveOwnership(t *testing.T) {
	t.Parallel()
	evaluation := startedIdentityEvaluationForTest(t).evaluation
	id := evaluation.selections[0].constraint
	table := map[string]identityTableEntry{"kept": {node: 7, originDepth: 1, path: explicitRetainedPath("/root/kept")}}
	evaluation.scopes[0].tables = map[xsdSchema.IdentityConstraintID]map[string]identityTableEntry{id: table}
	evaluation.scopes[0].refs = []identityTupleRef{{refer: id, key: "missing", path: explicitRetainedPath("/root/ref")}}
	stop := errors.New("stop")
	if _, err := evaluation.closeScopes(1, func(error) error { return stop }); !errors.Is(err, stop) {
		t.Fatalf("failed close = %v", err)
	}
	if evaluation.idleTable != nil || len(evaluation.scopes) != 1 || table["kept"].node != 7 {
		t.Fatal("failed close changed active root table ownership")
	}
	if _, err := evaluation.closeScopes(1, func(error) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(evaluation.scopes) != 0 || evaluation.idleTable == nil || len(table) != 0 {
		t.Fatal("consumed root table was not reclaimed after accepted reporting")
	}
}
