package runtime

import (
	"strings"
	"testing"
)

func TestElementDeclByID(t *testing.T) {
	t.Parallel()

	decls := []ElementDecl{
		{Name: QName{Local: 1}, Type: SimpleRef(0)},
		{Name: QName{Local: 2}, Type: ComplexRef(0)},
	}
	got, ok := ElementDeclByID(decls, 1)
	if !ok || got.Name != decls[1].Name {
		t.Fatalf("ElementDeclByID(valid) = %+v, %v; want decl 1, true", got, ok)
	}
	if typ, ok := ElementTypeByID(decls, 1); !ok || typ != decls[1].Type {
		t.Fatalf("ElementTypeByID(valid) = %v, %v; want %v, true", typ, ok, decls[1].Type)
	}
	for _, id := range []ElementID{NoElement, 2} {
		got, ok := ElementDeclByID(decls, id)
		if ok || got != nil {
			t.Fatalf("ElementDeclByID(%d) = %+v, %v; want nil, false", id, got, ok)
		}
		typ, ok := ElementTypeByID(decls, id)
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
	limits := DeclRefLimits{
		SimpleTypeCount:  1,
		ComplexTypeCount: 1,
		ElementCount:     1,
	}
	tests := []struct {
		name    string
		wantErr string
		decl    ElementDeclValidation
	}{
		{
			name: "valid",
			decl: ElementDeclValidation{
				Name: validName,
				Type: SimpleRef(0),
			},
		},
		{
			name: "invalid name",
			decl: ElementDeclValidation{
				Name: QName{Namespace: 9, Local: 9},
				Type: SimpleRef(0),
			},
			wantErr: "element declaration references invalid name or type",
		},
		{
			name: "invalid type",
			decl: ElementDeclValidation{
				Name: validName,
				Type: ComplexRef(1),
			},
			wantErr: "element declaration references invalid name or type",
		},
		{
			name: "invalid block",
			decl: ElementDeclValidation{
				Name:  validName,
				Type:  SimpleRef(0),
				Block: DerivationList,
			},
			wantErr: "element declaration block mask contains invalid derivation",
		},
		{
			name: "invalid final",
			decl: ElementDeclValidation{
				Name:  validName,
				Type:  SimpleRef(0),
				Final: DerivationUnion,
			},
			wantErr: "element declaration final mask contains invalid derivation",
		},
		{
			name: "invalid substitution head",
			decl: ElementDeclValidation{
				Name:      validName,
				Type:      SimpleRef(0),
				SubstHead: 1,
			},
			wantErr: "element declaration references invalid substitution head",
		},
		{
			name: "default and fixed",
			decl: ElementDeclValidation{
				Name:       validName,
				Type:       SimpleRef(0),
				HasDefault: true,
				HasFixed:   true,
			},
			wantErr: "element declaration stores both default and fixed value constraints",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateElementDeclRuntime(&names, tt.decl, limits)
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
	limits := DeclRefLimits{SimpleTypeCount: 1}
	tests := []struct {
		name    string
		wantErr string
		decl    AttributeDeclValidation
	}{
		{
			name: "valid",
			decl: AttributeDeclValidation{Name: validName, Type: 0},
		},
		{
			name:    "invalid name",
			decl:    AttributeDeclValidation{Name: QName{Namespace: 9, Local: 9}, Type: 0},
			wantErr: "attribute declaration references invalid name or type",
		},
		{
			name:    "invalid type",
			decl:    AttributeDeclValidation{Name: validName, Type: 1},
			wantErr: "attribute declaration references invalid name or type",
		},
		{
			name:    "xmlns name",
			decl:    AttributeDeclValidation{Name: xmlnsName, Type: 0},
			wantErr: "attribute cannot be named xmlns",
		},
		{
			name:    "xsi namespace",
			decl:    AttributeDeclValidation{Name: xsiName, Type: 0},
			wantErr: "attribute target namespace cannot be XMLSchema-instance",
		},
		{
			name: "default and fixed",
			decl: AttributeDeclValidation{
				Name:       validName,
				Type:       0,
				HasDefault: true,
				HasFixed:   true,
			},
			wantErr: "attribute declaration stores both default and fixed value constraints",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateAttributeDeclRuntime(&names, tt.decl, limits)
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

	rt := declarationValueConstraintRuntimeStub{
		identity: map[SimpleTypeID]SimpleIdentityKind{
			0: SimpleIdentityNone,
			1: SimpleIdentityID,
			2: SimpleIdentityNone,
			3: SimpleIdentityNone,
			4: SimpleIdentityNone,
			5: SimpleIdentityNone,
		},
		types: map[SimpleTypeID]ValueConstraintSimpleType{
			0: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveString},
			1: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveString},
			2: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveNotation},
			3: {Variety: SimpleVarietyAtomic, Primitive: PrimitiveNotation, HasEnumeration: true},
			4: {Variety: SimpleVarietyList, ListItem: 2, Primitive: PrimitiveString},
			5: {Variety: SimpleVarietyUnion, Union: []SimpleTypeID{0, 2}, Primitive: PrimitiveString},
		},
	}
	tests := []struct {
		name       string
		wantErr    string
		typ        SimpleTypeID
		hasDefault bool
		hasFixed   bool
		nilRuntime bool
	}{
		{name: "absent constraint", typ: 1},
		{name: "mixed constraint", typ: NoSimpleType, hasDefault: true},
		{name: "non-ID default", typ: 0, hasDefault: true},
		{name: "ID default", typ: 1, hasDefault: true, wantErr: "ID-typed element declaration stores value constraint"},
		{name: "ID fixed", typ: 1, hasFixed: true, wantErr: "ID-typed element declaration stores value constraint"},
		{name: "bare notation default", typ: 2, hasDefault: true, wantErr: "NOTATION value constraint requires enumeration"},
		{name: "enumerated notation default", typ: 3, hasDefault: true},
		{name: "bare notation list item", typ: 4, hasDefault: true, wantErr: "NOTATION value constraint requires enumeration"},
		{name: "bare notation union member", typ: 5, hasDefault: true, wantErr: "NOTATION value constraint requires enumeration"},
		{name: "invalid value type", typ: 9, hasDefault: true, wantErr: "declaration value constraint references invalid type"},
		{name: "missing runtime", typ: 0, hasDefault: true, nilRuntime: true, wantErr: "declaration value constraint references invalid type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var runtime interface {
				SimpleTypeIdentityRuntime
				ValueConstraintRuntime
			} = rt
			if tt.nilRuntime {
				runtime = nil
			}
			err := ValidateElementDeclValueConstraintRuntime(runtime, tt.typ, tt.hasDefault, tt.hasFixed)
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

type declarationValueConstraintRuntimeStub struct {
	identity map[SimpleTypeID]SimpleIdentityKind
	types    map[SimpleTypeID]ValueConstraintSimpleType
}

func (s declarationValueConstraintRuntimeStub) SimpleTypeIdentity(id SimpleTypeID) (SimpleIdentityKind, bool) {
	identity, ok := s.identity[id]
	return identity, ok
}

func (s declarationValueConstraintRuntimeStub) ValueConstraintSimpleType(id SimpleTypeID) (ValueConstraintSimpleType, bool) {
	typ, ok := s.types[id]
	return typ, ok
}

func TestValidateAttributeDeclValueConstraintRuntime(t *testing.T) {
	t.Parallel()

	rt := simpleIdentityRuntimeStub{
		0: SimpleIdentityNone,
		1: SimpleIdentityID,
	}
	tests := []struct {
		name       string
		wantErr    string
		typ        SimpleTypeID
		hasDefault bool
		hasFixed   bool
		nilRuntime bool
	}{
		{name: "absent constraint", typ: 1},
		{name: "non-ID default", typ: 0, hasDefault: true},
		{name: "ID default", typ: 1, hasDefault: true, wantErr: "ID-typed attribute declaration stores value constraint"},
		{name: "ID fixed", typ: 1, hasFixed: true, wantErr: "ID-typed attribute declaration stores value constraint"},
		{name: "invalid value type", typ: 9, hasDefault: true, wantErr: "declaration value constraint references invalid type"},
		{name: "missing runtime", typ: 0, hasDefault: true, nilRuntime: true, wantErr: "declaration value constraint references invalid type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var runtime SimpleTypeIdentityRuntime = rt
			if tt.nilRuntime {
				runtime = nil
			}
			err := ValidateAttributeDeclValueConstraintRuntime(runtime, tt.typ, tt.hasDefault, tt.hasFixed)
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

func declarationNameTable(t *testing.T) NameTable {
	t.Helper()

	names, err := NewNameTable(8,
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
