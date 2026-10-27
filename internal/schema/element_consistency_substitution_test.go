package schema

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestElementConsistencyChargesExcludedMembersAndRetriesAfterFailure(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, head, member string
	}{
		{name: "concrete"},
		{name: "abstract", member: ` abstract="true"`},
		{name: "blocked", head: ` block="substitution"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var document strings.Builder
			fmt.Fprintf(&document, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="head" type="xs:string"%s/>`, test.head)
			for i := range 64 {
				fmt.Fprintf(&document, `<xs:element name="m%d" type="xs:string" substitutionGroup="head"%s/>`, i, test.member)
			}
			document.WriteString(`<xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="head"/><xs:element name="head" type="xs:string"/></xs:sequence></xs:complexType></xs:element></xs:schema>`)
			c, model := newEmptyComplexTypePublicationCompiler(t, document.String())
			changeConsistencyLocalType(t, &c.rt, "head", "int")

			exhausted := errors.New("content work exhausted")
			remaining := 8
			checker := newElementDeclarationConsistencyChecker(&c.rt, func(steps int) error {
				remaining -= steps
				if remaining < 0 {
					return exhausted
				}
				return nil
			})
			if err := checker.checkModel(model); !errors.Is(err, exhausted) {
				t.Fatalf("limited consistency check = %v, want original work failure", err)
			}
			// A fresh test allowance isolates cache cleanup after failure. The
			// production content-work budget remains sticky and is never reset.
			remaining = 10_000
			expectDiagnostic(t, checker.checkModel(model), xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel)
		})
	}
}

func TestPublishSchemaChecksImplicitSubstitutionConsistencyAndAllowsRetry(t *testing.T) {
	t.Parallel()
	const document = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:string"/>
  <xs:element name="member" type="xs:string" substitutionGroup="head"/>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element ref="head"/><xs:element name="member" type="xs:string"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`
	c, _ := newEmptyComplexTypePublicationCompiler(t, document)
	if err := c.compileContentModels(); err != nil {
		t.Fatal(err)
	}
	local, original := changeConsistencyLocalType(t, &c.rt, "member", "int")
	published, err := c.publishSchema()
	expectDiagnostic(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaContentModel)
	if published != nil || len(c.rt.Models) == 0 {
		t.Fatal("failed EDC publication returned a schema or consumed the build")
	}
	c.rt.Elements[local].Type = original
	if published, err := c.publishSchema(); err != nil || published == nil {
		t.Fatalf("publication retry = %v, %v", published, err)
	}
}

func changeConsistencyLocalType(t *testing.T, build *schemaBuild, local, builtin string) (ElementID, TypeID) {
	t.Helper()
	name, ok := build.Names.LookupQName("", local)
	if !ok {
		t.Fatalf("missing element name %q", local)
	}
	typeName, ok := build.Names.LookupQName("http://www.w3.org/2001/XMLSchema", builtin)
	if !ok {
		t.Fatalf("missing builtin type name %q", builtin)
	}
	typ, ok := build.GlobalTypes[typeName]
	if !ok {
		t.Fatalf("missing builtin type %q", builtin)
	}
	for id, declaration := range build.Elements {
		if declaration.Name == name && declaration.Scope == DeclarationScopeNonGlobal {
			build.Elements[id].Type = typ
			return ElementID(id), declaration.Type
		}
	}
	t.Fatalf("missing local element %q", local)
	return NoElement, TypeID{}
}
