package model

import (
	"testing"
)

func TestSimpleType_IsDerivedFrom(t *testing.T) {
	// create a type hierarchy: decimal -> integer -> long
	decimalType := mustBuiltinSimpleType(t, TypeNameDecimal)
	integerType := mustBuiltinSimpleType(t, TypeNameInteger)
	integerType.ResolvedBase = decimalType
	longType := mustBuiltinSimpleType(t, TypeNameLong)
	longType.ResolvedBase = integerType

	// test direct derivation
	if !IsDerivedFrom(longType, integerType) {
		t.Error("long should be derived from integer")
	}

	// test indirect derivation
	if !IsDerivedFrom(longType, decimalType) {
		t.Error("long should be derived from decimal (indirectly)")
	}

	// test not derived
	if IsDerivedFrom(longType, longType) {
		t.Error("type should not be derived from itself")
	}

	// test unrelated type
	unrelatedType := mustBuiltinSimpleType(t, TypeNameString)
	if IsDerivedFrom(longType, unrelatedType) {
		t.Error("long should not be derived from string")
	}
}

func TestComplexType_IsDerivedFrom(t *testing.T) {
	// create a type hierarchy: BaseType -> DerivedType
	baseType := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		// content set via SetContent
	}
	baseType.SetContent(&ElementContent{})

	derivedType := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		// content set via SetContent
	}
	derivedType.SetContent(&ComplexContent{
		Base: baseType.QName,
		Extension: &Extension{
			Base: baseType.QName,
		},
	})
	derivedType.ResolvedBase = baseType

	// test direct derivation
	if !IsDerivedFrom(derivedType, baseType) {
		t.Error("DerivedType should be derived from BaseType")
	}

	// test not derived
	if IsDerivedFrom(derivedType, derivedType) {
		t.Error("type should not be derived from itself")
	}
}

func TestSimpleType_GetDerivationChain(t *testing.T) {
	// create a type hierarchy: decimal -> integer -> long
	decimalType := mustBuiltinSimpleType(t, TypeNameDecimal)
	integerType := mustBuiltinSimpleType(t, TypeNameInteger)
	integerType.ResolvedBase = decimalType
	longType := mustBuiltinSimpleType(t, TypeNameLong)
	longType.ResolvedBase = integerType

	// test derivation chain
	chain := GetDerivationChain(longType)
	if len(chain) != 2 {
		t.Fatalf("GetDerivationChain() length = %d, want 2", len(chain))
	}
	if chain[0] != integerType {
		t.Errorf("chain[0] = %v, want %v", chain[0], integerType)
	}
	if chain[1] != decimalType {
		t.Errorf("chain[1] = %v, want %v", chain[1], decimalType)
	}

	// test primitive type (no base)
	primitiveChain := GetDerivationChain(decimalType)
	if len(primitiveChain) != 0 {
		t.Errorf("primitive type chain length = %d, want 0", len(primitiveChain))
	}
}

func TestComplexType_GetDerivationChain(t *testing.T) {
	// create a type hierarchy: BaseType -> DerivedType -> FurtherDerived
	baseType := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		// content set via SetContent
	}
	baseType.SetContent(&ElementContent{})

	derivedType := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		// content set via SetContent
	}
	derivedType.SetContent(&ComplexContent{
		Base: baseType.QName,
		Extension: &Extension{
			Base: baseType.QName,
		},
	})
	derivedType.ResolvedBase = baseType

	furtherDerived := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "FurtherDerived",
		},
		// content set via SetContent
	}
	furtherDerived.SetContent(&ComplexContent{
		Base: derivedType.QName,
		Extension: &Extension{
			Base: derivedType.QName,
		},
	})
	furtherDerived.ResolvedBase = derivedType

	// test derivation chain
	chain := GetDerivationChain(furtherDerived)
	if len(chain) != 2 {
		t.Fatalf("GetDerivationChain() length = %d, want 2", len(chain))
	}
	if chain[0] != derivedType {
		t.Errorf("chain[0] = %v, want %v", chain[0], derivedType)
	}
	if chain[1] != baseType {
		t.Errorf("chain[1] = %v, want %v", chain[1], baseType)
	}
}

func TestSimpleType_GetDerivationChain_NilBase(t *testing.T) {
	// test with nil base type
	st := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "MyType",
		},
	}

	chain := GetDerivationChain(st)
	if chain == nil {
		t.Fatal("GetDerivationChain() should not return nil")
	}
	if len(chain) != 0 {
		t.Errorf("chain length = %d, want 0", len(chain))
	}
}
