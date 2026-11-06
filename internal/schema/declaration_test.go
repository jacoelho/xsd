package schema

import (
	"strings"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
)

func TestElementDeclByID(t *testing.T) {
	t.Parallel()

	decls := []elementDecl{
		{Name: QName{Local: 1}, Type: SimpleRef(0)},
		{Name: QName{Local: 2}, Type: ComplexRef(0)},
	}
	got, ok := elementDeclByID(decls, 1)
	if !ok || got.Name != decls[1].Name {
		t.Fatalf("ElementDeclByID(valid) = %+v, %v; want decl 1, true", got, ok)
	}
	if typ, ok := elementTypeByID(decls, 1); !ok || typ != decls[1].Type {
		t.Fatalf("ElementTypeByID(valid) = %v, %v; want %v, true", typ, ok, decls[1].Type)
	}
	for _, id := range []ElementID{NoElement, 2} {
		got, ok := elementDeclByID(decls, id)
		if ok || got != nil {
			t.Fatalf("ElementDeclByID(%d) = %+v, %v; want nil, false", id, got, ok)
		}
		typ, ok := elementTypeByID(decls, id)
		if ok || typ != (TypeID{}) {
			t.Fatalf("ElementTypeByID(%d) = %v, %v; want zero, false", id, typ, ok)
		}
	}
}

func TestValidateElementDeclRuntime(t *testing.T) {
	t.Parallel()

	names := declarationNameTable(t)
	validName, ok := names.LookupQName("", "item")
	if !ok {
		t.Fatal("missing item QName")
	}
	limits := declRefLimits{
		SimpleTypeCount:  1,
		ComplexTypeCount: 1,
		ElementCount:     1,
	}
	tests := []struct {
		name    string
		wantErr string
		decl    elementDecl
	}{
		{
			name: "valid",
			decl: elementDecl{
				Name: validName,
				Type: SimpleRef(0),
			},
		},
		{
			name: "invalid name",
			decl: elementDecl{
				Name: QName{Namespace: 9, Local: 9},
				Type: SimpleRef(0),
			},
			wantErr: "element declaration references invalid name or type",
		},
		{
			name: "invalid type",
			decl: elementDecl{
				Name: validName,
				Type: ComplexRef(1),
			},
			wantErr: "element declaration references invalid name or type",
		},
		{
			name: "invalid block",
			decl: elementDecl{
				Name:  validName,
				Type:  SimpleRef(0),
				Block: derivationList,
			},
			wantErr: "element declaration block mask contains invalid derivation",
		},
		{
			name: "invalid final",
			decl: elementDecl{
				Name:  validName,
				Type:  SimpleRef(0),
				Final: derivationUnion,
			},
			wantErr: "element declaration final mask contains invalid derivation",
		},
		{
			name: "invalid substitution head",
			decl: elementDecl{
				Name:      validName,
				Type:      SimpleRef(0),
				SubstHead: 1,
			},
			wantErr: "element declaration references invalid substitution head",
		},
		{
			name: "default and fixed",
			decl: elementDecl{
				Name:    validName,
				Type:    SimpleRef(0),
				Default: &valueConstraint{},
				Fixed:   &valueConstraint{},
			},
			wantErr: "element declaration stores both default and fixed value constraints",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateElementDeclRuntime(&names, tt.decl, limits)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateElementDeclRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateElementDeclRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAttributeDeclRuntime(t *testing.T) {
	t.Parallel()

	names := declarationNameTable(t)
	validName, ok := names.LookupQName("", "item")
	if !ok {
		t.Fatal("missing item QName")
	}
	xmlnsName, ok := names.LookupQName("", "xmlns")
	if !ok {
		t.Fatal("missing xmlns QName")
	}
	xsiName, ok := names.LookupQName(XSINamespaceURI, "bad")
	if !ok {
		t.Fatal("missing xsi QName")
	}
	limits := declRefLimits{SimpleTypeCount: 1}
	tests := []struct {
		name    string
		wantErr string
		decl    attributeDecl
	}{
		{
			name: "valid",
			decl: attributeDecl{Name: validName, Type: 0},
		},
		{
			name:    "invalid name",
			decl:    attributeDecl{Name: QName{Namespace: 9, Local: 9}, Type: 0},
			wantErr: "attribute declaration references invalid name or type",
		},
		{
			name:    "invalid type",
			decl:    attributeDecl{Name: validName, Type: 1},
			wantErr: "attribute declaration references invalid name or type",
		},
		{
			name:    "xmlns name",
			decl:    attributeDecl{Name: xmlnsName, Type: 0},
			wantErr: "attribute cannot be named xmlns",
		},
		{
			name:    "xsi namespace",
			decl:    attributeDecl{Name: xsiName, Type: 0},
			wantErr: "attribute target namespace cannot be XMLSchema-instance",
		},
		{
			name: "default and fixed",
			decl: attributeDecl{
				Name:    validName,
				Type:    0,
				Default: &valueConstraint{},
				Fixed:   &valueConstraint{},
			},
			wantErr: "attribute declaration stores both default and fixed value constraints",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAttributeDeclRuntime(&names, tt.decl, limits)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateAttributeDeclRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAttributeDeclRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateElementDeclValueConstraintRuntime(t *testing.T) {
	t.Parallel()
	fixture := notationConstraintFixture(t)
	rt := fixture.build
	stringID := valuepkg.BuiltinType(valuepkg.PrimitiveString)
	id, found := valuepkg.BuiltinTypeID("ID")
	if !found {
		t.Fatal("missing builtin ID")
	}
	tests := []struct {
		name       string
		typ        SimpleTypeID
		constraint declarationValueConstraint
		nilRuntime bool
		wantErr    string
	}{
		{name: "absent constraint", typ: id},
		{name: "mixed constraint", typ: NoSimpleType, constraint: declarationValueConstraintDefault},
		{name: "non-ID default", typ: stringID, constraint: declarationValueConstraintDefault},
		{name: "ID default", typ: id, constraint: declarationValueConstraintDefault, wantErr: "ID-typed element declaration stores value constraint"},
		{name: "ID fixed", typ: id, constraint: declarationValueConstraintFixed, wantErr: "ID-typed element declaration stores value constraint"},
		{name: "enumerated notation default", typ: fixture.enumerated, constraint: declarationValueConstraintDefault},
		{name: "invalid value type", typ: 999, constraint: declarationValueConstraintDefault, wantErr: "declaration value constraint references invalid type"},
		{name: "missing runtime", typ: stringID, constraint: declarationValueConstraintDefault, nilRuntime: true, wantErr: "declaration value constraint references invalid type"},
		{name: "conflicting constraint", typ: stringID, constraint: declarationValueConstraintConflict, wantErr: "declaration stores conflicting value constraints"},
		{name: "unknown constraint", typ: stringID, constraint: declarationValueConstraint(99), wantErr: "declaration value constraint kind is unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runtime := rt
			if tt.nilRuntime {
				runtime = nil
			}
			err := validateElementDeclValueConstraintRuntime(runtime, tt.typ, tt.constraint)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateElementDeclValueConstraintRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateElementDeclValueConstraintRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAttributeDeclValueConstraintRuntime(t *testing.T) {
	t.Parallel()
	rt, idTypes := attributeUseSetIDFixture(t)
	stringID := valuepkg.BuiltinType(valuepkg.PrimitiveString)
	id, list, union := idTypes[0].id, idTypes[1].id, idTypes[2].id
	tests := []struct {
		name       string
		typ        SimpleTypeID
		constraint declarationValueConstraint
		nilRuntime bool
		wantErr    string
	}{
		{name: "absent constraint", typ: id},
		{name: "non-ID default", typ: stringID, constraint: declarationValueConstraintDefault},
		{name: "ID default", typ: id, constraint: declarationValueConstraintDefault, wantErr: "ID-typed attribute declaration stores value constraint"},
		{name: "ID fixed", typ: id, constraint: declarationValueConstraintFixed, wantErr: "ID-typed attribute declaration stores value constraint"},
		{name: "list of ID default", typ: list, constraint: declarationValueConstraintDefault, wantErr: "ID-typed attribute declaration stores value constraint"},
		{name: "union with ID fixed", typ: union, constraint: declarationValueConstraintFixed, wantErr: "ID-typed attribute declaration stores value constraint"},
		{name: "invalid value type", typ: 999, constraint: declarationValueConstraintDefault, wantErr: "declaration value constraint references invalid type"},
		{name: "missing runtime", typ: stringID, constraint: declarationValueConstraintDefault, nilRuntime: true, wantErr: "declaration value constraint references invalid type"},
		{name: "conflicting constraint", typ: stringID, constraint: declarationValueConstraintConflict, wantErr: "declaration stores conflicting value constraints"},
		{name: "unknown constraint", typ: stringID, constraint: declarationValueConstraint(99), wantErr: "declaration value constraint kind is unknown"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runtime := rt
			if tt.nilRuntime {
				runtime = nil
			}
			err := validateAttributeDeclValueConstraintRuntime(runtime, tt.typ, tt.constraint)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateAttributeDeclValueConstraintRuntime() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAttributeDeclValueConstraintRuntime() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestDeclarationValueConstraintOf(t *testing.T) {
	t.Parallel()

	value := 1
	tests := []struct {
		name         string
		defaultValue *int
		fixedValue   *int
		want         declarationValueConstraint
	}{
		{name: "none", want: declarationValueConstraintNone},
		{name: "default", defaultValue: &value, want: declarationValueConstraintDefault},
		{name: "fixed", fixedValue: &value, want: declarationValueConstraintFixed},
		{name: "conflict", defaultValue: &value, fixedValue: &value, want: declarationValueConstraintConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := declarationValueConstraintOf(tt.defaultValue, tt.fixedValue); got != tt.want {
				t.Fatalf("DeclarationValueConstraintOf() = %d, want %d", got, tt.want)
			}
		})
	}
}

func declarationNameTable(t *testing.T) nameTable {
	t.Helper()

	names, err := newNameTable(8,
		[]string{EmptyNamespaceURI, XSINamespaceURI},
		[]ExpandedName{
			{Local: "item"},
			{Local: "xmlns"},
			{Namespace: XSINamespaceURI, Local: "bad"},
		},
	)
	if err != nil {
		t.Fatalf("NewNameTable() error = %v", err)
	}
	return names
}
