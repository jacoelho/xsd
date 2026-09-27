package value

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"unsafe"
)

func TestRawStringEnumerationMembershipAcrossShapes(t *testing.T) {
	for _, size := range []int{31, 32, 64, 512, 1024} {
		for _, shape := range []string{"short", "commonprefix", "reversed"} {
			t.Run(fmt.Sprintf("%d/%s", size, shape), func(t *testing.T) {
				values := testStringEnumerationValues(size, shape)
				builder := NewBuilder(BuilderOptions{})
				id, err := builder.Add(TypeSpec{
					Variety: Atomic, Primitive: PrimitiveString,
					Whitespace: WhitespacePreserve, WhitespacePresent: true,
					Base: NoType, ListItem: NoType,
					Facets: FacetSpec{Enumeration: testStringLiteralSpecs(values)},
				})
				if err != nil {
					t.Fatal(err)
				}
				program, err := builder.Seal()
				if err != nil {
					t.Fatal(err)
				}
				typ, ok := program.typeDef(id)
				if !ok || typ.facets.raw.kind != rawPlanStringEnumeration {
					t.Fatalf("raw plan = %#v, want string enumeration", typ.facets.raw.kind)
				}
				for _, tc := range []struct {
					name  string
					input string
					valid bool
				}{
					{name: "first", input: values[0], valid: true},
					{name: "last", input: values[len(values)-1], valid: true},
					{name: "miss", input: testStringEnumerationMiss(shape), valid: false},
				} {
					t.Run(tc.name, func(t *testing.T) {
						raw := []byte(tc.input)
						_, err := program.ValidateBytes(id, raw, Resolver{}, 0, 16<<20, nil)
						if (err == nil) != tc.valid {
							t.Fatalf("ValidateBytes(%q) error = %v, valid = %t", tc.input, err, tc.valid)
						}
						if !tc.valid && !errors.Is(err, ErrFacet) {
							t.Fatalf("ValidateBytes(%q) error = %v, want ErrFacet", tc.input, err)
						}
					})
				}
				raw := []byte(values[len(values)-1])
				if allocs := testing.AllocsPerRun(100, func() {
					if _, err := program.ValidateBytes(id, raw, Resolver{}, 0, 16<<20, nil); err != nil {
						t.Fatal(err)
					}
				}); allocs != 0 {
					t.Fatalf("large raw enumeration allocations = %v, want 0", allocs)
				}
			})
		}
	}
}

func TestLargeRawStringEnumerationRetainsDuplicateAndEmpty(t *testing.T) {
	values := []string{""}
	for i := 63; i >= 0; i-- {
		values = append(values, fmt.Sprintf("v%02d", i))
	}
	values = append(values, "v10")
	builder := NewBuilder(BuilderOptions{})
	id, err := builder.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveString,
		Whitespace: WhitespacePreserve, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{Enumeration: testStringLiteralSpecs(values)},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		input string
		valid bool
	}{
		{input: "", valid: true},
		{input: "v10", valid: true},
		{input: "v63", valid: true},
		{input: "v64", valid: false},
	} {
		_, err := program.ValidateBytes(id, []byte(tc.input), Resolver{}, 0, 16<<20, nil)
		if (err == nil) != tc.valid {
			t.Fatalf("ValidateBytes(%q) error = %v, valid = %t", tc.input, err, tc.valid)
		}
		if !tc.valid && !errors.Is(err, ErrFacet) {
			t.Fatalf("ValidateBytes(%q) error = %v, want ErrFacet", tc.input, err)
		}
	}
}

func TestLargeInheritedRawStringEnumerationKeepsGroupOwnership(t *testing.T) {
	baseValues := []string{"z"}
	for i := 63; i >= 0; i-- {
		baseValues = append(baseValues, fmt.Sprintf("v%02d", i))
	}
	baseValues = append(baseValues, "")
	derivedValues := append([]string(nil), baseValues...)
	derivedValues = append(derivedValues, "v10")

	builder := NewBuilder(BuilderOptions{})
	base, err := builder.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveString,
		Whitespace: WhitespacePreserve, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{Enumeration: testStringLiteralSpecs(baseValues)},
	})
	if err != nil {
		t.Fatal(err)
	}
	derived, err := builder.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveString,
		Whitespace: WhitespacePreserve, WhitespacePresent: true,
		Base: base, ListItem: NoType,
		Facets: FacetSpec{Enumeration: testStringLiteralSpecs(derivedValues)},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	baseType, ok := program.typeDef(base)
	if !ok {
		t.Fatal("missing base enumeration type")
	}
	derivedType, ok := program.typeDef(derived)
	if !ok {
		t.Fatal("missing derived enumeration type")
	}
	if baseType.facets.raw.kind != rawPlanStringEnumeration || derivedType.facets.raw.kind != rawPlanStringEnumeration {
		t.Fatalf("raw plans = (%d, %d), want string enumeration", baseType.facets.raw.kind, derivedType.facets.raw.kind)
	}
	if len(baseType.facets.enumGroups) != 1 || len(derivedType.facets.enumGroups) != 2 {
		t.Fatalf("enumeration groups = (%d, %d), want inherited base plus owned group", len(baseType.facets.enumGroups), len(derivedType.facets.enumGroups))
	}
	for _, input := range []string{"z", "", "v10"} {
		if _, err := program.ValidateBytes(base, []byte(input), Resolver{}, 0, 16<<20, nil); err != nil {
			t.Fatalf("base ValidateBytes(%q) error = %v", input, err)
		}
	}
	for _, input := range []string{"z", "", "v10"} {
		if _, err := program.ValidateBytes(derived, []byte(input), Resolver{}, 0, 16<<20, nil); err != nil {
			t.Fatalf("derived ValidateBytes(%q) error = %v", input, err)
		}
	}
	if _, err := program.ValidateBytes(derived, []byte("v64"), Resolver{}, 0, 16<<20, nil); !errors.Is(err, ErrFacet) {
		t.Fatalf("derived ValidateBytes(%q) error = %v, want ErrFacet", "v64", err)
	}
}
func TestBase64CanonicalReadiness(t *testing.T) {
	for _, tc := range []struct {
		name      string
		raw       string
		canonical string
		length    uint32
	}{
		{name: "empty", raw: "", canonical: "", length: 0},
		{name: "whitespace", raw: " Y\n Q == ", canonical: "YQ==", length: 1},
		{name: "two bytes", raw: "Y W I =", canonical: "YWI=", length: 2},
		{name: "no padding", raw: "YWJj", canonical: "YWJj", length: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseBinaryValue(PrimitiveBase64Binary, tc.raw, PrimitiveNeedCanonical)
			if err != nil {
				t.Fatal(err)
			}
			if got.Canonical != tc.canonical || got.Length != tc.length || !got.canonicalReady {
				t.Fatalf("ParseBinaryValue() = %#v, want canonical=%q length=%d ready", got, tc.canonical, tc.length)
			}
			unprojected, err := ParseBinaryValue(PrimitiveBase64Binary, tc.raw, 0)
			if err != nil {
				t.Fatal(err)
			}
			if unprojected.Canonical != tc.raw || unprojected.Length != tc.length || unprojected.canonicalReady {
				t.Fatalf("unprojected ParseBinaryValue() = %#v, want original spelling and not ready", unprojected)
			}
		})
	}
	if _, err := ParseBinaryValue(PrimitiveBase64Binary, "YR==", PrimitiveNeedCanonical); err == nil {
		t.Fatal("ParseBinaryValue accepted nonzero base64 pad bits")
	}
}

func TestBase64EnumerationCompactsDocumentValueOnce(t *testing.T) {
	builder := NewBuilder(BuilderOptions{})
	id, err := builder.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveBase64Binary,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{Lexical: "YQ==", Type: NoType}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"YQ==", " Y\n Q == ", "YQ=="} {
		if _, validationErr := program.Validate(id, input, Resolver{}, 0, 16<<20, nil); validationErr != nil {
			t.Fatalf("Validate(%q) error = %v", input, validationErr)
		}
	}
	canonical, err := program.Validate(id, " Y\n Q == ", Resolver{}, NeedCanonical|NeedIdentity, 16<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if canonical.CanonicalText() != "YQ==" || canonical.IdentityKey() != PrimitiveIdentityKey(PrimitiveBase64Binary, "YQ==") {
		t.Fatalf("projections = (%q, %q), want compact base64 projections", canonical.CanonicalText(), canonical.IdentityKey())
	}
	plain, err := program.Validate(id, "YQ==", Resolver{}, NeedIdentity, 16<<20, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !canonical.Equal(plain) {
		t.Fatal("equivalent base64 values were not equal")
	}
	if _, err := program.Validate(id, "YQ==", Resolver{}, 0, 4, nil); !errors.Is(err, ErrLimit) {
		t.Fatalf("runtime work below lexical charge = %v, want ErrLimit", err)
	}
	if _, err := program.Validate(id, "YQ==", Resolver{}, 0, 5, nil); err != nil {
		t.Fatalf("runtime work at lexical charge = %v", err)
	}
}

func TestBase64EqualityDoesNotMutateEitherOperand(t *testing.T) {
	left := parsedValue{atom: atomicValue{
		kind:   PrimitiveBase64Binary,
		binary: BinaryValue{Canonical: " YQ=="},
	}}
	right := parsedValue{atom: atomicValue{
		kind:   PrimitiveBase64Binary,
		binary: BinaryValue{Canonical: "YQ=="},
	}}
	if !equalParsed(&left, &right) {
		t.Fatal("equivalent unready base64 values were not equal")
	}
	if left.atom.binary.canonicalReady || right.atom.binary.canonicalReady {
		t.Fatal("equality mutated an operand; preparation belongs to enumeration evaluation")
	}
}

func TestBase64EnumerationPreparationOwnsDocumentValue(t *testing.T) {
	value := parsedValue{atom: atomicValue{
		kind:   PrimitiveBase64Binary,
		binary: BinaryValue{Canonical: " YQ=="},
	}}
	literal := parsedValue{atom: atomicValue{
		kind:   PrimitiveBase64Binary,
		binary: BinaryValue{Canonical: "YQ==", canonicalReady: true},
	}}
	facets := facetProgram{
		enumGroups: [][]parsedValue{{literal}},
		enumBase64: true,
	}
	if err := applyEnumerationFacets(&facets, &value); err != nil {
		t.Fatal(err)
	}
	if !value.atom.binary.canonicalReady || value.atom.binary.Canonical != "YQ==" {
		t.Fatalf("prepared document value = %#v, want compact ready value", value.atom.binary)
	}
	if !literal.atom.binary.canonicalReady || literal.atom.binary.Canonical != "YQ==" {
		t.Fatal("enumeration preparation mutated the schema literal")
	}
}

func TestBase64ListEnumerationPreparesRetainedItems(t *testing.T) {
	builder := NewBuilder(BuilderOptions{})
	itemID, err := builder.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveBase64Binary,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	listID, err := builder.Add(TypeSpec{
		Variety: List, Primitive: PrimitiveBase64Binary,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: itemID,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{Lexical: "YQ== Yg==", Type: NoType}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	budget, err := newEvaluationBudget(16 << 20)
	if err != nil {
		t.Fatal(err)
	}
	var value parsedValue
	if _, err := program.eval(listID, "YQ== Yg==", evalOptions{
		needs:         0,
		enforceFacets: true,
		work:          &budget,
	}, &value); err != nil {
		t.Fatal(err)
	}
	if len(value.items) != 2 {
		t.Fatalf("retained list items = %d, want 2", len(value.items))
	}
	for i := range value.items {
		if !value.items[i].atom.binary.canonicalReady {
			t.Fatalf("item %d was not prepared: %#v", i, value.items[i].atom.binary)
		}
	}
}

func TestBase64EnumerationPreparesSelectedNestedUnionListLeaves(t *testing.T) {
	builder := NewBuilder(BuilderOptions{})
	base64ID, err := builder.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveBase64Binary,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	itemUnionID, err := builder.Add(TypeSpec{
		Variety: Union, Primitive: PrimitiveBase64Binary,
		Union:      []TypeID{base64ID, builtinString},
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
	})
	if err != nil {
		t.Fatal(err)
	}
	listID, err := builder.Add(TypeSpec{
		Variety: List, Primitive: PrimitiveBase64Binary,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: itemUnionID,
	})
	if err != nil {
		t.Fatal(err)
	}
	outerID, err := builder.Add(TypeSpec{
		Variety: Union, Primitive: PrimitiveBase64Binary,
		Union:      []TypeID{listID, builtinString},
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{Lexical: "YQ== Zg==", Type: NoType}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	typ, ok := program.typeDef(outerID)
	if !ok || !typ.facets.enumBase64 {
		t.Fatalf("outer enum base64 marker = (%v, %t), want present", ok, ok && typ.facets.enumBase64)
	}
	literal := &typ.facets.enumGroups[0][0]
	if !literal.isList || len(literal.items) != 2 {
		t.Fatalf("literal shape = (list=%t, items=%d), want two-item list", literal.isList, len(literal.items))
	}
	for i := range literal.items {
		if literal.items[i].atom.kind != PrimitiveBase64Binary || !literal.items[i].atom.binary.canonicalReady {
			t.Fatalf("literal item %d = %#v, want ready base64", i, literal.items[i])
		}
	}
	literalCanonical := make([]string, len(literal.items))
	for i := range literal.items {
		literalCanonical[i] = literal.items[i].atom.binary.Canonical
	}

	budget, err := newEvaluationBudget(16 << 20)
	if err != nil {
		t.Fatal(err)
	}
	var value parsedValue
	if _, evalErr := program.eval(outerID, " YQ==  Zg== ", evalOptions{
		needs:              0,
		enforceFacets:      true,
		retainAllListItems: true,
		work:               &budget,
	}, &value); evalErr != nil {
		t.Fatalf("nested union/list evaluation error = %v", evalErr)
	}
	if value.selected != listID || !value.isList || len(value.items) != 2 {
		t.Fatalf("selected value = (selected=%d, list=%t, items=%d), want list %d with two items", value.selected, value.isList, len(value.items), listID)
	}
	for i := range value.items {
		if value.items[i].atom.kind != PrimitiveBase64Binary || !value.items[i].atom.binary.canonicalReady {
			t.Fatalf("document item %d = %#v, want prepared base64", i, value.items[i])
		}
	}
	got, err := program.Validate(outerID, " YQ==  Zg== ", Resolver{}, NeedCanonical, 16<<20, nil)
	if err != nil {
		t.Fatalf("public nested union/list validation error = %v", err)
	}
	if got.SelectedType() != listID || got.CanonicalText() != "YQ== Zg==" {
		t.Fatalf("public value = (selected=%d, canonical=%q), want list %d and compact list", got.SelectedType(), got.CanonicalText(), listID)
	}
	if _, err := program.Validate(outerID, "YQ== nope", Resolver{}, 0, 16<<20, nil); !errors.Is(err, ErrFacet) {
		t.Fatalf("mixed base64/string list error = %v, want ErrFacet", err)
	}
	for i := range literal.items {
		if literal.items[i].atom.binary.Canonical != literalCanonical[i] || !literal.items[i].atom.binary.canonicalReady {
			t.Fatalf("schema literal item %d changed after validation: %#v", i, literal.items[i])
		}
	}
}

func TestInheritedBase64EnumerationKeepsLiteralsImmutable(t *testing.T) {
	builder := NewBuilder(BuilderOptions{})
	baseID, err := builder.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveBase64Binary,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: NoType, ListItem: NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{Lexical: "YQ==", Type: NoType}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	derivedID, err := builder.Add(TypeSpec{
		Variety: Atomic, Primitive: PrimitiveBase64Binary,
		Whitespace: WhitespaceCollapse, WhitespacePresent: true,
		Base: baseID, ListItem: NoType,
		Facets: FacetSpec{Enumeration: []LiteralSpec{{Lexical: " YQ== ", Type: NoType}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	program, err := builder.Seal()
	if err != nil {
		t.Fatal(err)
	}
	typ, ok := program.typeDef(derivedID)
	if !ok || !typ.facets.enumBase64 || len(typ.facets.enumGroups) != 2 {
		t.Fatalf("derived enum state = (ok=%t, base64=%t, groups=%d), want inherited base64 groups", ok, ok && typ.facets.enumBase64, len(typ.facets.enumGroups))
	}
	literals := make([]string, 0, 2)
	for groupIndex, group := range typ.facets.enumGroups {
		if len(group) != 1 || !group[0].atom.binary.canonicalReady {
			t.Fatalf("group %d literal = %#v, want one ready base64 literal", groupIndex, group)
		}
		literals = append(literals, group[0].atom.binary.Canonical)
	}
	for _, input := range []string{"YQ==", " Y\n Q == "} {
		got, err := program.Validate(derivedID, input, Resolver{}, NeedCanonical, 16<<20, nil)
		if err != nil {
			t.Fatalf("Validate(%q) error = %v", input, err)
		}
		if got.CanonicalText() != "YQ==" {
			t.Fatalf("Validate(%q) canonical = %q, want YQ==", input, got.CanonicalText())
		}
	}
	for groupIndex, group := range typ.facets.enumGroups {
		if group[0].atom.binary.Canonical != literals[groupIndex] || !group[0].atom.binary.canonicalReady {
			t.Fatalf("schema literal group %d changed: %#v", groupIndex, group[0])
		}
	}
}

func TestBinaryValueCanonicalReadyFitsSupportedLayouts(t *testing.T) {
	want := uintptr(16)
	if unsafe.Sizeof(uintptr(0)) == 8 {
		want = 24
	}
	if got := unsafe.Sizeof(BinaryValue{}); got > want {
		t.Fatalf("BinaryValue size = %d, want at most %d for pointer size %d", got, want, unsafe.Sizeof(uintptr(0)))
	}
	if unsafe.Sizeof(uintptr(0)) == 8 && unsafe.Sizeof(parsedValue{}) > 648 {
		t.Fatalf("parsedValue size = %d, want at most 648 bytes", unsafe.Sizeof(parsedValue{}))
	}
	if got := unsafe.Sizeof(typeDef{}); uint64(got) > typeStorageBase {
		t.Fatalf("typeDef size = %d, exceeds typeStorageBase %d", got, typeStorageBase)
	}
}

func testStringEnumerationValues(size int, shape string) []string {
	values := make([]string, size)
	for i := range values {
		switch shape {
		case "short":
			values[i] = fmt.Sprintf("v%04d", i)
		case "commonprefix":
			values[i] = fmt.Sprintf("%s%04d", strings.Repeat("prefix-", 8), i)
		case "reversed":
			values[i] = fmt.Sprintf("v%04d", size-1-i)
		default:
			panic("unknown enumeration shape")
		}
	}
	return values
}

func testStringLiteralSpecs(values []string) []LiteralSpec {
	literals := make([]LiteralSpec, len(values))
	for i, value := range values {
		literals[i] = LiteralSpec{Lexical: value, Type: NoType}
	}
	return literals
}

func testStringEnumerationMiss(shape string) string {
	if shape == "commonprefix" {
		return fmt.Sprintf("%s9999", strings.Repeat("prefix-", 8))
	}
	return "missing"
}
