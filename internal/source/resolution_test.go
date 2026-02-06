package source

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/parser"
	resolver "github.com/jacoelho/xsd/internal/semanticresolve"
	"github.com/jacoelho/xsd/internal/types"
)

func TestTwoPhaseResolution_SimpleType(t *testing.T) {
	// test that simple type base types are resolved in phase 2
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	baseType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
	}
	schema.TypeDefs[baseType.QName] = baseType

	// create derived type with QName reference (not yet resolved)
	derivedType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		Restriction: &types.Restriction{
			Base: baseType.QName,
		},
		// BaseType is nil initially (not resolved)
	}
	schema.TypeDefs[derivedType.QName] = derivedType

	// phase 2: Resolve base types
	if err := resolver.ResolveTypeReferences(schema); err != nil {
		t.Fatalf("resolver.ResolveTypeReferences failed: %v", err)
	}

	if derivedType.ResolvedBase == nil {
		t.Fatal("BaseType should be resolved after phase 2")
	}
	if derivedType.BaseType() != baseType {
		t.Errorf("BaseType = %v, want %v", derivedType.BaseType(), baseType)
	}
}

func TestLoadCachesSchema(t *testing.T) {
	fs := fstest.MapFS{
		"main.xsd": {
			Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           xmlns:tns="urn:test"
           targetNamespace="urn:test">
  <xs:simpleType name="IDT">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
  <xs:complexType name="CT">
    <xs:attribute name="a" type="tns:IDT"/>
  </xs:complexType>
</xs:schema>`),
		},
	}

	loader := NewLoader(Config{FS: fs})
	schema, err := loader.Load("main.xsd")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	ctQName := types.QName{Namespace: "urn:test", Local: "CT"}
	ct, ok := schema.TypeDefs[ctQName].(*types.ComplexType)
	if !ok || ct == nil || len(ct.Attributes()) == 0 {
		t.Fatalf("expected complex type %s with attributes", ctQName)
	}

	attr := ct.Attributes()[0]
	if st, ok := attr.Type.(*types.SimpleType); !ok || types.IsPlaceholderSimpleType(st) {
		t.Fatalf("expected resolved attribute type, got %T", attr.Type)
	}
	before := attr.Type

	loaded, ok := loader.GetLoaded("main.xsd", types.NamespaceURI("urn:test"))
	if !ok {
		t.Fatalf("GetLoaded did not return cached schema")
	}
	ctLoaded, ok := loaded.TypeDefs[ctQName].(*types.ComplexType)
	if !ok || ctLoaded == nil || len(ctLoaded.Attributes()) == 0 {
		t.Fatalf("expected complex type %s after Load", ctQName)
	}
	if ctLoaded.Attributes()[0].Type != before {
		t.Fatalf("attribute type pointer changed after Load")
	}
}

func TestTwoPhaseResolution_ComplexType(t *testing.T) {
	// test that complex type base types are resolved in phase 2
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	baseType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		// content set via SetContent
	}
	baseType.SetContent(&types.ElementContent{})
	schema.TypeDefs[baseType.QName] = baseType

	derivedType := &types.ComplexType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		// content set via SetContent
		// BaseType is nil initially
	}
	derivedType.SetContent(&types.ComplexContent{
		Base: baseType.QName,
		Extension: &types.Extension{
			Base: baseType.QName,
		},
	})
	schema.TypeDefs[derivedType.QName] = derivedType

	// phase 2: Resolve base types
	if err := resolver.ResolveTypeReferences(schema); err != nil {
		t.Fatalf("resolver.ResolveTypeReferences failed: %v", err)
	}

	if derivedType.ResolvedBase == nil {
		t.Fatal("BaseType should be resolved after phase 2")
	}
	if derivedType.BaseType() != baseType {
		t.Errorf("BaseType = %v, want %v", derivedType.BaseType(), baseType)
	}
}

func TestTwoPhaseResolution_ForwardReference(t *testing.T) {
	// test that forward references work (type A can reference type B defined later)
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// create type A that references type B (forward reference)
	typeA := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "TypeA",
		},
		Restriction: &types.Restriction{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "TypeB",
			},
		},
	}
	schema.TypeDefs[typeA.QName] = typeA

	// create type B (defined after type A)
	typeB := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "TypeB",
		},
	}
	schema.TypeDefs[typeB.QName] = typeB

	// phase 2: Resolve base types (should work even though B is defined after A)
	if err := resolver.ResolveTypeReferences(schema); err != nil {
		t.Fatalf("resolver.ResolveTypeReferences failed: %v", err)
	}

	if typeA.ResolvedBase == nil {
		t.Fatal("Forward reference should be resolved")
	}
	if typeA.BaseType() != typeB {
		t.Errorf("BaseType = %v, want %v", typeA.BaseType(), typeB)
	}
}

func TestTwoPhaseResolution_CircularDependency(t *testing.T) {
	// test that circular dependencies are detected
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// type A references Type B
	typeA := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "TypeA",
		},
		Restriction: &types.Restriction{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "TypeB",
			},
		},
	}
	schema.TypeDefs[typeA.QName] = typeA

	// type B references Type A (circular)
	typeB := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "TypeB",
		},
		Restriction: &types.Restriction{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "TypeA",
			},
		},
	}
	schema.TypeDefs[typeB.QName] = typeB

	// phase 2: Should detect circular dependency
	err := resolver.ResolveTypeReferences(schema)
	if err == nil {
		t.Fatal("Should detect circular dependency")
	}
	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

func TestTwoPhaseResolution_MissingBaseType(t *testing.T) {
	// test that missing base types are detected
	schema := &parser.Schema{
		TargetNamespace: "http://example.com",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// type that references non-existent base type
	derivedType := &types.SimpleType{
		QName: types.QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		Restriction: &types.Restriction{
			Base: types.QName{
				Namespace: "http://example.com",
				Local:     "NonExistentType",
			},
		},
	}
	schema.TypeDefs[derivedType.QName] = derivedType

	// phase 2: Should detect missing base type
	err := resolver.ResolveTypeReferences(schema)
	if err == nil {
		t.Fatal("Should detect missing base type")
	}
}

func TestTwoPhaseResolution_ValidCircularUnion(t *testing.T) {
	// test that union types can have circular member references (this is valid in XSD)
	// this is based on MS-SimpleType2006-07-15/ste110 test case
	schema := &parser.Schema{
		TargetNamespace: "",
		TypeDefs:        make(map[types.QName]types.Type),
	}

	// type st is a union with member types: xsd:int, xsd:string, and st2
	st := &types.SimpleType{
		QName: types.QName{
			Namespace: "",
			Local:     "st",
		},
	}
	st.Union = &types.UnionType{
		MemberTypes: []types.QName{
			{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "int"},
			{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "string"},
			{Namespace: "", Local: "st2"},
		},
	}
	schema.TypeDefs[st.QName] = st

	// type st2 is a union with member type: st (circular reference)
	st2 := &types.SimpleType{
		QName: types.QName{
			Namespace: "",
			Local:     "st2",
		},
	}
	st2.Union = &types.UnionType{
		MemberTypes: []types.QName{
			{Namespace: "", Local: "st"},
		},
	}
	schema.TypeDefs[st2.QName] = st2

	// phase 2: Should NOT detect this as a circular dependency (union circular references are valid)
	err := resolver.ResolveTypeReferences(schema)
	if err != nil {
		t.Fatalf("resolver.ResolveTypeReferences should not fail for valid circular union: %v", err)
	}

	if len(st.MemberTypes) != 3 {
		t.Fatalf("st.MemberTypes should have 3 members, got %d", len(st.MemberTypes))
	}
	if len(st2.MemberTypes) != 1 {
		t.Fatalf("st2.MemberTypes should have 1 member, got %d", len(st2.MemberTypes))
	}
	if st2.MemberTypes[0] != st {
		t.Errorf("st2.MemberTypes[0] should be st, got %v", st2.MemberTypes[0])
	}
}
