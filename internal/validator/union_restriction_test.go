package validator

import (
	"bytes"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/loader"
)

func TestUnionRestrictionLoadCompiledValidation(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="BaseUnion">
    <xs:union memberTypes="xs:int xs:boolean"/>
  </xs:simpleType>
  <xs:simpleType name="RestrictedUnion">
    <xs:restriction base="BaseUnion">
      <xs:pattern value="a+"/>
    </xs:restriction>
  </xs:simpleType>
  <xs:element name="root" type="RestrictedUnion"/>
</xs:schema>`

	testFS := fstest.MapFS{
		"test.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}

	l := loader.NewLoader(loader.Config{FS: testFS})
	compiled, err := l.LoadCompiled("test.xsd")
	if err != nil {
		t.Fatalf("LoadCompiled error: %v", err)
	}

	v := New(compiled)
	instance := `<?xml version="1.0"?><root>aaa</root>`
	violations, err := v.ValidateStream(bytes.NewReader([]byte(instance)))
	if err != nil {
		t.Fatalf("ValidateStream error: %v", err)
	}
	if len(violations) == 0 {
		t.Fatal("expected union member validation to report violations")
	}
	found := false
	for _, violation := range violations {
		if violation.Code == string(errors.ErrDatatypeInvalid) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected datatype violation, got %+v", violations)
	}
}
