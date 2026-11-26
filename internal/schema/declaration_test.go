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
				Name:                         validName,
				Type:                         SimpleRef(0),
				SubstitutionGroupAffiliation: 1,
			},
			wantErr: "element declaration references invalid substitution head",
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

func TestElementDeclarationValueConstraintIdentity(t *testing.T) {
	t.Parallel()
	fixture := notationConstraintFixture(t)
	rt := fixture.build
	stringID := valuepkg.BuiltinType(valuepkg.PrimitiveString)
	id, found := builtinTypeID("ID")
	if !found {
		t.Fatal("missing builtin ID")
	}
	tests := []struct {
		name       string
		typ        SimpleTypeID
		constraint *valueConstraint
		nilRuntime bool
		wantErr    string
	}{
		{name: "absent constraint", typ: id},
		{name: "mixed constraint", typ: NoSimpleType, constraint: &valueConstraint{}},
		{name: "non-ID default", typ: stringID, constraint: &valueConstraint{}},
		{name: "ID default", typ: id, constraint: &valueConstraint{}, wantErr: "ID-typed element declaration stores value constraint"},
		{name: "ID fixed", typ: id, constraint: &valueConstraint{Fixed: true}, wantErr: "ID-typed element declaration stores value constraint"},
		{name: "enumerated notation default", typ: fixture.enumerated, constraint: &valueConstraint{}},
		{name: "invalid value type", typ: 999, constraint: &valueConstraint{}, wantErr: "declaration value constraint references invalid type"},
		{name: "missing runtime", typ: stringID, constraint: &valueConstraint{}, nilRuntime: true, wantErr: "declaration value constraint references invalid type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runtime := rt
			if tt.nilRuntime {
				runtime = nil
			}
			err := validateValueConstraintIdentity(runtime, tt.typ, tt.constraint, valueConstraintOwnerElementDeclaration)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateValueConstraintIdentity() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateValueConstraintIdentity() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestAttributeDeclarationValueConstraintIdentity(t *testing.T) {
	t.Parallel()
	rt, idTypes := attributeUseSetIDFixture(t)
	stringID := valuepkg.BuiltinType(valuepkg.PrimitiveString)
	id, list, union := idTypes[0].id, idTypes[1].id, idTypes[2].id
	tests := []struct {
		name       string
		typ        SimpleTypeID
		constraint *valueConstraint
		nilRuntime bool
		wantErr    string
	}{
		{name: "absent constraint", typ: id},
		{name: "non-ID default", typ: stringID, constraint: &valueConstraint{}},
		{name: "ID default", typ: id, constraint: &valueConstraint{}, wantErr: "ID-typed attribute declaration stores value constraint"},
		{name: "ID fixed", typ: id, constraint: &valueConstraint{Fixed: true}, wantErr: "ID-typed attribute declaration stores value constraint"},
		{name: "list of ID default", typ: list, constraint: &valueConstraint{}, wantErr: "ID-typed attribute declaration stores value constraint"},
		{name: "union with ID fixed", typ: union, constraint: &valueConstraint{Fixed: true}, wantErr: "ID-typed attribute declaration stores value constraint"},
		{name: "invalid value type", typ: 999, constraint: &valueConstraint{}, wantErr: "declaration value constraint references invalid type"},
		{name: "missing runtime", typ: stringID, constraint: &valueConstraint{}, nilRuntime: true, wantErr: "declaration value constraint references invalid type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			runtime := rt
			if tt.nilRuntime {
				runtime = nil
			}
			err := validateValueConstraintIdentity(runtime, tt.typ, tt.constraint, valueConstraintOwnerAttributeDeclaration)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateValueConstraintIdentity() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateValueConstraintIdentity() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestSourceValueConstraint(t *testing.T) {
	t.Parallel()

	present := func(value string) lexicalAttribute { return lexicalAttribute{Value: value, Present: true} }
	tests := []struct {
		name        string
		defaultAttr lexicalAttribute
		fixedAttr   lexicalAttribute
		want        *valueConstraint
		wantKind    valueConstraintKind
	}{
		{name: "none", wantKind: valueConstraintNone},
		{name: "default", defaultAttr: present("d"), want: &valueConstraint{Lexical: "d"}, wantKind: valueConstraintDefault},
		{name: "fixed", fixedAttr: present("f"), want: &valueConstraint{Lexical: "f", Fixed: true}, wantKind: valueConstraintFixed},
		{name: "empty fixed", fixedAttr: present(""), want: &valueConstraint{Fixed: true}, wantKind: valueConstraintFixed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := sourceValueConstraint(&schemaNode{local: attributeChild}, tt.defaultAttr, tt.fixedAttr)
			if err != nil {
				t.Fatal(err)
			}
			if (got == nil) != (tt.want == nil) || got != nil && (got.Lexical != tt.want.Lexical || got.Fixed != tt.want.Fixed) {
				t.Fatalf("sourceValueConstraint() = %+v, want %+v", got, tt.want)
			}
			if got.kind() != tt.wantKind {
				t.Fatalf("constraint kind = %d, want %d", got.kind(), tt.wantKind)
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
