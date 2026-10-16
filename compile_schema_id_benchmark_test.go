package xsd_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

type schemaIDFixture455 struct {
	name, schema, valid string
	invalid             []string
	codes               [][]xsderrors.Code
}

func schemaIDFlat455(name string) schemaIDFixture455 {
	var b strings.Builder
	b.WriteString("<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n")
	for i := range 512 {
		fmt.Fprintf(&b, "<xs:group name=\"g%03d\"", i)
		if name == "Dense512" || name == "LateDuplicate512" || name == "Sparse512" && i%64 == 0 {
			id := i
			if name == "LateDuplicate512" && i == 511 {
				id = 0
			}
			fmt.Fprintf(&b, " id=\"id%03d\"", id)
		}
		fmt.Fprintf(&b, "><xs:sequence><xs:element name=\"v%03d\" type=\"xs:integer\"/></xs:sequence></xs:group>\n", i)
	}
	b.WriteString("<xs:element name=\"root\" type=\"xs:integer\"/>\n</xs:schema>\n")
	return schemaIDFixture455{
		name: name, schema: b.String(), valid: `<root>7</root>`,
		invalid: []string{`<other>7</other>`, `<root/>`, `<root>bad</root>`},
		codes:   [][]xsderrors.Code{{xsderrors.CodeValidationRoot}, {xsderrors.CodeValidationFacet}, {xsderrors.CodeValidationFacet}},
	}
}

func schemaIDDeep455() schemaIDFixture455 {
	var schema, valid, bad, missing strings.Builder
	schema.WriteString("<xs:schema xmlns:xs=\"http://www.w3.org/2001/XMLSchema\">\n")
	for i := range 64 {
		fmt.Fprintf(&valid, "<e%02d>", i)
		fmt.Fprintf(&bad, "<e%02d>", i)
		if i < 63 {
			fmt.Fprintf(&missing, "<e%02d>", i)
			fmt.Fprintf(&schema, "<xs:element name=\"e%02d\" id=\"deep%02d\"><xs:complexType><xs:sequence>\n", i, i)
		} else {
			fmt.Fprintf(&schema, "<xs:element name=\"e%02d\" id=\"deep%02d\" type=\"xs:integer\"/>\n", i, i)
		}
	}
	valid.WriteString("7")
	bad.WriteString("bad")
	for i := 63; i >= 0; i-- {
		fmt.Fprintf(&valid, "</e%02d>", i)
		fmt.Fprintf(&bad, "</e%02d>", i)
		if i < 63 {
			fmt.Fprintf(&missing, "</e%02d>", i)
			schema.WriteString("</xs:sequence></xs:complexType></xs:element>\n")
		}
	}
	schema.WriteString("</xs:schema>\n")
	return schemaIDFixture455{
		name: "Deep64", schema: schema.String(), valid: valid.String(),
		invalid: []string{`<other>7</other>`, missing.String(), bad.String()},
		codes:   [][]xsderrors.Code{{xsderrors.CodeValidationRoot}, {xsderrors.CodeValidationContent}, {xsderrors.CodeValidationFacet}},
	}
}

func schemaIDSuccesses455() []schemaIDFixture455 {
	return []schemaIDFixture455{schemaIDFlat455("Zero512"), schemaIDFlat455("Sparse512"), schemaIDFlat455("Dense512"), schemaIDDeep455()}
}

func schemaIDOptions455() xsd.CompileOptions {
	return xsd.CompileOptions{MaxSchemaDepth: 256}
}

func checkSchemaIDEngine455(tb testing.TB, e *xsd.Engine, err error, f schemaIDFixture455) {
	tb.Helper()
	if err != nil || e == nil {
		tb.Fatalf("%s compile: engine=%v error=%v", f.name, e, err)
	}
	s, err := e.NewSession(xsd.ValidateOptions{MaxInstanceDepth: 128})
	if err != nil {
		tb.Fatal(err)
	}
	if err := s.Validate(strings.NewReader(f.valid)); err != nil {
		tb.Fatalf("%s valid document: %v", f.name, err)
	}
	for i, doc := range f.invalid {
		err := s.Validate(strings.NewReader(doc))
		if !xsderrors.IsConclusiveValidation(err) {
			tb.Fatalf("%s invalid[%d] inconclusive: %v", f.name, i, err)
		}
		var codes []xsderrors.Code
		for _, child := range xsderrors.Flatten(err) {
			d, ok := errors.AsType[*xsderrors.Error](child)
			if !ok || d.Category() != xsderrors.CategoryValidation || d.Cause() != nil {
				tb.Fatalf("%s invalid[%d] diagnostic=%#v", f.name, i, child)
			}
			codes = append(codes, d.Code())
		}
		if !slices.Equal(codes, f.codes[i]) {
			tb.Fatalf("%s invalid[%d] codes=%v want=%v", f.name, i, codes, f.codes[i])
		}
		if err := s.Validate(strings.NewReader(f.valid)); err != nil {
			tb.Fatalf("%s recovery after invalid[%d]: %v", f.name, i, err)
		}
	}
}

func checkSchemaIDDuplicate455(tb testing.TB, e *xsd.Engine, err error) {
	tb.Helper()
	d, ok := errors.AsType[*xsderrors.Error](err)
	if e != nil || !ok || d.Category() != xsderrors.CategorySchemaCompile || d.Code() != xsderrors.CodeSchemaInvalidAttribute || d.Message() != "duplicate schema id id000" || d.Path() != "LateDuplicate512455.xsd" || d.Line() != 513 || d.Column() != 1 || d.Cause() != nil {
		tb.Fatalf("late duplicate: engine=%v diagnostic=%#v error=%v", e, d, err)
	}
}

func checkSchemaIDFreshRecovery455(tb testing.TB) {
	tb.Helper()
	f := schemaIDFlat455("Dense512")
	src := xsd.Bytes(f.name+"455.xsd", []byte(f.schema))
	e, err := xsd.CompileWithOptions(schemaIDOptions455(), src)
	checkSchemaIDEngine455(tb, e, err, f)
}

func TestCompileSchemaIDs455Fixtures(t *testing.T) {
	for _, f := range schemaIDSuccesses455() {
		t.Run(f.name, func(t *testing.T) {
			src := xsd.Bytes(f.name+"455.xsd", []byte(f.schema))
			e, err := xsd.CompileWithOptions(schemaIDOptions455(), src)
			checkSchemaIDEngine455(t, e, err, f)
		})
	}
}

func TestCompileSchemaIDs455LateDuplicate(t *testing.T) {
	f := schemaIDFlat455("LateDuplicate512")
	src := xsd.Bytes(f.name+"455.xsd", []byte(f.schema))
	e, err := xsd.CompileWithOptions(schemaIDOptions455(), src)
	checkSchemaIDDuplicate455(t, e, err)
	checkSchemaIDFreshRecovery455(t)
}

func BenchmarkCompileSchemaIDs455(b *testing.B) {
	for _, f := range schemaIDSuccesses455() {
		b.Run(f.name, func(b *testing.B) {
			src := xsd.Bytes(f.name+"455.xsd", []byte(f.schema))
			opts := schemaIDOptions455()
			e, err := xsd.CompileWithOptions(opts, src)
			checkSchemaIDEngine455(b, e, err, f)
			b.ReportAllocs()
			for b.Loop() {
				e, err = xsd.CompileWithOptions(opts, src)
				if err != nil || e == nil {
					b.Fatalf("%s timed compile: engine=%v error=%v", f.name, e, err)
				}
			}
			checkSchemaIDEngine455(b, e, err, f)
		})
	}
	b.Run("LateDuplicate512", func(b *testing.B) {
		f := schemaIDFlat455("LateDuplicate512")
		src := xsd.Bytes(f.name+"455.xsd", []byte(f.schema))
		opts := schemaIDOptions455()
		e, err := xsd.CompileWithOptions(opts, src)
		checkSchemaIDDuplicate455(b, e, err)
		checkSchemaIDFreshRecovery455(b)
		b.ReportAllocs()
		for b.Loop() {
			e, err = xsd.CompileWithOptions(opts, src)
			if err == nil || e != nil {
				b.Fatalf("late duplicate timed compile: engine=%v error=%v", e, err)
			}
		}
		checkSchemaIDDuplicate455(b, e, err)
		checkSchemaIDFreshRecovery455(b)
	})
}
