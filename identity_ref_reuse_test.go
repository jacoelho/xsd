package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestIdentityReferencesSurviveNestedScopeReuse(t *testing.T) {
	t.Parallel()
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
<xs:element name="root"><xs:complexType><xs:choice minOccurs="0" maxOccurs="unbounded">
<xs:element ref="root"/><xs:element name="row"><xs:complexType><xs:attribute name="id" type="xs:string"/><xs:attribute name="ref" type="xs:string"/></xs:complexType></xs:element>
</xs:choice></xs:complexType>
<xs:key name="k"><xs:selector xpath="row"/><xs:field xpath="@id"/></xs:key>
<xs:keyref name="r" refer="k"><xs:selector xpath="row"/><xs:field xpath="@ref"/></xs:keyref>
</xs:element></xs:schema>`
	engine := compileIdentityDispatchTestEngine(t, schema)
	session, err := engine.NewSession(xsd.ValidateOptions{MaxErrors: 1})
	if err != nil {
		t.Fatal(err)
	}
	const children = `<root><row id="child" ref="child"/></root><root><row id="sibling" ref="sibling"/></root>`
	const valid = `<root><row id="parent" ref="parent"/>` + children + `</root>`
	for range 2 {
		if validateErr := session.Validate(strings.NewReader(valid)); validateErr != nil {
			t.Fatal(validateErr)
		}
	}
	err = session.Validate(strings.NewReader(`<root><row id="parent" ref="missing"/>` + children + `</root>`))
	diagnostics := xsderrors.Flatten(err)
	if len(diagnostics) != 1 {
		t.Fatalf("parent unresolved reference: %v", err)
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](diagnostics[0])
	if !ok || diagnostic.Code() != xsderrors.CodeValidationIdentity || diagnostic.Path() != "/root/row" {
		t.Fatalf("parent unresolved reference lost its location: %v", err)
	}
	snapshot := err.Error()
	if validateErr := session.Validate(strings.NewReader(`<root><root><row id="child" ref="missing"/></root></root>`)); validateErr == nil {
		t.Fatal("unresolved child reference accepted")
	}
	if validateErr := session.Validate(strings.NewReader(valid)); validateErr != nil {
		t.Fatalf("reuse after nested reference failure: %v", validateErr)
	}
	if err.Error() != snapshot || diagnostic.Path() != "/root/row" {
		t.Fatal("later session reuse changed a returned diagnostic")
	}
}
