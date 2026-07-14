package xsd_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestOpenDefersAndBoundsSchemaReadUntilCompile(t *testing.T) {
	t.Parallel()

	reads := 0
	src := xsd.Open("schema.xsd", func() (io.ReadCloser, error) {
		return io.NopCloser(countingReader{
			read: func(p []byte) (int, error) {
				reads += len(p)
				for i := range p {
					p[i] = 'x'
				}
				return len(p), nil
			},
		}), nil
	})
	if reads != 0 {
		t.Fatalf("Open() consumed %d bytes before compilation", reads)
	}
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSchemaSourceBytes: 8}, src)
	expectCategoryCode(t, err, xsderrors.CategorySchemaCompile, xsderrors.CodeSchemaLimit)
	if reads != 9 {
		t.Fatalf("CompileWithOptions() consumed %d bytes, want limit plus one", reads)
	}
}

func TestOpenRejectsRepeatedEmptyReadsAndCloses(t *testing.T) {
	t.Parallel()

	reader := &emptySchemaReadCloser{terminal: errors.New("unbounded empty reads")}
	_, err := xsd.CompileWithOptions(
		xsd.CompileOptions{MaxSchemaSourceBytes: 1},
		xsd.Open("schema.xsd", func() (io.ReadCloser, error) { return reader, nil }),
	)
	expectCategoryCode(t, err, xsderrors.CategorySchemaParse, xsderrors.CodeSchemaRead)
	if reader.reads > 100 || !reader.closed {
		t.Fatalf("reader = %d empty reads, closed %v; want bounded and closed", reader.reads, reader.closed)
	}
}

func TestFileResolvesPercentEncodedSchemaLocation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root := filepath.Join(dir, "root.xsd")
	child := filepath.Join(dir, "child name.xsd")
	writeTestFile(t, root, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child%20name.xsd"/></xs:schema>`)
	writeTestFile(t, child, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="child"/></xs:schema>`)
	engine, err := xsd.Compile(xsd.File(root))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := engine.Validate(strings.NewReader(`<child/>`)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidateUnsupportedXMLDeclarationClassificationDoesNotDependOnPreviewLength(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		content string
		code    xsderrors.Code
	}{
		{name: "version", content: `version="1.1"`, code: xsderrors.CodeUnsupportedXML11},
		{name: "encoding", content: `version="1.0" encoding="ISO-8859-1"`, code: xsderrors.CodeUnsupportedNonUTF8},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doc := `<?xml` + strings.Repeat(" ", 70<<10) + test.content + `?><root/>`
			expectCategoryCode(t, engine.Validate(strings.NewReader(doc)), xsderrors.CategoryUnsupported, test.code)
		})
	}
}

func TestResolverReceivesInheritedXMLBase(t *testing.T) {
	t.Parallel()

	var gotBase, gotLocation string
	resolver := xsd.ResolverFunc(func(base, location string) (xsd.SchemaSource, error) {
		gotBase, gotLocation = base, location
		return xsd.Bytes("ignored.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="child"/></xs:schema>`)), nil
	})
	root := xsd.Bytes("schemas/root.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xml:base="sub/"><xs:include xml:base="nested/" schemaLocation="child.xsd"/></xs:schema>`)).WithResolver(resolver)
	engine, err := xsd.Compile(root)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	wantBase := filepath.Join("schemas", "sub", "nested") + string(filepath.Separator)
	if gotBase != wantBase || gotLocation != "child.xsd" {
		t.Fatalf("resolver arguments = (%q, %q), want (%q, child.xsd)", gotBase, gotLocation, wantBase)
	}
	if err := engine.Validate(strings.NewReader(`<child/>`)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestIdentityFieldsRejectComplexElements(t *testing.T) {
	for _, test := range []struct {
		name     string
		element  string
		instance string
	}{
		{
			name:     "mixed fixed text",
			element:  `<xs:element name="item" type="Mixed" fixed="alpha"/>`,
			instance: `<root><item>alpha</item></root>`,
		},
		{
			name:     "nilled mixed complex",
			element:  `<xs:element name="item" type="Mixed" nillable="true"/>`,
			instance: `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><item xsi:nil="true"/></root>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
			  <xs:complexType name="Mixed" mixed="true"/>
			  <xs:element name="root">
			    <xs:complexType><xs:sequence>` + test.element + `</xs:sequence></xs:complexType>
			    <xs:unique name="u"><xs:selector xpath="item"/><xs:field xpath="."/></xs:unique>
			  </xs:element>
			</xs:schema>`
			engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			expectCategoryCode(t, engine.Validate(strings.NewReader(test.instance)), xsderrors.CategoryValidation, xsderrors.CodeValidationIdentity)
		})
	}
}

func TestIdentityConstraintValueSpaceRules(t *testing.T) {
	t.Run("duration equality", func(t *testing.T) {
		const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="item" type="xs:duration" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
    <xs:unique name="durations"><xs:selector xpath="item"/><xs:field xpath="."/></xs:unique>
  </xs:element>
</xs:schema>`
		engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		expectCategoryCode(
			t,
			engine.Validate(strings.NewReader(`<root><item>P1Y</item><item>P12M</item></root>`)),
			xsderrors.CategoryValidation,
			xsderrors.CodeValidationIdentity,
		)
	})

	t.Run("duration list equality", func(t *testing.T) {
		const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="durations"><xs:list itemType="xs:duration"/></xs:simpleType>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="item" maxOccurs="unbounded"><xs:complexType><xs:attribute name="v" type="durations"/></xs:complexType></xs:element></xs:sequence></xs:complexType>
    <xs:unique name="values"><xs:selector xpath="item"/><xs:field xpath="@v"/></xs:unique>
  </xs:element>
</xs:schema>`
		engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		expectCategoryCode(
			t,
			engine.Validate(strings.NewReader(`<root><item v="P1Y"/><item v="P12M"/></root>`)),
			xsderrors.CategoryValidation,
			xsderrors.CodeValidationIdentity,
		)
	})

	t.Run("duration list keyref equality", func(t *testing.T) {
		const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="durations"><xs:list itemType="xs:duration"/></xs:simpleType>
  <xs:complexType name="rows"><xs:sequence><xs:element name="item" maxOccurs="unbounded"><xs:complexType><xs:attribute name="v" type="durations"/></xs:complexType></xs:element></xs:sequence></xs:complexType>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="keys" type="rows"/><xs:element name="refs" type="rows"/></xs:sequence></xs:complexType>
    <xs:key name="values"><xs:selector xpath="keys/item"/><xs:field xpath="@v"/></xs:key>
    <xs:keyref name="references" refer="values"><xs:selector xpath="refs/item"/><xs:field xpath="@v"/></xs:keyref>
  </xs:element>
</xs:schema>`
		engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		if err := engine.Validate(strings.NewReader(`<root><keys><item v="P1Y"/></keys><refs><item v="P12M"/></refs></root>`)); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	})

	t.Run("duration list union equality", func(t *testing.T) {
		const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="durations"><xs:list itemType="xs:duration"/></xs:simpleType>
  <xs:simpleType name="durationUnion"><xs:union memberTypes="durations"/></xs:simpleType>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="item" type="durationUnion" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
    <xs:unique name="values"><xs:selector xpath="item"/><xs:field xpath="."/></xs:unique>
  </xs:element>
</xs:schema>`
		engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		expectCategoryCode(
			t,
			engine.Validate(strings.NewReader(`<root><item>P1Y</item><item>P12M</item></root>`)),
			xsderrors.CategoryValidation,
			xsderrors.CodeValidationIdentity,
		)
	})

	t.Run("key rejects nillable declaration", func(t *testing.T) {
		const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="row"><xs:complexType><xs:sequence>
      <xs:element name="code" type="xs:string" nillable="true"/>
    </xs:sequence></xs:complexType></xs:element></xs:sequence></xs:complexType>
    <xs:key name="codes"><xs:selector xpath="row"/><xs:field xpath="code"/></xs:key>
  </xs:element>
</xs:schema>`
		engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		for _, doc := range []string{
			`<root><row><code>present</code></row></root>`,
			`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><row><code xsi:nil="true"/></row></root>`,
		} {
			err = engine.Validate(strings.NewReader(doc))
			expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationIdentity)
			if !strings.Contains(err.Error(), "key field selects nillable element declaration") {
				t.Fatalf("Validate(%s) error = %v, want nillable key-field error", doc, err)
			}
		}
	})

	t.Run("unique permits nillable declaration", func(t *testing.T) {
		const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="row"><xs:complexType><xs:sequence>
      <xs:element name="code" type="xs:string" nillable="true"/>
    </xs:sequence></xs:complexType></xs:element></xs:sequence></xs:complexType>
    <xs:unique name="codes"><xs:selector xpath="row"/><xs:field xpath="code"/></xs:unique>
  </xs:element>
</xs:schema>`
		engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		if err := engine.Validate(strings.NewReader(`<root><row><code>present</code></row></root>`)); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
	})

	t.Run("failed assessment suppresses nillable key error", func(t *testing.T) {
		const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="code" type="xs:int" nillable="true"/></xs:sequence></xs:complexType>
    <xs:key name="codes"><xs:selector xpath="."/><xs:field xpath="code"/></xs:key>
  </xs:element>
</xs:schema>`
		engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
		if err != nil {
			t.Fatalf("Compile() error = %v", err)
		}
		for _, doc := range []string{
			`<root><code>not-an-int</code></root>`,
			`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><code xsi:nil="bogus">1</code></root>`,
		} {
			err = engine.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{MaxErrors: 0})
			if err == nil {
				t.Fatalf("Validate(%s) succeeded", doc)
			}
			if errorTreeContains(err, "key field selects nillable element declaration") {
				t.Fatalf("Validate(%s) error = %v, want no secondary nillable-key error", doc, err)
			}
		}

		const attributeSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="code"><xs:simpleContent><xs:extension base="xs:int"><xs:attribute name="required" use="required"/></xs:extension></xs:simpleContent></xs:complexType>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="code" type="code" nillable="true"/></xs:sequence></xs:complexType>
    <xs:key name="codes"><xs:selector xpath="."/><xs:field xpath="code"/></xs:key>
  </xs:element>
</xs:schema>`
		attributeEngine, err := xsd.Compile(xsd.Bytes("attribute-schema.xsd", []byte(attributeSchema)))
		if err != nil {
			t.Fatalf("Compile(attribute schema) error = %v", err)
		}
		err = attributeEngine.ValidateWithOptions(strings.NewReader(`<root><code>1</code></root>`), xsd.ValidateOptions{MaxErrors: 0})
		if err == nil {
			t.Fatal("Validate(missing required attribute) succeeded")
		}
		if errorTreeContains(err, "key field selects nillable element declaration") {
			t.Fatalf("Validate(missing required attribute) error = %v, want no secondary nillable-key error", err)
		}

		const complexSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="code" nillable="true"><xs:complexType><xs:sequence>
      <xs:element name="value" type="xs:int"/>
    </xs:sequence></xs:complexType></xs:element></xs:sequence></xs:complexType>
    <xs:key name="codes"><xs:selector xpath="."/><xs:field xpath="code"/></xs:key>
  </xs:element>
</xs:schema>`
		complexEngine, err := xsd.Compile(xsd.Bytes("complex-schema.xsd", []byte(complexSchema)))
		if err != nil {
			t.Fatalf("Compile(complex schema) error = %v", err)
		}
		for _, doc := range []string{
			`<root><code><value>1</value></code></root>`,
			`<root><code><value>not-an-int</value></code></root>`,
		} {
			err = complexEngine.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{MaxErrors: 0})
			if err == nil || !errorTreeContains(err, "identity field has no simple value") {
				t.Fatalf("Validate(%s) error = %v, want complex-field identity error", doc, err)
			}
			if errorTreeContains(err, "key field selects nillable element declaration") {
				t.Fatalf("Validate(%s) error = %v, want no nillable-key error for an unqualified sequence", doc, err)
			}
		}

		const incompleteKeySchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="row"><xs:complexType><xs:sequence>
      <xs:element name="code" type="xs:string" nillable="true" minOccurs="0"/>
      <xs:element name="other" type="xs:string" minOccurs="0"/>
    </xs:sequence></xs:complexType></xs:element></xs:sequence></xs:complexType>
    <xs:key name="codes"><xs:selector xpath="row"/><xs:field xpath="code"/><xs:field xpath="other"/></xs:key>
  </xs:element>
</xs:schema>`
		incompleteKeyEngine, err := xsd.Compile(xsd.Bytes("incomplete-key-schema.xsd", []byte(incompleteKeySchema)))
		if err != nil {
			t.Fatalf("Compile(incomplete key schema) error = %v", err)
		}
		err = incompleteKeyEngine.ValidateWithOptions(strings.NewReader(`<root><row><code>present</code></row></root>`), xsd.ValidateOptions{MaxErrors: 0})
		if err == nil || !errorTreeContains(err, "key field is missing") {
			t.Fatalf("Validate(incomplete key) error = %v, want missing-field error", err)
		}
		if errorTreeContains(err, "key field selects nillable element declaration") {
			t.Fatalf("Validate(incomplete key) error = %v, want no nillable-key error for an unqualified sequence", err)
		}
	})
}

func errorTreeContains(err error, text string) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), text) {
		return true
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range many.Unwrap() {
			if errorTreeContains(child, text) {
				return true
			}
		}
	}
	return false
}

func errorTreeLeafCount(err error) int {
	if err == nil {
		return 0
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		count := 0
		for _, child := range many.Unwrap() {
			count += errorTreeLeafCount(child)
		}
		return count
	}
	return 1
}

func TestIdentityCaptureDoesNotDuplicateXSIStartDiagnostics(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <xs:element name="root" nillable="true">
    <xs:complexType><xs:sequence>
      <xs:element name="child" type="xs:string" nillable="true" minOccurs="0"/>
      <xs:element name="number" type="xs:int" minOccurs="0"/>
    </xs:sequence></xs:complexType>
    <xs:unique name="nilValue"><xs:selector xpath="."/><xs:field xpath="@xsi:nil"/></xs:unique>
    <xs:unique name="typeValue"><xs:selector xpath="."/><xs:field xpath="@xsi:type"/></xs:unique>
  </xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	tests := []struct {
		name string
		doc  string
		text string
	}{
		{name: "selected nil", doc: `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="bogus"/>`, text: "invalid xsi:nil value"},
		{name: "unselected nil", doc: `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><child xsi:nil="bogus"/></root>`, text: "invalid xsi:nil value"},
		{name: "selected type", doc: `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="p:Missing"/>`, text: "xsi:type"},
		{name: "unselected type", doc: `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><child xsi:type="p:Missing"/></root>`, text: "xsi:type"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			validationErr := engine.ValidateWithOptions(strings.NewReader(test.doc), xsd.ValidateOptions{MaxErrors: 0})
			if validationErr == nil || !errorTreeContains(validationErr, test.text) {
				t.Fatalf("Validate() error = %v, want %q", validationErr, test.text)
			}
			if count := errorTreeLeafCount(validationErr); count != 1 {
				t.Fatalf("Validate() errors = %d, want 1: %v", count, validationErr)
			}
		})
	}

	err = engine.ValidateWithOptions(
		strings.NewReader(`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="bogus"><number>not-an-int</number></root>`),
		xsd.ValidateOptions{MaxErrors: 2},
	)
	if count := errorTreeLeafCount(err); count != 2 {
		t.Fatalf("Validate(MaxErrors=2) errors = %d, want 2: %v", count, err)
	}
	if !errorTreeContains(err, "invalid simple content") {
		t.Fatalf("Validate(MaxErrors=2) error = %v, want later facet diagnostic", err)
	}
}

func TestXSIHintIdentityKeysMatchOrdinaryAnyURIValues(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <xs:simpleType name="URIs"><xs:list itemType="xs:anyURI"/></xs:simpleType>
  <xs:element name="root">
    <xs:complexType><xs:sequence>
      <xs:element name="source"><xs:complexType/></xs:element>
      <xs:element name="ref" minOccurs="0"><xs:complexType>
        <xs:attribute name="single" type="xs:anyURI" use="required"/>
        <xs:attribute name="many" type="URIs" use="required"/>
      </xs:complexType></xs:element>
    </xs:sequence></xs:complexType>
    <xs:key name="noNamespace"><xs:selector xpath="source"/><xs:field xpath="@xsi:noNamespaceSchemaLocation"/></xs:key>
    <xs:keyref name="noNamespaceRef" refer="noNamespace"><xs:selector xpath="ref"/><xs:field xpath="@single"/></xs:keyref>
    <xs:key name="locations"><xs:selector xpath="source"/><xs:field xpath="@xsi:schemaLocation"/></xs:key>
    <xs:keyref name="locationsRef" refer="locations"><xs:selector xpath="ref"/><xs:field xpath="@many"/></xs:keyref>
  </xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	const prefix = `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <source xsi:noNamespaceSchemaLocation="&#x9;one.xsd&#xA;" xsi:schemaLocation="urn:a&#x9;a.xsd&#xA;urn:b  b.xsd"/>`
	err = engine.Validate(strings.NewReader(prefix + `
  <ref single="one.xsd" many="urn:a a.xsd urn:b b.xsd"/>
</root>`))
	if err != nil {
		t.Fatalf("Validate(equivalent anyURI values) error = %v", err)
	}

	err = engine.Validate(strings.NewReader(prefix + `
  <ref single="one.xsd" many="urn:a a.xsd urn:b different.xsd"/>
</root>`))
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationIdentity)

	err = engine.Validate(strings.NewReader(`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <source xsi:noNamespaceSchemaLocation="&#x9; " xsi:schemaLocation="urn:a a.xsd urn:b b.xsd"/>
  <ref single="" many="urn:a a.xsd urn:b b.xsd"/>
</root>`))
	if err != nil {
		t.Fatalf("Validate(empty noNamespaceSchemaLocation) error = %v", err)
	}

	for _, test := range []struct {
		name string
		doc  string
		text string
	}{
		{
			name: "invalid schemaLocation item",
			doc:  `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><source xsi:noNamespaceSchemaLocation="one.xsd" xsi:schemaLocation="urn:a %zz"/></root>`,
			text: "invalid xsi:schemaLocation URI %zz",
		},
		{
			name: "invalid noNamespaceSchemaLocation",
			doc:  `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><source xsi:noNamespaceSchemaLocation="http://[bad]/" xsi:schemaLocation="urn:a a.xsd"/></root>`,
			text: "invalid xsi:noNamespaceSchemaLocation URI http://[bad]/",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := engine.ValidateWithOptions(strings.NewReader(test.doc), xsd.ValidateOptions{MaxErrors: 10})
			expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationAttribute)
			if count := errorTreeLeafCount(err); count != 1 || !errorTreeContains(err, test.text) {
				t.Fatalf("Validate() error = %v, want one %q diagnostic", err, test.text)
			}
		})
	}
}

func TestXSIStartAssessmentIsAttributeOrderIndependent(t *testing.T) {
	tests := []struct {
		name        string
		schema      string
		documents   [2]string
		diagnostics []string
	}{
		{
			name: "valid nil survives invalid type",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string" nillable="true"/>
</xs:schema>`,
			documents: [2]string{
				`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true" xsi:type="p:Missing">value</root>`,
				`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="p:Missing" xsi:nil="true">value</root>`,
			},
			diagnostics: []string{"xsi:type", "nilled element must be empty"},
		},
		{
			name: "valid type survives invalid nil",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:decimal" nillable="true"/>
</xs:schema>`,
			documents: [2]string{
				`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" xsi:nil="bogus" xsi:type="xs:int">1.5</root>`,
				`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema" xsi:type="xs:int" xsi:nil="bogus">1.5</root>`,
			},
			diagnostics: []string{"invalid xsi:nil value", "invalid simple content"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(test.schema)))
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{MaxErrors: 2})
			if err != nil {
				t.Fatalf("NewSession() error = %v", err)
			}
			var got [2]error
			for i, document := range test.documents {
				got[i] = session.Validate(strings.NewReader(document))
				if count := errorTreeLeafCount(got[i]); count != len(test.diagnostics) {
					t.Fatalf("Validate(document %d) errors = %d, want %d: %v", i, count, len(test.diagnostics), got[i])
				}
				for _, diagnostic := range test.diagnostics {
					if !errorTreeContains(got[i], diagnostic) {
						t.Fatalf("Validate(document %d) error = %v, want %q", i, got[i], diagnostic)
					}
				}
			}
			if got[0].Error() != got[1].Error() {
				t.Fatalf("attribute permutations produced different diagnostics:\nfirst:  %v\nsecond: %v", got[0], got[1])
			}
		})
	}
}

func TestOwnedIdentityFailureSuppressesOuterNillableKeyError(t *testing.T) {
	const selfKeySchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string" nillable="true">
    <xs:key name="self"><xs:selector xpath="."/><xs:field xpath="."/></xs:key>
  </xs:element>
</xs:schema>`
	selfKeyEngine, compileErr := xsd.Compile(xsd.Bytes("self-key-schema.xsd", []byte(selfKeySchema)))
	if compileErr != nil {
		t.Fatalf("Compile(self-key schema) error = %v", compileErr)
	}
	for _, document := range []string{
		`<root>value</root>`,
		`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"/>`,
	} {
		validationErr := selfKeyEngine.Validate(strings.NewReader(document))
		if !errorTreeContains(validationErr, "key field selects nillable element declaration") {
			t.Fatalf("Validate(self key %s) error = %v, want nillable-key error", document, validationErr)
		}
	}

	const schemaTemplate = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence>
      <xs:element name="code" nillable="true">
        <xs:complexType><xs:simpleContent><xs:extension base="xs:string">
          <xs:attribute name="id" type="xs:string"/>
        </xs:extension></xs:simpleContent></xs:complexType>
        <xs:key name="inner"><xs:selector xpath="."/><xs:field xpath="@id"/></xs:key>
      </xs:element>
    </xs:sequence></xs:complexType>
    %s
  </xs:element>
</xs:schema>`
	outerKeys := []struct {
		name string
		key  string
	}{
		{name: "ancestor selection", key: `<xs:key name="outer"><xs:selector xpath="."/><xs:field xpath="code"/></xs:key>`},
		{name: "selected element", key: `<xs:key name="outer"><xs:selector xpath="code"/><xs:field xpath="."/></xs:key>`},
	}
	for _, outer := range outerKeys {
		t.Run(outer.name, func(t *testing.T) {
			schema := strings.Replace(schemaTemplate, "%s", outer.key, 1)
			engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			session, err := engine.NewSession(xsd.ValidateOptions{MaxErrors: 2})
			if err != nil {
				t.Fatalf("NewSession() error = %v", err)
			}

			validationErr := session.Validate(strings.NewReader(`<root><code>value</code></root>`))
			if !errorTreeContains(validationErr, "key field is missing") {
				t.Fatalf("Validate(missing inner key) error = %v, want missing-field error", validationErr)
			}
			if errorTreeContains(validationErr, "key field selects nillable element declaration") {
				t.Fatalf("Validate(missing inner key) error = %v, want no secondary nillable-key error", validationErr)
			}
			if count := errorTreeLeafCount(validationErr); count != 1 {
				t.Fatalf("Validate(missing inner key) errors = %d, want 1: %v", count, validationErr)
			}

			validationErr = session.Validate(strings.NewReader(`<root><code id="present">value</code></root>`))
			if !errorTreeContains(validationErr, "key field selects nillable element declaration") {
				t.Fatalf("Validate(valid inner key) error = %v, want nillable-key error", validationErr)
			}
			if errorTreeContains(validationErr, "key field is missing") {
				t.Fatalf("Validate(valid inner key) error = %v, stale missing-field state", validationErr)
			}
		})
	}

	const keyrefSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence>
      <xs:element name="code" nillable="true">
        <xs:complexType><xs:simpleContent><xs:extension base="xs:string">
          <xs:attribute name="id" type="xs:string" use="required"/>
          <xs:attribute name="ref" type="xs:string" use="required"/>
        </xs:extension></xs:simpleContent></xs:complexType>
        <xs:key name="inner"><xs:selector xpath="."/><xs:field xpath="@id"/></xs:key>
        <xs:keyref name="innerRef" refer="inner"><xs:selector xpath="."/><xs:field xpath="@ref"/></xs:keyref>
      </xs:element>
    </xs:sequence></xs:complexType>
    <xs:key name="outer"><xs:selector xpath="code"/><xs:field xpath="."/></xs:key>
  </xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("keyref-schema.xsd", []byte(keyrefSchema)))
	if err != nil {
		t.Fatalf("Compile(keyref schema) error = %v", err)
	}
	validationErr := engine.ValidateWithOptions(
		strings.NewReader(`<root><code id="one" ref="two">value</code></root>`),
		xsd.ValidateOptions{MaxErrors: 2},
	)
	if !errorTreeContains(validationErr, "keyref does not resolve") {
		t.Fatalf("Validate(unresolved inner keyref) error = %v, want keyref error", validationErr)
	}
	if errorTreeContains(validationErr, "key field selects nillable element declaration") {
		t.Fatalf("Validate(unresolved inner keyref) error = %v, want no secondary nillable-key error", validationErr)
	}
}

func TestFixedAttributeConstraintComparisonUsesItsSchemaOwner(t *testing.T) {
	const schemaPrefix = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:test" targetNamespace="urn:test" elementFormDefault="qualified">
  <xs:attribute name="v" type="xs:duration" fixed="P1Y"/>`
	const schemaSuffix = `</xs:schema>`
	tests := []struct {
		name    string
		body    string
		doc     string
		wantErr bool
	}{
		{
			name: "referenced declaration uses value equality",
			body: `<xs:element name="root"><xs:complexType><xs:attribute ref="t:v"/></xs:complexType>
  <xs:unique name="values"><xs:selector xpath="."/><xs:field xpath="@t:v"/></xs:unique></xs:element>`,
			doc: `<t:root xmlns:t="urn:test" t:v="P12M"/>`,
		},
		{
			name: "referenced declaration without identity constraint uses value equality",
			body: `<xs:element name="root"><xs:complexType><xs:attribute ref="t:v"/></xs:complexType></xs:element>`,
			doc:  `<t:root xmlns:t="urn:test" t:v="P12M"/>`,
		},
		{
			name: "wildcard declaration uses value equality",
			body: `<xs:element name="root"><xs:complexType><xs:anyAttribute processContents="strict"/></xs:complexType>
  <xs:unique name="values"><xs:selector xpath="."/><xs:field xpath="@t:v"/></xs:unique></xs:element>`,
			doc: `<t:root xmlns:t="urn:test" t:v="P12M"/>`,
		},
		{
			name:    "local use retains canonical lexical equality",
			body:    `<xs:element name="root"><xs:complexType><xs:attribute name="v" type="xs:duration" fixed="P1Y"/></xs:complexType></xs:element>`,
			doc:     `<t:root xmlns:t="urn:test" v="P12M"/>`,
			wantErr: true,
		},
		{
			name:    "explicit reference override uses canonical lexical equality",
			body:    `<xs:element name="root"><xs:complexType><xs:attribute ref="t:v" fixed="P12M"/></xs:complexType></xs:element>`,
			doc:     `<t:root xmlns:t="urn:test" t:v="P1Y"/>`,
			wantErr: true,
		},
		{
			name: "group and extension preserve declaration provenance",
			body: `<xs:attributeGroup name="values"><xs:attribute ref="t:v"/></xs:attributeGroup>
  <xs:complexType name="base"><xs:attributeGroup ref="t:values"/></xs:complexType>
  <xs:complexType name="derived"><xs:complexContent><xs:extension base="t:base"/></xs:complexContent></xs:complexType>
  <xs:element name="root" type="t:derived"/>`,
			doc: `<t:root xmlns:t="urn:test" t:v="P12M"/>`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schemaPrefix+test.body+schemaSuffix)))
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			err = engine.Validate(strings.NewReader(test.doc))
			if test.wantErr {
				expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationAttribute)
				return
			}
			if err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestIdentityFieldsRejectUnassessedWildcardElements(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="known" type="xs:string"/>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:any processContents="skip" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
    <xs:unique name="u"><xs:selector xpath="*"/><xs:field xpath="."/></xs:unique>
  </xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	for _, test := range []struct {
		name string
		doc  string
	}{
		{name: "unknown", doc: `<root><unknown>value</unknown></root>`},
		{name: "global declaration ignored", doc: `<root><known>value</known></root>`},
		{name: "xsi simple type ignored", doc: `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema"><unknown xsi:type="xs:string">value<child/></unknown></root>`},
		{name: "xsi any type ignored", doc: `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema"><unknown xsi:type="xs:anyType">value</unknown></root>`},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := engine.Validate(strings.NewReader(test.doc))
			expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationIdentity)
			if !strings.Contains(err.Error(), "identity field has no simple value") {
				t.Fatalf("Validate() error = %v, want no-simple-value identity error", err)
			}
		})
	}
}

func TestIdentityFieldsRejectUnassessedWildcardAttributes(t *testing.T) {
	for _, test := range []struct {
		name     string
		schema   string
		instance string
	}{
		{
			name: "attribute wildcard skip",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="known" type="xs:string"/>
  <xs:element name="root">
    <xs:complexType><xs:anyAttribute processContents="skip"/></xs:complexType>
    <xs:unique name="u"><xs:selector xpath="."/><xs:field xpath="@*"/></xs:unique>
  </xs:element>
</xs:schema>`,
			instance: `<root unknown="value"/>`,
		},
		{
			name: "global attribute ignored by skip",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="known" type="xs:string"/>
  <xs:element name="root">
    <xs:complexType><xs:anyAttribute processContents="skip"/></xs:complexType>
    <xs:unique name="u"><xs:selector xpath="."/><xs:field xpath="@known"/></xs:unique>
  </xs:element>
</xs:schema>`,
			instance: `<root known="value"/>`,
		},
		{
			name: "attribute on skipped element",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:any processContents="skip"/></xs:sequence></xs:complexType>
    <xs:unique name="u"><xs:selector xpath="*"/><xs:field xpath="@*"/></xs:unique>
  </xs:element>
</xs:schema>`,
			instance: `<root><unknown code="value"/></root>`,
		},
		{
			name: "attribute wildcard lax missing",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:anyAttribute processContents="lax"/></xs:complexType>
    <xs:unique name="u"><xs:selector xpath="."/><xs:field xpath="@*"/></xs:unique>
  </xs:element>
</xs:schema>`,
			instance: `<root unknown="value"/>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(test.schema)))
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			err = engine.Validate(strings.NewReader(test.instance))
			expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationIdentity)
			if !strings.Contains(err.Error(), "identity field has no simple value") {
				t.Fatalf("Validate() error = %v, want no-simple-value identity error", err)
			}
		})
	}
}

func TestIdentityFieldsCaptureAssessedLaxWildcardAttributes(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:attribute name="code" type="xs:string"/>
  <xs:element name="root">
    <xs:complexType><xs:sequence>
      <xs:element name="row" maxOccurs="unbounded"><xs:complexType><xs:anyAttribute processContents="lax"/></xs:complexType></xs:element>
    </xs:sequence></xs:complexType>
    <xs:unique name="u"><xs:selector xpath="row"/><xs:field xpath="@code"/></xs:unique>
  </xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := engine.Validate(strings.NewReader(`<root><row code="a"/><row code="b"/></root>`)); err != nil {
		t.Fatalf("Validate(distinct) error = %v", err)
	}
	expectCategoryCode(
		t,
		engine.Validate(strings.NewReader(`<root><row code="a"/><row code="a"/></root>`)),
		xsderrors.CategoryValidation,
		xsderrors.CodeValidationIdentity,
	)
}

func TestIdentityRecoveryDoesNotReclassifyMatchedNodesAsMissing(t *testing.T) {
	schema := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence>
      <xs:element name="row"><xs:complexType><xs:sequence><xs:element name="good" minOccurs="0"/></xs:sequence></xs:complexType></xs:element>
    </xs:sequence></xs:complexType>
    <xs:key name="k"><xs:selector xpath="row"/><xs:field xpath="bad"/></xs:key>
  </xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	err = engine.ValidateWithOptions(strings.NewReader(`<root><row><bad/></row></root>`), xsd.ValidateOptions{MaxErrors: 10})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationElement)
	if strings.Contains(err.Error(), "key field is missing") {
		t.Fatalf("Validate() added missing-field cascade after recovery: %v", err)
	}
}

func TestInvalidTypedIdentityFieldsDoNotBecomeMissingFields(t *testing.T) {
	for _, test := range []struct {
		name     string
		schema   string
		instance string
	}{
		{
			name: "element value",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element name="row" type="xs:int"/></xs:sequence></xs:complexType>
    <xs:key name="k"><xs:selector xpath="row"/><xs:field xpath="."/></xs:key>
  </xs:element>
</xs:schema>`,
			instance: `<root><row>bad</row></root>`,
		},
		{
			name: "attribute value",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType><xs:attribute name="code" type="xs:int"/></xs:complexType>
    <xs:key name="k"><xs:selector xpath="."/><xs:field xpath="@code"/></xs:key>
  </xs:element>
</xs:schema>`,
			instance: `<root code="bad"/>`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(test.schema)))
			if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			err = engine.ValidateWithOptions(strings.NewReader(test.instance), xsd.ValidateOptions{MaxErrors: 10})
			expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationFacet)
			if strings.Contains(err.Error(), "key field is missing") {
				t.Fatalf("Validate() added missing-field cascade after invalid typed value: %v", err)
			}
		})
	}
}

type countingReader struct {
	read func([]byte) (int, error)
}

func (r countingReader) Read(p []byte) (int, error) { return r.read(p) }

type emptySchemaReadCloser struct {
	terminal error
	reads    int
	closed   bool
}

func (r *emptySchemaReadCloser) Read([]byte) (int, error) {
	if r.reads > 100 {
		return 0, r.terminal
	}
	r.reads++
	return 0, nil
}

func (r *emptySchemaReadCloser) Close() error {
	r.closed = true
	return nil
}

type oneByteCountingReader struct {
	data string
	off  int
}

func (r *oneByteCountingReader) Read(p []byte) (int, error) {
	if r.off == len(r.data) {
		return 0, io.EOF
	}
	p[0] = r.data[r.off]
	r.off++
	return 1, nil
}

func writeTestFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestNilReadersReturnStructuredErrors(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	err = engine.Validate(nil)
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)

	err = engine.ValidateWithOptions(nil, xsd.ValidateOptions{})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)

	var typedNil *bytes.Reader
	err = engine.Validate(typedNil)
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)

	err = engine.ValidateWithOptions(typedNil, xsd.ValidateOptions{})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)

	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	err = session.Validate(typedNil)
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)
	err = session.Validate(strings.NewReader(`<root/>`))
	if err != nil {
		t.Fatalf("Session.Validate() after typed nil error = %v", err)
	}

	_, err = xsd.Compile(xsd.Open("schema.xsd", func() (io.ReadCloser, error) {
		return nil, errors.New("nil schema reader")
	}))
	expectCategoryCode(t, err, xsderrors.CategorySchemaParse, xsderrors.CodeSchemaRead)
	if !strings.Contains(err.Error(), "nil schema reader") {
		t.Fatalf("Compile() error = %v, want nil schema reader", err)
	}
}

func TestNewSessionRejectsInvalidOptions(t *testing.T) {
	t.Parallel()

	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	session, err := engine.NewSession(xsd.ValidateOptions{MaxErrors: -1})
	if session != nil {
		t.Fatal("NewSession() session is non-nil for invalid options")
	}
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationOption)
}

func TestNewSessionRejectsMissingEngineState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		engine *xsd.Engine
	}{
		{name: "nil engine"},
		{name: "zero engine", engine: new(xsd.Engine)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			session, err := test.engine.NewSession(xsd.ValidateOptions{})
			if session != nil {
				t.Fatal("NewSession() returned a session without compiled engine state")
			}
			expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
		})
	}
}

type blockingReader struct {
	data    *strings.Reader
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (r *blockingReader) Read(p []byte) (int, error) {
	r.once.Do(func() {
		close(r.started)
		<-r.release
	})
	return r.data.Read(p)
}

func TestCopiedSessionRejectsOverlappingValidationBeforeReading(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)))
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	copySession := *session
	activeReader := &blockingReader{
		data:    strings.NewReader(`<root/>`),
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	activeErr := make(chan error, 1)
	go func() { activeErr <- session.Validate(activeReader) }()
	<-activeReader.started

	losingReads := 0
	err = copySession.Validate(countingReader{read: func([]byte) (int, error) {
		losingReads++
		return 0, io.EOF
	}})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationSession)
	if losingReads != 0 {
		t.Fatalf("overlapping Validate() consumed losing reader %d times", losingReads)
	}

	close(activeReader.release)
	if err := <-activeErr; err != nil {
		t.Fatalf("active Validate() error = %v", err)
	}
	if err := copySession.Validate(strings.NewReader(`<root/>`)); err != nil {
		t.Fatalf("Validate() after overlap error = %v", err)
	}
}

func TestValidateOptionsMaxInstanceTokenBytes(t *testing.T) {
	t.Parallel()

	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r" type="xs:anyType"/></xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	input := `<r a="12" b="34"/>`
	tests := []struct {
		name     string
		limit    int64
		validate func(*xsd.Engine, string, xsd.ValidateOptions) error
		wantErr  bool
	}{
		{
			name:  "engine exact limit",
			limit: 7,
			validate: func(engine *xsd.Engine, input string, opts xsd.ValidateOptions) error {
				return engine.ValidateWithOptions(strings.NewReader(input), opts)
			},
		},
		{
			name:  "engine over limit",
			limit: 6,
			validate: func(engine *xsd.Engine, input string, opts xsd.ValidateOptions) error {
				return engine.ValidateWithOptions(strings.NewReader(input), opts)
			},
			wantErr: true,
		},
		{
			name:  "session exact limit",
			limit: 7,
			validate: func(engine *xsd.Engine, input string, opts xsd.ValidateOptions) error {
				session, err := engine.NewSession(opts)
				if err != nil {
					return err
				}
				return session.Validate(strings.NewReader(input))
			},
		},
		{
			name:  "session over limit",
			limit: 6,
			validate: func(engine *xsd.Engine, input string, opts xsd.ValidateOptions) error {
				session, err := engine.NewSession(opts)
				if err != nil {
					return err
				}
				return session.Validate(strings.NewReader(input))
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.validate(engine, input, xsd.ValidateOptions{MaxInstanceTokenBytes: tt.limit})
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if xerr, ok := errors.AsType[*xsderrors.Error](err); !ok || xerr.Code != xsderrors.CodeValidationLimit {
				t.Fatalf("Validate() error = %v, want validation limit", err)
			}
		})
	}
}

func TestValidateOptionsBoundSchemaLocationNamespaces(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)))
	if err != nil {
		t.Fatal(err)
	}
	doc := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="urn:a a.xsd urn:b b.xsd"/>`

	err = engine.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{
		MaxErrors:                   10,
		MaxSchemaLocationNamespaces: 1,
	})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)

	session, err := engine.NewSession(xsd.ValidateOptions{MaxSchemaLocationNamespaceBytes: 4})
	if err != nil {
		t.Fatal(err)
	}
	err = session.Validate(strings.NewReader(doc))
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
	if err := session.Validate(strings.NewReader(`<root/>`)); err != nil {
		t.Fatalf("session reuse after schema-location limit: %v", err)
	}
}

func TestMaxIdentityEntriesRejectsPendingSelectionBeforeItsEnd(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:complexType name="A"><xs:sequence><xs:element ref="a" minOccurs="0"/></xs:sequence><xs:attribute name="id" use="required"/></xs:complexType>
  <xs:element name="a" type="A"/>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:element ref="a" minOccurs="0"/></xs:sequence></xs:complexType>
    <xs:key name="ids"><xs:selector xpath=".//a"/><xs:field xpath="@id"/></xs:key>
  </xs:element>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{MaxIdentityEntries: 1})
	if err != nil {
		t.Fatal(err)
	}
	doc := `<root><a id="one"><a id="two"></a></a></root>`
	reader := &oneByteCountingReader{data: doc}
	err = session.Validate(reader)
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
	if firstClose := strings.Index(doc, `</a>`); reader.off > firstClose {
		t.Fatalf("validation consumed %d bytes, want at most %d before the first selected element end", reader.off, firstClose)
	}
	if err := session.Validate(strings.NewReader(`<root><a id="one"/></root>`)); err != nil {
		t.Fatalf("session reuse after pending identity limit: %v", err)
	}
}

func TestCompileOptionsAggregateSchemaSetLimits(t *testing.T) {
	t.Parallel()

	rootData := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:include schemaLocation="child.xsd"/><xs:include schemaLocation="child.xsd"/></xs:schema>`)
	childData := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	resolver := xsd.ResolverFunc(func(_, location string) (xsd.SchemaSource, error) {
		if location == "child.xsd" {
			return xsd.Bytes("child.xsd", childData), nil
		}
		return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
	})
	root := xsd.Bytes("root.xsd", rootData).WithResolver(resolver)
	// The duplicate reference is resolved and read again to verify that the
	// resolver did not return different content for the same document identity.
	totalBytes := int64(len(rootData) + 2*len(childData))

	tests := []struct {
		name    string
		opts    xsd.CompileOptions
		wantErr bool
	}{
		{name: "exact source count", opts: xsd.CompileOptions{MaxSchemaSources: 2}},
		{name: "source count exceeded", opts: xsd.CompileOptions{MaxSchemaSources: 1}, wantErr: true},
		{name: "exact total bytes", opts: xsd.CompileOptions{MaxSchemaTotalBytes: totalBytes}},
		{name: "total bytes exceeded", opts: xsd.CompileOptions{MaxSchemaTotalBytes: totalBytes - 1}, wantErr: true},
		{name: "exact references", opts: xsd.CompileOptions{MaxSchemaReferences: 2}},
		{name: "references exceeded", opts: xsd.CompileOptions{MaxSchemaReferences: 1}, wantErr: true},
		{name: "exact target contexts", opts: xsd.CompileOptions{MaxSchemaTargetContexts: 2}},
		{name: "target contexts exceeded", opts: xsd.CompileOptions{MaxSchemaTargetContexts: 1}, wantErr: true},
		{name: "exact instantiated nodes", opts: xsd.CompileOptions{MaxSchemaInstantiatedNodes: 4}},
		{name: "instantiated nodes exceeded", opts: xsd.CompileOptions{MaxSchemaInstantiatedNodes: 3}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := xsd.CompileWithOptions(tt.opts, root)
			if !tt.wantErr {
				if err != nil {
					t.Fatalf("CompileWithOptions() error = %v", err)
				}
				return
			}
			if xerr, ok := errors.AsType[*xsderrors.Error](err); !ok || xerr.Code != xsderrors.CodeSchemaLimit {
				t.Fatalf("CompileWithOptions() error = %v, want schema limit", err)
			}
		})
	}
}

func TestCompileExplicitSourceLimitPrecedesSameIdentityOpeners(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	openCalls := 0
	sourceFor := func() xsd.SchemaSource {
		return xsd.Open("same.xsd", func() (io.ReadCloser, error) {
			openCalls++
			return io.NopCloser(strings.NewReader(schema)), nil
		})
	}
	_, err := xsd.CompileWithOptions(
		xsd.CompileOptions{MaxSchemaSources: 1},
		sourceFor(),
		sourceFor(),
	)
	if xerr, ok := errors.AsType[*xsderrors.Error](err); !ok || xerr.Code != xsderrors.CodeSchemaLimit {
		t.Fatalf("CompileWithOptions() error = %v, want schema limit", err)
	}
	if openCalls != 0 {
		t.Fatalf("schema opener calls = %d, want 0", openCalls)
	}
}

func TestCompileOptionsSubstitutionClosureLimit(t *testing.T) {
	t.Parallel()

	schema := xsd.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="h0" type="xs:string"/>
  <xs:element name="h1" substitutionGroup="h0"/>
  <xs:element name="h2" substitutionGroup="h1"/>
  <xs:element name="h3" substitutionGroup="h2"/>
</xs:schema>`))
	if _, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSubstitutionClosureEntries: 6}, schema); err != nil {
		t.Fatalf("CompileWithOptions(exact limit) error = %v", err)
	}
	_, err := xsd.CompileWithOptions(xsd.CompileOptions{MaxSubstitutionClosureEntries: 5}, schema)
	if xerr, ok := errors.AsType[*xsderrors.Error](err); !ok || xerr.Code != xsderrors.CodeSchemaLimit {
		t.Fatalf("CompileWithOptions(over limit) error = %v, want schema limit", err)
	}
}

func TestBytesSourceCopiesInput(t *testing.T) {
	data := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)
	source := xsd.Bytes("schema.xsd", data)
	for i := range data {
		data[i] = 0
	}
	if _, err := xsd.Compile(source); err != nil {
		t.Fatalf("Compile(Bytes(...)) error after caller mutation = %v", err)
	}
}

func TestCopiedEngineSharesPublishedSchema(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:int"/></xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	engineCopy := *engine

	var wg sync.WaitGroup
	for _, e := range []*xsd.Engine{engine, &engineCopy} {
		wg.Add(1)
		go func(e *xsd.Engine) {
			defer wg.Done()
			if err := e.Validate(strings.NewReader(`<root>7</root>`)); err != nil {
				t.Errorf("Validate() error = %v", err)
			}
		}(e)
	}
	wg.Wait()
}

// TestEngineConcurrentValidation is the executable form of the runtime schema
// sharing contract: sessions only read the schema published to an Engine. The
// schema routes workers through identity key/keyref tables, attribute default
// and fixed values, a variable-length pattern facet, a wide DFA row index, a
// substitution group, and xs:ID/xs:IDREF tracking.
func TestEngineConcurrentValidation(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="head" type="xs:string" abstract="true"/>
  <xs:element name="sub" type="xs:string" substitutionGroup="head"/>
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" maxOccurs="unbounded">
          <xs:complexType>
            <xs:choice>
              <xs:element name="c1" type="xs:string"/>
              <xs:element name="c2" type="xs:string"/>
              <xs:element name="c3" type="xs:string"/>
              <xs:element name="c4" type="xs:string"/>
              <xs:element name="c5" type="xs:string"/>
              <xs:element name="c6" type="xs:string"/>
              <xs:element name="c7" type="xs:string"/>
              <xs:element name="c8" type="xs:string"/>
              <xs:element ref="head"/>
            </xs:choice>
            <xs:attribute name="id" type="xs:ID" use="required"/>
            <xs:attribute name="mode" type="xs:string" default="std"/>
            <xs:attribute name="kind" type="xs:string" fixed="leaf"/>
            <xs:attribute name="code">
              <xs:simpleType>
                <xs:restriction base="xs:string">
                  <xs:pattern value="[a-z]+[0-9]*"/>
                </xs:restriction>
              </xs:simpleType>
            </xs:attribute>
          </xs:complexType>
        </xs:element>
        <xs:element name="link" minOccurs="0" maxOccurs="unbounded">
          <xs:complexType>
            <xs:attribute name="ref" type="xs:IDREF" use="required"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:key name="itemKey">
      <xs:selector xpath="item"/>
      <xs:field xpath="@id"/>
    </xs:key>
    <xs:keyref name="linkRef" refer="itemKey">
      <xs:selector xpath="link"/>
      <xs:field xpath="@ref"/>
    </xs:keyref>
  </xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	docs := []struct {
		xml   string
		valid bool
	}{
		{`<root><item id="a1" code="abc12"><c5>x</c5></item><item id="a2" kind="leaf"><sub>y</sub></item><link ref="a1"/></root>`, true},
		{`<root><item id="b1"><c1>x</c1></item><link ref="missing"/></root>`, false},
		{`<root><item id="b2" code="123"><c8>x</c8></item></root>`, false},
		{`<root><item id="b3" kind="other"><c2>x</c2></item></root>`, false},
	}
	check := func(name string, validate func(io.Reader) error) {
		for i, doc := range docs {
			err := validate(strings.NewReader(doc.xml))
			if doc.valid && err != nil {
				t.Errorf("%s doc %d: Validate() error = %v", name, i, err)
				return
			}
			if !doc.valid && err == nil {
				t.Errorf("%s doc %d: Validate() succeeded, want error", name, i)
				return
			}
		}
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 25 {
				check("engine", engine.Validate)
			}
		})
		wg.Go(func() {
			session, err := engine.NewSession(xsd.ValidateOptions{})
			if err != nil {
				t.Errorf("NewSession() error = %v", err)
				return
			}
			for range 25 {
				check("session", session.Validate)
			}
		})
	}
	wg.Wait()
}

func TestCopiedSessionResolvesQNameValuesWithCopyState(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:QName"/>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	copySession := *session
	if err := copySession.Validate(strings.NewReader(`<root xmlns:p="urn:test">p:item</root>`)); err != nil {
		t.Fatalf("copied Session.Validate() error = %v", err)
	}
}

func TestValidationPathsPreserveNameSpelling(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="known" type="xs:int"/>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:any processContents="lax" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
  </xs:element>
  <xs:element name="strictRoot">
    <xs:complexType><xs:sequence><xs:any processContents="strict" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		doc  string
		path string
	}{
		{name: "known", doc: `<root><known>x</known></root>`, path: "/root/known"},
		{name: "unknown local", doc: `<root><unknown></other></root>`, path: "/root/unknown"},
		{name: "unknown namespaced lax", doc: `<root><o:unknown xmlns:o="urn:o"></o:other></root>`, path: "/root/{urn:o}unknown"},
		{name: "skipped", doc: `<strictRoot><o:unknown xmlns:o="urn:o"></o:other></strictRoot>`, path: "/strictRoot/unknown"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := engine.Validate(strings.NewReader(tc.doc))
			x, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("Validate() error = %v, want structured error", err)
			}
			if x.Path != tc.path {
				t.Fatalf("path = %q, want %q; err=%v", x.Path, tc.path, err)
			}
		})
	}
}

func TestLaxWildcardValidationAllocationsMatchKnownName(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:s" xmlns:s="urn:s" elementFormDefault="qualified">
  <xs:element name="known" type="xs:anyType"/>
  <xs:element name="root">
    <xs:complexType><xs:sequence><xs:any processContents="lax" maxOccurs="unbounded"/></xs:sequence></xs:complexType>
  </xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatal(err)
	}
	known, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	lax, err := engine.NewSession(xsd.ValidateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	knownDoc := `<s:root xmlns:s="urn:s"><k:known xmlns:k="urn:s"/></s:root>`
	laxDoc := `<s:root xmlns:s="urn:s"><o:unknown xmlns:o="urn:o"/></s:root>`
	for range 10 {
		if err := known.Validate(strings.NewReader(knownDoc)); err != nil {
			t.Fatal(err)
		}
		if err := lax.Validate(strings.NewReader(laxDoc)); err != nil {
			t.Fatal(err)
		}
	}
	knownAllocs := testing.AllocsPerRun(100, func() {
		if err := known.Validate(strings.NewReader(knownDoc)); err != nil {
			panic(err)
		}
	})
	laxAllocs := testing.AllocsPerRun(100, func() {
		if err := lax.Validate(strings.NewReader(laxDoc)); err != nil {
			panic(err)
		}
	})
	if laxAllocs != knownAllocs {
		t.Fatalf("lax wildcard allocations = %.0f, known name = %.0f", laxAllocs, knownAllocs)
	}
}

func expectCategoryCode(t *testing.T, err error, category xsderrors.Category, code xsderrors.Code) {
	t.Helper()
	x, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error %v is not *xsderrors.Error", err)
	}
	if x.Category != category || x.Code != code {
		t.Fatalf("error = %s/%s, want %s/%s; err=%v", x.Category, x.Code, category, code, err)
	}
}
