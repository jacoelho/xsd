package xsd_test

import (
	"bytes"
	"errors"
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
		sb.WriteString(`        <xs:element name="f`)
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(`" type="xs:string"/>`)
		sb.WriteByte('\n')
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

// BenchmarkNewSessionPerOperation creates a fresh session per operation, so
// per-session setup such as the input buffer is charged on every document.
func BenchmarkNewSessionPerOperation(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(benchmarkSchema)))
	if err != nil {
		b.Fatal(err)
	}
	doc := benchmarkDoc()
	invalid := strings.Replace(doc, `code="AB1234"`, `code="ab1234"`, 1)
	for _, document := range []struct {
		text  string
		valid bool
	}{{doc, true}, {invalid, false}} {
		session, err := engine.NewSession(xsd.ValidateOptions{})
		if err != nil {
			b.Fatal(err)
		}
		if err := session.Validate(strings.NewReader(document.text)); (err == nil) != document.valid {
			b.Fatalf("fresh session valid=%t document: %v", document.valid, err)
		}
	}
	b.Run("new_only", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := engine.NewSession(xsd.ValidateOptions{}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("new_validate_small", func(b *testing.B) {
		b.SetBytes(int64(len(doc)))
		b.ReportAllocs()
		for b.Loop() {
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				b.Fatal(err)
			}
			if err := session.Validate(strings.NewReader(doc)); err != nil {
				b.Fatal(err)
			}
		}
	})
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

func BenchmarkSessionValidateNamespaceAdmissionChurn(b *testing.B) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="e">
    <xs:complexType>
      <xs:sequence><xs:element ref="e" minOccurs="0"/></xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("namespace-admission.xsd", []byte(schema)))
	if err != nil {
		b.Fatal(err)
	}
	for _, depth := range []int{16, 64, 256} {
		for _, mode := range []namespaceAdmissionBenchmarkMode{namespaceAdmissionStable, namespaceAdmissionChurn} {
			name := fmt.Sprintf("depth_%d/%s", depth, mode)
			b.Run(name, func(b *testing.B) {
				doc := namespaceAdmissionBenchmarkDocument(depth, mode)
				session, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceDepth: depth})
				if err != nil {
					b.Fatal(err)
				}
				if err := session.Validate(strings.NewReader(doc)); err != nil {
					b.Fatal(err)
				}
				b.SetBytes(int64(len(doc)))
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					if err := session.Validate(strings.NewReader(doc)); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

type namespaceAdmissionBenchmarkMode uint8

const (
	namespaceAdmissionStable namespaceAdmissionBenchmarkMode = iota
	namespaceAdmissionChurn
)

func (m namespaceAdmissionBenchmarkMode) String() string {
	if m == namespaceAdmissionChurn {
		return "churn_true"
	}
	return "churn_false"
}

func namespaceAdmissionBenchmarkDocument(depth int, mode namespaceAdmissionBenchmarkMode) string {
	var doc strings.Builder
	for i := range depth {
		doc.WriteString("<e")
		if mode == namespaceAdmissionChurn {
			doc.WriteString(` xmlns:p`)
			doc.WriteString(strconv.Itoa(i))
			doc.WriteString(`="urn:`)
			doc.WriteString(strconv.Itoa(i))
			doc.WriteByte('"')
		}
		doc.WriteByte('>')
	}
	for range depth {
		doc.WriteString("</e>")
	}
	return doc.String()
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

func BenchmarkSessionValidateIDAttributeStart(b *testing.B) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="item" maxOccurs="unbounded"><xs:complexType><xs:attribute name="id" type="xs:ID" use="required"/></xs:complexType></xs:element>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`
	var doc strings.Builder
	doc.WriteString(`<root>`)
	for i := range 100 {
		fmt.Fprintf(&doc, `<item id="id%d"/>`, i)
	}
	doc.WriteString(`</root>`)
	benchmarkSessionDocument(b, schema, doc.String())
}

func BenchmarkSessionValidateSimpleIDEnd(b *testing.B) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="id" type="xs:ID" maxOccurs="unbounded"/>
  </xs:sequence></xs:complexType></xs:element>
</xs:schema>`
	var doc strings.Builder
	doc.WriteString(`<root>`)
	for i := range 100 {
		fmt.Fprintf(&doc, `<id>id%d</id>`, i)
	}
	doc.WriteString(`</root>`)
	benchmarkSessionDocument(b, schema, doc.String())
}

func benchmarkSessionDocument(b *testing.B, schema, doc string) {
	b.Helper()
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	if err := session.Validate(strings.NewReader(doc)); err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(doc)); err != nil {
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
		sb.WriteByte('<')
		sb.WriteString(name)
		sb.WriteString(">x</")
		sb.WriteString(name)
		sb.WriteByte('>')
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
	engine, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaDepth: depth*3 + 16},
		xsd.Bytes("schema.xsd", []byte(deepSchema(depth))))

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
	const depth = 4096
	session, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceDepth: depth + 1})
	if err != nil {
		b.Fatal(err)
	}
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
		validateErr := engine.ValidateWithOptions(f, xsd.ValidateOptions{MaxInstanceBytes: info.Size()})
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

func BenchmarkSessionValidateIdentityConstraintsRows(b *testing.B) {
	for _, rows := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("rows_%d", rows), func(b *testing.B) {
			engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(identityBenchmarkSchema)))
			if err != nil {
				b.Fatal(err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				b.Fatal(err)
			}
			doc := identityBenchmarkDoc(rows)
			b.SetBytes(int64(len(doc)))
			b.ReportAllocs()
			for b.Loop() {
				if err := session.Validate(strings.NewReader(doc)); err != nil {
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

func BenchmarkSessionValidateNestedIdentitySelections(b *testing.B) {
	for _, depth := range []int{16, 64, 256} {
		b.Run(fmt.Sprintf("depth_%d", depth), func(b *testing.B) {
			engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(nestedIdentitySelectionsBenchmarkSchema)))
			if err != nil {
				b.Fatal(err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceDepth: depth + 1})
			if err != nil {
				b.Fatal(err)
			}
			doc := nestedIdentitySelectionsBenchmarkDoc(depth)
			b.SetBytes(int64(len(doc)))
			b.ReportAllocs()
			for b.Loop() {
				if err := session.Validate(strings.NewReader(doc)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSessionValidateNestedIdentitySelectionPaths(b *testing.B) {
	const (
		rootNameBytes = 64 << 10
		depth         = 64
	)
	root := "r" + strings.Repeat("x", rootNameBytes-1)
	schema := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="%s">
    <xs:complexType><xs:sequence><xs:any processContents="skip"/></xs:sequence></xs:complexType>
    <xs:unique name="ids"><xs:selector xpath=".//*"/><xs:field xpath="@id"/></xs:unique>
  </xs:element>
</xs:schema>`, root)
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceDepth: depth + 1})
	if err != nil {
		b.Fatal(err)
	}
	doc := nestedIdentitySelectionPathBenchmarkDoc(root, depth)
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionValidateRetainedIdentityPaths(b *testing.B) {
	const (
		rootNameBytes = 64 << 10
		rows          = 1000
	)
	root := "r" + strings.Repeat("x", rootNameBytes-1)
	schema := fmt.Sprintf(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="%s">
    <xs:complexType><xs:sequence><xs:element name="row" maxOccurs="unbounded"><xs:complexType>
      <xs:attribute name="id" type="xs:ID" use="required"/>
      <xs:attribute name="ref" type="xs:IDREF" use="optional"/>
    </xs:complexType></xs:element></xs:sequence></xs:complexType>
    <xs:key name="rowID"><xs:selector xpath="row"/><xs:field xpath="@id"/></xs:key>
    <xs:keyref name="rowRef" refer="rowID"><xs:selector xpath="row"/><xs:field xpath="@ref"/></xs:keyref>
  </xs:element>
</xs:schema>`, root)
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	doc := retainedIdentityPathsBenchmarkDoc(root, rows)
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionValidateDisjointIdentityPaths(b *testing.B) {
	const (
		branches = 256
		depth    = 128
	)
	tests := []struct {
		name   string
		schema string
		doc    string
	}{
		{
			name:   "lexical",
			schema: disjointIdentityPathsBenchmarkSchema,
			doc:    disjointIdentityPathsBenchmarkDoc(branches, depth),
		},
		{
			name:   "expanded",
			schema: disjointExpandedIdentityPathsBenchmarkSchema,
			doc: disjointExpandedIdentityPathsBenchmarkDoc(
				"urn:"+strings.Repeat("u", 200),
				branches,
				depth,
			),
		},
		{
			name:   "expanded_distinct_namespaces",
			schema: disjointExpandedIdentityPathsBenchmarkSchema,
			doc:    disjointDistinctExpandedIdentityPathsBenchmarkDoc(branches, depth),
		},
		{
			name:   "expanded_distinct_control",
			schema: strings.Replace(disjointExpandedIdentityPathsBenchmarkSchema, `type="xs:ID"`, `type="xs:string"`, 1),
			doc:    disjointDistinctExpandedIdentityPathsBenchmarkDoc(branches, depth),
		},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(tt.schema)))
			if err != nil {
				b.Fatal(err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceDepth: depth + 1})
			if err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(tt.doc)))
			b.ReportAllocs()
			for b.Loop() {
				if err := session.Validate(strings.NewReader(tt.doc)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkSessionValidateExpandedIdentityPaths(b *testing.B) {
	const (
		namespaceBytes = 64 << 10
		rows           = 1000
	)
	namespace := "urn:" + strings.Repeat("u", namespaceBytes-4)
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(expandedIdentityPathsBenchmarkSchema)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	doc := expandedIdentityPathsBenchmarkDoc(namespace, rows)
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSessionValidateSharedExpandedIdentityPrefix(b *testing.B) {
	const (
		commonDepth = 62
		rows        = 10_000
	)
	doc := sharedExpandedIdentityPrefixBenchmarkDoc(commonDepth, rows)
	tests := []struct {
		name   string
		schema string
	}{
		{name: "identity", schema: sharedExpandedIdentityPrefixBenchmarkSchema},
		{
			name:   "control",
			schema: strings.Replace(sharedExpandedIdentityPrefixBenchmarkSchema, `type="xs:ID"`, `type="xs:string"`, 1),
		},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(tt.schema)))
			if err != nil {
				b.Fatal(err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceDepth: commonDepth + 2})
			if err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(len(doc)))
			b.ReportAllocs()
			for b.Loop() {
				if err := session.Validate(strings.NewReader(doc)); err != nil {
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

const nestedIdentitySelectionsBenchmarkSchema = `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="node">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="node" minOccurs="0"/>
      </xs:sequence>
      <xs:attribute name="id" type="xs:string" use="required"/>
    </xs:complexType>
  </xs:element>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element ref="node"/>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="nodeID">
      <xs:selector xpath=".//node"/>
      <xs:field xpath="@id"/>
    </xs:key>
  </xs:element>
</xs:schema>`

const disjointIdentityPathsBenchmarkSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="node"><xs:complexType><xs:sequence><xs:element ref="node" minOccurs="0"/></xs:sequence>
    <xs:attribute name="id" type="xs:ID"/>
  </xs:complexType></xs:element>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:element ref="node" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`

const disjointExpandedIdentityPathsBenchmarkSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T"><xs:sequence><xs:any processContents="lax" minOccurs="0"/></xs:sequence>
    <xs:attribute name="id" type="xs:ID"/>
  </xs:complexType>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:any processContents="lax" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`

const expandedIdentityPathsBenchmarkSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:any processContents="lax" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
    <xs:unique name="values"><xs:selector xpath="*"/><xs:field xpath="."/></xs:unique>
  </xs:element>
</xs:schema>`

const sharedExpandedIdentityPrefixBenchmarkSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="T"><xs:sequence><xs:any processContents="lax" minOccurs="0" maxOccurs="unbounded"/></xs:sequence>
    <xs:attribute name="id" type="xs:ID"/>
  </xs:complexType>
  <xs:element name="root"><xs:complexType><xs:sequence><xs:any processContents="lax" maxOccurs="unbounded"/></xs:sequence></xs:complexType></xs:element>
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

func nestedIdentitySelectionsBenchmarkDoc(depth int) string {
	var b strings.Builder
	b.WriteString("<root>")
	for i := range depth {
		fmt.Fprintf(&b, `<node id="id%d">`, i)
	}
	for range depth {
		b.WriteString("</node>")
	}
	b.WriteString("</root>")
	return b.String()
}

func nestedIdentitySelectionPathBenchmarkDoc(root string, depth int) string {
	var b strings.Builder
	b.Grow(2*len(root) + depth*7 + 5)
	b.WriteByte('<')
	b.WriteString(root)
	b.WriteByte('>')
	for range depth {
		b.WriteString("<n>")
	}
	for range depth {
		b.WriteString("</n>")
	}
	b.WriteString("</")
	b.WriteString(root)
	b.WriteByte('>')
	return b.String()
}

func retainedIdentityPathsBenchmarkDoc(root string, rows int) string {
	var b strings.Builder
	b.Grow(2*len(root) + rows*32 + 5)
	b.WriteByte('<')
	b.WriteString(root)
	b.WriteByte('>')
	for i := range rows {
		fmt.Fprintf(&b, `<row id="id%d"`, i)
		if i > 0 {
			fmt.Fprintf(&b, ` ref="id%d"`, i-1)
		}
		b.WriteString(`/>`)
	}
	b.WriteString("</")
	b.WriteString(root)
	b.WriteByte('>')
	return b.String()
}

func disjointIdentityPathsBenchmarkDoc(branches, depth int) string {
	var b strings.Builder
	b.Grow(branches*depth*13 + 13)
	b.WriteString("<root>")
	for branch := range branches {
		for level := range depth {
			if level == depth-1 {
				fmt.Fprintf(&b, `<node id="id%d">`, branch)
			} else {
				b.WriteString("<node>")
			}
		}
		for range depth {
			b.WriteString("</node>")
		}
	}
	b.WriteString("</root>")
	return b.String()
}

func disjointExpandedIdentityPathsBenchmarkDoc(namespace string, branches, depth int) string {
	var b strings.Builder
	b.Grow(len(namespace) + branches*depth*40 + 96)
	fmt.Fprintf(
		&b,
		`<root xmlns:p="%s" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">`,
		namespace,
	)
	for branch := range branches {
		for level := range depth {
			if level == depth-1 {
				fmt.Fprintf(&b, `<p:n xsi:type="T" id="id%d">`, branch)
			} else {
				b.WriteString(`<p:n xsi:type="T">`)
			}
		}
		for range depth {
			b.WriteString("</p:n>")
		}
	}
	b.WriteString("</root>")
	return b.String()
}

func disjointDistinctExpandedIdentityPathsBenchmarkDoc(branches, depth int) string {
	var b strings.Builder
	b.Grow(branches*depth*52 + 80)
	b.WriteString(`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">`)
	for branch := range branches {
		for level := range depth {
			fmt.Fprintf(&b, `<p:n xmlns:p="urn:b%d:l%d" xsi:type="T"`, branch, level)
			if level == depth-1 {
				fmt.Fprintf(&b, ` id="id%d"`, branch)
			}
			b.WriteByte('>')
		}
		for range depth {
			b.WriteString("</p:n>")
		}
	}
	b.WriteString("</root>")
	return b.String()
}

func sharedExpandedIdentityPrefixBenchmarkDoc(commonDepth, rows int) string {
	var b strings.Builder
	b.Grow(rows*48 + commonDepth*64 + 96)
	b.WriteString(`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:l="urn:leaf"`)
	for level := range commonDepth {
		fmt.Fprintf(&b, ` xmlns:p%d="urn:%02d"`, level, level)
	}
	b.WriteByte('>')
	for level := range commonDepth {
		fmt.Fprintf(&b, `<p%d:n xsi:type="T">`, level)
	}
	for row := range rows {
		fmt.Fprintf(&b, `<l:n xsi:type="T" id="id%d"/>`, row)
	}
	for level := commonDepth - 1; level >= 0; level-- {
		fmt.Fprintf(&b, `</p%d:n>`, level)
	}
	b.WriteString(`</root>`)
	return b.String()
}

func expandedIdentityPathsBenchmarkDoc(namespace string, rows int) string {
	var b strings.Builder
	b.Grow(len(namespace) + rows*64 + 160)
	fmt.Fprintf(
		&b,
		`<root xmlns:p="%s" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema">`,
		namespace,
	)
	for row := range rows {
		fmt.Fprintf(&b, `<p:a xsi:type="xs:string">value%d</p:a>`, row)
	}
	b.WriteString("</root>")
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

func BenchmarkSessionValidateUndeclaredRootXSIType(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("root-xsi.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)))
	if err != nil {
		b.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		b.Fatal(err)
	}
	const doc = `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" xsi:type="xs:string">value</root>`
	b.SetBytes(int64(len(doc)))
	b.ReportAllocs()
	for b.Loop() {
		if err := session.Validate(strings.NewReader(doc)); err != nil {
			b.Fatal(err)
		}
	}
}

const slotSchemaHeader = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:slots" targetNamespace="urn:slots" elementFormDefault="qualified" attributeFormDefault="unqualified">`
const slotRootOpen = `<t:r xmlns:t="urn:slots"`

type slotOpponent struct{ name, document string }
type slotFixture struct {
	name, schema, document string
	accepted               []string
	opponents              []slotOpponent
}

type slotUseMode uint8

const (
	slotOptional slotUseMode = iota
	slotRequired
)

func slotAttributes(useMode slotUseMode) string {
	var out strings.Builder
	for i := range 128 {
		mode := ""
		if useMode == slotRequired {
			mode = ` use="required"`
		}
		fmt.Fprintf(&out, `<xs:attribute name="a%d" type="xs:string"%s/>`, i, mode)
	}
	return out.String()
}

func slotDocumentAttributes(omit int) string {
	var out strings.Builder
	for i := range 128 {
		if i != omit {
			fmt.Fprintf(&out, ` a%d="v%d"`, i, i)
		}
	}
	return out.String()
}

func attributeSlotFixtures() []slotFixture {
	wrap := func(content string) string { return slotSchemaHeader + content + `</xs:schema>` }
	element := func(attributes string) string {
		return wrap(`<xs:element name="r"><xs:complexType>` + attributes + `</xs:complexType></xs:element>`)
	}
	all := slotDocumentAttributes(-1)
	fixtures := []slotFixture{
		{name: "None", schema: element(""), document: slotRootOpen + `/>`, opponents: []slotOpponent{{"unknown", slotRootOpen + ` extra="x"/>`}}},
		{name: "OneRequired", schema: element(`<xs:attribute name="a0" type="xs:string" use="required"/>`), document: slotRootOpen + ` a0="v0"/>`, opponents: []slotOpponent{{"missing", slotRootOpen + `/>`}}},
		{name: "ManyOptional128", schema: element(slotAttributes(slotOptional)), document: slotRootOpen + all + `/>`, accepted: []string{slotRootOpen + `/>`}, opponents: []slotOpponent{{"unknown128", slotRootOpen + all + ` a128="extra"/>`}}},
		{name: "Required128", schema: element(slotAttributes(slotRequired)), document: slotRootOpen + all + `/>`, opponents: []slotOpponent{{"missing64", slotRootOpen + slotDocumentAttributes(64) + `/>`}, {"missing127", slotRootOpen + slotDocumentAttributes(127) + `/>`}}},
	}
	var constrained strings.Builder
	for i := range 128 {
		facet, literal := "default", fmt.Sprintf("d%d", i)
		if i%2 != 0 {
			facet, literal = "fixed", fmt.Sprintf("f%d", i)
		}
		if i == 0 {
			literal = ""
		}
		fmt.Fprintf(&constrained, `<xs:attribute name="a%d" type="xs:string" %s="%s"/>`, i, facet, literal)
	}
	constraintSchema := wrap(`<xs:element name="r"><xs:complexType><xs:sequence><xs:element name="row" maxOccurs="2"><xs:complexType>` + constrained.String() + `</xs:complexType></xs:element></xs:sequence></xs:complexType><xs:key name="defaultsAreValues"><xs:selector xpath="t:row"/><xs:field xpath="@a0"/></xs:key></xs:element>`)
	fixtures = append(fixtures, slotFixture{name: "Constraints128", schema: constraintSchema, document: slotRootOpen + `><t:row/></t:r>`, accepted: []string{slotRootOpen + `><t:row/><t:row a0="other"/></t:r>`}, opponents: []slotOpponent{{"duplicateDefault", slotRootOpen + `><t:row/><t:row/></t:r>`}, {"fixedMismatch", slotRootOpen + `><t:row a1="wrong"/></t:r>`}}})
	nested := wrap(`<xs:attributeGroup name="G0"><xs:attribute name="a0" type="xs:string" use="required"/><xs:attribute name="a1" type="xs:string" default="d1"/></xs:attributeGroup><xs:attributeGroup name="G1"><xs:attributeGroup ref="t:G0"/><xs:attribute name="a2" type="xs:string" use="required"/></xs:attributeGroup><xs:attributeGroup name="G2"><xs:attributeGroup ref="t:G1"/><xs:attribute name="a3" type="xs:string"/></xs:attributeGroup><xs:complexType name="B"><xs:attributeGroup ref="t:G2"/><xs:attribute name="gone" type="xs:string"/></xs:complexType><xs:complexType name="R"><xs:complexContent><xs:restriction base="t:B"><xs:attributeGroup ref="t:G2"/><xs:attribute name="gone" use="prohibited"/></xs:restriction></xs:complexContent></xs:complexType><xs:element name="r" type="t:R"/>`)
	fixtures = append(fixtures, slotFixture{name: "NestedRestriction", schema: nested, document: slotRootOpen + ` a0="v0" a2="v2"/>`, opponents: []slotOpponent{{"missing0", slotRootOpen + ` a2="v2"/>`}, {"missing2", slotRootOpen + ` a0="v0"/>`}, {"prohibited", slotRootOpen + ` a0="v0" a2="v2" gone="x"/>`}}})
	var reuse strings.Builder
	reuse.WriteString(`<xs:attributeGroup name="G">` + slotAttributes(slotRequired) + `</xs:attributeGroup>`)
	for i := range 32 {
		fmt.Fprintf(&reuse, `<xs:element name="e%d"><xs:complexType><xs:attributeGroup ref="t:G"/></xs:complexType></xs:element>`, i)
	}
	reuse.WriteString(`<xs:element name="r"><xs:complexType><xs:choice maxOccurs="unbounded">`)
	for i := range 32 {
		fmt.Fprintf(&reuse, `<xs:element ref="t:e%d"/>`, i)
	}
	reuse.WriteString(`</xs:choice></xs:complexType></xs:element>`)
	fixtures = append(fixtures, slotFixture{name: "SoleGroupReuse128", schema: wrap(reuse.String()), document: slotRootOpen + `><t:e0` + all + `/><t:e31` + all + `/></t:r>`, opponents: []slotOpponent{{"missing127", slotRootOpen + `><t:e31` + slotDocumentAttributes(127) + `/></t:r>`}}})
	return fixtures
}

func checkAttributeSlotFixture(tb testing.TB, fixture slotFixture) *xsd.Session {
	tb.Helper()
	if len(fixture.schema) > 96<<10 || len(fixture.document) > 32<<10 {
		tb.Fatal("fixture byte bound")
	}
	engine, err := xsd.Compile(xsd.Bytes("attribute-slots.xsd", []byte(fixture.schema)))
	if err != nil || engine == nil {
		tb.Fatalf("%s compile engine=%v error=%v", fixture.name, engine, err)
	}
	session, sessionErr := engine.NewSession(xsd.ValidateOptions{})
	if sessionErr != nil {
		tb.Fatal(sessionErr)
	}
	valid := append([]string{fixture.document}, fixture.accepted...)
	for _, document := range valid {
		if len(document) > 32<<10 {
			tb.Fatal("document byte bound")
		}
		if validationErr := session.Validate(strings.NewReader(document)); validationErr != nil {
			tb.Fatalf("%s accepted document: %v", fixture.name, validationErr)
		}
	}
	for _, opponent := range fixture.opponents {
		if len(opponent.document) > 32<<10 {
			tb.Fatal("opponent byte bound")
		}
		validationErr := session.Validate(strings.NewReader(opponent.document))
		if validationErr == nil || !xsderrors.IsConclusiveValidation(validationErr) {
			tb.Fatalf("%s/%s requires rejection: %v", fixture.name, opponent.name, validationErr)
		}
		failures := xsderrors.Flatten(validationErr)
		if len(failures) == 0 {
			tb.Fatal("missing structured diagnostics")
		}
		for _, failure := range failures {
			diagnostic, ok := errors.AsType[*xsderrors.Error](failure)
			if !ok || diagnostic.Category() != xsderrors.CategoryValidation || diagnostic.Code() == "" {
				tb.Fatalf("%s/%s diagnostic=%v", fixture.name, opponent.name, failure)
			}
			wantCode := xsderrors.CodeValidationAttribute
			if fixture.name == "Constraints128" && opponent.name == "duplicateDefault" {
				wantCode = xsderrors.CodeValidationIdentity
			}
			if diagnostic.Code() != wantCode {
				tb.Fatalf("%s/%s diagnostic code = %s, want %s", fixture.name, opponent.name, diagnostic.Code(), wantCode)
			}
		}
		if recoveryErr := session.Validate(strings.NewReader(fixture.document)); recoveryErr != nil {
			tb.Fatalf("same-session %s recovery: %v", fixture.name, recoveryErr)
		}
	}
	return session
}

func BenchmarkCompileAttributeSlots(b *testing.B) {
	for _, fixture := range attributeSlotFixtures() {
		b.Run(fixture.name, func(b *testing.B) {
			checkAttributeSlotFixture(b, fixture)
			source := []byte(fixture.schema)
			b.ReportAllocs()
			for b.Loop() {
				engine, err := xsd.Compile(xsd.Bytes("attribute-slots.xsd", source))
				if err != nil || engine == nil {
					b.Fatalf("compile engine=%v error=%v", engine, err)
				}
			}
		})
	}
}

func BenchmarkSessionAttributeSlots(b *testing.B) {
	for _, fixture := range attributeSlotFixtures() {
		b.Run(fixture.name, func(b *testing.B) {
			session := checkAttributeSlotFixture(b, fixture)
			reader := strings.NewReader(fixture.document)
			b.ReportAllocs()
			for b.Loop() {
				reader.Reset(fixture.document)
				if err := session.Validate(reader); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

const floatCompositionSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="Enumeration"><xs:restriction base="FLOAT_KIND"><xs:enumeration value="1"/><xs:enumeration value="1.5"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Bounded"><xs:restriction base="FLOAT_KIND"><xs:minInclusive value="0"/><xs:maxExclusive value="10"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="Earlier"><xs:restriction base="xs:decimal"><xs:minInclusive value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="FloatList"><xs:list itemType="FLOAT_KIND"/></xs:simpleType>
  <xs:simpleType name="FloatUnion"><xs:union memberTypes="Earlier FLOAT_KIND"/></xs:simpleType>
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="enum" type="Enumeration"/>
    <xs:element name="bound" type="Bounded"/>
    <xs:element name="fixed" type="FLOAT_KIND" fixed="1"/>
    <xs:element name="list" type="FloatList"/>
    <xs:element name="union" type="FloatUnion"/>
    <xs:element name="row"><xs:complexType><xs:attribute name="value" type="FLOAT_KIND" use="required"/></xs:complexType></xs:element>
    <xs:element name="ref"><xs:complexType><xs:attribute name="value" type="FLOAT_KIND" use="required"/></xs:complexType></xs:element>
  </xs:sequence></xs:complexType>
    <xs:key name="ids"><xs:selector xpath="row"/><xs:field xpath="@value"/></xs:key>
    <xs:keyref name="refs" refer="ids"><xs:selector xpath="ref"/><xs:field xpath="@value"/></xs:keyref>
  </xs:element>
</xs:schema>`

const floatCompositionDocument = `<root><enum>1.5</enum><bound>2</bound><fixed>1.0</fixed><list>1 1.5</list><union>1.5</union><row value="1.00"/><ref value="1E0"/></root>`

type floatCompositionOpponent struct {
	document string
	code     xsderrors.Code
}

type floatCompositionFixture struct {
	name, schema string
	rejected     []floatCompositionOpponent
}

func floatCompositionFixtures(tb testing.TB) [2]floatCompositionFixture {
	tb.Helper()
	var fixtures [2]floatCompositionFixture
	for i, entry := range []struct{ name, kind string }{{"Float32", "xs:float"}, {"Float64", "xs:double"}} {
		fixture := floatCompositionFixture{name: entry.name, schema: strings.ReplaceAll(floatCompositionSchema, "FLOAT_KIND", entry.kind)}
		if len(fixture.schema) > 8<<10 || len(floatCompositionDocument) > 1<<10 {
			tb.Fatal("floating fixture exceeds source/document bounds")
		}
		for _, opponent := range []struct {
			old, new string
			code     xsderrors.Code
		}{
			{"<enum>1.5</enum>", "<enum>2</enum>", xsderrors.CodeValidationFacet},
			{"<bound>2</bound>", "<bound>10</bound>", xsderrors.CodeValidationFacet},
			{"<fixed>1.0</fixed>", "<fixed>2</fixed>", xsderrors.CodeValidationElement},
			{"<list>1 1.5</list>", "<list>1 INX</list>", xsderrors.CodeValidationFacet},
			{"<union>1.5</union>", "<union>INX</union>", xsderrors.CodeValidationFacet},
			{`<ref value="1E0"/>`, `<ref value="2"/>`, xsderrors.CodeValidationIdentity},
		} {
			if strings.Count(floatCompositionDocument, opponent.old) != 1 {
				tb.Fatal("opponent anchor is not unique")
			}
			fixture.rejected = append(fixture.rejected, floatCompositionOpponent{strings.Replace(floatCompositionDocument, opponent.old, opponent.new, 1), opponent.code})
		}
		fixtures[i] = fixture
	}
	return fixtures
}

func checkFloatCompositionRejection(tb testing.TB, err error, code xsderrors.Code) {
	tb.Helper()
	diagnostics := xsderrors.Flatten(err)
	if len(diagnostics) == 0 {
		tb.Fatal("invalid floating composition accepted")
	}
	for _, diagnostic := range diagnostics {
		item, ok := errors.AsType[*xsderrors.Error](diagnostic)
		if !ok || item.Category() != xsderrors.CategoryValidation || item.Code() != code {
			tb.Fatalf("floating opponent error=%v; provisional expected validation/%s", diagnostic, code)
		}
	}
}

func checkFloatCompositionFixture(tb testing.TB, session *xsd.Session, fixture floatCompositionFixture) {
	tb.Helper()
	if err := session.Validate(strings.NewReader(floatCompositionDocument)); err != nil {
		tb.Fatalf("valid %s composition: %v", fixture.name, err)
	}
	for _, opponent := range fixture.rejected {
		err := session.Validate(strings.NewReader(opponent.document))
		checkFloatCompositionRejection(tb, err, opponent.code)
		if err := session.Validate(strings.NewReader(floatCompositionDocument)); err != nil {
			tb.Fatalf("same-session recovery: %v", err)
		}
	}
}

func newFloatCompositionSession(tb testing.TB, engine *xsd.Engine) *xsd.Session {
	tb.Helper()
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		tb.Fatal(err)
	}
	return session
}

func BenchmarkFloatCanonicalComposition(b *testing.B) {
	for _, fixture := range floatCompositionFixtures(b) {
		b.Run("Compile"+fixture.name+"EnumBounds", func(b *testing.B) {
			source := xsd.Bytes("float-composition.xsd", []byte(fixture.schema))
			engine, err := xsd.Compile(source)
			if err != nil || engine == nil {
				b.Fatalf("Compile: engine=%v error=%v", engine, err)
			}
			checkFloatCompositionFixture(b, newFloatCompositionSession(b, engine), fixture)
			b.ReportAllocs()
			for b.Loop() {
				engine, err = xsd.Compile(source)
				if err != nil || engine == nil {
					b.Fatalf("Compile: engine=%v error=%v", engine, err)
				}
			}
			checkFloatCompositionFixture(b, newFloatCompositionSession(b, engine), fixture)
		})
		b.Run("Session"+fixture.name+"Composition", func(b *testing.B) {
			engine, err := xsd.Compile(xsd.Bytes("float-composition.xsd", []byte(fixture.schema)))
			if err != nil || engine == nil {
				b.Fatalf("Compile: engine=%v error=%v", engine, err)
			}
			session := newFloatCompositionSession(b, engine)
			checkFloatCompositionFixture(b, session, fixture)
			reader := strings.NewReader(floatCompositionDocument)
			b.ReportAllocs()
			for b.Loop() {
				reader.Reset(floatCompositionDocument)
				if err := session.Validate(reader); err != nil {
					b.Fatal(err)
				}
			}
			checkFloatCompositionFixture(b, session, fixture)
		})
	}
}

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
