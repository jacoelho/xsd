package schema

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestPatternFacetAlternativesIntersectInheritedGroups(t *testing.T) {
	const text = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Grouped"><xs:restriction base="xs:string">
    <xs:pattern value="a|ab"/><xs:pattern value="c|cd"/>
  </xs:restriction></xs:simpleType>
  <xs:simpleType name="Derived"><xs:restriction base="Grouped">
    <xs:pattern value="[a-z]{2}|[0-9]+"/>
  </xs:restriction></xs:simpleType>
</xs:schema>`
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	compiler, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := compiler.compileMappedSources([]source.Source{source.Bytes("patterns.xsd", []byte(text))})
	if err != nil {
		t.Fatal(err)
	}
	name, known := compiled.LookupQName("", "Derived")
	if !known {
		t.Fatal("missing Derived name")
	}
	typeID, known := compiled.Type(name)
	if !known {
		t.Fatal("missing Derived type")
	}
	simple, valid := typeID.Simple()
	if !valid {
		t.Fatal("Derived is not simple")
	}
	for _, test := range []struct {
		input string
		valid bool
	}{{"ab", true}, {"cd", true}, {"a", false}, {"12", false}, {"az", false}} {
		_, err := compiled.ValueProgram().Validate(simple, test.input, value.Resolver{}, 0, 16<<20, nil)
		if (err == nil) != test.valid {
			t.Fatalf("input %q: %v", test.input, err)
		}
	}
}

func TestPatternSyntaxErrorPreservesSourceLocation(t *testing.T) {
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	compiler, err := newCompiler(limits)
	if err != nil {
		t.Fatal(err)
	}
	const text = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Bad"><xs:restriction base="xs:string">
    <xs:pattern value="a|ab"/>
    <xs:pattern value="["/>
  </xs:restriction></xs:simpleType>
</xs:schema>`
	compiled, err := compiler.compileMappedSources([]source.Source{source.Bytes("patterns.xsd", []byte(text))})
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if compiled != nil || !ok || diagnostic.Code() != xsderrors.CodeSchemaFacet || diagnostic.Path() != "patterns.xsd" || diagnostic.Line() != 4 || !strings.Contains(diagnostic.Error(), "invalid pattern:") {
		t.Fatalf("invalid pattern source diagnostic = %+v, %v", compiled, err)
	}
}
