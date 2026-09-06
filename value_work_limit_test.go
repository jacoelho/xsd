package xsd_test

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const valueWorkStringSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

func TestInstanceValueWorkIsIndependentOfSchemaCompilationLimits(t *testing.T) {
	text := strings.Repeat("x", 20_000)
	instance := "<root>" + text + "</root>"
	compile := func(opts xsd.CompileOptions) *xsd.Engine {
		t.Helper()
		engine, err := xsd.CompileWithOptions(opts, xsd.Bytes("schema.xsd", []byte(valueWorkStringSchema)))
		if err != nil {
			t.Fatalf("CompileWithOptions() error = %v", err)
		}
		return engine
	}

	low := compile(xsd.CompileOptions{
		MaxSchemaTokenBytes:        256,
		MaxSchemaInstantiatedNodes: 2,
	})
	high := compile(xsd.CompileOptions{
		MaxSchemaTokenBytes:        1 << 20,
		MaxSchemaInstantiatedNodes: 128,
	})
	validate := func(engine *xsd.Engine, work uint64) {
		t.Helper()
		err := engine.ValidateWithOptions(strings.NewReader(instance), xsd.ValidateOptions{
			MaxInstanceTextBytes:  30_000,
			MaxInstanceTokenBytes: 30_000,
			MaxInstanceValueWork:  work,
		})
		if err != nil {
			t.Fatalf("ValidateWithOptions() error = %v", err)
		}
	}
	for _, work := range []struct {
		name  string
		value uint64
	}{
		{name: "default", value: 0},
		{name: "explicit", value: 100_000},
	} {
		t.Run(work.name, func(t *testing.T) {
			validate(low, work.value)
			validate(high, work.value)
		})
	}
}

func TestInstanceValueWorkRemainsIndependentAcrossValueShapes(t *testing.T) {
	text := strings.Repeat("x", 20_000)
	listText := strings.Repeat("x ", 9_999) + "x"
	tests := []struct {
		name     string
		schema   string
		instance string
	}{
		{
			name: "projected attribute",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType>
    <xs:attribute name="value" type="xs:string" use="required"/>
  </xs:complexType>
    <xs:unique name="valueKey"><xs:selector xpath="."/><xs:field xpath="@value"/></xs:unique>
  </xs:element>
</xs:schema>`,
			instance: `<root value="` + text + `"/>`,
		},
		{
			name: "restricted string",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="restricted"><xs:restriction base="xs:string"><xs:minLength value="1"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="restricted"/>
</xs:schema>`,
			instance: `<root>` + text + `</root>`,
		},
		{
			name: "list",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="items"><xs:list itemType="xs:string"/></xs:simpleType>
  <xs:element name="root" type="items"/>
</xs:schema>`,
			instance: `<root>` + listText + `</root>`,
		},
		{
			name: "late union",
			schema: `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="late"><xs:union memberTypes="xs:int xs:string"/></xs:simpleType>
  <xs:element name="root" type="late"/>
</xs:schema>`,
			instance: `<root>` + text + `</root>`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, schemaLimit := range []struct {
				name string
				opts xsd.CompileOptions
			}{
				{name: "low", opts: xsd.CompileOptions{MaxSchemaTokenBytes: 256, MaxSchemaInstantiatedNodes: 32}},
				{name: "high", opts: xsd.CompileOptions{MaxSchemaTokenBytes: 1 << 20, MaxSchemaInstantiatedNodes: 128}},
			} {
				t.Run(schemaLimit.name, func(t *testing.T) {
					engine, err := xsd.CompileWithOptions(schemaLimit.opts, xsd.Bytes("schema.xsd", []byte(test.schema)))
					if err != nil {
						t.Fatalf("CompileWithOptions() error = %v", err)
					}
					if err := engine.ValidateWithOptions(strings.NewReader(test.instance), xsd.ValidateOptions{
						MaxIdentityTupleBytes: 30_000,
						MaxInstanceTextBytes:  30_000,
						MaxInstanceTokenBytes: 30_000,
						MaxInstanceValueWork:  100_000,
					}); err != nil {
						t.Fatalf("ValidateWithOptions() error = %v", err)
					}
				})
			}
		})
	}
}

func TestInstanceValueWorkScalarBoundaryAndSessionReuse(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(valueWorkStringSchema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	for _, test := range []struct {
		name  string
		limit uint64
		want  xsderrors.Code
	}{
		{name: "below charge", limit: 1, want: xsderrors.CodeValidationLimit},
		{name: "at charge", limit: 2},
		{name: "above charge", limit: 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			validationErr := engine.ValidateWithOptions(strings.NewReader(`<root>x</root>`), xsd.ValidateOptions{
				MaxInstanceValueWork: test.limit,
			})
			if test.want == "" {
				if validationErr != nil {
					t.Fatalf("ValidateWithOptions() error = %v", validationErr)
				}
				return
			}
			expectCategoryCode(t, validationErr, xsderrors.CategoryValidation, test.want)
		})
	}

	session, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceValueWork: 1})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	expectCategoryCode(t, session.Validate(strings.NewReader(`<root>x</root>`)), xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
	if err := session.Validate(strings.NewReader(`<root/>`)); err != nil {
		t.Fatalf("reused Session.Validate() error = %v", err)
	}

	if err := engine.ValidateWithOptions(strings.NewReader(`<root>x</root>`), xsd.ValidateOptions{
		MaxInstanceValueWork: uint64(math.MaxInt64) + 1,
	}); err != nil {
		t.Fatalf("MaxInstanceValueWork above MaxInt64 error = %v", err)
	}
}

const valueWorkUnionListSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="item">
    <xs:union memberTypes="xs:int xs:string"/>
  </xs:simpleType>
  <xs:simpleType name="items"><xs:list itemType="item"/></xs:simpleType>
  <xs:element name="root" type="items"/>
</xs:schema>`

func TestInstanceValueWorkIsSharedByNestedListUnionEvaluation(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(valueWorkUnionListSchema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	instance := strings.NewReader(`<root>late</root>`)
	expectCategoryCode(t, engine.ValidateWithOptions(instance, xsd.ValidateOptions{
		MaxInstanceValueWork: 9,
	}), xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
	if err := engine.ValidateWithOptions(strings.NewReader(`<root>late</root>`), xsd.ValidateOptions{
		MaxInstanceValueWork: 64,
	}); err != nil {
		t.Fatalf("sufficient nested evaluation budget error = %v", err)
	}
}

const valueWorkAttributeSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root"><xs:complexType>
    <xs:attribute name="value" type="xs:string" use="required"/>
  </xs:complexType>
    <xs:unique name="valueKey"><xs:selector xpath="."/><xs:field xpath="@value"/></xs:unique>
  </xs:element>
</xs:schema>`

func TestInstanceValueWorkCoversProjectedAttributeValues(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(valueWorkAttributeSchema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := engine.ValidateWithOptions(strings.NewReader(`<root value="x"/>`), xsd.ValidateOptions{
		MaxInstanceValueWork: 2,
	}); err != nil {
		t.Fatalf("attribute at scalar charge error = %v", err)
	}
	expectCategoryCode(t, engine.ValidateWithOptions(strings.NewReader(`<root value="x"/>`), xsd.ValidateOptions{
		MaxInstanceValueWork: 1,
	}), xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
}

const valueWorkDefaultFixedSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="long"><xs:restriction base="xs:string"><xs:minLength value="8"/></xs:restriction></xs:simpleType>
  <xs:element name="defaulted" type="long" default="longtext"/>
  <xs:element name="fixed" type="long" fixed="longtext"/>
</xs:schema>`

func TestInstanceValueWorkUsesPrevalidatedDefaultAndFixedValues(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(valueWorkDefaultFixedSchema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	for _, instance := range []string{`<defaulted/>`, `<fixed/>`} {
		if err := engine.ValidateWithOptions(strings.NewReader(instance), xsd.ValidateOptions{
			MaxInstanceTextBytes: 1,
			MaxInstanceValueWork: 1,
		}); err != nil {
			t.Fatalf("ValidateWithOptions(%s) error = %v", instance, err)
		}
	}
	if err := engine.ValidateWithOptions(strings.NewReader(`<fixed>longtext</fixed>`), xsd.ValidateOptions{
		MaxInstanceTextBytes: 8,
		MaxInstanceValueWork: 9,
	}); err != nil {
		t.Fatalf("fixed value revalidation error = %v", err)
	}
}

func TestInstanceValueWorkRevalidatesElementConstraintForXSIType(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <xs:simpleType name="restricted"><xs:restriction base="xs:string"><xs:minLength value="8"/></xs:restriction></xs:simpleType>
  <xs:element name="root" type="xs:string" fixed="longtext"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if err := engine.ValidateWithOptions(strings.NewReader(`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="restricted"/>`), xsd.ValidateOptions{
		MaxInstanceTextBytes: 1,
		MaxInstanceValueWork: 9,
	}); err != nil {
		t.Fatalf("XSI type element constraint validation error = %v", err)
	}
}

func TestInstanceValueWorkEvaluatesAssembledCharacterDataOnce(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(valueWorkDefaultFixedSchema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	const assembled = `<defaulted><![CDATA[12345678]]><![CDATA[abcdefgh]]><![CDATA[IJKLMNOP]]><![CDATA[qrstuvwx]]></defaulted>`
	if err := engine.ValidateWithOptions(strings.NewReader(assembled), xsd.ValidateOptions{
		MaxInstanceTokenBytes: 16,
		MaxInstanceTextBytes:  32,
		MaxInstanceValueWork:  33,
	}); err != nil {
		t.Fatalf("assembled character data validation error = %v", err)
	}
	expectCategoryCode(t, engine.ValidateWithOptions(strings.NewReader(assembled), xsd.ValidateOptions{
		MaxInstanceTokenBytes: 16,
		MaxInstanceTextBytes:  32,
		MaxInstanceValueWork:  32,
	}), xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
}

func TestInstanceValueWorkPreservesXMLByteLimits(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(valueWorkStringSchema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	doc := `<root>` + strings.Repeat("x", 32) + `</root>`
	for _, test := range []struct {
		name  string
		text  int64
		token int64
		input int64
	}{
		{name: "at byte limits", text: 32, token: 32, input: int64(len(doc))},
		{name: "text exhausted", text: 31, token: 32, input: int64(len(doc))},
		{name: "token exhausted", text: 32, token: 31, input: int64(len(doc))},
		{name: "input exhausted", text: 32, token: 32, input: int64(len(doc) - 1)},
	} {
		t.Run(test.name, func(t *testing.T) {
			validationErr := engine.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{
				MaxInstanceTextBytes:  test.text,
				MaxInstanceTokenBytes: test.token,
				MaxInstanceBytes:      test.input,
				MaxInstanceValueWork:  33,
			})
			if test.name == "at byte limits" {
				if validationErr != nil {
					t.Fatalf("validation at byte limits = %v", validationErr)
				}
				return
			}
			expectCategoryCode(t, validationErr, xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
		})
	}
}

const valueWorkXSIIdentitySchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <xs:element name="root" nillable="true">
    <xs:complexType><xs:sequence/></xs:complexType>
    <xs:unique name="nilValue"><xs:selector xpath="."/><xs:field xpath="@xsi:nil"/></xs:unique>
  </xs:element>
</xs:schema>`

func TestInstanceValueWorkLimitInXSIIdentityReleasesTarget(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(valueWorkXSIIdentitySchema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	session, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceValueWork: 1})
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	expectCategoryCode(t, session.Validate(strings.NewReader(`<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="false"/>`)), xsderrors.CategoryValidation, xsderrors.CodeValidationLimit)
	if err := session.Validate(strings.NewReader(`<root/>`)); err != nil {
		t.Fatalf("validation after XSI value-limit failure = %v", err)
	}
}

const valueWorkXSISchemaLocationSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <xs:element name="root"><xs:complexType><xs:sequence>
    <xs:element name="source"><xs:complexType/></xs:element>
  </xs:sequence></xs:complexType>
    <xs:unique name="locations"><xs:selector xpath="source"/><xs:field xpath="@xsi:schemaLocation"/></xs:unique>
  </xs:element>
</xs:schema>`

func TestInstanceValueWorkGivesEachXSISchemaLocationItemItsOwnBudget(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(valueWorkXSISchemaLocationSchema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	doc := `<root xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><source xsi:schemaLocation="a b"/></root>`
	if err := engine.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{
		MaxInstanceValueWork: 2,
	}); err != nil {
		t.Fatalf("separate xsi:schemaLocation item evaluations error = %v", err)
	}
}

func TestInstanceValueWorkDoesNotCoupleConcurrentSessions(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(valueWorkStringSchema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	limited, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceValueWork: 1})
	if err != nil {
		t.Fatalf("NewSession(limited) error = %v", err)
	}
	open, err := engine.NewSession(xsd.ValidateOptions{MaxInstanceValueWork: 2})
	if err != nil {
		t.Fatalf("NewSession(open) error = %v", err)
	}
	results := make(chan error, 2)
	go func() { results <- limited.Validate(strings.NewReader(`<root>x</root>`)) }()
	go func() { results <- open.Validate(strings.NewReader(`<root>x</root>`)) }()
	var limitErr, success bool
	for range 2 {
		err := <-results
		if err == nil {
			success = true
			continue
		}
		var diagnostic *xsderrors.Error
		if !errors.As(err, &diagnostic) || diagnostic.Code() != xsderrors.CodeValidationLimit {
			t.Fatalf("concurrent validation error = %v, want validation.limit", err)
		}
		limitErr = true
	}
	if !limitErr || !success {
		t.Fatalf("concurrent sessions outcomes = limit %v, success %v", limitErr, success)
	}
}

const valueWorkOuterUnionSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="item"><xs:union memberTypes="xs:int xs:string"/></xs:simpleType>
  <xs:simpleType name="items"><xs:list itemType="item"/></xs:simpleType>
  <xs:simpleType name="two"><xs:restriction base="items"><xs:length value="2"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="three"><xs:restriction base="items"><xs:minLength value="3"/></xs:restriction></xs:simpleType>
  <xs:simpleType name="outer"><xs:union memberTypes="two three"/></xs:simpleType>
  <xs:element name="root" type="outer"/>
</xs:schema>`

func TestInstanceValueWorkBoundsOuterUnionListCrossProduct(t *testing.T) {
	compile := func(opts xsd.CompileOptions) *xsd.Engine {
		t.Helper()
		engine, err := xsd.CompileWithOptions(opts, xsd.Bytes("schema.xsd", []byte(valueWorkOuterUnionSchema)))
		if err != nil {
			t.Fatalf("CompileWithOptions() error = %v", err)
		}
		return engine
	}
	lowSchema := compile(xsd.CompileOptions{
		MaxSchemaTokenBytes:        256,
		MaxSchemaInstantiatedNodes: 32,
	})
	highSchema := compile(xsd.CompileOptions{
		MaxSchemaTokenBytes:        1 << 20,
		MaxSchemaInstantiatedNodes: 128,
	})
	doc := `<root>late late late</root>`
	for _, test := range []struct {
		name  string
		work  uint64
		isErr bool
	}{
		{name: "small shared budget", work: 64, isErr: true},
		{name: "sufficient shared budget", work: 256},
	} {
		for _, engine := range []struct {
			name  string
			value *xsd.Engine
		}{
			{name: "low schema limits", value: lowSchema},
			{name: "high schema limits", value: highSchema},
		} {
			t.Run(test.name+"/"+engine.name, func(t *testing.T) {
				err := engine.value.ValidateWithOptions(strings.NewReader(doc), xsd.ValidateOptions{
					MaxInstanceValueWork: test.work,
				})
				if test.isErr {
					var diagnostic *xsderrors.Error
					if !errors.As(err, &diagnostic) || diagnostic.Code() != xsderrors.CodeValidationLimit {
						t.Fatalf("validation error = %v, want validation.limit", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("validation error = %v", err)
				}
			})
		}
	}
}
