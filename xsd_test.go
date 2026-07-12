package xsd_test

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestNilReadersReturnStructuredErrors(t *testing.T) {
	engine, err := xsd.Compile(xsd.Reader("schema.xsd", strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	err = engine.Validate(nil)
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)

	err = engine.ValidateWithOptions(nil, xsd.ValidateOptions{})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationXML)

	_, err = xsd.Compile(xsd.Reader("schema.xsd", nil))
	expectCategoryCode(t, err, xsderrors.CategorySchemaParse, xsderrors.CodeSchemaRead)
	if !strings.Contains(err.Error(), "nil schema reader") {
		t.Fatalf("Compile() error = %v, want nil schema reader", err)
	}
}

func TestNewSessionRejectsInvalidOptions(t *testing.T) {
	t.Parallel()

	engine, err := xsd.Compile(xsd.Reader("schema.xsd", strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	session, err := engine.NewSession(xsd.ValidateOptions{MaxErrors: -1})
	if session != nil {
		t.Fatal("NewSession() session is non-nil for invalid options")
	}
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationOption)
}

func TestValidateOptionsMaxInstanceTokenBytes(t *testing.T) {
	t.Parallel()

	engine, err := xsd.Compile(xsd.Reader("schema.xsd", strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="r" type="xs:anyType"/></xs:schema>`)))
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
	totalBytes := int64(len(rootData) + len(childData))

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
	engine, err := xsd.Compile(xsd.Reader("schema.xsd", strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:int"/></xs:schema>`)))
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
