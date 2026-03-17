package compiler_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestPrepareParityWithPrepareOwned(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:complexType name="T">
    <xs:sequence>
      <xs:element name="a" type="xs:string"/>
      <xs:element name="b" type="xs:int"/>
    </xs:sequence>
  </xs:complexType>
  <xs:element name="root" type="tns:T"/>
</xs:schema>`

	parsedCloned, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	parsedOwned, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("parse owned schema: %v", err)
	}

	preparedCloned, err := compiler.Prepare(parsedCloned)
	if err != nil {
		t.Fatalf("prepare cloned: %v", err)
	}
	preparedOwned, err := compiler.PrepareOwned(parsedOwned)
	if err != nil {
		t.Fatalf("prepare owned: %v", err)
	}

	cfg := compiler.BuildConfig{MaxDFAStates: 2048, MaxOccursLimit: 2048}
	runtimeCloned, err := preparedCloned.Build(cfg)
	if err != nil {
		t.Fatalf("build runtime cloned: %v", err)
	}
	runtimeOwned, err := preparedOwned.Build(cfg)
	if err != nil {
		t.Fatalf("build runtime owned: %v", err)
	}

	if runtimeOwned.BuildHash != runtimeCloned.BuildHash {
		t.Fatalf("build hash mismatch: owned=%x cloned=%x", runtimeOwned.BuildHash, runtimeCloned.BuildHash)
	}
	if got, want := runtimeOwned.CanonicalDigest(), runtimeCloned.CanonicalDigest(); got != want {
		t.Fatalf("canonical digest mismatch: owned=%x cloned=%x", got, want)
	}
}

func TestPrepareRejectsNilSchema(t *testing.T) {
	if _, err := compiler.Prepare(nil); err == nil {
		t.Fatal("Prepare(nil) expected error")
	}
}

func TestPrepareResolvesTypeReferences(t *testing.T) {
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

	prepared, err := compiler.Prepare(sch)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	derivedQName := model.QName{Namespace: "urn:test", Local: "Derived"}

	derived, ok := sch.TypeDefs[derivedQName].(*model.ComplexType)
	if !ok || derived == nil {
		t.Fatalf("expected complex type %s", derivedQName)
	}
	if derived.ResolvedBase != nil {
		t.Fatal("expected input schema to remain unresolved")
	}

	resolvedDerived, ok := prepared.Schema().TypeDefs[derivedQName].(*model.ComplexType)
	if !ok || resolvedDerived == nil {
		t.Fatalf("expected resolved complex type %s", derivedQName)
	}
	if resolvedDerived.ResolvedBase == nil {
		t.Fatal("expected ResolvedBase after Prepare()")
	}
}

func TestPrepareReturnsIndependentClone(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	prepared, err := compiler.Prepare(sch)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	delete(sch.ElementDecls, model.QName{Local: "root"})
	if _, ok := prepared.Schema().ElementDecls[model.QName{Local: "root"}]; !ok {
		t.Fatal("prepared clone should remain independent from caller mutations")
	}
}

func TestPrepareReturnsResolveErrors(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="MissingType"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	_, err = compiler.Prepare(sch)
	if err == nil {
		t.Fatal("Prepare() expected error for unresolved type")
	}
	if !strings.Contains(err.Error(), "prepare schema: resolve type references:") {
		t.Fatalf("error = %v, want resolve type references prefix", err)
	}
}

func TestPrepareOwnedMatchesClonePath(t *testing.T) {
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

	clonedPrepared, err := compiler.Prepare(sch)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	owned := parser.CloneSchema(sch)
	ownedPrepared, err := compiler.PrepareOwned(owned)
	if err != nil {
		t.Fatalf("PrepareOwned() error = %v", err)
	}

	if !reflect.DeepEqual(clonedPrepared.Schema(), ownedPrepared.Schema()) {
		t.Fatal("PrepareOwned() diverged from Prepare() clone behavior")
	}
}

func TestPrepareOwnedAllocatesLessThanClonePath(t *testing.T) {
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
		input := parser.CloneSchema(base)
		if _, prepareErr := compiler.Prepare(input); prepareErr != nil {
			panic(prepareErr)
		}
	})
	ownedAllocs := testing.AllocsPerRun(runs, func() {
		input := parser.CloneSchema(base)
		if _, prepareErr := compiler.PrepareOwned(input); prepareErr != nil {
			panic(prepareErr)
		}
	})

	if ownedAllocs >= cloneAllocs {
		t.Fatalf("expected owned path to allocate less than clone path: owned=%.2f clone=%.2f", ownedAllocs, cloneAllocs)
	}
}
