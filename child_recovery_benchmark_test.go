package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func BenchmarkSessionValidateChildSelectionRecovery(b *testing.B) {
	const schemaText = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="row" type="xs:string" minOccurs="0" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schemaText)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{MaxErrors: 200})
	if err != nil {
		b.Fatal(err)
	}
	const valid = `<root><row>valid</row></root>`
	document := `<root>` + strings.Repeat(`<unknown/>`, 100) + `<row>valid</row></root>`
	reader := strings.NewReader(document)
	check := func(validateErr error) {
		b.Helper()
		diagnostics := xsderrors.Flatten(validateErr)
		if len(diagnostics) != 100 {
			b.Fatalf("child-selection diagnostics = %d, want 100: %v", len(diagnostics), validateErr)
		}
		for _, diagnostic := range diagnostics {
			value, ok := errors.AsType[*xsderrors.Error](diagnostic)
			if !ok || value.Code() != xsderrors.CodeValidationElement || value.Path() != "/root" || !strings.Contains(value.Error(), `unexpected child element "unknown"`) {
				b.Fatalf("unexpected child-selection diagnostic: %v", diagnostic)
			}
		}
	}
	check(session.Validate(reader))
	if validateErr := session.Validate(strings.NewReader(valid)); validateErr != nil {
		b.Fatalf("valid reuse after child recovery: %v", validateErr)
	}
	b.SetBytes(int64(len(document)))
	b.ReportAllocs()
	var result error
	for b.Loop() {
		reader.Reset(document)
		result = session.Validate(reader)
		if !xsderrors.IsConclusiveValidation(result) {
			b.Fatalf("child selection did not reject: %v", result)
		}
	}
	check(result)
	if validateErr := session.Validate(strings.NewReader(valid)); validateErr != nil {
		b.Fatalf("valid reuse after measured child recovery: %v", validateErr)
	}
}
