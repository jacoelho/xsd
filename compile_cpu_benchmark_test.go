package xsd_test

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

// BenchmarkCompileIndependentContentModels measures the cost of running the
// content-model checks over many unrelated global groups. The larger fixtures
// require explicit admission limits so the benchmark stays focused on the
// compiler path rather than the default token/work ceilings.
func BenchmarkCompileIndependentContentModels(b *testing.B) {
	for _, count := range []int{5_000, 20_000, 50_000, 100_000} {
		b.Run(fmt.Sprintf("models_%d", count), func(b *testing.B) {
			schema := independentContentModelsSchema(count)
			b.SetBytes(int64(len(schema)))
			b.ReportAllocs()
			opts := xsd.CompileOptions{
				MaxSchemaTokenBytes:          8 << 20,
				MaxContentModelAnalysisSteps: 2_000_000,
			}
			for b.Loop() {
				if _, err := xsd.CompileWithOptions(opts, xsd.Bytes("independent-models.xsd", schema)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func independentContentModelsSchema(count int) []byte {
	var schema strings.Builder
	schema.Grow(count * 48)
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	for i := range count {
		fmt.Fprintf(&schema, `<xs:group name="g%d"><xs:sequence/></xs:group>`, i)
	}
	schema.WriteString(`</xs:schema>`)
	return []byte(schema.String())
}

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

type canonicalChildSourceOpponent struct {
	document string
	code     xsderrors.Code
}

type canonicalChildSourceFixture struct {
	name, schema string
	accepted     []string
	rejected     []canonicalChildSourceOpponent
}

func canonicalChildSourceFixtures(tb testing.TB) []canonicalChildSourceFixture {
	tb.Helper()
	var attributes strings.Builder
	for index := range 128 {
		fmt.Fprintf(&attributes, `<xs:attribute name="a%d" type="xs:string"/>`, index)
	}
	wide := simpleContentProjectionSchema
	for _, anchor := range []string{
		`<xs:extension base="xs:decimal"><xs:attribute name="tag" type="xs:string" use="required"/></xs:extension>`,
		`<xs:restriction base="Amount"><xs:minInclusive value="0"/><xs:maxInclusive value="10"/><xs:attribute name="tag" type="xs:string" use="required"/></xs:restriction>`,
	} {
		if strings.Count(wide, anchor) != 1 {
			tb.Fatal("simple-content fixture anchor is not unique")
		}
		end := strings.LastIndex(anchor, "</")
		wide = strings.Replace(wide, anchor, anchor[:end]+attributes.String()+anchor[end:], 1)
	}
	const inline = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="Mixed" mixed="true"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:simpleContent>
        <xs:restriction base="Mixed">
          <xs:simpleType>
            <xs:restriction base="xs:string"/>
          </xs:simpleType>
        </xs:restriction>
      </xs:simpleContent>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	const valid = `<r xmlns:q="urn:p"><amount tag="x">1.5</amount><limited tag="x">2</limited><q>q:Thing</q></r>`
	fixtures := []canonicalChildSourceFixture{
		{name: "FacetsWide", schema: wide, accepted: []string{valid}},
		{name: "NoFacetsInline", schema: inline, accepted: []string{`<root>value</root>`, `<root/>`}},
	}
	fixtures[0].rejected = append(fixtures[0].rejected,
		canonicalChildSourceOpponent{strings.Replace(valid, `>2</limited>`, `>11</limited>`, 1), xsderrors.CodeValidationFacet},
		canonicalChildSourceOpponent{strings.Replace(valid, ` tag="x"`, ``, 1), xsderrors.CodeValidationAttribute},
		canonicalChildSourceOpponent{strings.Replace(valid, `q:Thing`, `q:Other`, 1), xsderrors.CodeValidationElement},
	)
	fixtures[1].rejected = append(fixtures[1].rejected,
		canonicalChildSourceOpponent{`<other>value</other>`, xsderrors.CodeValidationRoot},
		canonicalChildSourceOpponent{`<root x="1">value</root>`, xsderrors.CodeValidationAttribute},
	)
	for _, fixture := range fixtures {
		if len(fixture.schema) > 96<<10 {
			tb.Fatal("canonical child source fixture exceeds byte bound")
		}
	}
	return fixtures
}

func checkCanonicalChildSourceFixture(tb testing.TB, engine *xsd.Engine, fixture canonicalChildSourceFixture) {
	tb.Helper()
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		tb.Fatal(err)
	}
	for _, document := range fixture.accepted {
		if err := session.Validate(strings.NewReader(document)); err != nil {
			tb.Fatalf("%s accepted document: %v", fixture.name, err)
		}
	}
	for _, opponent := range fixture.rejected {
		err := session.Validate(strings.NewReader(opponent.document))
		diagnostics := xsderrors.Flatten(err)
		if len(diagnostics) == 0 {
			tb.Fatalf("%s opponent has no diagnostic: %v", fixture.name, err)
		}
		for _, diagnostic := range diagnostics {
			item, ok := errors.AsType[*xsderrors.Error](diagnostic)
			if !ok || item.Category() != xsderrors.CategoryValidation || item.Code() != opponent.code {
				tb.Fatalf("%s opponent diagnostic = %v, want validation/%s", fixture.name, diagnostic, opponent.code)
			}
		}
		for _, document := range fixture.accepted {
			if err := session.Validate(strings.NewReader(document)); err != nil {
				tb.Fatalf("%s same-session recovery: %v", fixture.name, err)
			}
		}
	}
}

func BenchmarkCompileCanonicalChildSources(b *testing.B) {
	for _, fixture := range canonicalChildSourceFixtures(b) {
		b.Run(fixture.name, func(b *testing.B) {
			source := xsd.Bytes("canonical-children.xsd", []byte(fixture.schema))
			engine, err := xsd.Compile(source)
			if err != nil || engine == nil {
				b.Fatalf("Compile: engine=%v error=%v", engine, err)
			}
			checkCanonicalChildSourceFixture(b, engine, fixture)
			b.ReportAllocs()
			for b.Loop() {
				engine, err = xsd.Compile(source)
				if err != nil || engine == nil {
					b.Fatalf("Compile: engine=%v error=%v", engine, err)
				}
			}
			checkCanonicalChildSourceFixture(b, engine, fixture)
		})
	}
}

const builtinOwnershipSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="L"><xs:list itemType="xs:int"/></xs:simpleType>
  <xs:simpleType name="U"><xs:union memberTypes="xs:boolean L"/></xs:simpleType>
  <xs:simpleType name="Amount"><xs:restriction base="xs:decimal"><xs:minInclusive value="1.5"/></xs:restriction></xs:simpleType>
  <xs:complexType name="TextBase"><xs:simpleContent><xs:extension base="xs:string"><xs:attribute name="note"/></xs:extension></xs:simpleContent></xs:complexType>
  <xs:complexType name="Text3"><xs:simpleContent><xs:restriction base="TextBase"><xs:maxLength value="3"/></xs:restriction></xs:simpleContent></xs:complexType>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element name="l" type="L"/><xs:element name="u" type="U"/><xs:element name="amount" type="Amount"/><xs:element name="text" type="Text3"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`

const builtinOwnershipValid = `<root><l>1 2</l><u>true</u><amount>2.0</amount><text note="raw p:q">abc</text></root>`

func checkBuiltinOwnership(tb testing.TB, engine *xsd.Engine) {
	tb.Helper()
	session, sessionErr := engine.NewSession(xsd.ValidateOptions{})
	if sessionErr != nil {
		tb.Fatal(sessionErr)
	}
	reader := bytes.NewReader(nil)
	for _, document := range []string{
		builtinOwnershipValid,
		`<root><l>1 2</l><u>1 2</u><amount>2.0</amount><text note="raw p:q">abc</text></root>`,
	} {
		reader.Reset([]byte(document))
		if err := session.Validate(reader); err != nil {
			tb.Fatalf("valid builtin routes: %v", err)
		}
	}
	for _, opponent := range []struct {
		name     string
		document string
		message  string
	}{
		{"list", `<root><l>1 x</l><u>true</u><amount>2.0</amount><text note="raw p:q">abc</text></root>`, `value "1 x" is not a valid list of xs:int`},
		{"union", `<root><l>1 2</l><u>not-a-value</u><amount>2.0</amount><text note="raw p:q">abc</text></root>`, `value "not-a-value" does not match any union member`},
		{"bound", `<root><l>1 2</l><u>true</u><amount>1.0</amount><text note="raw p:q">abc</text></root>`, `value "1.0" does not satisfy minInclusive "1.5"`},
		{"length", `<root><l>1 2</l><u>true</u><amount>2.0</amount><text note="raw p:q">abcd</text></root>`, `has length 4 characters; expected at most 3`},
	} {
		reader.Reset([]byte(opponent.document))
		err := session.Validate(reader)
		if err == nil || !xsderrors.IsConclusiveValidation(err) {
			tb.Fatalf("%s opponent requires semantic rejection: %v", opponent.name, err)
		}
		failures := xsderrors.Flatten(err)
		if len(failures) != 1 {
			tb.Fatalf("%s diagnostic count = %d, want 1: %v", opponent.name, len(failures), err)
		}
		diagnostic, ok := errors.AsType[*xsderrors.Error](failures[0])
		if !ok || diagnostic.Category() != xsderrors.CategoryValidation || diagnostic.Code() != xsderrors.CodeValidationFacet || !strings.Contains(diagnostic.Message(), opponent.message) {
			tb.Fatalf("%s diagnostic = %v", opponent.name, err)
		}
		reader.Reset([]byte(builtinOwnershipValid))
		if recoveryErr := session.Validate(reader); recoveryErr != nil {
			tb.Fatalf("same-session recovery after %s: %v", opponent.name, recoveryErr)
		}
	}
}

func BenchmarkCompileBuiltinSimpleRoutes(b *testing.B) {
	source := xsd.Bytes("builtin-ownership.xsd", []byte(builtinOwnershipSchema))
	engine, err := xsd.Compile(source)
	if err != nil || engine == nil {
		b.Fatalf("compile builtin routes: engine=%v err=%v", engine, err)
	}
	checkBuiltinOwnership(b, engine)
	b.ReportAllocs()
	for b.Loop() {
		engine, err = xsd.Compile(source)
		if err != nil || engine == nil {
			b.Fatalf("compile builtin routes: engine=%v err=%v", engine, err)
		}
	}
	checkBuiltinOwnership(b, engine)
}

const compileOwnerNamespaceSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:r="urn:root" xmlns:a="urn:a" targetNamespace="urn:root" elementFormDefault="qualified">
<xs:import namespace="urn:a"/>
<xs:element name="r"><xs:complexType><xs:sequence><xs:any namespace="urn:a urn:b ##local" processContents="lax" minOccurs="0" maxOccurs="unbounded"/></xs:sequence><xs:anyAttribute namespace="urn:attr ##targetNamespace" processContents="lax"/></xs:complexType>
<xs:unique name="ids"><xs:selector xpath="a:item"/><xs:field xpath="@a:*"/></xs:unique></xs:element>
</xs:schema>`

const compileOwnerNamespaceItemSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:a" elementFormDefault="qualified" attributeFormDefault="qualified"><xs:element name="item"><xs:complexType><xs:attribute name="id" type="xs:string" use="required"/></xs:complexType></xs:element></xs:schema>`

func checkCompileOwnerNamespace(tb testing.TB, engine *xsd.Engine) {
	tb.Helper()
	session, sessionErr := engine.NewSession(xsd.ValidateOptions{})
	if sessionErr != nil {
		tb.Fatal(sessionErr)
	}
	valid := []byte(`<r xmlns="urn:root" xmlns:a="urn:a" xmlns:q="urn:attr" q:code="x"><a:item a:id="1"/></r>`)
	reader := bytes.NewReader(valid)
	if err := session.Validate(reader); err != nil {
		tb.Fatalf("valid namespaces: %v", err)
	}
	reader.Reset(bytes.Replace(valid, []byte(`xmlns:a="urn:a"`), []byte(`xmlns:a="urn:outside"`), 1))
	if err := session.Validate(reader); err == nil {
		tb.Fatal("child outside admitted wildcard namespaces accepted")
	}
	reader.Reset(valid)
	if err := session.Validate(reader); err != nil {
		tb.Fatalf("same-session recovery: %v", err)
	}
	unbound := strings.Replace(compileOwnerNamespaceSchema, `xpath="@a:*"`, `xpath="@missing:*"`, 1)
	failed, err := xsd.Compile(xsd.Bytes("unbound-identity.xsd", []byte(unbound)), xsd.Bytes("item.xsd", []byte(compileOwnerNamespaceItemSchema)))
	if failed != nil || err == nil {
		tb.Fatalf("unbound identity wildcard: engine=%v err=%v", failed, err)
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaReference || diagnostic.Message() != "unbound QName prefix missing" {
		tb.Fatalf("unbound identity wildcard diagnostic: %v", err)
	}
}

func BenchmarkCompileOwnerNamespaceMetadata(b *testing.B) {
	source := []byte(compileOwnerNamespaceSchema)
	itemSource := []byte(compileOwnerNamespaceItemSchema)
	engine, err := xsd.Compile(xsd.Bytes("namespace.xsd", source), xsd.Bytes("item.xsd", itemSource))
	if err != nil {
		b.Fatal(err)
	}
	checkCompileOwnerNamespace(b, engine)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := xsd.Compile(xsd.Bytes("namespace.xsd", source), xsd.Bytes("item.xsd", itemSource)); err != nil {
			b.Fatal(err)
		}
	}
}

type schemaIDFixture struct {
	name, schema, valid string
	invalid             []string
	codes               [][]xsderrors.Code
}

func schemaIDFlat(name string) schemaIDFixture {
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
	return schemaIDFixture{
		name: name, schema: b.String(), valid: `<root>7</root>`,
		invalid: []string{`<other>7</other>`, `<root/>`, `<root>bad</root>`},
		codes:   [][]xsderrors.Code{{xsderrors.CodeValidationRoot}, {xsderrors.CodeValidationFacet}, {xsderrors.CodeValidationFacet}},
	}
}

func schemaIDDeep() schemaIDFixture {
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
	return schemaIDFixture{
		name: "Deep64", schema: schema.String(), valid: valid.String(),
		invalid: []string{`<other>7</other>`, missing.String(), bad.String()},
		codes:   [][]xsderrors.Code{{xsderrors.CodeValidationRoot}, {xsderrors.CodeValidationContent}, {xsderrors.CodeValidationFacet}},
	}
}

func schemaIDSuccesses() []schemaIDFixture {
	return []schemaIDFixture{schemaIDFlat("Zero512"), schemaIDFlat("Sparse512"), schemaIDFlat("Dense512"), schemaIDDeep()}
}

func schemaIDOptions() xsd.CompileOptions {
	return xsd.CompileOptions{MaxSchemaDepth: 256}
}

func checkSchemaIDEngine(tb testing.TB, e *xsd.Engine, err error, f schemaIDFixture) {
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

func checkSchemaIDDuplicate(tb testing.TB, e *xsd.Engine, err error) {
	tb.Helper()
	d, ok := errors.AsType[*xsderrors.Error](err)
	if e != nil || !ok || d.Category() != xsderrors.CategorySchemaCompile || d.Code() != xsderrors.CodeSchemaInvalidAttribute || d.Message() != "duplicate schema id id000" || d.Path() != "LateDuplicate512.xsd" || d.Line() != 513 || d.Column() != 1 || d.Cause() != nil {
		tb.Fatalf("late duplicate: engine=%v diagnostic=%#v error=%v", e, d, err)
	}
}

func checkSchemaIDFreshRecovery(tb testing.TB) {
	tb.Helper()
	f := schemaIDFlat("Dense512")
	src := xsd.Bytes(f.name+".xsd", []byte(f.schema))
	e, err := xsd.CompileWithOptions(schemaIDOptions(), src)
	checkSchemaIDEngine(tb, e, err, f)
}

func BenchmarkCompileSchemaIDs(b *testing.B) {
	for _, f := range schemaIDSuccesses() {
		b.Run(f.name, func(b *testing.B) {
			src := xsd.Bytes(f.name+".xsd", []byte(f.schema))
			opts := schemaIDOptions()
			e, err := xsd.CompileWithOptions(opts, src)
			checkSchemaIDEngine(b, e, err, f)
			b.ReportAllocs()
			for b.Loop() {
				e, err = xsd.CompileWithOptions(opts, src)
				if err != nil || e == nil {
					b.Fatalf("%s timed compile: engine=%v error=%v", f.name, e, err)
				}
			}
			checkSchemaIDEngine(b, e, err, f)
		})
	}
	b.Run("LateDuplicate512", func(b *testing.B) {
		f := schemaIDFlat("LateDuplicate512")
		src := xsd.Bytes(f.name+".xsd", []byte(f.schema))
		opts := schemaIDOptions()
		e, err := xsd.CompileWithOptions(opts, src)
		checkSchemaIDDuplicate(b, e, err)
		checkSchemaIDFreshRecovery(b)
		b.ReportAllocs()
		for b.Loop() {
			e, err = xsd.CompileWithOptions(opts, src)
			if err == nil || e != nil {
				b.Fatalf("late duplicate timed compile: engine=%v error=%v", e, err)
			}
		}
		checkSchemaIDDuplicate(b, e, err)
		checkSchemaIDFreshRecovery(b)
	})
}

func complexDerivationRoutesSource(memberSimpleType string) []byte {
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:complexType name="C0"/><xs:complexType name="S0"><xs:simpleContent><xs:extension base="xs:string"/></xs:simpleContent></xs:complexType>`)
	for i := 1; i <= 8; i++ {
		fmt.Fprintf(&schema, `<xs:complexType name="C%d"><xs:complexContent><xs:extension base="C%d"/></xs:complexContent></xs:complexType><xs:complexType name="S%d"><xs:simpleContent><xs:extension base="S%d"/></xs:simpleContent></xs:complexType>`, i, i-1, i, i-1)
	}
	fmt.Fprintf(&schema, `<xs:element name="headC" type="C0" abstract="true"/><xs:element name="memberC" type="C8" substitutionGroup="headC"/><xs:element name="headS" type="xs:string" abstract="true"/><xs:element name="memberS" type="%s" substitutionGroup="headS"/><xs:element name="headA" type="xs:anyType" abstract="true"/><xs:element name="memberA" type="S8" substitutionGroup="headA"/><xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="headC"/><xs:element ref="headS"/><xs:element ref="headA"/></xs:sequence></xs:complexType></xs:element></xs:schema>`, memberSimpleType)
	return []byte(schema.String())
}

func checkComplexDerivationRoutes(tb testing.TB, engine *xsd.Engine) {
	tb.Helper()
	session, sessionErr := engine.NewSession(xsd.ValidateOptions{})
	if sessionErr != nil {
		tb.Fatal(sessionErr)
	}
	valid := []byte(`<root><memberC/><memberS>x</memberS><memberA>y</memberA></root>`)
	reader := bytes.NewReader(valid)
	if err := session.Validate(reader); err != nil {
		tb.Fatalf("valid substitution routes: %v", err)
	}
	for _, invalid := range []string{
		`<root><memberS>x</memberS><memberC/><memberA>y</memberA></root>`,
		`<unknown/>`,
	} {
		reader.Reset([]byte(invalid))
		if err := session.Validate(reader); err == nil {
			tb.Fatalf("invalid substitution document accepted: %s", invalid)
		}
		reader.Reset(valid)
		if err := session.Validate(reader); err != nil {
			tb.Fatalf("same-session recovery: %v", err)
		}
	}
	failed, err := xsd.Compile(xsd.Bytes("incompatible-substitution.xsd", complexDerivationRoutesSource("C8")))
	if failed != nil || err == nil {
		tb.Fatalf("unrelated complex member accepted for string head: engine=%v err=%v", failed, err)
	}
	diagnostic, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaReference || diagnostic.Message() != "substitution group member memberS type C8 is not derived from head headS type {http://www.w3.org/2001/XMLSchema}string" {
		tb.Fatalf("incompatible substitution diagnostic: %v", err)
	}
}

func BenchmarkCompileComplexDerivationRoutes(b *testing.B) {
	source := xsd.Bytes("complex-derivation.xsd", complexDerivationRoutesSource("S8"))
	engine, err := xsd.Compile(source)
	if err != nil {
		b.Fatal(err)
	}
	checkComplexDerivationRoutes(b, engine)
	b.ReportAllocs()
	for b.Loop() {
		engine, err = xsd.Compile(source)
		if err != nil {
			b.Fatal(err)
		}
	}
	checkComplexDerivationRoutes(b, engine)
}

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

type facetPolicyFixture struct {
	name     string
	schema   string
	accepted []string
	rejected []string
}

func facetPolicyFixtures() []facetPolicyFixture {
	const header = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`
	const root = `<xs:element name="e" type="V"/></xs:schema>`
	return []facetPolicyFixture{
		{
			name: "Decimal",
			schema: header + `<xs:simpleType name="V"><xs:restriction base="xs:decimal">
<xs:minInclusive value="-10"/><xs:maxInclusive value="10"/>
<xs:totalDigits value="3"/><xs:fractionDigits value="1"/>
</xs:restriction></xs:simpleType>` + root,
			accepted: []string{`<e>2.5</e>`},
			rejected: []string{`<e>12.5</e>`, `<e>1.25</e>`},
		},
		{
			name: "String",
			schema: header + `<xs:simpleType name="V"><xs:restriction base="xs:string">
<xs:length value="3"/><xs:pattern value="[a-z]+"/>
<xs:enumeration value="abc"/><xs:whiteSpace value="collapse"/>
</xs:restriction></xs:simpleType>` + root,
			accepted: []string{`<e>abc</e>`, `<e>  abc  </e>`},
			rejected: []string{`<e>abcd</e>`, `<e>xyz</e>`},
		},
		{
			name: "List",
			schema: header + `<xs:simpleType name="Items"><xs:list itemType="xs:int"/></xs:simpleType>
<xs:simpleType name="V"><xs:restriction base="Items">
<xs:length value="2"/><xs:pattern value=".*"/><xs:enumeration value="1 2"/>
</xs:restriction></xs:simpleType>` + root,
			accepted: []string{`<e>1 2</e>`},
			rejected: []string{`<e>1 3</e>`, `<e>1</e>`},
		},
		{
			name: "Union",
			schema: header + `<xs:simpleType name="Mix"><xs:union memberTypes="xs:int xs:boolean"/></xs:simpleType>
<xs:simpleType name="V"><xs:restriction base="Mix">
<xs:pattern value="[12]|true"/><xs:enumeration value="1"/><xs:enumeration value="true"/>
</xs:restriction></xs:simpleType>` + root,
			accepted: []string{`<e>1</e>`, `<e>true</e>`},
			rejected: []string{`<e>2</e>`, `<e>3</e>`},
		},
	}
}

func checkFacetPolicyFixture(tb testing.TB, engine *xsd.Engine, fixture facetPolicyFixture) {
	tb.Helper()
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		tb.Fatal(err)
	}
	for _, document := range fixture.accepted {
		if validationErr := session.Validate(strings.NewReader(document)); validationErr != nil {
			tb.Fatalf("%s accepted %q: %v", fixture.name, document, validationErr)
		}
	}
	for _, document := range fixture.rejected {
		validationErr := session.Validate(strings.NewReader(document))
		if validationErr == nil || !xsderrors.IsConclusiveValidation(validationErr) {
			tb.Fatalf("%s rejected %q: %v", fixture.name, document, validationErr)
		}
		diagnostics := xsderrors.Flatten(validationErr)
		if len(diagnostics) != 1 {
			tb.Fatalf("%s rejected %q diagnostic count = %d, want 1: %v", fixture.name, document, len(diagnostics), validationErr)
		}
		diagnostic, ok := errors.AsType[*xsderrors.Error](diagnostics[0])
		if !ok || diagnostic.Category() != xsderrors.CategoryValidation || diagnostic.Code() != xsderrors.CodeValidationFacet {
			tb.Fatalf("%s rejected %q diagnostic = %v, want validation facet", fixture.name, document, validationErr)
		}
		for _, accepted := range fixture.accepted {
			if recoveryErr := session.Validate(strings.NewReader(accepted)); recoveryErr != nil {
				tb.Fatalf("%s same-session recovery after %q: %v", fixture.name, document, recoveryErr)
			}
		}
	}
}

func BenchmarkCompileFacetPolicy(b *testing.B) {
	for _, fixture := range facetPolicyFixtures() {
		b.Run(fixture.name, func(b *testing.B) {
			source := xsd.Bytes("facet-policy.xsd", []byte(fixture.schema))
			engine, err := xsd.Compile(source)
			if err != nil || engine == nil {
				b.Fatalf("%s Compile: engine=%v err=%v", fixture.name, engine, err)
			}
			checkFacetPolicyFixture(b, engine, fixture)
			b.ReportAllocs()
			for b.Loop() {
				engine, err = xsd.Compile(source)
				if err != nil || engine == nil {
					b.Fatalf("%s Compile: engine=%v err=%v", fixture.name, engine, err)
				}
			}
			checkFacetPolicyFixture(b, engine, fixture)
		})
	}
}

func occurrenceSchema(maximum string) string {
	var s strings.Builder
	s.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:simpleType name="Text"><xs:restriction base="xs:string"><xs:minLength value="+0001"/><xs:maxLength value="+0008"/></xs:restriction></xs:simpleType><xs:simpleType name="Number"><xs:restriction base="xs:decimal"><xs:totalDigits value="6"/><xs:fractionDigits value="2"/></xs:restriction></xs:simpleType><xs:element name="root"><xs:complexType><xs:sequence>`)
	for i := range 32 {
		fmt.Fprintf(&s, `<xs:element name="e%d" type="Text" minOccurs="+0001" maxOccurs="%s"/>`, i, maximum)
	}
	s.WriteString(`</xs:sequence></xs:complexType></xs:element><xs:element name="number" type="Number"/></xs:schema>`)
	return s.String()
}

func occurrenceDoc(first int) string {
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

func occurrencePublished(tb testing.TB, engine *xsd.Engine, fifthCode xsderrors.Code) {
	tb.Helper()
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		tb.Fatal(err)
	}
	good := occurrenceDoc(1)
	for _, doc := range []string{good, occurrenceDoc(4), strings.Replace(good, "<e0>x</e0>", "<e0>12345678</e0>", 1), `<number>1234.56</number>`, `<number>123456</number>`} {
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			tb.Fatal(err)
		}
	}
	bad := []struct {
		document string
		code     xsderrors.Code
		message  string
	}{
		{occurrenceDoc(0), xsderrors.CodeValidationContent, `content is incomplete; expected next child ["e0"]`},
		{`<number>1234.567</number>`, xsderrors.CodeValidationFacet, `value "1234.567" has 7 total digits; totalDigits permits at most 6`},
		{`<number>12.345</number>`, xsderrors.CodeValidationFacet, `value "12.345" has 3 fractional digits; fractionDigits permits at most 2`},
		{`<number>12345.67</number>`, xsderrors.CodeValidationFacet, `value "12345.67" has 7 total digits; totalDigits permits at most 6`},
		{strings.Replace(good, "<e0>x</e0>", "<e0/>", 1), xsderrors.CodeValidationFacet, `has length 0 characters; expected at least 1`},
		{strings.Replace(good, "<e0>x</e0>", "<e0>123456789</e0>", 1), xsderrors.CodeValidationFacet, `has length 9 characters; expected at most 8`},
	}
	if fifthCode != "" {
		bad = append(bad, struct {
			document string
			code     xsderrors.Code
			message  string
		}{occurrenceDoc(5), fifthCode, `unexpected child element "e0"; expected next child ["e1"]`})
	} else {
		if err := session.Validate(strings.NewReader(occurrenceDoc(5))); err != nil {
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
			if ok && d.Category() == xsderrors.CategoryValidation && d.Code() == c.code && strings.Contains(d.Message(), c.message) {
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
			data := []byte(occurrenceSchema(c.max))
			if len(data) > 16384 {
				b.Fatal("fixture source bound")
			}
			source := xsd.Bytes("occurrence.xsd", data)
			pre, err := xsd.Compile(source)
			if err != nil || pre == nil {
				b.Fatalf("preflight: %v", err)
			}
			occurrencePublished(b, pre, c.fifthCode)
			var last *xsd.Engine
			b.ReportAllocs()
			for b.Loop() {
				var err error
				last, err = xsd.Compile(source)
				if err != nil || last == nil {
					b.Fatalf("compile: %v", err)
				}
			}
			occurrencePublished(b, last, c.fifthCode)
		})
	}
}

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
