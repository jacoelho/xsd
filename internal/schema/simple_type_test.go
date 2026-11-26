package schema

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"testing"

	"github.com/jacoelho/xsd/internal/value"
)

func TestMissingSimpleType(t *testing.T) {
	t.Parallel()

	name := QName{Namespace: 1, Local: 2}
	got := missingSimpleType(name)
	if got.decl != (simpleType{Name: name, Missing: true}) {
		t.Fatalf("missingSimpleType().decl = %+v", got.decl)
	}
	if got.spec.Variety != value.Atomic ||
		got.spec.Primitive != value.PrimitiveString ||
		got.spec.Whitespace != value.WhitespaceCollapse ||
		!got.spec.WhitespacePresent ||
		got.spec.Base != value.BuiltinType(value.PrimitiveString) || got.spec.ListItem != value.NoType {
		t.Fatalf("missingSimpleType().spec = %+v", got.spec)
	}
	if missingSimpleTypeLocalName() != "missing" {
		t.Fatalf("MissingSimpleTypeLocalName() = %q", missingSimpleTypeLocalName())
	}
}

// The build stores only simpleType, so a declaration field of a type from
// internal/value would give schema a second copy of value-owned structure
// that compiler code could read after Builder.Complete.
func TestSimpleTypeDeclarationHoldsNoValueStructure(t *testing.T) {
	t.Parallel()

	file, err := goparser.ParseFile(token.NewFileSet(), "simple_type.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fields *ast.FieldList
	ast.Inspect(file, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Name.Name == "simpleType" {
			if st, ok := ts.Type.(*ast.StructType); ok {
				fields = st.Fields
			}
		}
		return fields == nil
	})
	if fields == nil {
		t.Fatal("simple_type.go declares no simpleType struct")
	}
	for _, field := range fields.List {
		ast.Inspect(field.Type, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.SelectorExpr:
				if pkg, ok := n.X.(*ast.Ident); ok && pkg.Name == "value" {
					t.Errorf("simpleType field %s has type from internal/value: value.%s", fieldNames(field), n.Sel.Name)
				}
			case *ast.Ident:
				if n.Name == "SimpleTypeID" {
					t.Errorf("simpleType field %s references SimpleTypeID, a value.TypeID alias", fieldNames(field))
				}
			}
			return true
		})
	}
}

func fieldNames(field *ast.Field) string {
	if len(field.Names) == 0 {
		return "(embedded)"
	}
	return field.Names[0].Name
}

func TestSimpleTypeByID(t *testing.T) {
	t.Parallel()

	types := []simpleType{
		{Name: QName{Local: 1}},
		{Name: QName{Local: 2}},
	}
	got, ok := simpleTypeByID(types, 1)
	if !ok || got.Name != types[1].Name {
		t.Fatalf("SimpleTypeByID(valid) = %+v, %v; want type 1, true", got, ok)
	}
	for _, id := range []SimpleTypeID{NoSimpleType, 2} {
		got, ok := simpleTypeByID(types, id)
		if ok || got != nil {
			t.Fatalf("SimpleTypeByID(%d) = %+v, %v; want nil, false", id, got, ok)
		}
	}
}

func TestUsableSimpleTypeRejectsMissingSentinel(t *testing.T) {
	t.Parallel()

	types := []simpleType{
		{Name: QName{Local: 1}},
		missingSimpleType(QName{Local: 2}).decl,
	}
	if got, ok := usableSimpleType(types, 0); !ok || got.Name != types[0].Name {
		t.Fatalf("UsableSimpleType(valid) = %+v, %v; want type 0, true", got, ok)
	}
	if got, ok := usableSimpleType(types, 1); ok || got != nil {
		t.Fatalf("UsableSimpleType(missing) = %+v, %v; want nil, false", got, ok)
	}
	if got, ok := usableSimpleType(types, NoSimpleType); ok || got != nil {
		t.Fatalf("UsableSimpleType(invalid) = %+v, %v; want nil, false", got, ok)
	}
}

func TestValidateSimpleTypeFinalAllows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		wantErr    string
		final      DerivationMask
		derivation DerivationMask
	}{
		{
			name:       "restriction allowed",
			derivation: DerivationRestriction,
		},
		{
			name:       "list allowed",
			derivation: derivationList,
		},
		{
			name:       "union allowed",
			derivation: derivationUnion,
		},
		{
			name:       "restriction blocked",
			final:      DerivationRestriction,
			derivation: DerivationRestriction,
			wantErr:    "simple type final blocks restriction",
		},
		{
			name:       "list blocked",
			final:      derivationList | derivationUnion,
			derivation: derivationList,
			wantErr:    "simple type final blocks list",
		},
		{
			name:       "union blocked",
			final:      derivationUnion,
			derivation: derivationUnion,
			wantErr:    "simple type final blocks union",
		},
		{
			name:       "invalid final mask",
			final:      DerivationExtension,
			derivation: DerivationRestriction,
			wantErr:    "simple type final mask is invalid",
		},
		{
			name:       "invalid derivation",
			derivation: DerivationExtension,
			wantErr:    "simple type final derivation is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateSimpleTypeFinalAllows(tt.final, tt.derivation)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateSimpleTypeFinalAllows() error = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("ValidateSimpleTypeFinalAllows() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}
