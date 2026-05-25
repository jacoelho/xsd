package xsd

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"
)

func sourceBytes(name string, data []byte) SchemaSource {
	return SchemaSource{name: name, data: bytes.Clone(data)}
}

func mustCompile(t *testing.T, schema string) *Engine {
	t.Helper()
	engine, err := Compile(sourceBytes("schema.xsd", []byte(schema)))
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	return engine
}

func writeSchemaFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustValidate(t *testing.T, engine *Engine, doc string) {
	t.Helper()
	if err := engine.Validate(strings.NewReader(doc)); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func mustNotValidate(t *testing.T, engine *Engine, doc string, code ErrorCode) {
	t.Helper()
	err := engine.Validate(strings.NewReader(doc))
	if err == nil {
		t.Fatalf("Validate() expected error %s", code)
	}
	expectCode(t, err, code)
}

func TestNilReadersReturnStructuredErrors(t *testing.T) {
	engine := mustCompile(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"/></xs:schema>`)

	err := engine.Validate(nil)
	expectCategoryCode(t, err, ValidationErrorCategory, ErrValidationXML)

	err = engine.ValidateWithOptions(nil, ValidateOptions{})
	expectCategoryCode(t, err, ValidationErrorCategory, ErrValidationXML)

	_, err = Compile(Reader("schema.xsd", nil))
	expectCategoryCode(t, err, SchemaParseErrorCategory, ErrSchemaRead)
	if !strings.Contains(err.Error(), "nil schema reader") {
		t.Fatalf("Compile() error = %v, want nil schema reader", err)
	}
}

func expectCode(t *testing.T, err error, code ErrorCode) {
	t.Helper()
	x, ok := errors.AsType[*Error](err)
	if !ok {
		t.Fatalf("error %v is not *Error", err)
	}
	if x.Code != code {
		t.Fatalf("error code = %s, want %s; err=%v", x.Code, code, err)
	}
}

func expectCategoryCode(t *testing.T, err error, category ErrorCategory, code ErrorCode) {
	t.Helper()
	x, ok := errors.AsType[*Error](err)
	if !ok {
		t.Fatalf("error %v is not *Error", err)
	}
	if x.Category != category || x.Code != code {
		t.Fatalf("error = %s/%s, want %s/%s; err=%v", x.Category, x.Code, category, code, err)
	}
}
