package schemaflow

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
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
	derivedQName := model.QName{Namespace: "urn:test", Local: "Derived"}

	derived, ok := sch.TypeDefs[derivedQName].(*model.ComplexType)
	if !ok || derived == nil {
		t.Fatalf("expected complex type %s", derivedQName)
	}
	if derived.ResolvedBase != nil {
		t.Fatal("expected parse schema to remain unresolved")
	}

	resolvedDerived, ok := resolved.TypeDefs[derivedQName].(*model.ComplexType)
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

	delete(sch.ElementDecls, model.QName{Local: "root"})
	if _, ok := resolved.ElementDecls[model.QName{Local: "root"}]; !ok {
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

func TestResolveAndValidateOwnedMatchesClonePath(t *testing.T) {
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
  <xs:element name="root" type="tns:Base"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	resolvedClonePath, err := ResolveAndValidate(sch)
	if err != nil {
		t.Fatalf("ResolveAndValidate() error = %v", err)
	}
	owned, err := loadmerge.CloneSchemaDeep(sch)
	if err != nil {
		t.Fatalf("CloneSchemaDeep() error = %v", err)
	}
	if err := ResolveAndValidateOwned(owned); err != nil {
		t.Fatalf("ResolveAndValidateOwned() error = %v", err)
	}
	if !reflect.DeepEqual(resolvedClonePath, owned) {
		t.Fatal("ResolveAndValidateOwned() diverged from ResolveAndValidate() clone behavior")
	}
}

func TestResolveAndValidateOwnedAllocatesLessThanClonePath(t *testing.T) {
	base, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:alloc"
           xmlns:tns="urn:alloc"
           elementFormDefault="qualified">
  <xs:group name="G">
    <xs:sequence>
      <xs:element name="e1" type="xs:string"/>
      <xs:element name="e2" type="xs:int"/>
      <xs:element name="e3" type="xs:boolean"/>
    </xs:sequence>
  </xs:group>
  <xs:complexType name="T">
    <xs:sequence>
      <xs:group ref="tns:G"/>
      <xs:element name="e4" type="xs:string" minOccurs="0" maxOccurs="3"/>
    </xs:sequence>
    <xs:attribute name="a1" type="xs:string"/>
  </xs:complexType>
  <xs:element name="root" type="tns:T"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	const runs = 40
	cloneAllocs := testing.AllocsPerRun(runs, func() {
		input, cloneErr := loadmerge.CloneSchemaDeep(base)
		if cloneErr != nil {
			panic(cloneErr)
		}
		if _, resolveErr := ResolveAndValidate(input); resolveErr != nil {
			panic(resolveErr)
		}
	})
	ownedAllocs := testing.AllocsPerRun(runs, func() {
		input, cloneErr := loadmerge.CloneSchemaDeep(base)
		if cloneErr != nil {
			panic(cloneErr)
		}
		if resolveErr := ResolveAndValidateOwned(input); resolveErr != nil {
			panic(resolveErr)
		}
	})

	if ownedAllocs >= cloneAllocs {
		t.Fatalf("expected owned path to allocate less than clone path: owned=%.2f clone=%.2f", ownedAllocs, cloneAllocs)
	}
}
