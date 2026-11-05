package xsd_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

const sessionPoolSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="child" minOccurs="0"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

func TestEngineValidateReusesAutomaticSession(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(sessionPoolSchema)))
	if err != nil {
		t.Fatal(err)
	}
	const document = `<root/>`
	if err := engine.Validate(strings.NewReader(document)); err != nil {
		t.Fatal(err)
	}
	reader := strings.NewReader(document)
	allocs := testing.AllocsPerRun(100, func() {
		reader.Reset(document)
		if err := engine.Validate(reader); err != nil {
			t.Fatalf("reused validation: %v", err)
		}
	})
	if allocs != 0 {
		t.Fatalf("automatic validation allocations = %.0f, want 0", allocs)
	}
}

func BenchmarkEngineValidateReusableReader(b *testing.B) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(sessionPoolSchema)))
	if err != nil {
		b.Fatal(err)
	}
	const document = `<root/>`
	reader := strings.NewReader(document)
	b.SetBytes(int64(len(document)))
	b.ReportAllocs()
	for b.Loop() {
		reader.Reset(document)
		if err := engine.Validate(reader); err != nil {
			b.Fatal(err)
		}
	}
}

func TestEngineValidateOptionsArePerCall(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(sessionPoolSchema)))
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.ValidateWithOptions(
		strings.NewReader(`<root/>`),
		xsd.ValidateOptions{MaxInstanceBytes: 1},
	); err == nil {
		t.Fatal("byte-limited validation succeeded")
	}
	if err := engine.Validate(strings.NewReader(`<root/>`)); err != nil {
		t.Fatalf("validation after per-call limit: %v", err)
	}
}

const identityLimitSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="group" maxOccurs="unbounded">
          <xs:complexType>
            <xs:sequence>
              <xs:element name="item" maxOccurs="unbounded">
                <xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType>
              </xs:element>
            </xs:sequence>
          </xs:complexType>
          <xs:unique name="perGroup"><xs:selector xpath="item"/><xs:field xpath="@id"/></xs:unique>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
    <xs:unique name="all"><xs:selector xpath="group/item"/><xs:field xpath="@id"/></xs:unique>
  </xs:element>
</xs:schema>`

// The document holds two simultaneously active identity scopes (root and one
// group) and three stored entries, each with a non-empty key.
const identityLimitDocument = `<root><group><item id="aa"/><item id="bb"/></group><group><item id="cc"/></group></root>`

var identityLimitCases = []struct {
	name    string
	opts    xsd.ValidateOptions
	message string
}{
	{name: "scopes", opts: xsd.ValidateOptions{MaxIdentityScopes: 1}, message: "MaxIdentityScopes=1"},
	{name: "entries", opts: xsd.ValidateOptions{MaxIdentityEntries: 2}, message: "MaxIdentityEntries=2"},
	{name: "tuple bytes", opts: xsd.ValidateOptions{MaxIdentityTupleBytes: 1}, message: "MaxIdentityTupleBytes=1"},
}

func requireIdentityLimit(t *testing.T, err error, message string) {
	t.Helper()
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || xerr.Code() != xsderrors.CodeValidationLimit || !strings.Contains(xerr.Error(), message) {
		t.Fatalf("error = %v, want %s naming %s", err, xsderrors.CodeValidationLimit, message)
	}
}

func TestEngineValidateAppliesEachCallsIdentityLimits(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(identityLimitSchema)))
	if err != nil {
		t.Fatal(err)
	}
	validate := func(opts xsd.ValidateOptions) error {
		return engine.ValidateWithOptions(strings.NewReader(identityLimitDocument), opts)
	}
	if err := validate(xsd.ValidateOptions{}); err != nil {
		t.Fatalf("default limits: %v", err)
	}
	// Alternate limits on one engine so a reused session must replace every
	// identity limit the previous call configured.
	for _, round := range []string{"first", "second"} {
		for _, tc := range identityLimitCases {
			requireIdentityLimit(t, validate(tc.opts), tc.message)
			if err := validate(xsd.ValidateOptions{}); err != nil {
				t.Fatalf("%s round: default limits after %s limit: %v", round, tc.name, err)
			}
		}
	}
}

func TestEngineNewSessionAppliesIdentityLimits(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(identityLimitSchema)))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range identityLimitCases {
		t.Run(tc.name, func(t *testing.T) {
			session, err := engine.NewSession(tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			requireIdentityLimit(t, session.Validate(strings.NewReader(identityLimitDocument)), tc.message)
		})
	}
}

func TestEngineValidateResetsQNameStateAfterError(t *testing.T) {
	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:one" xmlns:t="urn:one">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Validate(strings.NewReader(`<t:root xmlns:t="urn:wrong"/>`)); err == nil {
		t.Fatal("wrong namespace validation succeeded")
	}
	if err := engine.Validate(strings.NewReader(`<t:root xmlns:t="urn:one"/>`)); err != nil {
		t.Fatalf("validation after QName error: %v", err)
	}
}

func TestEngineValidateDropsPanickingSession(t *testing.T) {
	engine, err := xsd.Compile(xsd.Bytes("schema.xsd", []byte(sessionPoolSchema)))
	if err != nil {
		t.Fatal(err)
	}
	if err := engine.Validate(strings.NewReader(`<root/>`)); err != nil {
		t.Fatalf("warm validation: %v", err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic reader did not panic")
			}
		}()
		if err := engine.Validate(sessionPoolPanicReader{}); err != nil {
			t.Errorf("panic reader returned error: %v", err)
		}
	}()
	if err := engine.Validate(strings.NewReader(`<root/>`)); err != nil {
		t.Fatalf("validation after reader panic: %v", err)
	}
}

func TestNilEngineNormalizesOptionsBeforeSchemaCheck(t *testing.T) {
	var engine *xsd.Engine
	err := engine.ValidateWithOptions(strings.NewReader(`<root/>`), xsd.ValidateOptions{MaxErrors: -1})
	expectCategoryCode(t, err, xsderrors.CategoryValidation, xsderrors.CodeValidationOption)
	err = engine.Validate(strings.NewReader(`<root/>`))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
}

type sessionPoolPanicReader struct{}

func (sessionPoolPanicReader) Read([]byte) (int, error) {
	panic("public session pool test panic")
}
