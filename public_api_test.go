package xsd_test

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestPublicTypesRemainRootDefined(t *testing.T) {
	tests := []reflect.Type{
		reflect.TypeFor[xsd.Engine](),
		reflect.TypeFor[xsd.Session](),
		reflect.TypeFor[xsd.CompileOptions](),
		reflect.TypeFor[xsd.ValidateOptions](),
		reflect.TypeFor[xsd.SchemaSource](),
		reflect.TypeFor[xsd.ResolverFunc](),
	}
	for _, typ := range tests {
		if got := typ.PkgPath(); got != "github.com/jacoelho/xsd" {
			t.Fatalf("%s PkgPath = %q, want github.com/jacoelho/xsd", typ, got)
		}
	}
}

func TestDiagnosticsTypesRemainPublic(t *testing.T) {
	tests := []reflect.Type{
		reflect.TypeFor[xsderrors.Error](),
		reflect.TypeFor[xsderrors.Errors](),
	}
	for _, typ := range tests {
		if got := typ.PkgPath(); got != "github.com/jacoelho/xsd/xsderrors" {
			t.Fatalf("%s PkgPath = %q, want github.com/jacoelho/xsd/xsderrors", typ, got)
		}
	}
}

func TestZeroAndNilValidationReceiversReturnPublicErrors(t *testing.T) {
	var zero xsd.Engine
	err := zero.Validate(strings.NewReader(`<root/>`))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	assertPublicErrorTree(t, err)

	var nilEngine *xsd.Engine
	err = nilEngine.Validate(strings.NewReader(`<root/>`))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	assertPublicErrorTree(t, err)

	var nilSession *xsd.Session
	err = nilSession.Validate(strings.NewReader(`<root/>`))
	expectCategoryCode(t, err, xsderrors.CategoryInternal, xsderrors.CodeInternalInvariant)
	assertPublicErrorTree(t, err)
	nilSession.Reset()
}

func TestPublicAggregateErrorsDoNotExposeInternalDiagnostics(t *testing.T) {
	engine, err := xsd.Compile(xsd.Reader("schema.xsd", strings.NewReader(`
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="a" type="xs:int"/>
        <xs:element name="b" type="xs:int"/>
      </xs:sequence>
      <xs:attribute name="req" type="xs:string" use="required"/>
    </xs:complexType>
  </xs:element>
</xs:schema>`)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	err = engine.Validate(strings.NewReader(`<root><a>x</a><b>y</b></root>`))
	var errs xsderrors.Errors
	if !errors.As(err, &errs) {
		t.Fatalf("Validate() error type = %T, want xsderrors.Errors; err=%v", err, err)
	}
	if len(errs) != 3 {
		t.Fatalf("len(xsderrors.Errors) = %d, want 3; err=%v", len(errs), err)
	}
	assertPublicErrorTree(t, err)
	for i, child := range errs {
		if child == nil {
			t.Fatalf("xsderrors.Errors[%d] is nil", i)
		}
		assertPublicErrorTree(t, child)
	}
}

func TestPublicDiagnosticsWrapping(t *testing.T) {
	err := xsderrors.SchemaParse(
		xsderrors.CodeSchemaXML,
		1,
		2,
		"invalid schema XML",
		xsderrors.Unsupported(xsderrors.CodeUnsupportedRegex, "unsupported regex"),
	)
	expectCategoryCode(t, err, xsderrors.CategorySchemaParse, xsderrors.CodeSchemaXML)
	if !xsderrors.IsUnsupported(err) {
		t.Fatalf("xsderrors.IsUnsupported(%v) = false", err)
	}
	wrapped := fmt.Errorf("outer: %w", err)
	if !xsderrors.IsUnsupported(wrapped) {
		t.Fatalf("xsderrors.IsUnsupported(%v) = false", wrapped)
	}
	assertPublicErrorTree(t, err)
}

func assertPublicErrorTree(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("error is nil")
	}
	assertPublicErrorValue(t, err)
	if e, ok := errors.AsType[*xsderrors.Error](err); ok {
		if e.Err != nil {
			assertPublicErrorTree(t, e.Err)
		}
	}
	if e, ok := errors.AsType[xsderrors.Errors](err); ok {
		for _, child := range e {
			assertPublicErrorTree(t, child)
		}
	}
}

func assertPublicErrorValue(t *testing.T, err error) {
	t.Helper()
	typ := reflect.TypeOf(err)
	if typ == nil {
		t.Fatal("error type is nil")
	}
	if strings.Contains(typ.PkgPath(), "/internal/") {
		t.Fatalf("error type leaks internal package: %T", err)
	}
	if strings.Contains(err.Error(), "/internal/") {
		t.Fatalf("error string leaks internal package: %v", err)
	}
}
