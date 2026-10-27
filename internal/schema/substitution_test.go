package schema

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestBuildSubstitutionTableBuildsSortedRawAndEffectiveEntries(t *testing.T) {
	t.Parallel()

	names, qnames := substitutionNameFixture(t)
	rt := derivationFixture{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: ComplexRef(0), Kind: DerivationKindExtension},
			{Base: ComplexRef(1), Kind: DerivationKindRestriction},
		},
	}
	elements := []ElementDecl{
		{Name: qnames["head"], Type: ComplexRef(0), SubstHead: NoElement},
		{Name: qnames["member"], Type: ComplexRef(1), SubstHead: 0, Abstract: true, Block: DerivationSubstitution},
		{Name: qnames["child"], Type: ComplexRef(2), SubstHead: 1},
	}
	table, err := BuildSubstitutionTable(rt.build(), &names, elements, substitutionGlobals(elements), 3, unboundedTypeDerivationWork)
	if err != nil {
		t.Fatalf("BuildSubstitutionTable() error = %v", err)
	}
	var raw []ElementID
	table.ForEachMember(0, func(member ElementID) bool {
		raw = append(raw, member)
		return true
	})
	wantRaw := []ElementID{1, 2}
	slices.SortFunc(wantRaw, func(a, b ElementID) int {
		return compareQName(elements[a].Name, elements[b].Name)
	})
	if !slices.Equal(raw, wantRaw) {
		t.Fatalf("raw members = %v, want QName-sorted %v", raw, wantRaw)
	}
	var effective []ElementID
	table.ForEachEntry(0, func(_ QName, member ElementID) bool {
		effective = append(effective, member)
		return true
	})
	if !slices.Equal(effective, []ElementID{2}) {
		t.Fatalf("effective members under original head = %v, want [2]", effective)
	}
	if _, ok := table.MemberByName(0, qnames["member"]); ok {
		t.Fatal("MemberByName() returned abstract member")
	}
	if got, ok := table.MemberByName(0, qnames["child"]); !ok || got != 2 {
		t.Fatalf("MemberByName(child) = %d, %v; want 2, true", got, ok)
	}
	if _, ok := table.MemberByName(1, qnames["child"]); ok {
		t.Fatal("MemberByName() ignored the original head's substitution block")
	}
	published := publishedContentSchema(contentSchemaFixture{
		elementNames: map[ElementID]QName{0: qnames["head"], 1: qnames["member"], 2: qnames["child"]},
		models: map[ContentModelID]CompiledModel{
			0: {Kind: CompiledModelDFA, Rows: []CompiledModelRow{{Edges: []CompiledModelEdge{{Particle: ElementParticle(0, Occurrence{Min: 1, Max: 1}), To: 1}}}, {Accept: true}}},
			1: {Kind: CompiledModelDFA, Rows: []CompiledModelRow{{Edges: []CompiledModelEdge{{Particle: ElementParticle(1, Occurrence{Min: 1, Max: 1}), To: 1}}}, {Accept: true}}},
		},
	})
	published.program.Substitutions = table
	for _, excluded := range []struct {
		model ContentModelID
		name  QName
	}{{0, qnames["member"]}, {1, qnames["child"]}} {
		transition, status := published.NextContent(ContentState{model: excluded.model, present: true}, ContentInput{Name: RuntimeName{Known: true, Name: excluded.name}}, &ContentScratch{})
		if status != ContentTransitionNoMatch || transition.IsPlanned() {
			t.Fatalf("NextContent admitted an excluded substitution: %+v/%v", transition, status)
		}
	}
}

func TestBuildSubstitutionTableTreatsEqualTypeEdgesAsNeutral(t *testing.T) {
	t.Parallel()

	names, qnames := substitutionNameFixture(t)
	rt := derivationFixture{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone, Block: DerivationExtension},
			{Base: ComplexRef(0), Kind: DerivationKindRestriction},
			{Base: ComplexRef(1), Kind: DerivationKindExtension},
		},
	}
	elements := []ElementDecl{
		{Name: qnames["head"], Type: ComplexRef(1), SubstHead: NoElement},
		{Name: qnames["member"], Type: ComplexRef(1), SubstHead: 0},
		{Name: qnames["child"], Type: ComplexRef(2), SubstHead: 1},
	}
	table, err := BuildSubstitutionTable(rt.build(), &names, elements, substitutionGlobals(elements), 3, unboundedTypeDerivationWork)
	if err != nil {
		t.Fatalf("BuildSubstitutionTable() error = %v", err)
	}
	if got, ok := table.MemberByName(0, qnames["child"]); !ok || got != 2 {
		t.Fatalf("MemberByName(head, child) = %d, %v; want 2, true", got, ok)
	}
}

func TestBuildSubstitutionTableClosureLimitIsExactAndTyped(t *testing.T) {
	t.Parallel()

	names, qnames := substitutionNameFixture(t)
	rt := derivationFixture{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: ComplexRef(0), Kind: DerivationKindExtension},
			{Base: ComplexRef(1), Kind: DerivationKindRestriction},
		},
	}
	elements := []ElementDecl{
		{Name: qnames["head"], Type: ComplexRef(0), SubstHead: NoElement, Block: DerivationSubstitution},
		{Name: qnames["member"], Type: ComplexRef(1), SubstHead: 0, Abstract: true},
		{Name: qnames["child"], Type: ComplexRef(2), SubstHead: 1, Abstract: true},
	}
	if _, err := BuildSubstitutionTable(rt.build(), &names, elements, substitutionGlobals(elements), 3, unboundedTypeDerivationWork); err != nil {
		t.Fatalf("exact closure limit failed: %v", err)
	}
	_, err := BuildSubstitutionTable(rt.build(), &names, elements, substitutionGlobals(elements), 2, unboundedTypeDerivationWork)
	var limit SubstitutionClosureLimitError
	if !errors.As(err, &limit) || limit.Limit != 2 {
		t.Fatalf("BuildSubstitutionTable() error = %T %v, want limit 2", err, err)
	}
}

func TestBuildSubstitutionTableBoundsDerivationWorkByDirectEdges(t *testing.T) {
	const count = 200
	names, elements, derivations := substitutionChainFixture(t, count)
	rt := (derivationFixture{complex: derivations}).build()
	budget := 1600
	table, err := BuildSubstitutionTable(rt, &names, elements, substitutionGlobals(elements), 19900, func(steps int) error {
		budget -= steps
		if budget < 0 {
			return errors.New("derivation work exhausted")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("BuildSubstitutionTable() error = %v", err)
	}
	if member, ok := table.MemberByName(0, elements[199].Name); !ok || member != 199 {
		t.Fatalf("last member = %d,%v; want199,true", member, ok)
	}
	var rawCount int
	table.ForEachMember(0, func(ElementID) bool { rawCount++; return true })
	if rawCount != 199 {
		t.Fatalf("first head has%d members,want199", rawCount)
	}
}
func TestBuildSubstitutionTableChecksClosureLimitBeforeDerivationWork(t *testing.T) {
	names, elements, derivations := substitutionChainFixture(t, 100)
	rt := (derivationFixture{complex: derivations}).build()
	calls := 0
	sentinel := errors.New("work callback failure")
	work := func(int) error { calls++; return sentinel }
	_, err := BuildSubstitutionTable(rt, &names, elements, substitutionGlobals(elements), 1, work)
	if diagnostic, ok := errors.AsType[SubstitutionClosureLimitError](err); !ok || diagnostic.Limit != 1 {
		t.Fatalf("error = %T %v,want closure limit1", err, err)
	}
	if calls != 0 {
		t.Fatalf("work callbacks = %d,want0 before limit rejection", calls)
	}
	_, err = BuildSubstitutionTable(rt, &names, elements, substitutionGlobals(elements), 4950, work)
	if !errors.Is(err, sentinel) || calls != 1 {
		t.Fatalf("sufficient closure error/calls = %v/%d,want sentinel/1", err, calls)
	}
}

func TestBuildSubstitutionTableRejectsInvalidDirectEdgeBeforeEffectiveFiltering(t *testing.T) {
	t.Parallel()

	names, qnames := substitutionNameFixture(t)
	rt := derivationFixture{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Kind: DerivationKindNone},
		},
	}
	elements := []ElementDecl{
		{Name: qnames["head"], Type: ComplexRef(0), SubstHead: NoElement, Block: DerivationSubstitution},
		{Name: qnames["member"], Type: ComplexRef(1), SubstHead: 0, Abstract: true},
	}
	_, err := BuildSubstitutionTable(rt.build(), &names, elements, substitutionGlobals(elements), 1, unboundedTypeDerivationWork)
	if err == nil || !strings.Contains(err.Error(), "member is not allowed by head") {
		t.Fatalf("BuildSubstitutionTable() error = %v, want invalid direct membership", err)
	}
	var membership SubstitutionMembershipError
	if !errors.As(err, &membership) || membership.Member != 1 || membership.Head != 0 ||
		!errors.Is(err, ErrSubstitutionMemberTypeNotDerived) {
		t.Fatalf("BuildSubstitutionTable() error = %#v, want typed member 1/head 0 derivation failure", err)
	}
}

func TestBuildSubstitutionTableValidatesEveryDirectGlobalBindingWithPresence(t *testing.T) {
	t.Parallel()

	names, qnames := substitutionNameFixture(t)
	rt := derivationFixture{anyType: 0, complex: []ComplexTypeDerivation{{Kind: DerivationKindNone}}}
	elements := []ElementDecl{
		{Name: qnames["head"], Type: ComplexRef(0), SubstHead: NoElement},
		{Name: qnames["member"], Type: ComplexRef(0), SubstHead: 0},
	}
	tests := []struct {
		name    string
		globals map[QName]ElementID
		want    string
	}{
		{name: "missing member whose zero value aliases head", globals: map[QName]ElementID{qnames["head"]: 0}, want: "member is not a global"},
		{name: "missing head whose zero value matches", globals: map[QName]ElementID{qnames["member"]: 1}, want: "head is not a global"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := BuildSubstitutionTable(rt.build(), &names, elements, tt.globals, 1, unboundedTypeDerivationWork)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("BuildSubstitutionTable() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestBuildSubstitutionTableDetectsCycleIteratively(t *testing.T) {
	t.Parallel()

	names, qnames := substitutionNameFixture(t)
	rt := derivationFixture{anyType: 0, complex: []ComplexTypeDerivation{{Kind: DerivationKindNone}}}
	elements := []ElementDecl{
		{Name: qnames["head"], Type: ComplexRef(0), SubstHead: 1},
		{Name: qnames["member"], Type: ComplexRef(0), SubstHead: 0},
	}
	_, err := BuildSubstitutionTable(rt.build(), &names, elements, substitutionGlobals(elements), 2, unboundedTypeDerivationWork)
	if diagnostic, ok := errors.AsType[SubstitutionCycleError](err); !ok || diagnostic.Error() == "" {
		t.Fatalf("BuildSubstitutionTable() error = %T %v, want cycle", err, err)
	}
}

func TestBuildSubstitutionTableDetectsDeepCycleWithoutRecursion(t *testing.T) {
	const count = 20_000

	expanded := make([]ExpandedName, count)
	for i := range expanded {
		expanded[i] = ExpandedName{Namespace: EmptyNamespaceURI, Local: "e" + strconv.Itoa(i)}
	}
	names, err := NewNameTable(count+1, []string{EmptyNamespaceURI}, expanded)
	if err != nil {
		t.Fatal(err)
	}
	elements := make([]ElementDecl, count)
	globals := make(map[QName]ElementID, count)
	for i, name := range expanded {
		qname, ok := names.LookupQName(name.Namespace, name.Local)
		if !ok {
			t.Fatalf("missing QName %d", i)
		}
		head := NoElement
		if i != 0 {
			head = ElementID(i - 1)
		}
		elements[i] = ElementDecl{Name: qname, SubstHead: head}
		globals[qname] = ElementID(i)
	}
	elements[0].SubstHead = ElementID(count - 1)
	_, err = BuildSubstitutionTable((derivationFixture{}).build(), &names, elements, globals, 0, unboundedTypeDerivationWork)
	if diagnostic, ok := errors.AsType[SubstitutionCycleError](err); !ok || diagnostic.Error() == "" {
		t.Fatalf("BuildSubstitutionTable() error = %T %v, want cycle", err, err)
	}
}

func TestNewSchemaRuntimeSharesImmutableSubstitutionTable(t *testing.T) {
	t.Parallel()

	table := SubstitutionTable{
		spans:   []substitutionSpan{{start: 0, count: 1}, {start: 1}},
		entries: []substitutionEntry{{name: QName{Local: 1}, member: 1, effective: true}},
	}
	build := schemaBuild{
		Substitutions: table,
		ComplexTypes:  []ComplexType{{Derivation: DerivationKindNone}},
	}
	reads, err := newSchemaProgram(&build, unlimitedContentModelWork)
	if err != nil {
		t.Fatalf("newSchemaProgram() error = %v", err)
	}
	if &reads.Substitutions.entries[0] != &build.Substitutions.entries[0] {
		t.Fatal("newSchemaProgram() cloned immutable substitution entries")
	}
}

func TestSubstitutionTableSkipsValidatedEmptySpan(t *testing.T) {
	t.Parallel()

	table := SubstitutionTable{
		spans:   []substitutionSpan{{start: 0, count: 0}},
		entries: []substitutionEntry{{member: 1}},
	}
	called := false
	table.ForEachMember(0, func(ElementID) bool {
		called = true
		return true
	})
	if called {
		t.Fatal("ForEachMember() called callback for an empty span")
	}
	called = false
	table.ForEachEntry(0, func(QName, ElementID) bool {
		called = true
		return true
	})
	if called {
		t.Fatal("ForEachEntry() called callback for an empty span")
	}
	if got, ok := table.MemberByName(0, QName{Local: 1}); ok || got != NoElement {
		t.Fatalf("MemberByName() = %d, %v for empty span, want %d, false", got, ok, NoElement)
	}
	if table.HasMembers(0) {
		t.Fatal("HasMembers() = true for an empty span")
	}
}

func TestValidateSubstitutionEdgeMembership(t *testing.T) {
	t.Parallel()

	rt := derivationFixture{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: ComplexRef(0), Kind: DerivationKindExtension},
			{Kind: DerivationKindNone},
		},
	}
	elements := []ElementDecl{{Type: ComplexRef(0)}, {Type: ComplexRef(1)}}
	if _, err := validateSubstitutionEdge(rt.build(), elements, 1, 0, unboundedTypeDerivationWork); err != nil {
		t.Fatalf("derived member rejected: %v", err)
	}
	elements[0].Final = DerivationExtension
	_, err := validateSubstitutionEdge(rt.build(), elements, 1, 0, unboundedTypeDerivationWork)
	if !errors.Is(err, ErrSubstitutionMemberTypeExcludedDerivation) {
		t.Fatalf("final error = %v", err)
	}
	membership, ok := errors.AsType[SubstitutionMembershipError](err)
	if !ok || !errors.Is(membership.Cause, ErrSubstitutionMemberTypeExcludedDerivation) || membership.Member != 1 || membership.Head != 0 {
		t.Fatalf("final error = %#v, want excluded derivation for member1/head0", err)
	}
	elements[0] = ElementDecl{Type: ComplexRef(0)}
	elements[1] = ElementDecl{Type: ComplexRef(2)}
	_, err = validateSubstitutionEdge(rt.build(), elements, 1, 0, unboundedTypeDerivationWork)
	if !errors.Is(err, ErrSubstitutionMemberTypeNotDerived) {
		t.Fatalf("unrelated error = %v", err)
	}
	membership, ok = errors.AsType[SubstitutionMembershipError](err)
	if !ok || !errors.Is(membership.Cause, ErrSubstitutionMemberTypeNotDerived) || membership.Member != 1 || membership.Head != 0 {
		t.Fatalf("unrelated error = %#v, want not derived for member1/head0", err)
	}
}

func substitutionChainFixture(t *testing.T, count int) (NameTable, []ElementDecl, []ComplexTypeDerivation) {
	t.Helper()
	expanded := make([]ExpandedName, count)
	for i := range expanded {
		expanded[i] = ExpandedName{Namespace: EmptyNamespaceURI, Local: "e" + strconv.Itoa(i)}
	}
	names, err := NewNameTable(count+1, []string{EmptyNamespaceURI}, expanded)
	if err != nil {
		t.Fatal(err)
	}
	elements := make([]ElementDecl, count)
	derivations := make([]ComplexTypeDerivation, count)
	for i, name := range expanded {
		qname, ok := names.LookupQName(name.Namespace, name.Local)
		if !ok {
			t.Fatalf("missing QName %d", i)
		}
		head := NoElement
		if i != 0 {
			head = ElementID(i - 1)
			derivations[i] = ComplexTypeDerivation{Base: ComplexRef(ComplexTypeID(i - 1)), Kind: DerivationKindExtension}
		}
		elements[i] = ElementDecl{Name: qname, Type: ComplexRef(ComplexTypeID(i)), SubstHead: head}
	}
	return names, elements, derivations
}

func substitutionGlobals(elements []ElementDecl) map[QName]ElementID {
	globals := make(map[QName]ElementID, len(elements))
	for id, element := range elements {
		globals[element.Name] = ElementID(id)
	}
	return globals
}

func substitutionNameFixture(t *testing.T) (NameTable, map[string]QName) {
	t.Helper()

	names, err := NewNameTable(16, []string{EmptyNamespaceURI}, []ExpandedName{
		{Namespace: EmptyNamespaceURI, Local: "head"},
		{Namespace: EmptyNamespaceURI, Local: "member"},
		{Namespace: EmptyNamespaceURI, Local: "child"},
	})
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	qnames := make(map[string]QName)
	for _, local := range []string{"head", "member", "child"} {
		q, ok := names.LookupQName(EmptyNamespaceURI, local)
		if !ok {
			t.Fatalf("missing QName for %s", local)
		}
		qnames[local] = q
	}
	return names, qnames
}

func fixtureSubstitutions(names []ElementDecl, raw map[ElementID][]ElementID, effective map[ElementID]map[QName]ElementID) SubstitutionTable {
	count := 0
	for id := range raw {
		count = max(count, int(id)+1)
	}
	for id := range effective {
		count = max(count, int(id)+1)
	}
	table := SubstitutionTable{spans: make([]substitutionSpan, count)}
	for head := range count {
		entries := make(map[ElementID]substitutionEntry)
		for _, id := range raw[ElementID(head)] {
			var name QName
			if int(id) < len(names) {
				name = names[id].Name
			}
			entries[id] = substitutionEntry{name: name, member: id}
		}
		for name, id := range effective[ElementID(head)] {
			entries[id] = substitutionEntry{name: name, member: id, effective: true}
		}
		span := substitutionSpan{start: len(table.entries), count: len(entries)}
		ordered := make([]substitutionEntry, 0, len(entries))
		for _, entry := range entries {
			ordered = append(ordered, entry)
		}
		slices.SortFunc(ordered, func(a, b substitutionEntry) int { return compareQName(a.name, b.name) })
		table.entries = append(table.entries, ordered...)
		table.spans[head] = span
	}
	return table
}
