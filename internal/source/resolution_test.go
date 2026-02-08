package source

import (
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/parser"
	resolver "github.com/jacoelho/xsd/internal/semanticresolve"
	"github.com/jacoelho/xsd/internal/types"
)

func TestTypeResolution_SimpleType(t *testing.T) {
	// simple type base types resolve during semantic resolution.
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

	// resolve base types
	if err := resolver.NewResolver(schema).Resolve(); err != nil {
		t.Fatalf("resolver.Resolve failed: %v", err)
	}

	if derivedType.ResolvedBase == nil {
		t.Fatal("ResolvedBase should be set after semantic resolution")
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
	if attr.Type == nil {
		t.Fatalf("expected attribute type")
	}
	before := attr.Type

	key := loader.loadKey("main.xsd", types.NamespaceURI("urn:test"))
	loaded, ok := loader.state.loadedSchema(key)
	if !ok {
		t.Fatalf("loader state did not return cached schema")
	}
	ctLoaded, ok := loaded.TypeDefs[ctQName].(*types.ComplexType)
	if !ok || ctLoaded == nil || len(ctLoaded.Attributes()) == 0 {
		t.Fatalf("expected complex type %s after Load", ctQName)
	}
	if ctLoaded.Attributes()[0].Type != before {
		t.Fatalf("attribute type pointer changed after Load")
	}
}

func TestTypeResolution_ComplexType(t *testing.T) {
	// complex type base types resolve during semantic resolution.
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

	// resolve base types
	if err := resolver.NewResolver(schema).Resolve(); err != nil {
		t.Fatalf("resolver.Resolve failed: %v", err)
	}

	if derivedType.ResolvedBase == nil {
		t.Fatal("ResolvedBase should be set after semantic resolution")
	}
	if derivedType.BaseType() != baseType {
		t.Errorf("BaseType = %v, want %v", derivedType.BaseType(), baseType)
	}
}

func TestTypeResolution_ForwardReference(t *testing.T) {
	// forward references should resolve even when the base type is declared later.
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

	// resolve base types (should work even though B is defined after A)
	if err := resolver.NewResolver(schema).Resolve(); err != nil {
		t.Fatalf("resolver.Resolve failed: %v", err)
	}

	if typeA.ResolvedBase == nil {
		t.Fatal("Forward reference should be resolved")
	}
	if typeA.BaseType() != typeB {
		t.Errorf("BaseType = %v, want %v", typeA.BaseType(), typeB)
	}
}

func TestTypeResolution_CircularDependency(t *testing.T) {
	// circular dependencies must be detected
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

	// resolve should detect circular dependency
	err := resolver.NewResolver(schema).Resolve()
	if err == nil {
		t.Fatal("Should detect circular dependency")
	}
	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

func TestTypeResolution_MissingBaseType(t *testing.T) {
	// missing base types must be detected
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

	// resolve should detect missing base type
	err := resolver.NewResolver(schema).Resolve()
	if err == nil {
		t.Fatal("Should detect missing base type")
	}
}

func TestTypeResolution_ValidCircularUnion(t *testing.T) {
	// union types can have circular member references (this is valid in XSD)
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

	// resolve should not treat this as an invalid circular dependency
	err := resolver.NewResolver(schema).Resolve()
	if err != nil {
		t.Fatalf("resolver.Resolve should not fail for valid circular union: %v", err)
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
