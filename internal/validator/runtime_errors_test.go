package validator

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	xsdErrors "github.com/jacoelho/xsd/errors"
)

func TestValidationDocumentURISet(t *testing.T) {
	schema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:int"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	doc := `<root><a>bad</a></root>`

	rt := mustBuildRuntimeSchema(t, schema)
	sess := NewSession(rt)
	err := sess.ValidateWithDocument(strings.NewReader(doc), "doc.xml")
	if err == nil {
		t.Fatalf("expected validation error")
	}

	var violations xsdErrors.ValidationList
	if !errors.As(err, &violations) {
		t.Fatalf("expected ValidationList error, got %T", err)
	}
	for i, v := range violations {
		if v.Document != "doc.xml" {
			t.Fatalf("violation %d document = %q, want %q", i, v.Document, "doc.xml")
		}
	}
}

func TestSortValidationListOrdering(t *testing.T) {
	list := xsdErrors.ValidationList{
		{Document: "b", Line: 1, Column: 1, Code: "a", Message: "zeta"},
		{Document: "a", Line: 1, Column: 1, Code: "a", Message: "beta"},
		{Document: "a", Line: 2, Column: 1, Code: "a", Message: "epsilon"},
		{Document: "a", Line: 1, Column: 1, Code: "a", Message: "alpha"},
		{Document: "a", Line: 1, Column: 1, Code: "b", Message: "gamma"},
		{Document: "a", Line: 1, Column: 2, Code: "a", Message: "delta"},
		{Document: "a", Line: -1, Column: -1, Code: "a", Message: "first"},
		{Document: "", Line: 1, Column: 1, Code: "a", Message: "omega"},
	}

	list.Sort()

	var got []string
	for _, v := range list {
		got = append(got, v.Message)
	}
	want := []string{"first", "alpha", "beta", "gamma", "delta", "epsilon", "zeta", "omega"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sorted order = %v, want %v", got, want)
	}
}
