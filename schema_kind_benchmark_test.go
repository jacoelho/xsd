package xsd_test

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

type schemaKindDocument struct {
	name string
	text string
}

type schemaKindRejection struct {
	document string
	recovery string
	codes    []xsderrors.Code
}

type schemaKindFixture struct {
	name     string
	roots    []schemaKindDocument
	resolved []schemaKindDocument
	valid    []string
	rejected []schemaKindRejection
}

func schemaKindFixtures() []schemaKindFixture {
	return []schemaKindFixture{schemaKindBroadFixture(), schemaKindDeepFixture(), schemaKindContextFixture()}
}

func schemaKindBroadFixture() schemaKindFixture {
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:k="urn:kind" targetNamespace="urn:kind" elementFormDefault="qualified">`)
	for j := range 64 {
		n := strconv.Itoa(j)
		schema.WriteString(`<xs:group name="G_` + n + `"><xs:sequence><xs:element name="e_` + n + `" type="xs:string"/><xs:choice><xs:element name="x_` + n + `" type="xs:string"/><xs:element name="y_` + n + `" type="xs:string"/></xs:choice><xs:any namespace="##other" processContents="lax" minOccurs="0" maxOccurs="1"/></xs:sequence></xs:group>`)
	}
	schema.WriteString(`<xs:element name="r"><xs:complexType><xs:sequence>`)
	for range 2 {
		for j := range 64 {
			schema.WriteString(`<xs:group ref="k:G_` + strconv.Itoa(j) + `"/>`)
		}
	}
	schema.WriteString(`</xs:sequence></xs:complexType></xs:element><xs:element name="allRoot"><xs:complexType><xs:all><xs:element name="a" type="xs:string"/><xs:element name="b" type="xs:string"/></xs:all></xs:complexType></xs:element></xs:schema>`)

	var good, alternate strings.Builder
	good.WriteString(`<k:r xmlns:k="urn:kind">`)
	alternate.WriteString(`<k:r xmlns:k="urn:kind">`)
	for range 2 {
		for j := range 64 {
			n := strconv.Itoa(j)
			good.WriteString(`<k:e_` + n + `/><k:x_` + n + `/>`)
			alternate.WriteString(`<k:e_` + n + `/>`)
			if j == 0 || j == 63 {
				alternate.WriteString(`<k:y_` + n + `/><f:extra xmlns:f="urn:foreign"/>`)
			} else {
				alternate.WriteString(`<k:x_` + n + `/>`)
			}
		}
	}
	good.WriteString(`</k:r>`)
	alternate.WriteString(`</k:r>`)
	const allAB = `<k:allRoot xmlns:k="urn:kind"><k:a/><k:b/></k:allRoot>`
	const allBA = `<k:allRoot xmlns:k="urn:kind"><k:b/><k:a/></k:allRoot>`
	return schemaKindFixture{
		name:  "BroadMixed64",
		roots: []schemaKindDocument{{name: "kind-broad.xsd", text: schema.String()}},
		valid: []string{good.String(), alternate.String(), allAB, allBA},
		rejected: []schemaKindRejection{
			{document: strings.Replace(good.String(), `<k:e_31/>`, "", 1), recovery: good.String(), codes: []xsderrors.Code{xsderrors.CodeValidationElement}},
			{document: strings.Replace(good.String(), `<k:e_31/><k:x_31/>`, `<k:x_31/><k:e_31/>`, 1), recovery: good.String(), codes: []xsderrors.Code{xsderrors.CodeValidationElement}},
			{document: `<k:allRoot xmlns:k="urn:kind"><k:a/></k:allRoot>`, recovery: allAB, codes: []xsderrors.Code{xsderrors.CodeValidationContent}},
		},
	}
}

func schemaKindDeepFixture() schemaKindFixture {
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:k="urn:kind:deep" targetNamespace="urn:kind:deep" elementFormDefault="qualified"><xs:group name="G_0"><xs:sequence><xs:element name="e_0" type="xs:string"/></xs:sequence></xs:group>`)
	for i := 1; i < 64; i++ {
		schema.WriteString(`<xs:group name="G_` + strconv.Itoa(i) + `"><xs:sequence><xs:element name="e_` + strconv.Itoa(i) + `" type="xs:string"/><xs:group ref="k:G_` + strconv.Itoa(i-1) + `"/></xs:sequence></xs:group>`)
	}
	schema.WriteString(`<xs:element name="r"><xs:complexType><xs:sequence>`)
	for range 8 {
		schema.WriteString(`<xs:group ref="k:G_63"/>`)
	}
	schema.WriteString(`</xs:sequence></xs:complexType></xs:element></xs:schema>`)
	var good, omitted strings.Builder
	good.WriteString(`<k:r xmlns:k="urn:kind:deep">`)
	omitted.WriteString(`<k:r xmlns:k="urn:kind:deep">`)
	for run := range 8 {
		for i := 63; i >= 0; i-- {
			element := `<k:e_` + strconv.Itoa(i) + `/>`
			good.WriteString(element)
			if run != 3 || i != 31 {
				omitted.WriteString(element)
			}
		}
	}
	good.WriteString(`</k:r>`)
	omitted.WriteString(`</k:r>`)
	return schemaKindFixture{
		name:  "DeepGroups64",
		roots: []schemaKindDocument{{name: "kind-deep.xsd", text: schema.String()}},
		valid: []string{good.String()},
		rejected: []schemaKindRejection{
			{document: omitted.String(), recovery: good.String(), codes: []xsderrors.Code{xsderrors.CodeValidationElement, xsderrors.CodeValidationContent}},
			{document: `<k:unknown xmlns:k="urn:kind:deep"/>`, recovery: good.String(), codes: []xsderrors.Code{xsderrors.CodeValidationRoot}},
		},
	}
}

func schemaKindContextFixture() schemaKindFixture {
	const common = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" elementFormDefault="qualified"><xs:group name="G"><xs:sequence><xs:element name="e" type="xs:string"/><xs:choice><xs:element name="x" type="xs:string"/><xs:element name="y" type="xs:string"/></xs:choice><xs:any namespace="##other" processContents="lax" minOccurs="0" maxOccurs="1"/></xs:sequence></xs:group><xs:complexType name="Mixed"><xs:sequence><xs:group ref="G"/><xs:group ref="G"/></xs:sequence></xs:complexType><xs:complexType name="AllType"><xs:all><xs:element name="a" type="xs:string"/><xs:element name="b" type="xs:string"/></xs:all></xs:complexType></xs:schema>`
	fixture := schemaKindFixture{
		name:     "ContextMixed8",
		resolved: []schemaKindDocument{{name: "kind-common.xsd", text: common}},
	}
	for j := range 8 {
		n := strconv.Itoa(j)
		fixture.roots = append(fixture.roots, schemaKindDocument{
			name: "root-" + n + ".xsd",
			text: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:kind:` + n + `" targetNamespace="urn:kind:` + n + `" elementFormDefault="qualified"><xs:include schemaLocation="kind-common.xsd"/><xs:element name="r" type="t:Mixed"/><xs:element name="allRoot" type="t:AllType"/></xs:schema>`,
		})
	}
	for _, namespace := range []string{"urn:kind:0", "urn:kind:7"} {
		good := `<k:r xmlns:k="` + namespace + `"><k:e/><k:x/><k:e/><k:y/></k:r>`
		fixture.valid = append(fixture.valid,
			good,
			`<k:r xmlns:k="`+namespace+`"><k:e/><k:x/><f:extra xmlns:f="urn:foreign"/><k:e/><k:y/></k:r>`,
			`<k:allRoot xmlns:k="`+namespace+`"><k:a/><k:b/></k:allRoot>`,
			`<k:allRoot xmlns:k="`+namespace+`"><k:b/><k:a/></k:allRoot>`,
		)
		fixture.rejected = append(fixture.rejected, schemaKindRejection{
			document: `<k:r xmlns:k="` + namespace + `"><k:x/><k:e/><k:e/><k:y/></k:r>`,
			recovery: good, codes: []xsderrors.Code{xsderrors.CodeValidationElement, xsderrors.CodeValidationContent},
		})
	}
	fixture.rejected = append(fixture.rejected, schemaKindRejection{
		document: `<k:r xmlns:k="urn:kind:8"><k:e/><k:x/><k:e/><k:y/></k:r>`,
		recovery: `<k:r xmlns:k="urn:kind:0"><k:e/><k:x/><k:e/><k:y/></k:r>`, codes: []xsderrors.Code{xsderrors.CodeValidationRoot},
	})
	return fixture
}

func prepareSchemaKindFixture(tb testing.TB, fixture schemaKindFixture) []xsd.SchemaSource {
	tb.Helper()
	physicalBytes := 0
	for _, documents := range [][]schemaKindDocument{fixture.roots, fixture.resolved} {
		for _, document := range documents {
			if len(document.text) > 256<<10 {
				tb.Fatalf("schema %q exceeds fixture's 256 KiB admission limit", document.name)
			}
			physicalBytes += len(document.text)
		}
	}
	if physicalBytes > 1<<20 {
		tb.Fatal("physical schemas exceed fixture's 1 MiB admission limit")
	}
	for _, document := range fixture.valid {
		if len(document) > 64<<10 {
			tb.Fatal("valid instance exceeds fixture's 64 KiB admission limit")
		}
	}
	for _, rejected := range fixture.rejected {
		if len(rejected.document) > 64<<10 || len(rejected.recovery) > 64<<10 {
			tb.Fatal("rejection/recovery instance exceeds fixture's 64 KiB admission limit")
		}
	}
	resolved := make(map[string]xsd.SchemaSource, len(fixture.resolved))
	for _, document := range fixture.resolved {
		resolved[document.name] = xsd.Bytes(document.name, []byte(document.text))
	}
	resolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
		source, ok := resolved[location]
		if !ok {
			return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
		}
		return source, nil
	})
	sources := make([]xsd.SchemaSource, len(fixture.roots))
	for i, document := range fixture.roots {
		sources[i] = xsd.Bytes(document.name, []byte(document.text))
		if len(fixture.resolved) != 0 {
			sources[i] = sources[i].WithResolver(resolver)
		}
	}
	return sources
}

func checkSchemaKindFixture(tb testing.TB, engine *xsd.Engine, fixture schemaKindFixture) {
	tb.Helper()
	if engine == nil {
		tb.Fatal("Compile() returned nil engine")
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		tb.Fatalf("NewSession() error = %v", err)
	}
	if session == nil {
		tb.Fatal("NewSession() returned nil session")
	}
	for _, document := range fixture.valid {
		if err := session.Validate(strings.NewReader(document)); err != nil {
			tb.Fatalf("valid Validate(%q) error = %v", document, err)
		}
	}
	for _, rejected := range fixture.rejected {
		err := session.Validate(strings.NewReader(rejected.document))
		if err == nil {
			tb.Fatalf("Validate(%q) succeeded for the rejected instance", rejected.document)
		}
		diagnostics := xsderrors.Flatten(err)
		if len(diagnostics) == 0 {
			tb.Fatal("rejected instance returned no diagnostics")
		}
		seen := make(map[xsderrors.Code]bool, len(rejected.codes))
		for _, diagnostic := range diagnostics {
			xerr, ok := errors.AsType[*xsderrors.Error](diagnostic)
			if !ok || xerr.Category() != xsderrors.CategoryValidation || !slices.Contains(rejected.codes, xerr.Code()) {
				tb.Fatalf("rejected diagnostic = %v, want validation codes %v", diagnostic, rejected.codes)
			}
			seen[xerr.Code()] = true
		}
		for _, code := range rejected.codes {
			if !seen[code] {
				tb.Fatalf("rejected instance did not report expected validation code %s", code)
			}
		}
		if err := session.Validate(strings.NewReader(rejected.recovery)); err != nil {
			tb.Fatalf("same-session recovery Validate(%q) error = %v", rejected.recovery, err)
		}
	}
}

func TestSchemaKindBenchmarkPreflight(t *testing.T) {
	t.Parallel()
	for _, fixture := range schemaKindFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			sources := prepareSchemaKindFixture(t, fixture)
			engine, err := xsd.Compile(sources...)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			checkSchemaKindFixture(t, engine, fixture)
		})
	}
}

func BenchmarkSchemaKindCompile(b *testing.B) {
	for _, fixture := range schemaKindFixtures() {
		b.Run(fixture.name, func(b *testing.B) {
			sources := prepareSchemaKindFixture(b, fixture)
			engine, err := xsd.Compile(sources...)
			if err != nil {
				b.Fatalf("Compile() preflight error = %v", err)
			}
			checkSchemaKindFixture(b, engine, fixture)
			b.ReportAllocs()
			for b.Loop() {
				engine, err = xsd.Compile(sources...)
				if err != nil {
					b.Fatalf("Compile() error = %v", err)
				}
				if engine == nil {
					b.Fatal("Compile() returned nil engine")
				}
			}
			checkSchemaKindFixture(b, engine, fixture)
		})
	}
}
