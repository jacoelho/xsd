package validationengine

import (
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/set"
)

func TestEngineValidateNilSchema(t *testing.T) {
	engine := NewEngine(nil)
	err := engine.Validate(strings.NewReader(`<root/>`))
	validations, ok := xsderrors.AsValidations(err)
	if !ok || len(validations) == 0 {
		t.Fatalf("Validate() errors = %v, want schema-not-loaded validation", err)
	}
	if got, want := validations[0].Code, string(xsderrors.ErrSchemaNotLoaded); got != want {
		t.Fatalf("Validate() code = %q, want %q", got, want)
	}
}

func TestSchemaValidatorValidateWithDocument(t *testing.T) {
	rt := buildRuntimeForTest(t)
	validator := NewSchemaValidator(rt)
	if err := validator.ValidateWithDocument(strings.NewReader(`<root xmlns="urn:test">ok</root>`), "doc.xml"); err != nil {
		t.Fatalf("ValidateWithDocument() error = %v", err)
	}
}

func TestEngineSchemaValidatorParity(t *testing.T) {
	rt := buildRuntimeForTest(t)
	engine := NewEngine(rt)
	validator := NewSchemaValidator(rt)
	doc := `<root xmlns="urn:test"><child>bad</child></root>`

	engineCodes := sortedValidationCodes(engine.Validate(strings.NewReader(doc)))
	validatorCodes := sortedValidationCodes(validator.Validate(strings.NewReader(doc)))
	if !slices.Equal(engineCodes, validatorCodes) {
		t.Fatalf("validation code mismatch: engine=%v validator=%v", engineCodes, validatorCodes)
	}
}

func buildRuntimeForTest(t *testing.T) *runtime.Schema {
	t.Helper()
	prepared, err := set.Prepare(set.PrepareConfig{
		FS: fstest.MapFS{
			"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
	targetNamespace="urn:test"
	xmlns:tns="urn:test"
	elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
		},
		Location: "schema.xsd",
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	rt, err := prepared.BuildRuntime(set.CompileConfig{})
	if err != nil {
		t.Fatalf("BuildRuntime() error = %v", err)
	}
	return rt
}

func sortedValidationCodes(err error) []string {
	if err == nil {
		return nil
	}
	violations, ok := xsderrors.AsValidations(err)
	if !ok {
		return []string{"ERR:" + err.Error()}
	}
	codes := make([]string, 0, len(violations))
	for _, violation := range violations {
		codes = append(codes, violation.Code)
	}
	slices.Sort(codes)
	return codes
}
