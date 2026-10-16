package xsd_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const simpleContentProjectionSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:p="urn:p">
<xs:complexType name="Amount"><xs:simpleContent><xs:extension base="xs:decimal"><xs:attribute name="tag" type="xs:string" use="required"/></xs:extension></xs:simpleContent></xs:complexType>
<xs:complexType name="LimitedAmount"><xs:simpleContent><xs:restriction base="Amount"><xs:minInclusive value="0"/><xs:maxInclusive value="10"/><xs:attribute name="tag" type="xs:string" use="required"/></xs:restriction></xs:simpleContent></xs:complexType>
<xs:complexType name="QNameText"><xs:simpleContent><xs:extension base="xs:QName"><xs:attribute name="tag" type="xs:string"/></xs:extension></xs:simpleContent></xs:complexType>
<xs:element name="r"><xs:complexType><xs:sequence maxOccurs="128"><xs:element name="amount" type="Amount"/><xs:element name="limited" type="LimitedAmount"/><xs:element name="q" type="QNameText" fixed="p:Thing"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`

type simpleContentProjectionMode uint8

const (
	simpleContentDeclared simpleContentProjectionMode = iota
	simpleContentXSIType
)

func simpleContentProjectionDocument(mode simpleContentProjectionMode) []byte {
	amount := `<amount tag="x">1.5</amount>`
	if mode == simpleContentXSIType {
		amount = `<amount xsi:type="LimitedAmount" tag="x">1.5</amount>`
	}
	group := amount + `<limited tag="x">2</limited><q>q:Thing</q>`
	return []byte(`<r xmlns:q="urn:p" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` + strings.Repeat(group, 128) + `</r>`)
}

func checkSimpleContentProjection(tb testing.TB, engine *xsd.Engine, mode simpleContentProjectionMode) {
	tb.Helper()
	session, sessionErr := engine.NewSession(xsd.ValidateOptions{})
	if sessionErr != nil {
		tb.Fatal(sessionErr)
	}
	valid := simpleContentProjectionDocument(mode)
	reader := bytes.NewReader(valid)
	if validationErr := session.Validate(reader); validationErr != nil {
		tb.Fatalf("valid document: %v", validationErr)
	}
	for _, test := range []struct {
		name, document string
		reject         bool
		code           xsderrors.Code
	}{
		{"amount11", strings.Replace(string(valid), `>1.5</amount>`, `>11</amount>`, 1), mode == simpleContentXSIType, xsderrors.CodeValidationFacet},
		{"limited11", strings.Replace(string(valid), `>2</limited>`, `>11</limited>`, 1), true, xsderrors.CodeValidationFacet},
		{"missingTag", strings.Replace(string(valid), ` tag="x"`, ``, 1), true, xsderrors.CodeValidationAttribute},
		{"wrongQNameLocal", strings.Replace(string(valid), `q:Thing`, `q:Other`, 1), true, xsderrors.CodeValidationElement},
		{"wrongQNameURI", strings.Replace(string(valid), `xmlns:q="urn:p"`, `xmlns:q="urn:wrong"`, 1), true, xsderrors.CodeValidationElement},
	} {
		reader.Reset([]byte(test.document))
		validationErr := session.Validate(reader)
		if !test.reject {
			if validationErr != nil {
				tb.Fatalf("%s accepted: %v", test.name, validationErr)
			}
		} else {
			if validationErr == nil {
				tb.Fatalf("%s unexpectedly accepted", test.name)
			}
			diagnostics := xsderrors.Flatten(validationErr)
			if len(diagnostics) == 0 {
				tb.Fatalf("%s missing diagnostics", test.name)
			}
			codes := make(map[xsderrors.Code]bool)
			for _, item := range diagnostics {
				diagnostic, ok := errors.AsType[*xsderrors.Error](item)
				if !ok || diagnostic.Category() != xsderrors.CategoryValidation {
					tb.Fatalf("%s non-validation diagnostic: %v", test.name, item)
				}
				if diagnostic.Code() != test.code {
					tb.Fatalf("%s code = %s, want %s", test.name, diagnostic.Code(), test.code)
				}
				codes[diagnostic.Code()] = true
			}
			tb.Logf("characterization xsi=%v case=%s codes=%v", mode == simpleContentXSIType, test.name, codes)
		}
		reader.Reset(valid)
		if recoveryErr := session.Validate(reader); recoveryErr != nil {
			tb.Fatalf("same-session recovery after %s: %v", test.name, recoveryErr)
		}
	}
}

func TestSimpleContentProjectionSemantics(t *testing.T) {
	engine, compileErr := xsd.Compile(xsd.Bytes("simple-content.xsd", []byte(simpleContentProjectionSchema)))
	if compileErr != nil || engine == nil {
		t.Fatalf("Compile: engine=%v err=%v", engine, compileErr)
	}
	for _, test := range []struct {
		name string
		mode simpleContentProjectionMode
	}{{"Declared", simpleContentDeclared}, {"XSIType", simpleContentXSIType}} {
		t.Run(test.name, func(t *testing.T) { checkSimpleContentProjection(t, engine, test.mode) })
	}
}

func BenchmarkSimpleContentProjection(b *testing.B) {
	for _, test := range []struct {
		name string
		mode simpleContentProjectionMode
	}{{"Declared", simpleContentDeclared}, {"XSIType", simpleContentXSIType}} {
		b.Run(test.name, func(b *testing.B) {
			engine, compileErr := xsd.Compile(xsd.Bytes("simple-content.xsd", []byte(simpleContentProjectionSchema)))
			if compileErr != nil || engine == nil {
				b.Fatalf("Compile: engine=%v err=%v", engine, compileErr)
			}
			checkSimpleContentProjection(b, engine, test.mode)
			session, sessionErr := engine.NewSession(xsd.ValidateOptions{})
			if sessionErr != nil {
				b.Fatal(sessionErr)
			}
			document := simpleContentProjectionDocument(test.mode)
			reader := bytes.NewReader(document)
			if warmErr := session.Validate(reader); warmErr != nil {
				b.Fatal(warmErr)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(document)))
			for b.Loop() {
				reader.Reset(document)
				if validationErr := session.Validate(reader); validationErr != nil {
					b.Fatal(validationErr)
				}
			}
			checkSimpleContentProjection(b, engine, test.mode)
		})
	}
}
