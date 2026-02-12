package harness_test

import (
	"io"
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/internal/set"
	harness "github.com/jacoelho/xsd/internal/testing"
	validatorapi "github.com/jacoelho/xsd/internal/validationengine"
)

type publicAPIEngine struct{}

type schemaValidator struct {
	schema *xsd.Schema
}

func (v schemaValidator) Validate(r io.Reader) error {
	return v.schema.Validate(r)
}

func (publicAPIEngine) Load(fsys fs.FS, location string) (harness.Validator, error) {
	schema, err := xsd.LoadWithOptions(fsys, location, xsd.NewLoadOptions())
	if err != nil {
		return nil, err
	}
	return schemaValidator{schema: schema}, nil
}

type compiledRuntimeEngine struct{}

func (compiledRuntimeEngine) Load(fsys fs.FS, location string) (harness.Validator, error) {
	prepared, err := set.Prepare(set.PrepareConfig{
		FS:       fsys,
		Location: location,
	})
	if err != nil {
		return nil, err
	}
	rt, err := prepared.BuildRuntime(set.CompileConfig{})
	if err != nil {
		return nil, err
	}
	return validatorapi.NewEngine(rt), nil
}

func TestParitySimpleValidDocument(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	caseData := harness.Case{
		Name: "simple-valid",
		SchemaFS: fstest.MapFS{
			"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
		},
		SchemaPath: "schema.xsd",
		Document:   []byte(`<root xmlns="urn:test">ok</root>`),
	}

	diff := harness.Compare(publicAPIEngine{}, compiledRuntimeEngine{}, caseData)
	if !diff.Equal() {
		t.Fatalf("public API vs runtime mismatch: public(load=%v validate=%v) runtime(load=%v validate=%v)",
			diff.Left.LoadErr, diff.Left.ValidateErr, diff.Right.LoadErr, diff.Right.ValidateErr)
	}
}

func TestParitySimpleInvalidDocument(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:string"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

	caseData := harness.Case{
		Name: "simple-invalid",
		SchemaFS: fstest.MapFS{
			"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
		},
		SchemaPath: "schema.xsd",
		Document:   []byte(strings.TrimSpace(`<root xmlns="urn:test"><b>bad</b></root>`)),
	}

	diff := harness.Compare(publicAPIEngine{}, compiledRuntimeEngine{}, caseData)
	if !diff.Equal() {
		t.Fatalf("public API vs runtime mismatch: public(load=%v validate=%v) runtime(load=%v validate=%v)",
			diff.Left.LoadErr, diff.Left.ValidateErr, diff.Right.LoadErr, diff.Right.ValidateErr)
	}
}
