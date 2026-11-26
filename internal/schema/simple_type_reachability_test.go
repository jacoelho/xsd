package schema

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/value"

	"github.com/jacoelho/xsd/xsderrors"
)

// reachedSimpleTypesSchema names each anonymous simple type through one edge
// only: element e's type, element c's text type, root's attribute use a, the
// global attribute g, the global type T and T's base, the list item of L and
// the union member of U. UNREACHED marks where zero-count elements go.
const reachedSimpleTypesSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="g"><xs:simpleType><xs:restriction base="xs:string"><xs:maxLength value="4"/></xs:restriction></xs:simpleType></xs:attribute>
  <xs:simpleType name="T"><xs:restriction><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="[a-z]+"/></xs:restriction></xs:simpleType><xs:maxLength value="3"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="L"><xs:list><xs:simpleType><xs:restriction base="xs:int"><xs:minInclusive value="0"/></xs:restriction></xs:simpleType></xs:list></xs:simpleType>
  <xs:simpleType name="U"><xs:union memberTypes="xs:boolean"><xs:simpleType><xs:restriction base="xs:date"><xs:minInclusive value="2000-01-01"/></xs:restriction></xs:simpleType></xs:union></xs:simpleType>
  <xs:complexType name="B"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="e"><xs:simpleType><xs:restriction base="xs:string"><xs:length value="2"/></xs:restriction></xs:simpleType></xs:element>
    <xs:element name="c"><xs:complexType><xs:simpleContent><xs:restriction base="B"><xs:maxLength value="3"/></xs:restriction></xs:simpleContent></xs:complexType></xs:element>
    UNREACHED
  </xs:sequence><xs:attribute name="a"><xs:simpleType><xs:restriction base="xs:string"><xs:minLength value="1"/></xs:restriction></xs:simpleType></xs:attribute></xs:complexType></xs:element>
</xs:schema>`

const unreachedSimpleTypes = `<xs:element name="z1" minOccurs="0" maxOccurs="0"><xs:simpleType><xs:restriction base="xs:string"><xs:pattern value="[0-9]+"/><xs:enumeration value="12"/></xs:restriction></xs:simpleType></xs:element>
    <xs:element name="z2" minOccurs="0" maxOccurs="0"><xs:complexType><xs:attribute name="za"><xs:simpleType><xs:restriction base="xs:string"><xs:enumeration value="x"/></xs:restriction></xs:simpleType></xs:attribute></xs:complexType></xs:element>
    <xs:element name="z3" minOccurs="0" maxOccurs="0"><xs:complexType><xs:simpleContent><xs:restriction base="B"><xs:enumeration value="y"/></xs:restriction></xs:simpleContent></xs:complexType></xs:element>
    <xs:element name="z4" minOccurs="0" maxOccurs="0"><xs:simpleType><xs:list><xs:simpleType><xs:restriction base="xs:int"><xs:maxInclusive value="9"/></xs:restriction></xs:simpleType></xs:list></xs:simpleType></xs:element>`

func reachedSimpleTypesBuild(t *testing.T, unreached string) *schemaBuild {
	t.Helper()
	return compiledTestBuild(t, strings.Replace(reachedSimpleTypesSchema, "UNREACHED", unreached, 1))
}

func buildElementType(t *testing.T, build *schemaBuild, local string) TypeID {
	t.Helper()
	name := mustBuildQName(t, build, local)
	for i := range build.Elements {
		if build.Elements[i].Name == name {
			return build.Elements[i].Type
		}
	}
	t.Fatalf("no element declaration %q", local)
	return TypeID{}
}

func buildSimpleElementType(t *testing.T, build *schemaBuild, local string) SimpleTypeID {
	t.Helper()
	id, ok := buildElementType(t, build, local).Simple()
	if !ok {
		t.Fatalf("element %q has no simple type", local)
	}
	return id
}

func buildComplexElementType(t *testing.T, build *schemaBuild, local string) *complexTypeMetadata {
	t.Helper()
	id, ok := buildElementType(t, build, local).Complex()
	if !ok {
		t.Fatalf("element %q has no complex type", local)
	}
	return &build.ComplexTypes[id]
}

func buildFirstAttributeUseType(t *testing.T, build *schemaBuild, element string) SimpleTypeID {
	t.Helper()
	uses := build.AttributeUseSets[buildComplexElementType(t, build, element).Attrs].Uses
	if len(uses) == 0 {
		t.Fatalf("element %q has no attribute use", element)
	}
	return uses[0].Type
}

func buildGlobalSimpleType(t *testing.T, build *schemaBuild, local string) SimpleTypeID {
	t.Helper()
	id, ok := build.GlobalTypes[mustBuildQName(t, build, local)].Simple()
	if !ok {
		t.Fatalf("global type %q is not simple", local)
	}
	return id
}

// discardedSimpleTypes returns the user types whose validation reports
// value.ErrMetadata, the result of a type the value program did not keep.
func discardedSimpleTypes(t *testing.T, rt *Schema) []SimpleTypeID {
	t.Helper()
	var discarded []SimpleTypeID
	for i := int(value.BuiltinTypeCount); i < len(rt.program.SimpleTypeUnavailable); i++ {
		id := SimpleTypeID(i) // #nosec G115 -- i indexes the simple-type table.
		_, err := rt.ValueProgram().Validate(id, "1", value.Resolver{}, 0, value.EvaluationLimits{Work: 16 << 20, PatternWork: 1 << 30, PatternTotal: testPatternTotal()}, nil)
		if errors.Is(err, value.ErrMetadata) {
			discarded = append(discarded, id)
		}
	}
	return discarded
}

func TestSimpleTypesNamedThroughOneEdgeKeepTheirValueDefinition(t *testing.T) {
	t.Parallel()

	rt, err := publishSchema(reachedSimpleTypesBuild(t, ""), unlimitedContentModelWork)
	if err != nil {
		t.Fatalf("publishSchema() error = %v", err)
	}
	if discarded := discardedSimpleTypes(t, rt); len(discarded) != 0 {
		t.Fatalf("discarded simple types = %v, want none: every type is named through one edge", discarded)
	}
}

// zeroCountSimpleTypes returns the simple types of build declared under z2
// and z3, found by the enumerations only they declare: z2's attribute type
// and z3's text type.
func zeroCountSimpleTypes(t *testing.T, build *schemaBuild) []SimpleTypeID {
	t.Helper()
	var ids []SimpleTypeID
	for i := range build.SimpleTypes {
		id := SimpleTypeID(i) // #nosec G115 -- i indexes the simple-type table.
		if buildTypeView(t, build, id).HasEnumeration {
			ids = append(ids, id)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("zero-count simple types = %v, want z2's attribute type and z3's text type", ids)
	}
	return ids
}

func buildTypeView(t *testing.T, build *schemaBuild, id SimpleTypeID) value.TypeView {
	t.Helper()
	view, ok := build.valueBuilder.TypeView(id)
	if !ok {
		t.Fatalf("simple type %d has no value definition", id)
	}
	return view
}

func TestUnreachedSimpleTypesPublishNoValueDefinition(t *testing.T) {
	t.Parallel()

	build := reachedSimpleTypesBuild(t, unreachedSimpleTypes)
	want := zeroCountSimpleTypes(t, build)
	rt, err := publishSchema(build, unlimitedContentModelWork)
	if err != nil {
		t.Fatalf("publishSchema() error = %v", err)
	}
	if discarded := discardedSimpleTypes(t, rt); !slices.Equal(discarded, want) {
		t.Fatalf("discarded simple types = %v, want %v, the types under zero-count elements", discarded, want)
	}
}

func TestZeroCountElementKeepsNoInlineSimpleType(t *testing.T) {
	t.Parallel()

	withInline := reachedSimpleTypesBuild(t, unreachedSimpleTypes)
	withoutInline := reachedSimpleTypesBuild(t, strings.Join(slices.DeleteFunc(strings.Split(unreachedSimpleTypes, "\n"), func(line string) bool {
		return strings.Contains(line, `name="z1"`) || strings.Contains(line, `name="z4"`)
	}), "\n"))
	if got, want := len(withInline.SimpleTypes), len(withoutInline.SimpleTypes); got != want {
		t.Fatalf("simple types with z1 and z4 = %d, want %d as without them: their inline types and z4's list item are dropped", got, want)
	}
	if got, want := userTypeCount(withInline.valueBuilder), userTypeCount(withoutInline.valueBuilder); got != want {
		t.Fatalf("value types with z1 and z4 = %d, want %d as without them", got, want)
	}
}

func TestProhibitedAttributeKeepsNoInlineSimpleType(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="B"><xs:attribute name="p" type="xs:string"/></xs:complexType>
  <xs:element name="r"><xs:complexType><xs:complexContent><xs:restriction base="B">
    <xs:attribute name="p" use="prohibited">PROHIBITED</xs:attribute>
  </xs:restriction></xs:complexContent></xs:complexType></xs:element>
</xs:schema>`
	withInline := compiledTestBuild(t, strings.Replace(schema, "PROHIBITED", `<xs:simpleType><xs:list><xs:simpleType><xs:restriction base="xs:int"><xs:maxInclusive value="9"/></xs:restriction></xs:simpleType></xs:list></xs:simpleType>`, 1))
	withoutInline := compiledTestBuild(t, strings.Replace(schema, "PROHIBITED", "", 1))
	if got, want := len(withInline.SimpleTypes), len(withoutInline.SimpleTypes); got != want {
		t.Fatalf("simple types with p's inline list = %d, want %d as without it: the list and its item are dropped", got, want)
	}
	if got, want := userTypeCount(withInline.valueBuilder), userTypeCount(withoutInline.valueBuilder); got != want {
		t.Fatalf("value types with p's inline list = %d, want %d as without it", got, want)
	}
}

func TestZeroCountInlineTypeCreatingTheMissingTypeKeepsOnlyThePlaceholder(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:o="urn:o">
  <xs:import namespace="urn:o"/>
  <xs:complexType name="W"><xs:sequence>
    ZERO
  </xs:sequence></xs:complexType>
  <xs:attribute name="g"><xs:simpleType><xs:list itemType="o:x"/></xs:simpleType></xs:attribute>
</xs:schema>`
	build := compiledTestBuild(t, strings.Replace(schema, "ZERO", `<xs:element name="z" minOccurs="0" maxOccurs="0"><xs:simpleType><xs:list itemType="o:x"/></xs:simpleType></xs:element>`, 1))
	withoutZ := compiledTestBuild(t, strings.Replace(schema, "ZERO", "", 1))
	if got, want := len(build.SimpleTypes), len(withoutZ.SimpleTypes); got != want {
		t.Fatalf("simple types with z = %d, want %d as without z: z's inline list is dropped and the placeholder it created is kept", got, want)
	}
	g := build.Attributes[build.GlobalAttributes[mustBuildQName(t, build, "g")]].Type
	item := buildTypeView(t, build, g).ListItem
	if !validSimpleTypeID(item, len(build.SimpleTypes)) || !build.SimpleTypes[item].Missing {
		t.Fatalf("g's list item = %d of %d simple types, want the missing-type placeholder", item, len(build.SimpleTypes))
	}
	rt, err := publishSchema(build, unlimitedContentModelWork)
	if err != nil {
		t.Fatalf("publishSchema() error = %v", err)
	}
	if discarded := discardedSimpleTypes(t, rt); len(discarded) != 0 {
		t.Fatalf("discarded simple types = %v, want none: g's list and the placeholder it names are reached", discarded)
	}
}

func TestPublicationRejectsAReadNamingAnUnreachedSimpleType(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		holder  func(*testing.T, *schemaBuild) SimpleTypeID
		message string
		global  bool
	}{
		{name: "element type", message: "published element declaration names an unreached simple type", holder: func(t *testing.T, build *schemaBuild) SimpleTypeID {
			t.Helper()
			return buildSimpleElementType(t, build, "e")
		}},
		{name: "complex text type", message: "published complex type names an unreached text type", holder: func(t *testing.T, build *schemaBuild) SimpleTypeID {
			t.Helper()
			return buildComplexElementType(t, build, "c").TextType
		}},
		{name: "attribute use type", message: "published attribute use names an unreached simple type", holder: func(t *testing.T, build *schemaBuild) SimpleTypeID {
			t.Helper()
			return buildFirstAttributeUseType(t, build, "root")
		}},
		{name: "global attribute type", message: "published attribute declaration names an unreached simple type", holder: func(t *testing.T, build *schemaBuild) SimpleTypeID {
			t.Helper()
			name := mustBuildQName(t, build, "g")
			for _, decl := range build.Attributes {
				if decl.Name == name {
					return decl.Type
				}
			}
			t.Fatal("no global attribute g")
			return NoSimpleType
		}},
		{name: "global type", message: "global simple type is unreached", global: true, holder: func(t *testing.T, build *schemaBuild) SimpleTypeID {
			t.Helper()
			return buildGlobalSimpleType(t, build, "T")
		}},
		{name: "base of a reached type", message: "sealing the value program: type", holder: func(t *testing.T, build *schemaBuild) SimpleTypeID {
			t.Helper()
			return buildTypeView(t, build, buildGlobalSimpleType(t, build, "T")).Base
		}},
		{name: "list item of a reached type", message: "sealing the value program: type", holder: func(t *testing.T, build *schemaBuild) SimpleTypeID {
			t.Helper()
			return buildTypeView(t, build, buildGlobalSimpleType(t, build, "L")).ListItem
		}},
		{name: "union member of a reached type", message: "sealing the value program: type", holder: func(t *testing.T, build *schemaBuild) SimpleTypeID {
			t.Helper()
			return buildTypeView(t, build, buildGlobalSimpleType(t, build, "U")).Union[1]
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			build := reachedSimpleTypesBuild(t, "")
			id := test.holder(t, build)
			if global := build.SimpleTypes[id].Scope == declarationScopeGlobal; global != test.global {
				t.Fatalf("holder names type %d, global %v, want global %v", id, global, test.global)
			}
			build.ReachableSimpleTypes[id-value.BuiltinTypeCount] = false
			_, err := publishSchema(build, unlimitedContentModelWork)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("publishSchema() error = %v, want %q", err, test.message)
			}
			expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
			build.ReachableSimpleTypes[id-value.BuiltinTypeCount] = true
			if _, err := publishSchema(build, unlimitedContentModelWork); err != nil {
				t.Fatalf("publishSchema() after the rejected publication error = %v, want the build unchanged", err)
			}
		})
	}
}

func TestPublicationRejectsReachableSimpleTypesThatDoNotCoverTheSimpleTypes(t *testing.T) {
	t.Parallel()

	build := reachedSimpleTypesBuild(t, "")
	reached := build.ReachableSimpleTypes
	for _, cover := range [][]bool{reached[:len(reached)-1], append(slices.Clone(reached), true), nil} {
		build.ReachableSimpleTypes = cover
		_, err := publishSchema(build, unlimitedContentModelWork)
		if err == nil || !strings.Contains(err.Error(), "reachable simple types do not cover the simple types") {
			t.Fatalf("publishSchema(%d reachable for %d simple types) error = %v, want a coverage invariant", len(cover), len(build.SimpleTypes), err)
		}
		expectDiagnostic(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	}
	build.ReachableSimpleTypes = reached
	if _, err := publishSchema(build, unlimitedContentModelWork); err != nil {
		t.Fatalf("publishSchema() after the rejected publications error = %v, want the build unchanged", err)
	}
}
