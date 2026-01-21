package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestIdentityNormalizationCycle(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="Cycle">
    <xs:union>
      <xs:simpleType>
        <xs:restriction base="tns:Cycle"/>
      </xs:simpleType>
    </xs:union>
  </xs:simpleType>
  <xs:element name="root" type="tns:Cycle">
    <xs:key name="rootKey">
      <xs:selector xpath="."/>
      <xs:field xpath="."/>
    </xs:key>
  </xs:element>
</xs:schema>`

	docXML := `<root xmlns="urn:test">value</root>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	violations := validateStream(t, v, docXML)
	if len(violations) == 0 {
		t.Fatalf("expected identity constraint violation")
	}

	found := false
	for _, v := range violations {
		if v.Code == string(errors.ErrIdentityAbsent) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected %s violation, got: %v", errors.ErrIdentityAbsent, violations)
	}
}

func TestIdentityNormalizationListCycle(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:simpleType name="CycleList">
    <xs:list itemType="tns:CycleUnion"/>
  </xs:simpleType>
  <xs:simpleType name="CycleUnion">
    <xs:union>
      <xs:simpleType>
        <xs:list itemType="tns:CycleUnion"/>
      </xs:simpleType>
    </xs:union>
  </xs:simpleType>
</xs:schema>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	run := v.newStreamRun()
	listType := schema.TypeDefs[types.QName{Namespace: "urn:test", Local: "CycleList"}]
	if listType == nil {
		t.Fatalf("expected CycleList type")
	}
	_, state := run.normalizeValueByTypeStream("a", listType, 0)
	if state != KeyInvalidValue {
		t.Fatalf("expected %v, got %v", KeyInvalidValue, state)
	}
}
