package validator

import (
	"testing"
	"testing/fstest"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/loader"
)

func TestValidatorReuseWithSchemaLocationHints(t *testing.T) {
	baseSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:base"
           elementFormDefault="qualified">
  <xs:element name="baseRoot" type="xs:string"/>
</xs:schema>`

	hintSchema := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:hint"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fs := fstest.MapFS{
		"base.xsd": {Data: []byte(baseSchema)},
		"hint.xsd": {Data: []byte(hintSchema)},
	}

	l := loader.NewLoader(loader.Config{FS: fs})
	compiled, err := l.LoadCompiled("base.xsd")
	if err != nil {
		t.Fatalf("load compiled schema: %v", err)
	}

	v := New(compiled, WithSchemaLocationLoader(l))

	docWithHint := `<?xml version="1.0"?>
<root xmlns="urn:hint"
      xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
      xsi:schemaLocation="urn:hint hint.xsd">value</root>`
	violations := validateStream(t, v, docWithHint)
	if len(violations) > 0 {
		t.Fatalf("expected no violations with schemaLocation hint, got %d: %s", len(violations), violations[0].Error())
	}

	docWithoutHint := `<?xml version="1.0"?>
<root xmlns="urn:hint">value</root>`
	violations = validateStream(t, v, docWithoutHint)
	if len(violations) == 0 {
		t.Fatal("expected violations without schemaLocation hint, got none")
	}
	if !hasValidationCode(violations, xsderrors.ErrElementNotDeclared) {
		t.Fatalf("expected ErrElementNotDeclared without hint, got %s", violations[0].Error())
	}
}

func hasValidationCode(violations []xsderrors.Validation, code xsderrors.ErrorCode) bool {
	for _, violation := range violations {
		if violation.Code == string(code) {
			return true
		}
	}
	return false
}
