package compiler

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/schemaast"
)

func TestPrepareDeterministicAcrossParses(t *testing.T) {
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

	parsedCloned, err := parseDocumentSet(schemaXML)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	parsedSecond, err := parseDocumentSet(schemaXML)
	if err != nil {
		t.Fatalf("parse second schema: %v", err)
	}

	preparedFirst, err := Prepare(parsedCloned)
	if err != nil {
		t.Fatalf("prepare first: %v", err)
	}
	preparedSecond, err := Prepare(parsedSecond)
	if err != nil {
		t.Fatalf("prepare second: %v", err)
	}

	cfg := BuildConfig{MaxDFAStates: 2048, MaxOccursLimit: 2048}
	runtimeFirst, err := preparedFirst.Build(cfg)
	if err != nil {
		t.Fatalf("build runtime first: %v", err)
	}
	runtimeSecond, err := preparedSecond.Build(cfg)
	if err != nil {
		t.Fatalf("build runtime second: %v", err)
	}

	if runtimeSecond.BuildHash != runtimeFirst.BuildHash {
		t.Fatalf("build hash mismatch: second=%x first=%x", runtimeSecond.BuildHash, runtimeFirst.BuildHash)
	}
	if got, want := runtimeSecond.CanonicalDigest(), runtimeFirst.CanonicalDigest(); got != want {
		t.Fatalf("canonical digest mismatch: second=%x first=%x", got, want)
	}
}

func TestPrepareRejectsNilSchema(t *testing.T) {
	if _, err := Prepare(nil); err == nil {
		t.Fatal("Prepare(nil) expected error")
	}
}

func TestPrepareResolvesTypeReferences(t *testing.T) {
	docs, err := parseDocumentSet(`<?xml version="1.0"?>
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
</xs:schema>`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	prepared, err := Prepare(docs)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	derivedQName := schemaast.QName{Namespace: "urn:test", Local: "Derived"}

	var derived *schemaast.ComplexTypeDecl
	for i := range docs.Documents[0].Decls {
		decl := &docs.Documents[0].Decls[i]
		if decl.Name == derivedQName {
			derived = decl.ComplexType
			break
		}
	}
	if derived == nil {
		t.Fatalf("expected complex type %s", derivedQName)
	}
	if derived.Base != derivedQName && derived.Base.Local != "Base" {
		t.Fatalf("expected lexical base to remain in input, got %s", derived.Base)
	}

	var found bool
	for _, typ := range prepared.ir.Types {
		if typ.Name.Namespace != derivedQName.Namespace || typ.Name.Local != derivedQName.Local {
			continue
		}
		found = true
		if typ.Base.ID == 0 && !typ.Base.Builtin {
			t.Fatal("expected resolved base in IR")
		}
	}
	if !found {
		t.Fatalf("expected IR type %s", derivedQName)
	}
}

func TestPrepareReturnsIndependentClone(t *testing.T) {
	docs, err := parseDocumentSet(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	prepared, err := Prepare(docs)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	docs.Documents[0].Decls = nil
	rt, err := prepared.Build(BuildConfig{})
	if err != nil {
		t.Fatalf("Build() after caller mutation error = %v", err)
	}
	if len(rt.GlobalElements) == 0 {
		t.Fatal("prepared clone should remain independent from caller mutations")
	}
}

func TestPrepareReturnsResolveErrors(t *testing.T) {
	docs, err := parseDocumentSet(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="MissingType"/>
</xs:schema>`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	_, err = Prepare(docs)
	if err == nil {
		t.Fatal("Prepare() expected error for unresolved type")
	}
	if !strings.Contains(err.Error(), "prepare schema: schema ir: type") {
		t.Fatalf("error = %v, want resolve type references prefix", err)
	}
}

func TestPrepareDoesNotMutateParsedSchema(t *testing.T) {
	base, err := parseDocumentSet(`<?xml version="1.0"?>
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
</xs:schema>`)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	root := base.Documents[0].Decls[len(base.Documents[0].Decls)-1].Element
	before := root.Type
	if _, err := Prepare(base); err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if root.Type != before {
		t.Fatalf("Prepare() mutated parsed root type")
	}
}
