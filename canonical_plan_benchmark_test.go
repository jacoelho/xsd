package xsd_test

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func canonicalPlanFixtures() []declarationIndexFixture {
	const noResolved = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`
	const commonElement = `<xs:element name="common" type="xs:string"/></xs:schema>`
	return []declarationIndexFixture{
		{
			name:    "NoResolved128",
			sources: []xsd.SchemaSource{xsd.Bytes("no-resolved.xsd", []byte(noResolved+canonicalPlanImports("")+commonElement))},
			valid:   []string{`<common>v</common>`},
			invalid: `<unknown/>`,
		},
		canonicalPlanResolvedImportFixture(),
		canonicalPlanFanoutFixture("NoLocation128Targets32", ""),
		canonicalPlanFanoutFixture("Unresolved128Targets32", "urn:missing:"),
	}
}

func canonicalPlanImports(locationPrefix string) string {
	var imports strings.Builder
	for i := range 128 {
		n := strconv.Itoa(i)
		imports.WriteString(`<xs:import namespace="urn:unused:` + n + `"`)
		if locationPrefix != "" {
			imports.WriteString(` schemaLocation="` + locationPrefix + n + `"`)
		}
		imports.WriteString(`/>`)
	}
	return imports.String()
}

func canonicalPlanResolvedImportFixture() declarationIndexFixture {
	const root = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:i="urn:i" targetNamespace="urn:root"><xs:import namespace="urn:i" schemaLocation="imported.xsd"/><xs:element name="r" type="i:S"/></xs:schema>`
	const imported = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:i"><xs:simpleType name="S"><xs:restriction base="xs:string"/></xs:simpleType></xs:schema>`
	source := xsd.Bytes("imported.xsd", []byte(imported))
	resolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
		if location != "imported.xsd" {
			return xsd.SchemaSource{}, fmt.Errorf("unexpected schema location %q", location)
		}
		return source, nil
	})
	return declarationIndexFixture{
		name:    "ResolvedImport",
		sources: []xsd.SchemaSource{xsd.Bytes("root.xsd", []byte(root)).WithResolver(resolver)},
		valid:   []string{`<r xmlns="urn:root">v</r>`},
		invalid: `<r xmlns="urn:outside">v</r>`,
	}
}

func canonicalPlanFanoutFixture(name, locationPrefix string) declarationIndexFixture {
	const leaf = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="leaf" type="xs:string"/></xs:schema>`
	common := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">` + canonicalPlanImports(locationPrefix) + `<xs:include schemaLocation="leaf.xsd"/><xs:element name="common" type="xs:string"/></xs:schema>`
	resolved := map[string]xsd.SchemaSource{
		"common.xsd": xsd.Bytes("common.xsd", []byte(common)),
		"leaf.xsd":   xsd.Bytes("leaf.xsd", []byte(leaf)),
	}
	missing := make(map[string]struct{})
	if locationPrefix != "" {
		for i := range 128 {
			missing[locationPrefix+strconv.Itoa(i)] = struct{}{}
		}
	}
	resolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
		if source, ok := resolved[location]; ok {
			return source, nil
		}
		if _, ok := missing[location]; ok {
			return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
		}
		return xsd.SchemaSource{}, fmt.Errorf("unexpected schema location %q", location)
	})
	sources := make([]xsd.SchemaSource, 32)
	for i := range sources {
		n := strconv.Itoa(i)
		root := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:target:` + n + `"><xs:include schemaLocation="common.xsd"/></xs:schema>`
		sources[i] = xsd.Bytes("root-"+n+".xsd", []byte(root)).WithResolver(resolver)
	}
	valid := make([]string, 0, 64)
	for i := range 32 {
		namespace := "urn:target:" + strconv.Itoa(i)
		valid = append(valid,
			`<common xmlns="`+namespace+`">v</common>`,
			`<leaf xmlns="`+namespace+`">v</leaf>`,
		)
	}
	return declarationIndexFixture{
		name:    name,
		sources: sources,
		valid:   valid,
		invalid: `<common xmlns="urn:target:32">v</common>`,
	}
}

func TestCanonicalPlanBenchmarkPreflight(t *testing.T) {
	t.Parallel()
	for _, fixture := range canonicalPlanFixtures() {
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

func BenchmarkCompileTargetReferencePlanning(b *testing.B) {
	for _, fixture := range canonicalPlanFixtures() {
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
