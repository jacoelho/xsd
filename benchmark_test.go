package xsd_test

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const benchmarkSchema = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="rows">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="id" type="xs:int"/>
              <xs:element name="name" type="xs:string"/>
              <xs:element name="amount">
                <xs:simpleType>
                  <xs:restriction base="xs:decimal">
                    <xs:minInclusive value="0"/>
                    <xs:maxInclusive value="999999"/>
                  </xs:restriction>
                </xs:simpleType>
              </xs:element>
            </xs:sequence>
            <xs:attribute name="code" use="required">
              <xs:simpleType>
                <xs:restriction base="xs:string">
                  <xs:pattern value="[A-Z]{2}\d{4}"/>
                </xs:restriction>
              </xs:simpleType>
            </xs:attribute>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

func wideChoiceSchema(width int, extraParticles string) string {
	var sb strings.Builder
	sb.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="r">
    <xs:complexType>
      <xs:choice minOccurs="0" maxOccurs="unbounded">
`)
	for i := range width {
		sb.WriteString(`        <xs:element name="f` + strconv.Itoa(i) + `" type="xs:string"/>` + "\n")
	}
	sb.WriteString(extraParticles)
	sb.WriteString(`      </xs:choice>
    </xs:complexType>
  </xs:element>
</xs:schema>`)
	return sb.String()
}

func BenchmarkCompileSmallSchema(b *testing.B) {
	for b.Loop() {
		if _, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(benchmarkSchema))); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileAndFirstSession(b *testing.B) {
	for b.Loop() {
		engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(benchmarkSchema)))
		if err != nil {
			b.Fatal(err)
		}
		if _, err := engine.NewSession(xsd.ValidateOptions{}); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateRepeatedSmallDocument(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(benchmarkSchema)))
	if err != nil {
		b.Fatal(err)
	}
	doc := benchmarkDoc()
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := engine.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionValidateRepeatedSmallDocument(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(benchmarkSchema)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	doc := benchmarkDoc()
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionValidateRepeatedXSIType(b *testing.B) {
	const depth = 256
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:t" xmlns:t="urn:t" elementFormDefault="qualified">`)
	schema.WriteString(`<xs:complexType name="T0"/>`)
	for i := 1; i < depth; i++ {
		fmt.Fprintf(&schema, `<xs:complexType name="T%d"><xs:complexContent><xs:extension base="t:T%d"/></xs:complexContent></xs:complexType>`, i, i-1)
	}
	schema.WriteString(`<xs:element name="root"><xs:complexType><xs:sequence><xs:element name="item" type="t:T0" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element></xs:schema>`)
	engine, err := xsd.Compile(xsd.Bytes("xsi-type.xsd", []byte(schema.String())))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	var doc strings.Builder
	doc.WriteString(`<t:root xmlns:t="urn:t" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">`)
	for range 100 {
		fmt.Fprintf(&doc, `<t:item xsi:type="t:T%d"/>`, depth-1)
	}
	doc.WriteString(`</t:root>`)
	instance := doc.String()
	b.SetBytes(int64(len(instance)))
	b.ReportAllocs()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(instance)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionValidateRepeatedQNameValues(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="value" type="xs:QName" maxOccurs="unbounded"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	doc := benchmarkQNameDoc()
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionValidateRepeatedLaxWildcard(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:any processContents="lax" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	doc := `<root><o:unknown xmlns:o="urn:other"/></root>`
	if err := session.Validate(strings.NewReader(doc)); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionValidateStringLengthFacet(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:element name="root">
	    <xs:simpleType>
	      <xs:restriction base="xs:string">
	        <xs:minLength value="1024"/>
	      </xs:restriction>
	    </xs:simpleType>
	  </xs:element>
	</xs:schema>`)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	doc := `<root>` + strings.Repeat("é", 1024) + `</root>`
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionValidateWideChoice(b *testing.B) {
	const width = 200
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(wideChoiceSchema(width, ""))))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	var sb strings.Builder
	sb.WriteString("<r>")
	for i := range 4000 {
		name := "f" + strconv.Itoa(i%width)
		sb.WriteString("<" + name + ">x</" + name + ">")
	}
	sb.WriteString("</r>")
	doc := sb.String()
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateSubstitutionGroup(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(substitutionBenchmarkSchema(16))))
	if err != nil {
		b.Fatal(err)
	}
	doc := substitutionBenchmarkDoc(1000, 16)
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := engine.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionValidateDateDecimalRows(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="rows">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="when" type="xs:dateTime"/>
              <xs:element name="amount" type="xs:decimal"/>
            </xs:sequence>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	var doc strings.Builder
	doc.WriteString("<rows>")
	for i := range 100 {
		fmt.Fprintf(&doc, "<row><when>12026-05-%02dT23:59:60.123456789123+14:00</when><amount>+000%d.4500</amount></row>", i%28+1, i)
	}
	doc.WriteString("</rows>")
	text := doc.String()
	b.SetBytes(int64(len(text)))
	b.ReportAllocs()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(text)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateSmallInvalidDocument(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(benchmarkSchema)))
	if err != nil {
		b.Fatal(err)
	}
	doc := `<rows><row code="bad"><id>x</id><name>alpha</name><amount>-1</amount></row></rows>`
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := engine.Validate(strings.NewReader(doc)); err == nil {
			b.Fatal("Validate() succeeded unexpectedly")
		}
	}
}

func BenchmarkValidateManyRecoverablePathErrors(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`
	<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
	  <xs:element name="rows">
	    <xs:complexType>
	      <xs:sequence>
	        <xs:element name="row" type="xs:int" maxOccurs="unbounded"/>
	      </xs:sequence>
	    </xs:complexType>
	  </xs:element>
	</xs:schema>`)))
	if err != nil {
		b.Fatal(err)
	}
	doc := `<rows>` + strings.Repeat(`<row>x</row>`, 100) + `</rows>`
	opts := xsd.ValidateOptions{MaxErrors: 100}
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := engine.ValidateWithOptions(strings.NewReader(doc), opts); err == nil {
			b.Fatal("ValidateWithOptions() succeeded")
		}
	}
}

func BenchmarkValidateDeeplyNestedDocument(b *testing.B) {
	const depth = 128
	engine, err := xsd.CompileWithOptions(
		xsd.CompileOptions{MaxSchemaDepth: depth*3 + 16},
		xsd.Bytes("schema.xsd", []byte(deepSchema(depth))),
	)
	if err != nil {
		b.Fatal(err)
	}
	doc := deepDoc(depth)
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := engine.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionValidateDeepThenShallow(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:anyType"/></xs:schema>`)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	const depth = 4096
	deep := `<root>` + strings.Repeat(`<a>`, depth) + strings.Repeat(`</a>`, depth) + `</root>`
	if err := session.Validate(strings.NewReader(deep)); err != nil {
		b.Fatal(err)
	}
	shallow := `<root/>`
	b.SetBytes(int64(len(shallow)))
	b.ReportAllocs()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(shallow)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateDuplicateAttributes(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)))
	if err != nil {
		b.Fatal(err)
	}
	doc := duplicateAttributeDoc()
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := engine.Validate(strings.NewReader(doc)); err == nil {
			b.Fatal("Validate() succeeded")
		}
	}
}

func BenchmarkCompileDuplicateSchemaSources(b *testing.B) {
	schema := largeSchemaWithText(64 << 10)
	sources := make([]xsd.SchemaSource, 8)
	for i := range sources {
		sources[i] = xsd.Bytes(fmt.Sprintf("schema%d.xsd", i), []byte(schema))
	}
	b.SetBytes(int64(len(schema) * len(sources)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := xsd.Compile(sources...); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileIncludeGraph(b *testing.B) {
	const (
		depth = 3
		width = 4
	)
	source, totalBytes := includeGraphSource(depth, width)
	b.SetBytes(totalBytes)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := xsd.Compile(source); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileChameleonTargetFanout(b *testing.B) {
	for _, targetCount := range []int{1, 8, 32} {
		b.Run(fmt.Sprintf("targets_%d", targetCount), func(b *testing.B) {
			sources, totalBytes := chameleonTargetFanoutSources(targetCount)
			b.SetBytes(totalBytes)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := xsd.Compile(sources...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func chameleonTargetFanoutSources(targetCount int) ([]xsd.SchemaSource, int64) {
	const common = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="leaf.xsd"/><xs:element name="common" type="xs:string"/></xs:schema>`
	const leaf = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="leaf" type="xs:string"/></xs:schema>`
	resolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
		switch location {
		case "common.xsd":
			return xsd.Bytes("common.xsd", []byte(common)), nil
		case "leaf.xsd":
			return xsd.Bytes("leaf.xsd", []byte(leaf)), nil
		default:
			return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
		}
	})
	sources := make([]xsd.SchemaSource, targetCount)
	totalBytes := int64(len(common) + len(leaf))
	for i := range sources {
		target := fmt.Sprintf("urn:target:%d", i)
		schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="` + target + `"><xs:include schemaLocation="common.xsd"/></xs:schema>`
		sources[i] = xsd.Bytes(fmt.Sprintf("root-%d.xsd", i), []byte(schema)).WithResolver(resolver)
		totalBytes += int64(len(schema))
	}
	return sources, totalBytes
}

func BenchmarkCompileSchemaText(b *testing.B) {
	schema := largeSchemaWithText(256 << 10)
	b.SetBytes(int64(len(schema)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema))); err != nil {
			b.Fatal(err)
		}
	}
}

func includeGraphSource(depth, width int) (xsd.SchemaSource, int64) {
	type graphNode struct {
		children []int
		level    int
	}
	nodes := []graphNode{{}}
	levelStart := 0
	levelEnd := 1
	for level := range depth {
		for i := levelStart; i < levelEnd; i++ {
			for range width {
				child := len(nodes)
				nodes[i].children = append(nodes[i].children, child)
				nodes = append(nodes, graphNode{level: level + 1})
			}
		}
		levelStart, levelEnd = levelEnd, len(nodes)
	}

	sources := make(map[string]xsd.SchemaSource, len(nodes))
	var totalBytes int64
	for i, node := range nodes {
		var schema strings.Builder
		schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
		for _, child := range node.children {
			fmt.Fprintf(&schema, `<xs:include schemaLocation="node-%d.xsd"/>`, child)
		}
		fmt.Fprintf(&schema, `<xs:simpleType name="T%d"><xs:restriction base="xs:string"/></xs:simpleType>`, i)
		if node.level == 0 {
			schema.WriteString(`<xs:element name="root" type="T0"/>`)
		}
		schema.WriteString(`</xs:schema>`)
		data := []byte(schema.String())
		totalBytes += int64(len(data))
		name := fmt.Sprintf("node-%d.xsd", i)
		sources[name] = xsd.Bytes(name, data)
	}
	resolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
		source, ok := sources[location]
		if !ok {
			return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
		}
		return source, nil
	})
	return sources["node-0.xsd"].WithResolver(resolver), totalBytes
}

func BenchmarkValidateGeneratedLargeXML(b *testing.B) {
	schema := os.Getenv("XSD_LARGE_SCHEMA")
	doc := os.Getenv("XSD_LARGE_XML")
	if schema == "" || doc == "" {
		b.Skip("set XSD_LARGE_SCHEMA and XSD_LARGE_XML")
	}
	engine, err := xsd.Compile(xsd.File(schema))
	if err != nil {
		b.Fatal(err)
	}
	info, err := os.Stat(doc) //nolint:gosec // Benchmark intentionally measures caller-provided XML path.
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(info.Size())
	b.ReportAllocs()
	for b.Loop() {
		f, err := os.Open(doc) //nolint:gosec // Benchmark intentionally validates caller-provided XML path.
		if err != nil {
			b.Fatal(err)
		}
		validateErr := engine.Validate(f)
		closeErr := f.Close()
		if validateErr != nil {
			b.Fatal(validateErr)
		}
		if closeErr != nil {
			b.Fatal(closeErr)
		}
	}
}

func BenchmarkValidateIdentityConstraints(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(identityBenchmarkSchema)))
	if err != nil {
		b.Fatal(err)
	}
	doc := identityBenchmarkDoc(100)
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := engine.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateIdentityConstraintsRows(b *testing.B) {
	for _, rows := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("rows_%d", rows), func(b *testing.B) {
			engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(identityBenchmarkSchema)))
			if err != nil {
				b.Fatal(err)
			}
			doc := identityBenchmarkDoc(rows)
			b.SetBytes(int64(len(doc)))
			b.ReportAllocs()
			for b.Loop() {
				if err := engine.Validate(strings.NewReader(doc)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkValidateIdentityConstraintsFields(b *testing.B) {
	for _, fields := range []int{1, 3, 8} {
		b.Run(fmt.Sprintf("fields_%d", fields), func(b *testing.B) {
			engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(identityFieldsBenchmarkSchema(fields))))
			if err != nil {
				b.Fatal(err)
			}
			doc := identityFieldsBenchmarkDoc(fields, 100)
			b.SetBytes(int64(len(doc)))
			b.ReportAllocs()
			for b.Loop() {
				if err := engine.Validate(strings.NewReader(doc)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCompileCountedChoiceDFA(b *testing.B) {
	tests := []struct {
		name      string
		branches  int
		maxOccurs int
	}{
		{name: "small", branches: 4, maxOccurs: 4},
		{name: "medium", branches: 8, maxOccurs: 8},
		{name: "large", branches: 12, maxOccurs: 12},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			schema := countedChoiceDFASchema(tt.branches, tt.maxOccurs)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema))); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCompileSubstitutionGroups(b *testing.B) {
	const (
		headCount      = 20
		membersPerHead = 10
	)
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	for head := range headCount {
		fmt.Fprintf(&schema, `<xs:element name="head%d" type="xs:string"/>`, head)
		for member := range membersPerHead {
			fmt.Fprintf(&schema, `<xs:element name="member%d_%d" substitutionGroup="head%d" type="xs:string"/>`, head, member, head)
		}
	}
	schema.WriteString(`</xs:schema>`)
	data := []byte(schema.String())
	b.ReportAllocs()
	for b.Loop() {
		if _, err := xsd.Compile(xsd.Bytes("schema.xsd", data)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileAttributeGroupFanout(b *testing.B) {
	for _, refs := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("refs_%d", refs), func(b *testing.B) {
			schema := attributeGroupFanoutSchema(refs)
			b.ReportAllocs()
			for b.Loop() {
				if _, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema))); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCompileRegexCategoryEscapes(b *testing.B) {
	schema := regexCategoryEscapesSchema(100)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema))); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateConcurrent(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(benchmarkSchema)))
	if err != nil {
		b.Fatal(err)
	}
	doc := benchmarkDoc()
	const workers = 8
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	var wg sync.WaitGroup
	jobs := make(chan int)
	for range workers {
		wg.Go(func() {
			for range jobs {
				if err := engine.Validate(strings.NewReader(doc)); err != nil {
					b.Error(err)
					return
				}
			}
		})
	}
	for b.Loop() {
		jobs <- 0
	}
	close(jobs)
	wg.Wait()
}

const benchmarkDocRows = 100

func benchmarkDoc() string {
	var b strings.Builder
	b.WriteString("<rows>")
	for range benchmarkDocRows {
		b.WriteString(`<row code="AB1234"><id>`)
		b.WriteString("7")
		b.WriteString(`</id><name>alpha</name><amount>42.50</amount></row>`)
	}
	b.WriteString("</rows>")
	return b.String()
}

func benchmarkQNameDoc() string {
	var b strings.Builder
	b.WriteString(`<root xmlns:p="urn:bench">`)
	for range benchmarkDocRows {
		b.WriteString(`<value>p:item</value>`)
	}
	b.WriteString(`</root>`)
	return b.String()
}

const benchmarkDuplicateAttrs = 1000

func duplicateAttributeDoc() string {
	var b strings.Builder
	b.WriteString("<root")
	for i := range benchmarkDuplicateAttrs {
		fmt.Fprintf(&b, ` a%d="%d"`, i, i)
	}
	fmt.Fprintf(&b, ` a%d="dup"/>`, benchmarkDuplicateAttrs-1)
	return b.String()
}

func largeSchemaWithText(n int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:test"><xs:annotation><xs:documentation>`)
	b.WriteString(strings.Repeat("x", n))
	b.WriteString(`</xs:documentation></xs:annotation><xs:element name="root" type="xs:string"/></xs:schema>`)
	return b.String()
}

func deepSchema(depth int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	for i := range depth {
		fmt.Fprintf(&b, `<xs:element name="n%d"><xs:complexType><xs:sequence>`, i)
	}
	b.WriteString(`<xs:element name="leaf" type="xs:string"/>`)
	for range depth {
		b.WriteString(`</xs:sequence></xs:complexType></xs:element>`)
	}
	b.WriteString(`</xs:schema>`)
	return b.String()
}

func deepDoc(depth int) string {
	var b strings.Builder
	for i := range depth {
		fmt.Fprintf(&b, `<n%d>`, i)
	}
	b.WriteString(`<leaf>x</leaf>`)
	for i := depth - 1; i >= 0; i-- {
		fmt.Fprintf(&b, `</n%d>`, i)
	}
	return b.String()
}

func substitutionBenchmarkSchema(members int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	b.WriteString(`<xs:element name="head" type="xs:string"/>`)
	for i := range members {
		fmt.Fprintf(&b, `<xs:element name="m%d" substitutionGroup="head" type="xs:string"/>`, i)
	}
	b.WriteString(`<xs:element name="rows"><xs:complexType><xs:sequence><xs:element ref="head" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element>`)
	b.WriteString(`</xs:schema>`)
	return b.String()
}

func substitutionBenchmarkDoc(rows, members int) string {
	var b strings.Builder
	b.WriteString("<rows>")
	for i := range rows {
		fmt.Fprintf(&b, `<m%d>x</m%d>`, i%members, i%members)
	}
	b.WriteString("</rows>")
	return b.String()
}

const identityBenchmarkSchema = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="rows">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="row" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="id" type="xs:ID" use="required"/>
            <xs:attribute name="ref" type="xs:IDREF" use="optional"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="rowID"><xs:selector xpath="row"/><xs:field xpath="@id"/></xs:key>
    <xs:keyref name="rowRef" refer="rowID"><xs:selector xpath="row"/><xs:field xpath="@ref"/></xs:keyref>
  </xs:element>
</xs:schema>`

func identityBenchmarkDoc(rows int) string {
	var b strings.Builder
	b.WriteString("<rows>")
	for i := range rows {
		fmt.Fprintf(&b, `<row id="id%d"`, i)
		if i > 0 {
			fmt.Fprintf(&b, ` ref="id%d"`, i-1)
		}
		b.WriteString(`/>`)
	}
	b.WriteString("</rows>")
	return b.String()
}

func identityFieldsBenchmarkSchema(fields int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="rows"><xs:complexType><xs:sequence><xs:element name="row" maxOccurs="unbounded"><xs:complexType>`)
	for i := range fields {
		fmt.Fprintf(&b, `<xs:attribute name="k%d" type="xs:string" use="required"/>`, i)
	}
	b.WriteString(`</xs:complexType></xs:element></xs:sequence></xs:complexType><xs:key name="rowKey"><xs:selector xpath="row"/>`)
	for i := range fields {
		fmt.Fprintf(&b, `<xs:field xpath="@k%d"/>`, i)
	}
	b.WriteString(`</xs:key></xs:element></xs:schema>`)
	return b.String()
}

func identityFieldsBenchmarkDoc(fields, rows int) string {
	var b strings.Builder
	b.WriteString("<rows>")
	for row := range rows {
		b.WriteString(`<row`)
		for field := range fields {
			fmt.Fprintf(&b, ` k%d="v%d-%d"`, field, row, field)
		}
		b.WriteString(`/>`)
	}
	b.WriteString("</rows>")
	return b.String()
}

func countedChoiceDFASchema(branches, maxOccurs int) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:choice minOccurs="0" maxOccurs="%d">`, maxOccurs)
	for i := range branches {
		fmt.Fprintf(&b, `<xs:element name="e%d" type="xs:string"/>`, i)
	}
	b.WriteString(`</xs:choice></xs:complexType></xs:element></xs:schema>`)
	return b.String()
}

func attributeGroupFanoutSchema(refs int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	b.WriteString(`<xs:attributeGroup name="common"><xs:attribute name="code" type="xs:string"/></xs:attributeGroup>`)
	for i := range refs {
		fmt.Fprintf(&b, `<xs:complexType name="T%d"><xs:attributeGroup ref="common"/></xs:complexType>`, i)
		fmt.Fprintf(&b, `<xs:element name="e%d" type="T%d"/>`, i, i)
	}
	b.WriteString(`</xs:schema>`)
	return b.String()
}

func regexCategoryEscapesSchema(types int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	for i := range types {
		fmt.Fprintf(&b, `<xs:simpleType name="T%d"><xs:restriction base="xs:string"><xs:pattern value="\p{Lu}"/></xs:restriction></xs:simpleType>`, i)
	}
	b.WriteString(`</xs:schema>`)
	return b.String()
}

// BenchmarkCompileDeepSimpleTypeChain guards compile cost on long derivation
// chains: publication indexes must remain linear rather than flattening every
// type's complete ancestry.
func BenchmarkCompileDeepSimpleTypeChain(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	sb.WriteString(`<xs:simpleType name="t0"><xs:restriction base="xs:int"/></xs:simpleType>`)
	for i := 1; i < 1000; i++ {
		fmt.Fprintf(&sb, `<xs:simpleType name="t%d"><xs:restriction base="t%d"/></xs:simpleType>`, i, i-1)
	}
	sb.WriteString(`<xs:element name="root" type="t999"/></xs:schema>`)
	schema := []byte(sb.String())
	b.ReportAllocs()
	for b.Loop() {
		if _, err := xsd.Compile(xsd.Bytes("chain.xsd", schema)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileRepeatedNestedUnionMembers(b *testing.B) {
	const depth = 256
	var sb strings.Builder
	sb.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	sb.WriteString(`<xs:simpleType name="u0"><xs:union memberTypes="xs:int"/></xs:simpleType>`)
	for i := 1; i < depth; i++ {
		fmt.Fprintf(&sb, `<xs:simpleType name="u%d"><xs:union memberTypes="u%d u%d"/></xs:simpleType>`, i, i-1, i-1)
	}
	fmt.Fprintf(&sb, `<xs:element name="root" type="u%d"/></xs:schema>`, depth-1)
	schema := []byte(sb.String())
	b.ReportAllocs()
	for b.Loop() {
		if _, err := xsd.Compile(xsd.Bytes("nested-unions.xsd", schema)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompileOpaqueAnnotationPayload(b *testing.B) {
	var sb strings.Builder
	sb.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:annotation><xs:appinfo>`)
	for i := range 1_000 {
		fmt.Fprintf(&sb, `<meta index="%d"><nested>payload</nested></meta>`, i)
	}
	sb.WriteString(`</xs:appinfo></xs:annotation><xs:element name="root"/></xs:schema>`)
	schema := []byte(sb.String())
	b.ReportAllocs()
	for b.Loop() {
		if _, err := xsd.Compile(xsd.Bytes("annotation.xsd", schema)); err != nil {
			b.Fatal(err)
		}
	}
}
