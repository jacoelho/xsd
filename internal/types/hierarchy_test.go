package types

import (
	"testing"
)

func TestSimpleType_IsDerivedFrom(t *testing.T) {
	// Create a type hierarchy: decimal -> integer -> long
	decimalType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     string(TypeNameDecimal),
		},
		// Variety set via SetVariety
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(AtomicVariety)
	decimalType.SetFundamentalFacets(ComputeFundamentalFacets(TypeNameDecimal))

	integerType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     string(TypeNameInteger),
		},
		// Variety set via SetVariety
		Restriction: &Restriction{
			Base: decimalType.QName,
		},
	}
	integerType.ResolvedBase = decimalType
	integerType.MarkBuiltin()

	longType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "long",
		},
		// Variety set via SetVariety
		Restriction: &Restriction{
			Base: integerType.QName,
		},
	}
	longType.ResolvedBase = integerType
	longType.MarkBuiltin()

	// Test direct derivation
	if !IsDerivedFrom(longType, integerType) {
		t.Error("long should be derived from integer")
	}

	// Test indirect derivation
	if !IsDerivedFrom(longType, decimalType) {
		t.Error("long should be derived from decimal (indirectly)")
	}

	// Test not derived
	if IsDerivedFrom(longType, longType) {
		t.Error("type should not be derived from itself")
	}

	// Test unrelated type
	unrelatedType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     string(TypeNameString),
		},
		// Variety set via SetVariety
	}
	unrelatedType.MarkBuiltin()
	unrelatedType.SetVariety(AtomicVariety)
	if IsDerivedFrom(longType, unrelatedType) {
		t.Error("long should not be derived from string")
	}
}

func TestComplexType_IsDerivedFrom(t *testing.T) {
	// Create a type hierarchy: BaseType -> DerivedType
	baseType := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		// Content set via SetContent
	}
	baseType.SetContent(&ElementContent{})

	derivedType := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		// Content set via SetContent
	}
	derivedType.SetContent(&ComplexContent{
		Base: baseType.QName,
		Extension: &Extension{
			Base: baseType.QName,
		},
	})
	derivedType.ResolvedBase = baseType

	// Test direct derivation
	if !IsDerivedFrom(derivedType, baseType) {
		t.Error("DerivedType should be derived from BaseType")
	}

	// Test not derived
	if IsDerivedFrom(derivedType, derivedType) {
		t.Error("type should not be derived from itself")
	}
}

func TestSimpleType_GetDerivationChain(t *testing.T) {
	// Create a type hierarchy: decimal -> integer -> long
	decimalType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     string(TypeNameDecimal),
		},
		// Variety set via SetVariety
	}
	decimalType.MarkBuiltin()
	decimalType.SetVariety(AtomicVariety)
	decimalType.SetFundamentalFacets(ComputeFundamentalFacets(TypeNameDecimal))

	integerType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     string(TypeNameInteger),
		},
		// Variety set via SetVariety
		Restriction: &Restriction{
			Base: decimalType.QName,
		},
	}
	integerType.ResolvedBase = decimalType
	integerType.MarkBuiltin()

	longType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     "long",
		},
		// Variety set via SetVariety
		Restriction: &Restriction{
			Base: integerType.QName,
		},
	}
	longType.ResolvedBase = integerType
	longType.MarkBuiltin()

	// Test derivation chain
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

	// Test primitive type (no base)
	primitiveChain := GetDerivationChain(decimalType)
	if len(primitiveChain) != 0 {
		t.Errorf("primitive type chain length = %d, want 0", len(primitiveChain))
	}
}

func TestComplexType_GetDerivationChain(t *testing.T) {
	// Create a type hierarchy: BaseType -> DerivedType -> FurtherDerived
	baseType := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		// Content set via SetContent
	}
	baseType.SetContent(&ElementContent{})

	derivedType := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		// Content set via SetContent
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
		// Content set via SetContent
	}
	furtherDerived.SetContent(&ComplexContent{
		Base: derivedType.QName,
		Extension: &Extension{
			Base: derivedType.QName,
		},
	})
	furtherDerived.ResolvedBase = derivedType

	// Test derivation chain
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
	// Test with nil base type
	st := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "MyType",
		},
		// Variety set via SetVariety
	}

	chain := GetDerivationChain(st)
	if chain == nil {
		t.Fatal("GetDerivationChain() should not return nil")
	}
	if len(chain) != 0 {
		t.Errorf("chain length = %d, want 0", len(chain))
	}
}
