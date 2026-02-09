package schemaflow

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantic"
)

func TestValidateUPARejectsUnresolvedSchema(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="MissingType"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	err = ValidateUPA(sch, &semantic.Registry{})
	if err == nil {
		t.Fatal("ValidateUPA() expected unresolved schema error")
	}
	if !strings.Contains(err.Error(), "unresolved placeholders") {
		t.Fatalf("ValidateUPA() error = %v, want unresolved placeholders", err)
	}
}

func TestValidateUPAOnResolvedSchema(t *testing.T) {
	sch, err := parser.Parse(strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	resolved, err := ResolveAndValidate(sch)
	if err != nil {
		t.Fatalf("ResolveAndValidate() error = %v", err)
	}
	reg, err := semantic.AssignIDs(resolved)
	if err != nil {
		t.Fatalf("AssignIDs() error = %v", err)
	}
	if err := ValidateUPA(resolved, reg); err != nil {
		t.Fatalf("ValidateUPA() error = %v", err)
	}
}
