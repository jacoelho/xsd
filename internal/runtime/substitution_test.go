package runtime

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
	rt := derivationRuntimeStub{
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
	table, err := BuildSubstitutionTable(rt, &names, elements, substitutionGlobals(elements), 3)
	if err != nil {
		t.Fatalf("BuildSubstitutionTable() error = %v", err)
	}
	if err := ValidateSubstitutionTable(rt, &names, elements, substitutionGlobals(elements), table); err != nil {
		t.Fatalf("ValidateSubstitutionTable() error = %v", err)
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
}

func TestBuildSubstitutionTableTreatsEqualTypeEdgesAsNeutral(t *testing.T) {
	t.Parallel()

	names, qnames := substitutionNameFixture(t)
	rt := derivationRuntimeStub{
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
	table, err := BuildSubstitutionTable(rt, &names, elements, substitutionGlobals(elements), 3)
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
	rt := derivationRuntimeStub{
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
	if _, err := BuildSubstitutionTable(rt, &names, elements, substitutionGlobals(elements), 3); err != nil {
		t.Fatalf("exact closure limit failed: %v", err)
	}
	_, err := BuildSubstitutionTable(rt, &names, elements, substitutionGlobals(elements), 2)
	var limit SubstitutionClosureLimitError
	if !errors.As(err, &limit) || limit.Limit != 2 {
		t.Fatalf("BuildSubstitutionTable() error = %T %v, want limit 2", err, err)
	}
}

func TestBuildSubstitutionTableBoundsDerivationWorkByDirectEdges(t *testing.T) {
	const count = 200

	names, elements, derivations := substitutionChainFixture(t, count)
	calls := 0
	rt := countingDerivationRuntime{complex: derivations, calls: &calls}
	if _, err := BuildSubstitutionTable(rt, &names, elements, substitutionGlobals(elements), count*(count-1)/2); err != nil {
		t.Fatalf("BuildSubstitutionTable() error = %v", err)
	}
	if calls > count*8 {
		t.Fatalf("ComplexTypeDerivation() calls = %d, want O(direct edges)", calls)
	}
}

func TestBuildSubstitutionTableChecksClosureLimitBeforeDerivationWork(t *testing.T) {
	const count = 100

	names, elements, derivations := substitutionChainFixture(t, count)
	calls := 0
	_, err := BuildSubstitutionTable(
		countingDerivationRuntime{complex: derivations, calls: &calls},
		&names,
		elements,
		substitutionGlobals(elements),
		1,
	)
	var limit SubstitutionClosureLimitError
	if !errors.As(err, &limit) {
		t.Fatalf("BuildSubstitutionTable() error = %T %v, want closure limit", err, err)
	}
	if calls != 0 {
		t.Fatalf("ComplexTypeDerivation() calls = %d, want 0 before over-limit rejection", calls)
	}
}

func TestBuildSubstitutionTableRejectsInvalidDirectEdgeBeforeEffectiveFiltering(t *testing.T) {
	t.Parallel()

	names, qnames := substitutionNameFixture(t)
	rt := derivationRuntimeStub{
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
	_, err := BuildSubstitutionTable(rt, &names, elements, substitutionGlobals(elements), 1)
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
	rt := derivationRuntimeStub{anyType: 0, complex: []ComplexTypeDerivation{{Kind: DerivationKindNone}}}
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

			_, err := BuildSubstitutionTable(rt, &names, elements, tt.globals, 1)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("BuildSubstitutionTable() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestBuildSubstitutionTableDetectsCycleIteratively(t *testing.T) {
	t.Parallel()

	names, qnames := substitutionNameFixture(t)
	rt := derivationRuntimeStub{anyType: 0, complex: []ComplexTypeDerivation{{Kind: DerivationKindNone}}}
	elements := []ElementDecl{
		{Name: qnames["head"], Type: ComplexRef(0), SubstHead: 1},
		{Name: qnames["member"], Type: ComplexRef(0), SubstHead: 0},
	}
	_, err := BuildSubstitutionTable(rt, &names, elements, substitutionGlobals(elements), 2)
	var cycle SubstitutionCycleError
	if !errors.As(err, &cycle) {
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
	_, err = BuildSubstitutionTable(derivationRuntimeStub{}, &names, elements, globals, 0)
	var cycle SubstitutionCycleError
	if !errors.As(err, &cycle) {
		t.Fatalf("BuildSubstitutionTable() error = %T %v, want cycle", err, err)
	}
}

func TestValidateSubstitutionTableRejectsStructuralAndExactDrift(t *testing.T) {
	t.Parallel()

	names, qnames := substitutionNameFixture(t)
	rt := derivationRuntimeStub{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: ComplexRef(0), Kind: DerivationKindExtension},
		},
	}
	elements := []ElementDecl{
		{Name: qnames["head"], Type: ComplexRef(0), SubstHead: NoElement},
		{Name: qnames["member"], Type: ComplexRef(1), SubstHead: 0},
	}
	globals := substitutionGlobals(elements)
	table, err := BuildSubstitutionTable(rt, &names, elements, globals, 1)
	if err != nil {
		t.Fatal(err)
	}

	wrongSpan := table
	wrongSpan.spans = slices.Clone(table.spans)
	wrongSpan.spans[0].start = 1
	if err := ValidateSubstitutionTable(rt, &names, elements, globals, wrongSpan); err == nil {
		t.Fatal("ValidateSubstitutionTable() accepted corrupt span")
	}
	wrongEntry := table
	wrongEntry.entries = slices.Clone(table.entries)
	wrongEntry.entries[0].effective = false
	if err := ValidateSubstitutionTable(rt, &names, elements, globals, wrongEntry); err == nil {
		t.Fatal("ValidateSubstitutionTable() accepted semantically stale entry")
	}
}

func TestSubstitutionTableCanonicalZero(t *testing.T) {
	t.Parallel()

	names, qnames := substitutionNameFixture(t)
	rt := derivationRuntimeStub{anyType: 0, complex: []ComplexTypeDerivation{{Kind: DerivationKindNone}}}
	elements := []ElementDecl{{Name: qnames["head"], Type: ComplexRef(0), SubstHead: NoElement}}
	table, err := BuildSubstitutionTable(rt, &names, elements, substitutionGlobals(elements), 0)
	if err != nil || table.spans != nil || table.entries != nil {
		t.Fatalf("BuildSubstitutionTable(no heads) = %#v, %v; want zero", table, err)
	}
	nonCanonical := SubstitutionTable{spans: []substitutionSpan{}}
	if err := ValidateSubstitutionTable(rt, &names, elements, substitutionGlobals(elements), nonCanonical); err == nil {
		t.Fatal("ValidateSubstitutionTable() accepted non-canonical empty table")
	}
}

func TestNewSchemaRuntimeSharesImmutableSubstitutionTable(t *testing.T) {
	t.Parallel()

	table := SubstitutionTable{
		spans:   []substitutionSpan{{start: 0, count: 1}, {start: 1}},
		entries: []substitutionEntry{{name: QName{Local: 1}, member: 1, effective: true}},
	}
	build := SchemaBuild{
		Substitutions: table,
		ComplexTypes:  []ComplexType{{Derivation: DerivationKindNone}},
	}
	reads, err := newSchemaRuntime(&build)
	if err != nil {
		t.Fatalf("newSchemaRuntime() error = %v", err)
	}
	if &reads.Substitutions.entries[0] != &build.Substitutions.entries[0] {
		t.Fatal("newSchemaRuntime() cloned immutable substitution entries")
	}
}

func TestValidateSubstitutionMembership(t *testing.T) {
	t.Parallel()

	rt := derivationRuntimeStub{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: ComplexRef(0), Kind: DerivationKindExtension},
			{Kind: DerivationKindNone},
		},
	}
	head := ElementDecl{Type: ComplexRef(0)}
	if err := ValidateSubstitutionMembership(rt, head, ElementDecl{Type: ComplexRef(1)}); err != nil {
		t.Fatalf("derived member rejected: %v", err)
	}
	head.Final = DerivationExtension
	if err := ValidateSubstitutionMembership(rt, head, ElementDecl{Type: ComplexRef(1)}); !errors.Is(err, ErrSubstitutionMemberTypeExcludedDerivation) {
		t.Fatalf("final error = %v", err)
	}
	if err := ValidateSubstitutionMembership(rt, ElementDecl{Type: ComplexRef(0)}, ElementDecl{Type: ComplexRef(2)}); !errors.Is(err, ErrSubstitutionMemberTypeNotDerived) {
		t.Fatalf("unrelated error = %v", err)
	}
}

type countingDerivationRuntime struct {
	complex []ComplexTypeDerivation
	calls   *int
}

func (r countingDerivationRuntime) AnyTypeID() ComplexTypeID { return 0 }
func (r countingDerivationRuntime) SimpleTypeCount() int     { return 0 }
func (r countingDerivationRuntime) ComplexTypeCount() int    { return len(r.complex) }

func (r countingDerivationRuntime) SimpleTypeDerivation(SimpleTypeID) (SimpleTypeDerivation, bool) {
	return SimpleTypeDerivation{}, false
}

func (r countingDerivationRuntime) ComplexTypeDerivation(id ComplexTypeID) (ComplexTypeDerivation, bool) {
	*r.calls++
	if !ValidUint32Index(uint32(id), len(r.complex)) {
		return ComplexTypeDerivation{}, false
	}
	return r.complex[id], true
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
