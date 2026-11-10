package xsd_test

import (
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestDerivationBudgetExhaustionKeepsSchemaLimitDiagnostic(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ name, schema string }{
		{name: "restriction", schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="base"><xs:attribute name="a" type="xs:string"/></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:restriction base="base">
    <xs:attribute name="a" type="xs:token"/>
  </xs:restriction></xs:complexContent></xs:complexType>
  <xs:element name="root" type="derived"/>
</xs:schema>`},
		{name: "extension after restriction", schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="A"><xs:attribute name="a" type="xs:int"/></xs:complexType>
  <xs:complexType name="R"><xs:complexContent><xs:restriction base="A"><xs:attribute name="a" use="prohibited"/></xs:restriction></xs:complexContent></xs:complexType>
  <xs:complexType name="E"><xs:complexContent><xs:extension base="R"><xs:attribute name="a" type="xs:int"/></xs:extension></xs:complexContent></xs:complexType>
  <xs:element name="root" type="E"/>
</xs:schema>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			schema := xsd.Bytes("derivation.xsd", []byte(test.schema))
			// Sweep failing phases without depending on an internal step count.
			for limit := 1; limit <= 1024; limit++ {
				engine, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaDependencySteps: limit}, schema)
				if err == nil {
					if engine == nil {
						t.Fatal("successful compilation returned no engine")
					}
					return
				}
				if engine != nil {
					t.Fatal("failed compilation returned an engine")
				}
				expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
				diagnostic, _ := errors.AsType[*xsderrors.Error](err)
				if diagnostic.Path() != "derivation.xsd" || diagnostic.Line() <= 0 {
					t.Fatalf("limit %d: failure located at %q:%d, want derivation.xsd with a line: %v", limit, diagnostic.Path(), diagnostic.Line(), err)
				}
			}
			t.Fatal("small derivation schema did not compile within the dependency budget")
		})
	}
}

func TestModelGroupDependencyBudgetPreservesReferenceLocation(t *testing.T) {
	t.Parallel()
	const schemaText = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:group name="G"><xs:sequence><xs:element name="v" type="xs:string"/></xs:sequence></xs:group>
  <xs:complexType name="C">
    <xs:group ref="G" minOccurs="2" maxOccurs="3"/>
  </xs:complexType>
  <xs:element name="r" type="C"/>
</xs:schema>`
	const path = "model-group-work.xsd"
	input := xsd.Bytes(path, []byte(schemaText))
	var bounded *xsd.Engine
	located := false
	for budget := 1; budget <= 1024; budget++ {
		engine, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaDependencySteps: budget}, input)
		if err == nil {
			if engine == nil {
				t.Fatal("successful bounded compile returned no engine")
			}
			bounded = engine
			break
		}
		if engine != nil {
			t.Fatal("failed compile published an engine")
		}
		diagnostic, ok := errors.AsType[*xsderrors.Error](err)
		if !ok || diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaLimit {
			t.Fatalf("bounded compile returned %v, want schema-limit failure", err)
		}
		if diagnostic.Path() == path && diagnostic.Line() == 4 && diagnostic.Column() == 5 {
			located = true
		}
	}
	if bounded == nil || !located {
		t.Fatalf("bounded compilation: engine=%v, reference-location failure=%v", bounded != nil, located)
	}
}

const xsNS = `xmlns:xs="http://www.w3.org/2001/XMLSchema"`

// doublingGroups returns group declarations g0..gN where g0 holds one element
// and every gi is a sequence of two references to g(i-1), so gN flattens into
// 2^N particles.
func doublingGroups(n int) string {
	var b strings.Builder
	b.WriteString(`<xs:group name="g0"><xs:sequence><xs:element name="e" type="xs:string"/></xs:sequence></xs:group>`)
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, `<xs:group name="g%d"><xs:sequence><xs:group ref="g%d"/><xs:group ref="g%d"/></xs:sequence></xs:group>`, i, i-1, i-1)
	}
	return b.String()
}

func doublingSchema(n int) string {
	return `<xs:schema ` + xsNS + `>` + doublingGroups(n) +
		fmt.Sprintf(`<xs:complexType name="T"><xs:group ref="g%d"/></xs:complexType><xs:element name="root" type="T"/></xs:schema>`, n)
}

// compileAllocated compiles one schema and returns the bytes allocated process
// wide while doing so. Callers must not run in parallel.
func compileAllocated(t *testing.T, schema string) (uint64, error) {
	t.Helper()
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{}, xsd.Bytes("admission.xsd", []byte(schema)))
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc, err
}

func expectDependencyLimit(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("compilation succeeded, want schema.limit")
	}
	expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
	if !strings.Contains(err.Error(), "MaxSchemaDependencySteps") {
		t.Fatalf("error %q does not name MaxSchemaDependencySteps", err)
	}
}

func expectNotDependencyLimit(t *testing.T, err error) {
	t.Helper()
	if err != nil && strings.Contains(err.Error(), "MaxSchemaDependencySteps") {
		t.Fatalf("unexpected dependency limit: %v", err)
	}
}

func expectAllocUnder(t *testing.T, got uint64, limitMiB uint64) {
	t.Helper()
	if got > limitMiB<<20 {
		t.Fatalf("allocated %d MiB, want under %d MiB", got>>20, limitMiB)
	}
}

func TestDerivedRecordsAreChargedToDependencyBudget(t *testing.T) {
	t.Run("doubling groups at default", func(t *testing.T) {
		alloc, err := compileAllocated(t, doublingSchema(20))
		expectDependencyLimit(t, err)
		expectAllocUnder(t, alloc, 128)
	})
	t.Run("doubling groups at small budget", func(t *testing.T) {
		schema := doublingSchema(10)
		if _, err := xsd.CompileWithOptions(xsd.CompileOptions{}, xsd.Bytes("d.xsd", []byte(schema))); err != nil {
			t.Fatalf("N=10 at default: %v", err)
		}
		_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaDependencySteps: 1000}, xsd.Bytes("d.xsd", []byte(schema)))
		expectDependencyLimit(t, err)
	})
	t.Run("fan-out of a large group", func(t *testing.T) {
		parent := func(refs int) string {
			var b strings.Builder
			b.WriteString(`<xs:schema ` + xsNS + `>` + doublingGroups(18) + `<xs:complexType name="T"><xs:sequence>`)
			for range refs {
				b.WriteString(`<xs:group ref="g18"/>`)
			}
			b.WriteString(`</xs:sequence></xs:complexType><xs:element name="root" type="T"/></xs:schema>`)
			return b.String()
		}
		alloc, err := compileAllocated(t, parent(64))
		expectDependencyLimit(t, err)
		expectAllocUnder(t, alloc, 128)
		_, err = compileAllocated(t, parent(1))
		expectNotDependencyLimit(t, err)
	})
}

func attributeList(prefix string, n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, `<xs:attribute name="%s%d" type="xs:string"/>`, prefix, i)
	}
	return b.String()
}

func TestRetainedAttributeUsesAreChargedToDependencyBudget(t *testing.T) {
	t.Parallel()
	extensions := `<xs:schema ` + xsNS + `><xs:complexType name="base">` + attributeList("a", 100) + `</xs:complexType>`
	viaGroup := `<xs:schema ` + xsNS + `><xs:attributeGroup name="ag">` + attributeList("a", 100) + `</xs:attributeGroup>`
	for i := range 100 {
		extensions += fmt.Sprintf(`<xs:complexType name="d%d"><xs:complexContent><xs:extension base="base"/></xs:complexContent></xs:complexType>`, i)
		viaGroup += fmt.Sprintf(`<xs:complexType name="d%d"><xs:attributeGroup ref="ag"/><xs:attribute name="extra" type="xs:string"/></xs:complexType>`, i)
	}
	extensions += `<xs:element name="root" type="base"/></xs:schema>`
	viaGroup += `<xs:element name="root" type="d0"/></xs:schema>`
	for name, schema := range map[string]string{"extensions": extensions, "attribute group": viaGroup} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaDependencySteps: 5000}, xsd.Bytes("uses.xsd", []byte(schema)))
			expectDependencyLimit(t, err)
			if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaDependencySteps: 100000}, xsd.Bytes("uses.xsd", []byte(schema))); err != nil {
				t.Fatalf("budget 100000: %v", err)
			}
		})
	}
}

func TestRetainedWildcardNamespacesAreChargedToDependencyBudget(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	b.WriteString(`<xs:schema ` + xsNS + `>`)
	b.WriteString(`<xs:complexType name="t0"><xs:anyAttribute namespace="urn:n:0:0" processContents="lax"/></xs:complexType>`)
	for i := 1; i <= 50; i++ {
		var ns []string
		for j := range 100 {
			ns = append(ns, fmt.Sprintf("urn:n:%d:%d", i, j))
		}
		fmt.Fprintf(&b, `<xs:complexType name="t%d"><xs:complexContent><xs:extension base="t%d"><xs:anyAttribute namespace="%s" processContents="lax"/></xs:extension></xs:complexContent></xs:complexType>`,
			i, i-1, strings.Join(ns, " "))
	}
	b.WriteString(`<xs:element name="root" type="t50"/></xs:schema>`)
	schema := b.String()
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaDependencySteps: 50000}, xsd.Bytes("ns.xsd", []byte(schema)))
	expectDependencyLimit(t, err)
	if _, err := xsd.CompileWithOptions(xsd.CompileOptions{}, xsd.Bytes("ns.xsd", []byte(schema))); err != nil {
		t.Fatalf("default budget: %v", err)
	}
}

func TestContentModelReadsShareStoredParticles(t *testing.T) {
	choiceSchema := func(k, refs int, occurs string) string {
		var b strings.Builder
		b.WriteString(`<xs:schema ` + xsNS + `><xs:group name="g"><xs:choice>`)
		for i := range k {
			fmt.Fprintf(&b, `<xs:element name="e%d" type="xs:string"/>`, i)
		}
		b.WriteString(`</xs:choice></xs:group><xs:complexType name="T"><xs:sequence>`)
		for range refs {
			fmt.Fprintf(&b, `<xs:group ref="g"%s/>`, occurs)
		}
		b.WriteString(`</xs:sequence></xs:complexType><xs:element name="root" type="T"/></xs:schema>`)
		return b.String()
	}
	for _, test := range []struct {
		name    string
		k, refs int
		occurs  string
		maxMiB  uint64
	}{
		{"4000x4000", 4000, 4000, "", 128},
		// The 1M expanded particles are walked by the element-declaration
		// consistency checker, which allocates about 145 bytes per visit
		// under MaxContentModelAnalysisSteps; that is not copying on read.
		{"1000x1000 optional", 1000, 1000, ` minOccurs="0"`, 256},
	} {
		t.Run(test.name, func(t *testing.T) {
			alloc, err := compileAllocated(t, choiceSchema(test.k, test.refs, test.occurs))
			if err != nil {
				expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
			}
			expectAllocUnder(t, alloc, test.maxMiB)
		})
	}
	t.Run("control", func(t *testing.T) {
		if _, err := xsd.CompileWithOptions(xsd.CompileOptions{}, xsd.Bytes("c.xsd", []byte(choiceSchema(100, 100, "")))); err != nil {
			t.Fatalf("100x100: %v", err)
		}
	})
	t.Run("choice of large sequences", func(t *testing.T) {
		var b strings.Builder
		b.WriteString(`<xs:schema ` + xsNS + `>` + doublingGroups(17) + `<xs:complexType name="T"><xs:choice>`)
		for range 1000 {
			b.WriteString(`<xs:group ref="g17"/>`)
		}
		b.WriteString(`</xs:choice></xs:complexType><xs:element name="root" type="T"/></xs:schema>`)
		alloc, err := compileAllocated(t, b.String())
		if err != nil {
			// Identical alternatives violate UPA before any limit is reached.
			if x, ok := errors.AsType[*xsderrors.Error](err); !ok || x.Category() != xsderrors.CategorySchemaCompile {
				t.Fatalf("unexpected error %v", err)
			}
		}
		expectAllocUnder(t, alloc, 256)
	})
}

// dependencyChain returns the 6-level complexContent extension chain; member
// returns the declaration each level adds.
func dependencyChain(member func(level int) string) string {
	var b strings.Builder
	b.WriteString("<xs:complexType name=\"t0\"><xs:sequence/></xs:complexType>\n")
	for level := 1; level <= 6; level++ {
		fmt.Fprintf(&b, "<xs:complexType name=\"t%d\"><xs:complexContent><xs:extension base=\"t%d\">%s</xs:extension></xs:complexContent></xs:complexType>\n", level, level-1, member(level))
	}
	return b.String()
}

func schemaDocumentText(body string) string {
	return "<xs:schema " + xsNS + ">\n" + body + "</xs:schema>"
}

// TestDependencyBudgetFailuresNameTheirDocument sweeps the dependency budget
// and requires every failure to name a document of the schema set. It does not
// prove which component scope locates a failure; see
// TestDependencyBudgetFailureNamesTheFailingDocument.
func TestDependencyBudgetFailuresNameTheirDocument(t *testing.T) {
	t.Parallel()

	elementChain := dependencyChain(func(l int) string {
		return fmt.Sprintf(`<xs:sequence><xs:element name="e%d" type="xs:string"/></xs:sequence>`, l)
	})
	attributeChain := dependencyChain(func(l int) string {
		return fmt.Sprintf(`<xs:attribute name="a%d" type="xs:string"/>`, l)
	})
	wildcards := `<xs:complexType name="w"><xs:sequence><xs:any namespace="urn:a urn:b urn:c urn:d" processContents="lax"/></xs:sequence>
<xs:anyAttribute namespace="urn:a urn:b urn:c urn:d" processContents="lax"/></xs:complexType>
`
	allScopes := `<xs:simpleType name="s"><xs:restriction base="xs:string"/></xs:simpleType>
<xs:attribute name="ga" type="xs:string"/>
<xs:attributeGroup name="inner"><xs:anyAttribute namespace="urn:a urn:b"/></xs:attributeGroup>
<xs:attributeGroup name="outer"><xs:attribute name="la" type="xs:string"/><xs:attributeGroup ref="inner"/></xs:attributeGroup>
<xs:group name="g"><xs:sequence><xs:element name="ge" type="xs:string"/></xs:sequence></xs:group>
<xs:element name="head" type="base"/>
<xs:element name="ref" type="xs:string"/>
<xs:complexType name="base"><xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence></xs:complexType>
<xs:complexType name="ext"><xs:complexContent><xs:extension base="base"><xs:sequence><xs:group ref="g" maxOccurs="2"/></xs:sequence><xs:attributeGroup ref="outer"/></xs:extension></xs:complexContent></xs:complexType>
<xs:complexType name="res"><xs:complexContent><xs:restriction base="base"><xs:sequence><xs:element name="b" type="xs:string"/></xs:sequence></xs:restriction></xs:complexContent></xs:complexType>
<xs:complexType name="holder"><xs:sequence>
<xs:element ref="ref"/>
<xs:element name="local"><xs:complexType><xs:attribute name="x"><xs:simpleType><xs:restriction base="xs:int"/></xs:simpleType></xs:attribute><xs:attribute ref="ga"/></xs:complexType></xs:element>
</xs:sequence></xs:complexType>
<xs:element name="member" substitutionGroup="head" type="ext"/>
`
	main := `<xs:schema ` + xsNS + `>
<xs:include schemaLocation="part.xsd"/>
<xs:element name="root" type="t6"/>
</xs:schema>`
	partSource := xsd.Bytes("main.xsd", []byte(main)).WithResolver(xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
		if location != "part.xsd" {
			return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
		}
		return xsd.Bytes("part.xsd", []byte(schemaDocumentText(elementChain))), nil
	}))
	for _, fixture := range []struct {
		name   string
		source xsd.SchemaSource
		paths  []string
	}{
		{"element chain", xsd.Bytes("a.xsd", []byte(schemaDocumentText(elementChain+`<xs:element name="root" type="t6"/>`))), []string{"a.xsd"}},
		{"attribute chain", xsd.Bytes("b.xsd", []byte(schemaDocumentText(attributeChain+`<xs:element name="root" type="t6"/>`))), []string{"b.xsd"}},
		{"wildcards", xsd.Bytes("c.xsd", []byte(schemaDocumentText(wildcards+`<xs:element name="root" type="w"/>`))), []string{"c.xsd"}},
		{"all scopes", xsd.Bytes("d.xsd", []byte(schemaDocumentText(allScopes))), []string{"d.xsd"}},
		{"include set", partSource, []string{"main.xsd", "part.xsd"}},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			if _, err := xsd.Compile(fixture.source); err != nil {
				t.Fatalf("fixture does not compile at the default limits: %v", err)
			}
			for limit := 1; ; limit++ {
				if limit > 5000 {
					t.Fatal("fixture never compiled within the dependency budget")
				}
				_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaDependencySteps: limit}, fixture.source)
				if err == nil {
					return
				}
				diagnostic, ok := errors.AsType[*xsderrors.Error](err)
				if !ok || diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaLimit {
					t.Fatalf("limit %d: error = %v, want schema-limit diagnostic", limit, err)
				}
				if !slices.Contains(fixture.paths, diagnostic.Path()) || diagnostic.Line() <= 0 {
					t.Fatalf("limit %d: failure located at %q:%d, want a line in one of %v: %v", limit, diagnostic.Path(), diagnostic.Line(), fixture.paths, err)
				}
			}
		})
	}
}

// TestDependencyBudgetFailureNamesTheFailingDocument requires a dependency
// budget failure to name the innermost component being compiled, including
// when that component is reached from a referrer in another document.
func TestDependencyBudgetFailureNamesTheFailingDocument(t *testing.T) {
	t.Parallel()

	// Half the documented cost of 500 wildcard namespaces (one step each,
	// docs/RESOURCE_LIMITS.md), so the budget ends inside big and not in a
	// set-wide phase.
	const limit = 250
	namespaces := make([]string, 500)
	for i := range namespaces {
		namespaces[i] = fmt.Sprintf("urn:n%d", i+1)
	}
	list := strings.Join(namespaces, " ")
	anyElement := `<xs:any namespace="` + list + `" processContents="lax"/>`
	anyAttribute := `<xs:anyAttribute namespace="` + list + `" processContents="lax"/>`
	shapes := []struct {
		name string
		big  []string
	}{
		{"complex type", []string{`<xs:complexType name="big">`, `<xs:sequence>`, anyElement, `</xs:sequence>`, `</xs:complexType>`}},
		{"attribute group", []string{`<xs:attributeGroup name="big">`, anyAttribute, `</xs:attributeGroup>`}},
		{"complex type with attribute wildcard", []string{`<xs:complexType name="big">`, anyAttribute, `</xs:complexType>`}},
		{"model group", []string{`<xs:group name="big">`, `<xs:sequence>`, anyElement, `</xs:sequence>`, `</xs:group>`}},
		{"element with anonymous type", []string{`<xs:element name="big">`, `<xs:complexType>`, `<xs:sequence>`, anyElement, `</xs:sequence>`, `</xs:complexType>`, `</xs:element>`}},
	}
	type row struct {
		name                string
		main, part          string
		wantPath            string
		firstLine, lastLine int
	}
	document := func(lines ...string) string {
		return strings.Join(append(append([]string{`<xs:schema ` + xsNS + `>`}, lines...), `</xs:schema>`), "\n")
	}
	include := `<xs:include schemaLocation="part.xsd"/>`
	var rows []row
	for _, shape := range shapes {
		n := len(shape.big)
		// big is on lines 2..n+1 of part.xsd, and on lines 3..n+2 of main.xsd
		// behind the include.
		rows = append(rows,
			row{name: shape.name + "/part", main: document(include), part: document(shape.big...), wantPath: "part.xsd", firstLine: 2, lastLine: n + 1},
			row{name: shape.name + "/main", main: document(append([]string{include}, shape.big...)...), part: document(`<xs:complexType name="trivial"/>`), wantPath: "main.xsd", firstLine: 3, lastLine: n + 2},
		)
	}
	complexBig, attributeGroupBig, groupBig := shapes[0].big, shapes[1].big, shapes[3].big
	rows = append(rows,
		row{name: "reference/element type", main: document(include, `<xs:element name="root" type="big"/>`), part: document(complexBig...), wantPath: "part.xsd", firstLine: 2, lastLine: len(complexBig) + 1},
		row{name: "reference/group", main: document(include, `<xs:complexType name="user">`, `<xs:sequence><xs:group ref="big"/></xs:sequence>`, `</xs:complexType>`), part: document(groupBig...), wantPath: "part.xsd", firstLine: 2, lastLine: len(groupBig) + 1},
		row{name: "reference/attribute group", main: document(include, `<xs:complexType name="user">`, `<xs:attributeGroup ref="big"/>`, `</xs:complexType>`), part: document(attributeGroupBig...), wantPath: "part.xsd", firstLine: 2, lastLine: len(attributeGroupBig) + 1},
	)
	for _, test := range rows {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := xsd.Bytes("main.xsd", []byte(test.main)).WithResolver(xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
				if location != "part.xsd" {
					return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
				}
				return xsd.Bytes("part.xsd", []byte(test.part)), nil
			}))
			_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaDependencySteps: limit}, input)
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Code() != xsderrors.CodeSchemaLimit {
				t.Fatalf("error = %v, want schema-limit diagnostic", err)
			}
			if diagnostic.Path() != test.wantPath || diagnostic.Line() < test.firstLine || diagnostic.Line() > test.lastLine {
				t.Fatalf("failure located at %q:%d, want %s lines %d-%d: %v", diagnostic.Path(), diagnostic.Line(), test.wantPath, test.firstLine, test.lastLine, err)
			}
		})
	}
}

// TestSetWideAndPlanningLimitFailuresAreLocated requires limit failures that
// belong to no component to name a document: a set-wide failure names the
// explicit root, not whichever document sorts first, and loading and
// target-context planning failures name a document of the set.
func TestSetWideAndPlanningLimitFailuresAreLocated(t *testing.T) {
	t.Parallel()

	parts := map[string]string{
		"0dep.xsd": `<xs:schema ` + xsNS + `><xs:complexType name="dep"/></xs:schema>`,
		"zz.xsd":   `<xs:schema ` + xsNS + `><xs:element name="h"/><xs:element name="a" substitutionGroup="h"/><xs:element name="b" substitutionGroup="h"/><xs:element name="c" substitutionGroup="h"/></xs:schema>`,
	}
	main := `<xs:schema ` + xsNS + `><xs:include schemaLocation="0dep.xsd"/><xs:include schemaLocation="zz.xsd"/></xs:schema>`
	tests := []struct {
		name     string
		opts     xsd.CompileOptions
		wantPath string
	}{
		{"substitution closure", xsd.CompileOptions{MaxSubstitutionClosureEntries: 1}, "main.xsd"},
		{"references", xsd.CompileOptions{MaxSchemaReferences: 1}, "main.xsd"},
		{"target contexts", xsd.CompileOptions{MaxSchemaTargetContexts: 1}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := xsd.Bytes("main.xsd", []byte(main)).WithResolver(xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
				part, ok := parts[location]
				if !ok {
					return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
				}
				return xsd.Bytes(location, []byte(part)), nil
			}))
			_, err := xsd.CompileWithOptions(test.opts, input)
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Code() != xsderrors.CodeSchemaLimit {
				t.Fatalf("error = %v, want schema-limit diagnostic", err)
			}
			if diagnostic.Path() == "" || diagnostic.Line() == 0 {
				t.Fatalf("failure is unlocated: %v", err)
			}
			if test.wantPath != "" && diagnostic.Path() != test.wantPath {
				t.Fatalf("failure located at %q, want %q: %v", diagnostic.Path(), test.wantPath, err)
			}
		})
	}
}
