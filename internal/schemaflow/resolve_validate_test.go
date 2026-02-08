package schemaflow

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestResolveAndValidateRejectsNilSchema(t *testing.T) {
	if _, err := ResolveAndValidate(nil); err == nil {
		t.Fatal("ResolveAndValidate(nil) expected error")
	}
}

func TestResolveAndValidateResolvesTypeReferences(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="Base">
    <xs:sequence>
      <xs:element name="value" type="xs:string"/>
    </xs:sequence>
  </xs:complexType>
  <xs:complexType name="Derived">
    <xs:complexContent>
      <xs:extension base="tns:Base"/>
    </xs:complexContent>
  </xs:complexType>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	resolved, err := ResolveAndValidate(sch)
	if err != nil {
		t.Fatalf("ResolveAndValidate() error = %v", err)
	}
	derivedQName := types.QName{Namespace: "urn:test", Local: "Derived"}

	derived, ok := sch.TypeDefs[derivedQName].(*types.ComplexType)
	if !ok || derived == nil {
		t.Fatalf("expected complex type %s", derivedQName)
	}
	if derived.ResolvedBase != nil {
		t.Fatal("expected parse schema to remain unresolved")
	}

	resolvedDerived, ok := resolved.TypeDefs[derivedQName].(*types.ComplexType)
	if !ok || resolvedDerived == nil {
		t.Fatalf("expected resolved complex type %s", derivedQName)
	}
	if resolvedDerived.ResolvedBase == nil {
		t.Fatal("expected ResolvedBase after ResolveAndValidate")
	}
}

func TestResolveAndValidateReturnsIndependentClone(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	resolved, err := ResolveAndValidate(sch)
	if err != nil {
		t.Fatalf("ResolveAndValidate() error = %v", err)
	}

	delete(sch.ElementDecls, types.QName{Local: "root"})
	if _, ok := resolved.ElementDecls[types.QName{Local: "root"}]; !ok {
		t.Fatal("resolved clone should remain independent from caller mutations")
	}
}

func TestResolveAndValidateReturnsResolveErrors(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="MissingType"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	_, err = ResolveAndValidate(sch)
	if err == nil {
		t.Fatal("ResolveAndValidate() expected error for unresolved type")
	}
	if !strings.Contains(err.Error(), "resolve type references:") {
		t.Fatalf("error = %v, want resolve type references prefix", err)
	}
}
