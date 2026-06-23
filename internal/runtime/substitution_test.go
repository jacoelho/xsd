package runtime

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestValidateSubstitutionMaps(t *testing.T) {
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
		{Name: qnames["member"], Type: ComplexRef(1), SubstHead: 0},
		{Name: qnames["child"], Type: ComplexRef(2), SubstHead: 1},
	}
	globals := map[QName]ElementID{
		qnames["head"]:   0,
		qnames["member"]: 1,
		qnames["child"]:  2,
	}
	substitutions := map[ElementID][]ElementID{
		0: {1, 2},
		1: {2},
	}
	lookup := map[ElementID]map[QName]ElementID{
		0: {qnames["member"]: 1, qnames["child"]: 2},
		1: {qnames["child"]: 2},
	}
	if err := ValidateSubstitutionMaps(rt, &names, elements, globals, substitutions, lookup); err != nil {
		t.Fatalf("ValidateSubstitutionMaps() error = %v", err)
	}
}

func TestEqualSubstitutionMap(t *testing.T) {
	t.Parallel()

	base := map[ElementID][]ElementID{
		1: {2, 3},
		4: nil,
	}
	if !EqualSubstitutionMap(base, map[ElementID][]ElementID{
		1: {2, 3},
		4: {},
	}) {
		t.Fatal("EqualSubstitutionMap() rejected equal closure maps")
	}
	if EqualSubstitutionMap(base, map[ElementID][]ElementID{1: {3, 2}, 4: nil}) {
		t.Fatal("EqualSubstitutionMap() accepted reordered members")
	}
	if EqualSubstitutionMap(base, map[ElementID][]ElementID{1: {2, 3}}) {
		t.Fatal("EqualSubstitutionMap() accepted missing head")
	}
}

func TestEqualSubstitutionLookup(t *testing.T) {
	t.Parallel()

	a := QName{Namespace: 1, Local: 1}
	b := QName{Namespace: 1, Local: 2}
	base := map[ElementID]map[QName]ElementID{
		1: {a: 2, b: 3},
		4: nil,
	}
	if !EqualSubstitutionLookup(base, map[ElementID]map[QName]ElementID{
		1: {a: 2, b: 3},
		4: {},
	}) {
		t.Fatal("EqualSubstitutionLookup() rejected equal lookup maps")
	}
	if EqualSubstitutionLookup(base, map[ElementID]map[QName]ElementID{
		1: {a: 3, b: 2},
		4: nil,
	}) {
		t.Fatal("EqualSubstitutionLookup() accepted mismatched members")
	}
	if EqualSubstitutionLookup(base, map[ElementID]map[QName]ElementID{1: {a: 2, b: 3}}) {
		t.Fatal("EqualSubstitutionLookup() accepted missing head")
	}
}

func TestValidateSubstitutionReadMaps(t *testing.T) {
	t.Parallel()

	a := QName{Namespace: 1, Local: 1}
	substitutions := map[ElementID][]ElementID{1: {2, 3}, 4: nil}
	lookup := map[ElementID]map[QName]ElementID{1: {a: 2}, 4: nil}

	if err := ValidateSubstitutionReadMaps(
		map[ElementID][]ElementID{1: {2, 3}, 4: {}},
		map[ElementID]map[QName]ElementID{1: {a: 2}, 4: {}},
		substitutions,
		lookup,
	); err != nil {
		t.Fatalf("ValidateSubstitutionReadMaps() error = %v", err)
	}
	if err := ValidateSubstitutionReadMaps(nil, lookup, substitutions, lookup); err == nil || err.Error() != "substitution read map does not match substitutions" {
		t.Fatalf("ValidateSubstitutionReadMaps(reads) error = %v, want substitution read invariant", err)
	}
	if err := ValidateSubstitutionReadMaps(substitutions, nil, substitutions, lookup); err == nil || err.Error() != "substitution lookup read map does not match lookup" {
		t.Fatalf("ValidateSubstitutionReadMaps(lookup) error = %v, want substitution lookup invariant", err)
	}
}

func TestSubstitutionReadAccessors(t *testing.T) {
	t.Parallel()

	head := ElementID(1)
	other := ElementID(4)
	a := QName{Namespace: 1, Local: 1}
	b := QName{Namespace: 1, Local: 2}
	reads := map[ElementID][]ElementID{
		head: {2, 3},
	}
	lookup := map[ElementID]map[QName]ElementID{
		head: {a: 2, b: 3},
	}

	var members []ElementID
	ForEachSubstitutionMember(reads, head, func(member ElementID) bool {
		members = append(members, member)
		return false
	})
	if !slices.Equal(members, []ElementID{2}) {
		t.Fatalf("ForEachSubstitutionMember early stop = %v, want [2]", members)
	}
	ForEachSubstitutionMember(reads, other, func(ElementID) bool {
		t.Fatal("ForEachSubstitutionMember called fn for missing head")
		return true
	})

	if got, ok := SubstitutionMemberByName(lookup, head, b); !ok || got != 3 {
		t.Fatalf("SubstitutionMemberByName() = %d, %v; want 3, true", got, ok)
	}
	if got, ok := SubstitutionMemberByName(lookup, other, b); ok || got != NoElement {
		t.Fatalf("SubstitutionMemberByName(missing head) = %d, %v; want no element, false", got, ok)
	}
	if got, ok := SubstitutionMemberByName(lookup, head, QName{Local: 9}); ok || got != NoElement {
		t.Fatalf("SubstitutionMemberByName(missing name) = %d, %v; want no element, false", got, ok)
	}
}

func TestValidateSubstitutionMapsRejectsDrift(t *testing.T) {
	t.Parallel()

	names, qnames := substitutionNameFixture(t)
	rt := derivationRuntimeStub{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: ComplexRef(0), Kind: DerivationKindExtension},
		},
	}
	baseElements := []ElementDecl{
		{Name: qnames["head"], Type: ComplexRef(0), SubstHead: NoElement},
		{Name: qnames["member"], Type: ComplexRef(1), SubstHead: 0},
	}
	globals := map[QName]ElementID{
		qnames["head"]:   0,
		qnames["member"]: 1,
	}
	tests := []struct {
		substitutions map[ElementID][]ElementID
		lookup        map[ElementID]map[QName]ElementID
		name          string
		wantErr       string
		elements      []ElementDecl
	}{
		{
			name:          "maps without heads",
			elements:      []ElementDecl{{Name: qnames["head"], Type: ComplexRef(0), SubstHead: NoElement}},
			substitutions: map[ElementID][]ElementID{0: {0}},
			wantErr:       "substitution maps exist without substitution heads",
		},
		{
			name:          "missing declared member",
			elements:      baseElements,
			substitutions: map[ElementID][]ElementID{0: nil},
			lookup:        map[ElementID]map[QName]ElementID{0: nil},
			wantErr:       "substitution map does not match element substitution heads",
		},
		{
			name:          "stale lookup",
			elements:      baseElements,
			substitutions: map[ElementID][]ElementID{0: {1}},
			lookup:        map[ElementID]map[QName]ElementID{},
			wantErr:       "substitution lookup does not match substitutions",
		},
		{
			name: "cycle",
			elements: []ElementDecl{
				{Name: qnames["head"], Type: ComplexRef(0), SubstHead: 1},
				{Name: qnames["member"], Type: ComplexRef(0), SubstHead: 0},
			},
			substitutions: map[ElementID][]ElementID{},
			lookup:        map[ElementID]map[QName]ElementID{},
			wantErr:       "cyclic substitution group",
		},
		{
			name: "final blocks member derivation",
			elements: []ElementDecl{
				{Name: qnames["head"], Type: ComplexRef(0), SubstHead: NoElement, Final: DerivationExtension},
				{Name: qnames["member"], Type: ComplexRef(1), SubstHead: 0},
			},
			substitutions: map[ElementID][]ElementID{0: {1}},
			lookup:        map[ElementID]map[QName]ElementID{0: {qnames["member"]: 1}},
			wantErr:       "substitution member is not allowed by head",
		},
		{
			name:          "lookup name mismatch",
			elements:      baseElements,
			substitutions: map[ElementID][]ElementID{0: {1}},
			lookup:        map[ElementID]map[QName]ElementID{0: {qnames["head"]: 1}},
			wantErr:       "substitution lookup name does not match element",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSubstitutionMaps(rt, &names, tt.elements, globals, tt.substitutions, tt.lookup)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateSubstitutionMaps() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateSubstitutionMapsAllowsLookupFilteredByBlock(t *testing.T) {
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
		{Name: qnames["head"], Type: ComplexRef(0), SubstHead: NoElement, Block: DerivationSubstitution},
		{Name: qnames["member"], Type: ComplexRef(1), SubstHead: 0},
	}
	globals := map[QName]ElementID{
		qnames["head"]:   0,
		qnames["member"]: 1,
	}
	substitutions := map[ElementID][]ElementID{0: {1}}
	if err := ValidateSubstitutionMaps(rt, &names, elements, globals, substitutions, nil); err != nil {
		t.Fatalf("ValidateSubstitutionMaps() error = %v", err)
	}
}

func TestValidateSubstitutionMembership(t *testing.T) {
	t.Parallel()

	rt := derivationRuntimeStub{
		anyType: 0,
		complex: []ComplexTypeDerivation{
			{Kind: DerivationKindNone},
			{Base: ComplexRef(0), Kind: DerivationKindExtension},
			{Base: ComplexRef(1), Kind: DerivationKindRestriction},
			{Kind: DerivationKindNone},
		},
	}
	head := ElementDecl{Type: ComplexRef(0)}
	tests := []struct {
		wantErr error
		name    string
		head    ElementDecl
		member  ElementDecl
	}{
		{
			name:   "extension-derived member allowed",
			head:   head,
			member: ElementDecl{Type: ComplexRef(1)},
		},
		{
			name:   "restriction-derived member allowed",
			head:   head,
			member: ElementDecl{Type: ComplexRef(2)},
		},
		{
			name:    "member type must derive from head",
			head:    head,
			member:  ElementDecl{Type: ComplexRef(3)},
			wantErr: ErrSubstitutionMemberTypeNotDerived,
		},
		{
			name: "head final can block extension path",
			head: func() ElementDecl {
				el := head
				el.Final = DerivationExtension
				return el
			}(),
			member:  ElementDecl{Type: ComplexRef(1)},
			wantErr: ErrSubstitutionMemberTypeExcludedDerivation,
		},
		{
			name: "head final can block restriction path",
			head: func() ElementDecl {
				el := head
				el.Final = DerivationRestriction
				return el
			}(),
			member:  ElementDecl{Type: ComplexRef(2)},
			wantErr: ErrSubstitutionMemberTypeExcludedDerivation,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateSubstitutionMembership(rt, tt.head, tt.member)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("ValidateSubstitutionMembership() error = %v", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("ValidateSubstitutionMembership() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildSubstitutionLookup(t *testing.T) {
	t.Parallel()

	_, qnames := substitutionNameFixture(t)
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
		{Name: qnames["member"], Type: ComplexRef(1), SubstHead: 0},
		{Name: qnames["child"], Type: ComplexRef(2), SubstHead: 1},
	}
	lookup := BuildSubstitutionLookup(rt, elements, map[ElementID][]ElementID{
		0: {1, 2},
		1: {2},
	})
	if lookup[0][qnames["member"]] != 1 || lookup[0][qnames["child"]] != 2 {
		t.Fatalf("BuildSubstitutionLookup()[head] = %v", lookup[0])
	}
	if lookup[1][qnames["child"]] != 2 {
		t.Fatalf("BuildSubstitutionLookup()[member] = %v", lookup[1])
	}

	elements[0].Block = DerivationSubstitution
	lookup = BuildSubstitutionLookup(rt, elements, map[ElementID][]ElementID{0: {1}})
	if len(lookup) != 0 {
		t.Fatalf("BuildSubstitutionLookup() with substitution block = %v, want empty", lookup)
	}
}

func TestBuildSubstitutionClosure(t *testing.T) {
	t.Parallel()

	got, err := BuildSubstitutionClosure(map[ElementID][]ElementID{
		0: {2, 1},
		1: {3},
		2: {3},
	})
	if err != nil {
		t.Fatalf("BuildSubstitutionClosure() error = %v", err)
	}
	if !slices.Equal(got[0], []ElementID{1, 2, 3}) {
		t.Fatalf("BuildSubstitutionClosure()[0] = %v, want [1 2 3]", got[0])
	}
	if !slices.Equal(got[1], []ElementID{3}) {
		t.Fatalf("BuildSubstitutionClosure()[1] = %v, want [3]", got[1])
	}

	_, err = BuildSubstitutionClosure(map[ElementID][]ElementID{
		0: {1},
		1: {0},
	})
	var cycle SubstitutionCycleError
	if !errors.As(err, &cycle) || cycle.Element != 0 {
		t.Fatalf("BuildSubstitutionClosure() error = %v, want cycle at 0", err)
	}
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
