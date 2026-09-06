package schema_test

import (
	"errors"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"

	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/validate"
	"github.com/jacoelho/xsd/xsderrors"
)

func mustCompile(t *testing.T, schema string) *xsdSchema.Schema {
	t.Helper()
	rt, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{
		source.Bytes("schema.xsd", []byte(schema)),
	})

	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	return rt
}

func engineRuntime(tb testing.TB, rt *xsdSchema.Schema) *xsdSchema.Schema {
	tb.Helper()
	if rt == nil {
		tb.Fatal("nil runtime")
		return nil
	}
	return rt
}

func mustValidate(t *testing.T, rt *xsdSchema.Schema, doc string) {
	t.Helper()
	if err := validateWithRuntime(rt, doc); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func mustNotValidate(t *testing.T, rt *xsdSchema.Schema, doc string, code xsderrors.Code) {
	t.Helper()
	err := validateWithRuntime(rt, doc)
	if err == nil {
		t.Fatalf("Validate() expected error %s", code)
	}
	expectCode(t, err, code)
}

func validateWithRuntime(rt *xsdSchema.Schema, doc string) error {
	session, err := validate.NewSession(rt, validate.Options{})
	if err != nil {
		return err
	}
	return session.Validate(strings.NewReader(doc))
}

func expectCode(t *testing.T, err error, code xsderrors.Code) {
	t.Helper()
	x, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error %v is not *xsderrors.Error", err)
	}
	if x.Code() != code {
		t.Fatalf("error code = %s, want %s; err=%v", x.Code(), code, err)
	}
}
