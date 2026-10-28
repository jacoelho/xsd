package value_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xsdregex"
)

// TestDocumentIdentityProjectionAcrossShapes pins the ID/IDREF projection for
// every way a simple type can reach xs:ID or xs:IDREF: directly, by
// restriction, as a list item, or as a union member at any depth.
func TestDocumentIdentityProjectionAcrossShapes(t *testing.T) {
	pattern, err := xsdregex.Compile("i[0-9]", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	unionPattern, err := xsdregex.Compile("[ir][0-9]", xsdregex.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	builder := value.NewBuilder(value.BuilderOptions{})
	builtin := func(name string) value.TypeID {
		id, ok := value.BuiltinTypeID(name)
		if !ok {
			t.Fatalf("builtin %s missing", name)
		}
		return id
	}
	id, idref, idrefs := builtin("ID"), builtin("IDREF"), builtin("IDREFS")
	integer, str := builtin("int"), builtin("string")
	restrictAtomic := func(base value.TypeID, facets value.FacetSpec) value.TypeID {
		return completeCanonicalProjectionType(t, builder, value.TypeSpec{
			Variety: value.Atomic, Primitive: value.PrimitiveString, Builtin: value.BuiltinNCName,
			Base: base, ListItem: value.NoType, Facets: facets,
		})
	}
	listOf := func(item value.TypeID) value.TypeID {
		return completeCanonicalProjectionType(t, builder, value.TypeSpec{
			Variety: value.List, Base: value.NoType, ListItem: item,
		})
	}
	unionOf := func(members ...value.TypeID) value.TypeID {
		return completeCanonicalProjectionType(t, builder, value.TypeSpec{
			Variety: value.Union, Base: value.NoType, ListItem: value.NoType, Union: members,
		})
	}
	restrictedID := restrictAtomic(id, value.FacetSpec{})
	restrictedIDREF := restrictAtomic(idref, value.FacetSpec{})
	patternID := restrictAtomic(id, value.FacetSpec{Patterns: [][]*value.Pattern{{pattern}}})
	pairOfIDREFS := completeCanonicalProjectionType(t, builder, value.TypeSpec{
		Variety: value.List, Primitive: value.PrimitiveString, Base: idrefs, ListItem: idref,
		Facets: value.FacetSpec{Length: value.CardinalityFacet{Value: 2, Present: true}},
	})
	idUnion := unionOf(patternID, idref)
	refList := listOf(idref)
	idList := listOf(id)
	singleIDList := completeCanonicalProjectionType(t, builder, value.TypeSpec{
		Variety: value.List, Primitive: value.PrimitiveString, Base: idList, ListItem: id,
		Facets: value.FacetSpec{MaxLength: value.CardinalityFacet{Value: 1, Present: true}},
	})
	shapes := map[string]value.TypeID{
		"union(patternID,IDREF)": idUnion,
		"restriction of union(patternID,IDREF)": completeCanonicalProjectionType(t, builder, value.TypeSpec{
			Variety: value.Union, Base: idUnion, ListItem: value.NoType, Union: []value.TypeID{patternID, idref},
			Facets: value.FacetSpec{Patterns: [][]*value.Pattern{{unionPattern}}},
		}),
		// A member rejected after collecting item projections must not leak them.
		"union(IDREFS length 2,string)":           unionOf(pairOfIDREFS, str),
		"union(maxLength 1 list(ID),list(IDREF))": unionOf(singleIDList, refList),
		"restriction of list(IDREF)": completeCanonicalProjectionType(t, builder, value.TypeSpec{
			Variety: value.List, Primitive: value.PrimitiveString, Base: refList, ListItem: idref,
			Facets: value.FacetSpec{MaxLength: value.CardinalityFacet{Value: 3, Present: true}},
		}),
		"ID":                             id,
		"restriction of ID":              restrictedID,
		"IDREF":                          idref,
		"restriction of IDREF":           restrictedIDREF,
		"IDREFS":                         idrefs,
		"IDREFS length 2":                pairOfIDREFS,
		"list(IDREF)":                    refList,
		"list(ID)":                       idList,
		"list(restriction of IDREF)":     listOf(restrictedIDREF),
		"list(union(patternID,IDREF))":   listOf(unionOf(patternID, idref)),
		"list(union(int,ID))":            listOf(unionOf(integer, id)),
		"list(union(int,IDREF))":         listOf(unionOf(integer, idref)),
		"union(list(ID),int)":            unionOf(listOf(id), integer),
		"union(list(IDREF),string)":      unionOf(refList, str),
		"union(union(int,IDREF),string)": unionOf(unionOf(integer, idref), str),
		"union(string,ID)":               unionOf(str, id),
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		shape, lexical, canonical, ids, idrefs string
		// selected names the shape the union must select, when asserted.
		selected     string
		carriesID    bool
		facetFailure bool
	}{
		{shape: "ID", lexical: " item ", canonical: "item", ids: "item", carriesID: true},
		{shape: "restriction of ID", lexical: " item ", canonical: "item", ids: "item", carriesID: true},
		{shape: "IDREF", lexical: " r ", canonical: "r", idrefs: "r"},
		{shape: "restriction of IDREF", lexical: " r ", canonical: "r", idrefs: "r"},
		{shape: "IDREFS", lexical: " a  b\tc ", canonical: "a b c", idrefs: "a b c"},
		{shape: "IDREFS length 2", lexical: "a b", canonical: "a b", idrefs: "a b"},
		{shape: "IDREFS length 2", lexical: "a b c", facetFailure: true},
		{shape: "union(patternID,IDREF)", lexical: "i1", canonical: "i1", ids: "i1", carriesID: true},
		{shape: "union(patternID,IDREF)", lexical: " r1 ", canonical: "r1", idrefs: "r1", carriesID: true},
		{shape: "restriction of union(patternID,IDREF)", lexical: "i1", canonical: "i1", ids: "i1", carriesID: true},
		{shape: "restriction of union(patternID,IDREF)", lexical: " r1 ", canonical: "r1", idrefs: "r1", carriesID: true},
		{shape: "restriction of union(patternID,IDREF)", lexical: "x1", carriesID: true, facetFailure: true},
		{shape: "union(IDREFS length 2,string)", lexical: "a b", canonical: "a b", idrefs: "a b"},
		{shape: "union(IDREFS length 2,string)", lexical: "a b c", canonical: "a b c"},
		{shape: "union(maxLength 1 list(ID),list(IDREF))", lexical: "a b", canonical: "a b", idrefs: "a b", carriesID: true},
		{shape: "IDREFS", lexical: "", facetFailure: true},
		{shape: "list(IDREF)", lexical: ""},
		{shape: "list(IDREF)", lexical: "  \t "},
		{shape: "list(union(int,ID))", lexical: "", carriesID: true},
		{shape: "union(list(IDREF),string)", lexical: "", selected: "list(IDREF)"},
		{shape: "restriction of list(IDREF)", lexical: "a  b", canonical: "a b", idrefs: "a b"},
		{shape: "restriction of list(IDREF)", lexical: "a b c d", facetFailure: true},
		{shape: "list(IDREF)", lexical: " a\tb ", canonical: "a b", idrefs: "a b"},
		{shape: "list(ID)", lexical: "a b", canonical: "a b", ids: "a b", carriesID: true},
		{shape: "list(restriction of IDREF)", lexical: "a  b", canonical: "a b", idrefs: "a b"},
		{shape: "list(union(patternID,IDREF))", lexical: "i1 r1 i2", canonical: "i1 r1 i2", ids: "i1 i2", idrefs: "r1", carriesID: true},
		{shape: "list(union(int,ID))", lexical: "5 x 7 y", canonical: "5 x 7 y", ids: "x y", carriesID: true},
		{shape: "list(union(int,IDREF))", lexical: "1 r 2", canonical: "1 r 2", idrefs: "r"},
		{shape: "union(list(ID),int)", lexical: "a b", canonical: "a b", ids: "a b", carriesID: true},
		{shape: "union(list(ID),int)", lexical: "5", canonical: "5", carriesID: true},
		{shape: "union(list(IDREF),string)", lexical: "r s", canonical: "r s", idrefs: "r s"},
		{shape: "union(union(int,IDREF),string)", lexical: "ref", canonical: "ref", idrefs: "ref"},
		{shape: "union(union(int,IDREF),string)", lexical: "7", canonical: "7"},
		// The first matching member wins; xs:string never projects identity.
		{shape: "union(string,ID)", lexical: "item", canonical: "item", carriesID: true},
	} {
		t.Run(tc.shape+"/"+tc.lexical, func(t *testing.T) {
			typ := shapes[tc.shape]
			input, ok := program.InputRequirements(typ)
			if !ok || !input.DocumentIdentity || input.CarriesID != tc.carriesID {
				t.Fatalf("InputRequirements = %+v, %t; want DocumentIdentity and CarriesID=%t", input, ok, tc.carriesID)
			}
			for _, needs := range []value.Needs{0, value.NeedCanonical, value.NeedIdentity, value.NeedCanonical | value.NeedIdentity} {
				fromString, stringErr := program.Validate(typ, tc.lexical, value.Resolver{}, needs, 16<<20, nil)
				fromBytes, bytesErr := program.ValidateBytes(typ, []byte(tc.lexical), value.Resolver{}, needs, 16<<20, nil)
				if tc.facetFailure {
					if !errors.Is(stringErr, value.ErrFacet) || !errors.Is(bytesErr, value.ErrFacet) {
						t.Fatalf("needs %d: errors = %v, %v; want facet failure", needs, stringErr, bytesErr)
					}
					continue
				}
				if stringErr != nil || bytesErr != nil {
					t.Fatalf("needs %d: errors = %v, %v", needs, stringErr, bytesErr)
				}
				for _, got := range []value.Value{fromString, fromBytes} {
					if got.IDs() != tc.ids || got.IDRefs() != tc.idrefs {
						t.Fatalf("needs %d: IDs=%q, IDRefs=%q; want %q, %q", needs, got.IDs(), got.IDRefs(), tc.ids, tc.idrefs)
					}
					// IDTyped follows the validating type, not the selected member.
					if got.IDTyped() != tc.carriesID {
						t.Fatalf("needs %d: IDTyped=%t, want %t", needs, got.IDTyped(), tc.carriesID)
					}
					if tc.selected != "" && got.SelectedType() != shapes[tc.selected] {
						t.Fatalf("needs %d: selected type %d, want %s (%d)", needs, got.SelectedType(), tc.selected, shapes[tc.selected])
					}
					wantCanonical := ""
					if needs.Has(value.NeedCanonical) {
						wantCanonical = tc.canonical
					}
					if got.CanonicalText() != wantCanonical {
						t.Fatalf("needs %d: canonical=%q, want %q", needs, got.CanonicalText(), wantCanonical)
					}
					if got.HasIdentity() != needs.Has(value.NeedIdentity) {
						t.Fatalf("needs %d: identity availability = %t", needs, got.HasIdentity())
					}
				}
				if fromString.IdentityKey() != fromBytes.IdentityKey() {
					t.Fatalf("needs %d: identity keys differ: %q vs %q", needs, fromString.IdentityKey(), fromBytes.IdentityKey())
				}
			}
		})
	}
}
