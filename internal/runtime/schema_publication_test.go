package runtime

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestPublishSchemaRejectsRawCorruptionWithoutMutation(t *testing.T) {
	badName := QName{Local: 1}
	build := SchemaBuild{
		GlobalElements: map[QName]ElementID{badName: 0},
		Elements:       []ElementDecl{{Name: badName}},
	}
	want := SchemaBuild{
		GlobalElements: map[QName]ElementID{badName: 0},
		Elements:       []ElementDecl{{Name: badName}},
	}

	_, err := PublishSchema(&build)
	if err == nil {
		t.Fatal("PublishSchema() succeeded for invalid name references")
	}
	if !reflect.DeepEqual(build, want) {
		t.Fatalf("PublishSchema() mutated failed build: got %#v want %#v", build, want)
	}
}

func TestProjectionAuditRejectsCorruption(t *testing.T) {
	audit := schemaAudit{
		build: SchemaBuild{Attributes: []AttributeDecl{{}}},
	}
	err := validateRuntimeReadProjections(&audit)
	if err == nil || !strings.Contains(err.Error(), "attribute declaration read projection count does not match declarations") {
		t.Fatalf("validateRuntimeReadProjections() error = %v", err)
	}
}

func TestSimpleTypeBaseAncestryPreservesAppendOrderAndIntervals(t *testing.T) {
	t.Parallel()

	types := []SimpleType{
		{Base: NoSimpleType},
		{Base: 0},
		{Base: 0},
		{Base: 1},
		{Base: NoSimpleType},
	}
	ancestry := newSimpleTypeBaseAncestry(types)
	wantStart := []int{0, 1, 3, 2, 4}
	wantEnd := []int{4, 3, 4, 3, 5}
	if !slices.Equal(ancestry.start, wantStart) || !slices.Equal(ancestry.end, wantEnd) {
		t.Fatalf("ancestry = start %v end %v, want start %v end %v", ancestry.start, ancestry.end, wantStart, wantEnd)
	}
	for _, pair := range [][2]SimpleTypeID{{0, 1}, {0, 2}, {0, 3}, {1, 3}} {
		if !ancestry.strictAncestor(pair[0], pair[1]) {
			t.Fatalf("strictAncestor(%d, %d) = false, want true", pair[0], pair[1])
		}
	}
	for _, pair := range [][2]SimpleTypeID{{1, 2}, {2, 3}, {0, 4}, {4, 0}, {1, 1}} {
		if ancestry.strictAncestor(pair[0], pair[1]) {
			t.Fatalf("strictAncestor(%d, %d) = true, want false", pair[0], pair[1])
		}
	}
}

func TestSimpleTypeBaseAncestryHandlesDeepAndFlatForests(t *testing.T) {
	t.Parallel()

	const count = 10_000
	deep := make([]SimpleType, count)
	deep[0].Base = NoSimpleType
	for i := 1; i < count; i++ {
		deep[i].Base = SimpleTypeID(i - 1)
	}
	ancestry := newSimpleTypeBaseAncestry(deep)
	if !ancestry.strictAncestor(0, count-1) || ancestry.start[count-1] != count-1 || ancestry.end[0] != count {
		t.Fatalf("deep ancestry = start tail %d end root %d", ancestry.start[count-1], ancestry.end[0])
	}

	flat := make([]SimpleType, count)
	for i := range flat {
		flat[i].Base = NoSimpleType
	}
	ancestry = newSimpleTypeBaseAncestry(flat)
	if ancestry.strictAncestor(0, count-1) || ancestry.start[count-1] != count-1 || ancestry.end[count-1] != count {
		t.Fatalf("flat ancestry = start tail %d end tail %d", ancestry.start[count-1], ancestry.end[count-1])
	}
}

func TestCompiledBoundLiteralReplayDeduplicatesSharedStorage(t *testing.T) {
	build := SchemaBuild{SimpleTypes: []SimpleType{{
		Variety:    SimpleVarietyAtomic,
		Primitive:  PrimitiveDecimal,
		Whitespace: WhitespaceCollapse,
	}}}
	audit := schemaAudit{
		Schema: Schema{runtime: newSchemaRuntime(&build)},
		build:  build,
	}
	ctx := schemaValidationContext{rt: &audit}
	literal := NewCompiledLiteralForSimpleType(
		build.SimpleTypes[0],
		0,
		"1",
		"1.0",
		nil,
	)

	if err := ctx.validateCompiledBoundLiteralOnce(&literal); err != nil {
		t.Fatalf("first validateCompiledBoundLiteralOnce() error = %v", err)
	}
	audit.runtime.SimpleValueRoutes = nil
	if err := ctx.validateCompiledBoundLiteralOnce(&literal); err != nil {
		t.Fatalf("shared validateCompiledBoundLiteralOnce() error = %v", err)
	}
	if got := len(ctx.validatedBoundLiterals); got != 1 {
		t.Fatalf("validated bound literal count = %d, want 1", got)
	}

	cloned := literal
	if err := ctx.validateCompiledBoundLiteralOnce(&cloned); err == nil {
		t.Fatal("validateCompiledBoundLiteralOnce() skipped distinct literal storage")
	}
}

func TestComplexTypeReadDerivesValidationViews(t *testing.T) {
	t.Parallel()

	ct := ComplexType{
		Content:     3,
		Attrs:       4,
		TextType:    5,
		ContentKind: ContentSimpleMixed,
		Block:       DerivationExtension,
		Abstract:    true,
	}
	read := newComplexTypeRead(ct)
	if read.contentModel != ct.Content || read.attributeUseSet != ct.Attrs {
		t.Fatalf("complex type IDs = content %d attrs %d", read.contentModel, read.attributeUseSet)
	}
	wantInfo := NewTypeInfo(TypeInfoShape{Block: ct.Block, Abstract: ct.Abstract})
	if got := read.typeInfo(); got != wantInfo {
		t.Fatalf("typeInfo() = %+v, want %+v", got, wantInfo)
	}
	wantSimple := NewSimpleContentTypeRead(SimpleContentTypeReadShape{Type: ct.TextType, Present: ct.SimpleContent()})
	if got := read.simpleContent(); got != wantSimple {
		t.Fatalf("simpleContent() = %+v, want %+v", got, wantSimple)
	}
	wantChild := NewElementChildContent(ElementChildContentShape{Complex: true, Simple: ct.SimpleContent()})
	if got := read.childContent(); got != wantChild {
		t.Fatalf("childContent() = %+v, want %+v", got, wantChild)
	}
	for _, fixed := range []bool{false, true} {
		wantText := NewElementTextContent(ElementTextContentShape{
			Simple: ct.SimpleContent(), Complex: true, Mixed: ct.Mixed(), Fixed: fixed,
		})
		if got := read.textContent(fixed); got != wantText {
			t.Fatalf("textContent(%v) = %+v, want %+v", fixed, got, wantText)
		}
	}
}

func TestSimpleValueColdReadExcludesCompilerSources(t *testing.T) {
	t.Parallel()

	pattern := NewFastStringPattern(CompileSimpleStringPattern("abc"))
	facets := FacetSet{
		Enumeration: []CompiledLiteral{{Lexical: "compiler-enumeration-source", Canonical: "value"}},
		Present:     FacetEnumeration | FacetPattern,
	}
	facets.patterns = newTestStringPatternSteps([][]StringPattern{{pattern}})
	SetBoundFacet(&facets, FacetMinInclusive, CompiledLiteral{
		Lexical:   "compiler-bound-source",
		Canonical: "bound",
	}, false)
	types := []SimpleType{{Facets: facets}}
	reads := newSimpleValueColdReadTable(types)

	read, ok := reads.read(0)
	if !ok || read == nil {
		t.Fatal("simple value cold read is missing")
	}
	if len(read.enumeration) != 1 || read.enumeration[0].canonical != "value" {
		t.Fatalf("enumeration read = %#v", read.enumeration)
	}
	bound, present := read.facets.bound(FacetMinInclusive)
	if !present || bound.canonical != "bound" {
		t.Fatalf("bound read = %#v, %v", bound, present)
	}
	if err := validateStringPatternStepReads(read.facets.patterns, "abc"); err != nil {
		t.Fatalf("pattern read rejected matching text: %v", err)
	}

	types[0].Facets.Enumeration[0].Lexical = "changed-enumeration-source"
	types[0].Facets.bounds[minInclusiveBoundIndex].Lexical = "changed-bound-source"
	if err := validateSimpleValueColdReadProjectionForTypes(reads, types); err != nil {
		t.Fatalf("projection audit depends on discarded compiler sources: %v", err)
	}
}

func TestSimpleValueColdReadAuditRejectsMissingBoundActual(t *testing.T) {
	t.Parallel()

	parsed, err := ParsePrimitiveActual(PrimitiveDecimal, "1", 0)
	if err != nil {
		t.Fatal(err)
	}
	facets := FacetSet{}
	SetBoundFacet(&facets, FacetMinInclusive, CompiledLiteral{
		Lexical:   "1",
		Canonical: parsed.Canonical,
		Actual:    parsed.Actual,
	}, false)
	types := []SimpleType{{Facets: facets}}
	reads := newSimpleValueColdReadTable(types)
	reads.values[0].facets.bounds[minInclusiveBoundIndex].actual.Valid = false

	if err := validateSimpleValueColdReadProjectionForTypes(reads, types); err == nil {
		t.Fatal("projection audit accepted a bound without its required actual value")
	}
}

func TestSimpleValueColdReadInternsInheritedBounds(t *testing.T) {
	t.Parallel()

	facets := FacetSet{}
	SetBoundFacet(&facets, FacetMinInclusive, CompiledLiteral{Canonical: "1"}, false)
	types := []SimpleType{{Facets: facets}, {Facets: facets}}
	reads := newSimpleValueColdReadTable(types)

	first := reads.values[0].facets.bounds[minInclusiveBoundIndex]
	second := reads.values[1].facets.bounds[minInclusiveBoundIndex]
	if first == nil || first != second || len(reads.boundReads) != 1 {
		t.Fatalf("inherited bound reads = %p, %p, pool %d; want one shared read", first, second, len(reads.boundReads))
	}
	if err := validateSimpleValueColdReadProjectionForTypes(reads, types); err != nil {
		t.Fatalf("validateSimpleValueColdReadProjectionForTypes() error = %v", err)
	}
	duplicate := *second
	reads.values[1].facets.bounds[minInclusiveBoundIndex] = &duplicate
	if err := validateSimpleValueColdReadProjectionForTypes(reads, types); err == nil {
		t.Fatal("projection audit accepted a duplicate inherited bound read")
	}
}

func TestSimpleValueColdReadInternsInheritedEnumerations(t *testing.T) {
	t.Parallel()

	facets := FacetSet{
		Enumeration: []CompiledLiteral{{Canonical: "a"}},
		Present:     FacetEnumeration,
	}
	distinct := FacetSet{
		Enumeration: slices.Clone(facets.Enumeration),
		Present:     FacetEnumeration,
	}
	types := []SimpleType{{Facets: facets}, {Facets: facets}, {Facets: distinct}}
	reads := newSimpleValueColdReadTable(types)

	first := reads.values[0].enumeration
	second := reads.values[1].enumeration
	third := reads.values[2].enumeration
	if len(first) != 1 || len(second) != 1 || len(third) != 1 {
		t.Fatalf("enumeration read lengths = %d, %d, %d; want 1", len(first), len(second), len(third))
	}
	if &first[0] != &second[0] {
		t.Fatalf("inherited enumeration reads do not share storage: %p, %p", &first[0], &second[0])
	}
	if &first[0] == &third[0] {
		t.Fatalf("distinct enumeration sources share read storage: %p, %p", &first[0], &third[0])
	}
	if err := validateSimpleValueColdReadProjectionForTypes(reads, types); err != nil {
		t.Fatalf("validateSimpleValueColdReadProjectionForTypes() error = %v", err)
	}
	reads.values[1].enumeration = slices.Clone(second)
	if err := validateSimpleValueColdReadProjectionForTypes(reads, types); err == nil {
		t.Fatal("projection audit accepted a duplicate inherited enumeration read")
	}
}

func TestSimpleValueColdReadInternsInheritedPatterns(t *testing.T) {
	t.Parallel()

	patterns := [][]StringPattern{{
		NewFastStringPattern(CompileSimpleStringPattern("[A-Z]")),
	}}
	facets := FacetSet{patterns: newTestStringPatternSteps(patterns), Present: FacetPattern}
	distinct := FacetSet{patterns: newTestStringPatternSteps(patterns), Present: FacetPattern}
	appended := facets
	AppendPatternFacetGroup(&appended, []StringPattern{
		NewFastStringPattern(CompileSimpleStringPattern("[0-9]")),
	})
	types := []SimpleType{{Facets: facets}, {Facets: facets}, {Facets: distinct}, {Facets: appended}}
	reads := newSimpleValueColdReadTable(types)

	first := reads.values[0].facets.patterns
	second := reads.values[1].facets.patterns
	third := reads.values[2].facets.patterns
	fourth := reads.values[3].facets.patterns
	if first == nil || second == nil || third == nil || first.count != 1 || second.count != 1 || third.count != 1 {
		t.Fatalf("pattern reads = %#v, %#v, %#v; want one step", first, second, third)
	}
	if first != second {
		t.Fatalf("inherited pattern reads do not share storage: %p, %p", first, second)
	}
	if first == third {
		t.Fatalf("distinct pattern sources share read storage: %p, %p", first, third)
	}
	if fourth == nil || fourth.count != 2 || fourth.parent != first {
		t.Fatalf("appended pattern read = %#v, want one new step over %p", fourth, first)
	}
	if err := validateSimpleValueColdReadProjectionForTypes(reads, types); err != nil {
		t.Fatalf("validateSimpleValueColdReadProjectionForTypes() error = %v", err)
	}
	duplicate := *second
	reads.values[1].facets.patterns = &duplicate
	if err := validateSimpleValueColdReadProjectionForTypes(reads, types); err == nil {
		t.Fatal("projection audit accepted a duplicate inherited pattern read")
	}
}
