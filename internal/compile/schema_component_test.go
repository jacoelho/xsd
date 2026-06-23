package compile

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestAddSchemaComponentRejectsDuplicate(t *testing.T) {
	t.Parallel()

	name := runtime.QName{Namespace: 1, Local: 2}
	components := map[runtime.QName]string{}
	if err := AddSchemaComponent(components, name, "first", "p:thing"); err != nil {
		t.Fatalf("AddSchemaComponent(first) error = %v", err)
	}
	if got := components[name]; got != "first" {
		t.Fatalf("component = %q, want first", got)
	}

	err := AddSchemaComponent(components, name, "second", "p:thing")
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("AddSchemaComponent(duplicate) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != xsderrors.CodeSchemaDuplicate {
		t.Fatalf("diagnostic = %s/%s, want schema compile duplicate", xerr.Category, xerr.Code)
	}
	if !strings.Contains(xerr.Message, "duplicate schema component p:thing") {
		t.Fatalf("message = %q, want duplicate component label", xerr.Message)
	}
	if got := components[name]; got != "first" {
		t.Fatalf("duplicate replaced component with %q", got)
	}
}

func TestCheckSchemaTypeNameAvailableRejectsDuplicate(t *testing.T) {
	t.Parallel()

	if err := CheckSchemaTypeNameAvailable(false, "p:Thing"); err != nil {
		t.Fatalf("CheckSchemaTypeNameAvailable(false) error = %v", err)
	}

	err := CheckSchemaTypeNameAvailable(true, "p:Thing")
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("CheckSchemaTypeNameAvailable(true) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != xsderrors.CodeSchemaDuplicate {
		t.Fatalf("diagnostic = %s/%s, want schema compile duplicate", xerr.Category, xerr.Code)
	}
	if !strings.Contains(xerr.Message, "duplicate type p:Thing") {
		t.Fatalf("message = %q, want duplicate type label", xerr.Message)
	}
}

func TestAddGlobalAttributeComponentDefersToBuiltin(t *testing.T) {
	t.Parallel()

	name := runtime.QName{Namespace: 2, Local: 4}
	components := map[runtime.QName]string{}
	globals := map[runtime.QName]runtime.AttributeID{name: 7}
	if err := AddGlobalAttributeComponent(components, globals, name, "schema", "xml:lang"); err != nil {
		t.Fatalf("AddGlobalAttributeComponent(builtin) error = %v", err)
	}
	if _, exists := components[name]; exists {
		t.Fatal("builtin attribute redeclaration was stored as a schema component")
	}
}

func TestAddGlobalAttributeComponentRejectsSchemaDuplicate(t *testing.T) {
	t.Parallel()

	name := runtime.QName{Namespace: 2, Local: 4}
	components := map[runtime.QName]string{name: "first"}
	globals := map[runtime.QName]runtime.AttributeID{}

	err := AddGlobalAttributeComponent(components, globals, name, "second", "p:attr")
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("AddGlobalAttributeComponent(duplicate) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != xsderrors.CodeSchemaDuplicate {
		t.Fatalf("diagnostic = %s/%s, want schema compile duplicate", xerr.Category, xerr.Code)
	}
	if !strings.Contains(xerr.Message, "duplicate schema component p:attr") {
		t.Fatalf("message = %q, want duplicate component label", xerr.Message)
	}
	if got := components[name]; got != "first" {
		t.Fatalf("duplicate replaced component with %q", got)
	}
}

func TestCheckSchemaComponentCycle(t *testing.T) {
	t.Parallel()

	if err := CheckSchemaComponentCycle(SchemaComponentSimpleType, false, "p:T"); err != nil {
		t.Fatalf("CheckSchemaComponentCycle(false) error = %v", err)
	}
	tests := []struct {
		name string
		kind SchemaComponentKind
		want string
	}{
		{name: "simple type", kind: SchemaComponentSimpleType, want: "cyclic simple type p:T"},
		{name: "complex type", kind: SchemaComponentComplexType, want: "cyclic complex type p:T"},
		{name: "attribute", kind: SchemaComponentAttribute, want: "cyclic attribute declaration p:T"},
		{name: "element", kind: SchemaComponentElement, want: "cyclic element declaration p:T"},
		{name: "attribute group", kind: SchemaComponentAttributeGroup, want: "cyclic attribute group p:T"},
		{name: "model group", kind: SchemaComponentModelGroup, want: "cyclic model group p:T"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := CheckSchemaComponentCycle(tt.kind, true, "p:T")
			xerr, ok := errors.AsType[*xsderrors.Error](err)
			if !ok {
				t.Fatalf("CheckSchemaComponentCycle(true) error = %T %v, want *xsderrors.Error", err, err)
			}
			if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != xsderrors.CodeSchemaReference {
				t.Fatalf("diagnostic = %s/%s, want schema compile reference", xerr.Category, xerr.Code)
			}
			if xerr.Message != tt.want {
				t.Fatalf("message = %q, want %q", xerr.Message, tt.want)
			}
		})
	}
}

func TestCheckSchemaComponentRecursion(t *testing.T) {
	t.Parallel()

	if err := CheckSchemaComponentRecursion(SchemaComponentModelGroup, false, "p:g"); err != nil {
		t.Fatalf("CheckSchemaComponentRecursion(false) error = %v", err)
	}

	err := CheckSchemaComponentRecursion(SchemaComponentModelGroup, true, "p:g")
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("CheckSchemaComponentRecursion(true) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != xsderrors.CodeSchemaReference {
		t.Fatalf("diagnostic = %s/%s, want schema compile reference", xerr.Category, xerr.Code)
	}
	if xerr.Message != "recursive model group p:g" {
		t.Fatalf("message = %q, want recursive model group label", xerr.Message)
	}

	err = CheckSchemaComponentRecursion(SchemaComponentModelGroup, true, "")
	xerr, ok = errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("CheckSchemaComponentRecursion(unlabeled) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Message != "recursive model group" {
		t.Fatalf("message = %q, want unlabeled recursive model group", xerr.Message)
	}
}

func TestCheckSchemaComponentExists(t *testing.T) {
	t.Parallel()

	if err := CheckSchemaComponentExists(SchemaComponentElement, true, "p:e"); err != nil {
		t.Fatalf("CheckSchemaComponentExists(true) error = %v", err)
	}

	err := CheckSchemaComponentExists(SchemaComponentElement, false, "p:missing")
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("CheckSchemaComponentExists(false) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != xsderrors.CodeSchemaReference {
		t.Fatalf("diagnostic = %s/%s, want schema compile reference", xerr.Category, xerr.Code)
	}
	if xerr.Message != "unknown element p:missing" {
		t.Fatalf("message = %q, want unknown element label", xerr.Message)
	}

	err = CheckSchemaComponentExists(SchemaComponentAttributeGroup, false, "p:attrs")
	xerr, ok = errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("CheckSchemaComponentExists(attribute group) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Message != "unknown attribute group p:attrs" {
		t.Fatalf("message = %q, want unknown attribute group label", xerr.Message)
	}

	err = CheckSchemaComponentExists(SchemaComponentType, false, "p:T")
	xerr, ok = errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("CheckSchemaComponentExists(type) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Message != "unknown type p:T" {
		t.Fatalf("message = %q, want unknown type label", xerr.Message)
	}
}

func TestAddNotationRejectsDuplicate(t *testing.T) {
	t.Parallel()

	name := runtime.QName{Namespace: 2, Local: 3}
	notations := map[runtime.QName]bool{}
	if err := AddNotation(notations, name, "p:notation"); err != nil {
		t.Fatalf("AddNotation(first) error = %v", err)
	}
	if !notations[name] {
		t.Fatal("AddNotation(first) did not store notation")
	}

	err := AddNotation(notations, name, "p:notation")
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("AddNotation(duplicate) error = %T %v, want *xsderrors.Error", err, err)
	}
	if xerr.Category != xsderrors.CategorySchemaCompile || xerr.Code != xsderrors.CodeSchemaDuplicate {
		t.Fatalf("diagnostic = %s/%s, want schema compile duplicate", xerr.Category, xerr.Code)
	}
	if !strings.Contains(xerr.Message, "duplicate notation p:notation") {
		t.Fatalf("message = %q, want duplicate notation label", xerr.Message)
	}
}
