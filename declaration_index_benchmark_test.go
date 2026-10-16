package xsd_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

type declarationIndexFixture struct {
	name    string
	sources []xsd.SchemaSource
	valid   []string
	invalid string
}

func declarationIndexFixtures() []declarationIndexFixture {
	return []declarationIndexFixture{
		{
			name:    "DirectiveContexts",
			sources: declarationIndexDirectiveSources(),
			valid: []string{
				`<common xmlns="urn:target:0"/>`,
				`<common xmlns="urn:target:31"/>`,
			},
			invalid: `<common xmlns="urn:target:32"/>`,
		},
		{
			name:    "Declarations512",
			sources: declarationIndexDeclarationSources(),
			valid: []string{
				`<e0 xmlns="urn:declaration:index"/>`,
				`<e511 xmlns="urn:declaration:index"/>`,
			},
			invalid: `<e512 xmlns="urn:declaration:index"/>`,
		},
	}
}

func declarationIndexDirectiveSources() []xsd.SchemaSource {
	const emptyLeaf = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	resolved := make(map[string]xsd.SchemaSource, 65)
	var common strings.Builder
	common.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	for i := range 64 {
		name := "leaf-" + strconv.Itoa(i) + ".xsd"
		common.WriteString(`<xs:include schemaLocation="` + name + `"/>`)
		resolved[name] = xsd.Bytes(name, []byte(emptyLeaf))
	}
	common.WriteString(`<xs:element name="common" type="xs:string"/></xs:schema>`)
	resolved["common.xsd"] = xsd.Bytes("common.xsd", []byte(common.String()))
	resolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
		source, ok := resolved[location]
		if !ok {
			return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
		}
		return source, nil
	})
	sources := make([]xsd.SchemaSource, 32)
	for i := range sources {
		target := "urn:target:" + strconv.Itoa(i)
		document := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="` + target + `"><xs:include schemaLocation="common.xsd"/></xs:schema>`
		sources[i] = xsd.Bytes("root-"+strconv.Itoa(i)+".xsd", []byte(document)).WithResolver(resolver)
	}
	return sources
}

func declarationIndexDeclarationSources() []xsd.SchemaSource {
	var document strings.Builder
	document.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:declaration:index">`)
	for i := range 512 {
		document.WriteString(`<xs:element name="e` + strconv.Itoa(i) + `" type="xs:string"/>`)
	}
	document.WriteString(`</xs:schema>`)
	return []xsd.SchemaSource{xsd.Bytes("declarations.xsd", []byte(document.String()))}
}

func checkDeclarationIndexFixture(tb testing.TB, engine *xsd.Engine, fixture declarationIndexFixture) {
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
		if validationErr := session.Validate(strings.NewReader(document)); validationErr != nil {
			tb.Fatalf("Validate(%q) error = %v", document, validationErr)
		}
	}
	err = session.Validate(strings.NewReader(fixture.invalid))
	if err == nil {
		tb.Fatalf("Validate(%q) succeeded for an undeclared root", fixture.invalid)
	}
	diagnostics := xsderrors.Flatten(err)
	if len(diagnostics) == 0 {
		tb.Fatal("undeclared root returned no diagnostics")
	}
	for _, diagnostic := range diagnostics {
		xerr, ok := errors.AsType[*xsderrors.Error](diagnostic)
		if !ok || xerr.Category() != xsderrors.CategoryValidation || xerr.Code() != xsderrors.CodeValidationRoot {
			tb.Fatalf("undeclared-root diagnostic = %v, want validation/root", diagnostic)
		}
	}
	for _, document := range fixture.valid {
		if validationErr := session.Validate(strings.NewReader(document)); validationErr != nil {
			tb.Fatalf("same-session recovery Validate(%q) error = %v", document, validationErr)
		}
	}
}

func TestDeclarationIndexFixtures(t *testing.T) {
	t.Parallel()
	for _, fixture := range declarationIndexFixtures() {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			engine, err := xsd.Compile(fixture.sources...)
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			checkDeclarationIndexFixture(t, engine, fixture)
		})
	}
}

func BenchmarkCompileDeclarationIndex(b *testing.B) {
	for _, fixture := range declarationIndexFixtures() {
		b.Run(fixture.name, func(b *testing.B) {
			engine, err := xsd.Compile(fixture.sources...)
			if err != nil {
				b.Fatalf("Compile() preflight error = %v", err)
			}
			checkDeclarationIndexFixture(b, engine, fixture)
			b.ReportAllocs()
			for b.Loop() {
				engine, err = xsd.Compile(fixture.sources...)
				if err != nil {
					b.Fatalf("Compile() error = %v", err)
				}
				if engine == nil {
					b.Fatal("Compile() returned nil engine")
				}
			}
			checkDeclarationIndexFixture(b, engine, fixture)
		})
	}
}
