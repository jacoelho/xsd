package xsd_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func occurrence286Error(tb testing.TB, err error, code xsderrors.Code, message string) {
	tb.Helper()
	d, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || d.Category() != xsderrors.CategorySchemaCompile || d.Code() != code || d.Message() != message {
		tb.Fatalf("unexpected diagnostic: %v", err)
	}
}

func occurrence286Schema(maximum string) string {
	var s strings.Builder
	s.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:simpleType name="Text"><xs:restriction base="xs:string"><xs:minLength value="+0001"/><xs:maxLength value="+0008"/></xs:restriction></xs:simpleType><xs:simpleType name="Number"><xs:restriction base="xs:decimal"><xs:totalDigits value="6"/><xs:fractionDigits value="2"/></xs:restriction></xs:simpleType><xs:element name="root"><xs:complexType><xs:sequence>`)
	for i := range 32 {
		fmt.Fprintf(&s, `<xs:element name="e%d" type="Text" minOccurs="+0001" maxOccurs="%s"/>`, i, maximum)
	}
	s.WriteString(`</xs:sequence></xs:complexType></xs:element><xs:element name="number" type="Number"/></xs:schema>`)
	return s.String()
}

func occurrence286Doc(first int) string {
	var s strings.Builder
	s.WriteString("<root>")
	for i := range 32 {
		n := 1
		if i == 0 {
			n = first
		}
		for range n {
			fmt.Fprintf(&s, "<e%d>x</e%d>", i, i)
		}
	}
	s.WriteString("</root>")
	return s.String()
}

func occurrence286Published(tb testing.TB, engine *xsd.Engine, fifthCode xsderrors.Code) {
	tb.Helper()
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		tb.Fatal(err)
	}
	good := occurrence286Doc(1)
	for _, doc := range []string{good, occurrence286Doc(4), strings.Replace(good, "<e0>x</e0>", "<e0>12345678</e0>", 1), `<number>1234.56</number>`, `<number>123456</number>`} {
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			tb.Fatal(err)
		}
	}
	bad := []struct {
		document string
		code     xsderrors.Code
		message  string
	}{
		{occurrence286Doc(0), xsderrors.CodeValidationContent, "missing required child element"},
		{`<number>1234.567</number>`, xsderrors.CodeValidationFacet, "invalid simple content: digit facet failed"},
		{`<number>12.345</number>`, xsderrors.CodeValidationFacet, "invalid simple content: digit facet failed"},
		{`<number>12345.67</number>`, xsderrors.CodeValidationFacet, "invalid simple content: digit facet failed"},
		{strings.Replace(good, "<e0>x</e0>", "<e0/>", 1), xsderrors.CodeValidationFacet, "invalid simple content: length facet failed"},
		{strings.Replace(good, "<e0>x</e0>", "<e0>123456789</e0>", 1), xsderrors.CodeValidationFacet, "invalid simple content: length facet failed"},
	}
	if fifthCode != "" {
		bad = append(bad, struct {
			document string
			code     xsderrors.Code
			message  string
		}{occurrence286Doc(5), fifthCode, "unexpected child element e0"})
	} else {
		if err := session.Validate(strings.NewReader(occurrence286Doc(5))); err != nil {
			tb.Fatal(err)
		}
	}
	for _, c := range bad {
		err := session.Validate(strings.NewReader(c.document))
		if !xsderrors.IsConclusiveValidation(err) {
			tb.Fatalf("expected validation rejection: %v", err)
		}
		found := false
		for _, failure := range xsderrors.Flatten(err) {
			d, ok := errors.AsType[*xsderrors.Error](failure)
			if ok && d.Category() == xsderrors.CategoryValidation && d.Code() == c.code && d.Message() == c.message {
				found = true
			}
		}
		if !found {
			tb.Fatalf("missing expected %s/%q diagnostic: %v", c.code, c.message, err)
		}
		if err := session.Validate(strings.NewReader(good)); err != nil {
			tb.Fatalf("recovery: %v", err)
		}
	}
}

func BenchmarkCompileOccurrences(b *testing.B) {
	for _, c := range []struct {
		name, max string
		fifthCode xsderrors.Code
	}{{"Finite32", "+0004", xsderrors.CodeValidationElement}, {"Unbounded32", "unbounded", ""}} {
		b.Run(c.name, func(b *testing.B) {
			data := []byte(occurrence286Schema(c.max))
			if len(data) > 16384 {
				b.Fatal("fixture source bound")
			}
			source := xsd.Bytes("occurrence286.xsd", data)
			pre, err := xsd.Compile(source)
			if err != nil || pre == nil {
				b.Fatalf("preflight: %v", err)
			}
			occurrence286Published(b, pre, c.fifthCode)
			var last *xsd.Engine
			b.ReportAllocs()
			for b.Loop() {
				var err error
				last, err = xsd.Compile(source)
				if err != nil || last == nil {
					b.Fatalf("compile: %v", err)
				}
			}
			occurrence286Published(b, last, c.fifthCode)
		})
	}
}

func TestCompileOccurrencesSemantics(t *testing.T) {
	for _, c := range []struct {
		name, max string
		fifthCode xsderrors.Code
	}{{"finite", "+0004", xsderrors.CodeValidationElement}, {"unbounded", "unbounded", ""}} {
		t.Run(c.name, func(t *testing.T) {
			engine, err := xsd.Compile(xsd.Bytes("occurrence.xsd", []byte(occurrence286Schema(c.max))))
			if err != nil || engine == nil {
				t.Fatalf("compile: %v", err)
			}
			occurrence286Published(t, engine, c.fifthCode)
		})
	}
}

func TestCompileCardinalitySignPolicy(t *testing.T) {
	occurrence := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence><xs:element name="e" minOccurs="-0"/></xs:sequence></xs:complexType></xs:element></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("occurrence-sign.xsd", []byte(occurrence)))
	if engine != nil {
		t.Fatal("rejected occurrence published an engine")
	}
	occurrence286Error(t, err, xsderrors.CodeSchemaOccurrence, "invalid minOccurs -0")
	facet := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:simpleType><xs:restriction base="xs:string"><xs:length value="-0"/></xs:restriction></xs:simpleType></xs:element></xs:schema>`
	engine, err = xsd.Compile(xsd.Bytes("facet-sign.xsd", []byte(facet)))
	if err != nil || engine == nil {
		t.Fatalf("signed zero length: %v", err)
	}
	if err := engine.Validate(strings.NewReader("<root/>")); err != nil {
		t.Fatalf("zero length: %v", err)
	}
	if err := engine.Validate(strings.NewReader("<root>x</root>")); !xsderrors.IsConclusiveValidation(err) {
		t.Fatalf("positive length accepted: %v", err)
	}
}
