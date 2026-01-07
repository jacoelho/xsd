package types

import (
	"testing"
)

func TestComplexType_BaseType_ForExtension(t *testing.T) {
	// test base type for complex type with extension
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

	// BaseType() should never be nil now (returns anyType if ResolvedBase is nil)
	if derivedType.BaseType() != baseType {
		t.Fatal("BaseType should be set to baseType")
	}
	if derivedType.BaseType().Name().Local != "BaseType" {
		t.Errorf("BaseType = %q, want %q", derivedType.BaseType().Name().Local, "BaseType")
	}
}

func TestComplexType_BaseType_ForRestriction(t *testing.T) {
	// test base type for complex type with restriction
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
		Restriction: &Restriction{
			Base: baseType.QName,
		},
	})
	derivedType.ResolvedBase = baseType

	// BaseType() should never be nil now (returns anyType if ResolvedBase is nil)
	if derivedType.BaseType() != baseType {
		t.Fatal("BaseType should be set to baseType")
	}
	if derivedType.BaseType().Name().Local != "BaseType" {
		t.Errorf("BaseType = %q, want %q", derivedType.BaseType().Name().Local, "BaseType")
	}
}

func TestComplexType_BaseType_ForSimpleContent(t *testing.T) {
	// test base type for simpleContent (base is a simple type)
	baseSimpleType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     string(TypeNameString),
		},
		// variety set via SetVariety
	}
	baseSimpleType.MarkBuiltin()
	baseSimpleType.SetVariety(AtomicVariety)

	complexType := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "StringWithAttr",
		},
		// content set via SetContent
	}
	complexType.SetContent(&SimpleContent{
		Base: baseSimpleType.QName,
		Extension: &Extension{
			Base: baseSimpleType.QName,
		},
	})
	complexType.ResolvedBase = baseSimpleType

	// BaseType() should never be nil now (returns anyType if ResolvedBase is nil)
	if complexType.BaseType() != baseSimpleType {
		t.Fatal("BaseType should be set to baseSimpleType")
	}
	if complexType.BaseType().Name().Local != string(TypeNameString) {
		t.Errorf("BaseType = %q, want %q", complexType.BaseType().Name().Local, "string")
	}
}

func TestComplexType_BaseType(t *testing.T) {
	// test BaseType method
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

	got := derivedType.BaseType()
	if got == nil {
		t.Fatal("BaseType() returned nil")
	}
	if got != baseType {
		t.Errorf("BaseType() = %v, want %v", got, baseType)
	}
}

func TestNewComplexTypeFromParsed_SimpleContentMissingBase(t *testing.T) {
	ct := &ComplexType{}
	ct.SetContent(&SimpleContent{
		Extension: &Extension{},
	})

	if _, err := NewComplexTypeFromParsed(ct); err == nil {
		t.Fatal("expected error for simpleContent without base type")
	}
}

func TestNewComplexTypeFromParsed_ComplexContentBothDerivations(t *testing.T) {
	ct := &ComplexType{}
	ct.SetContent(&ComplexContent{
		Extension: &Extension{
			Base: QName{Local: "BaseType"},
		},
		Restriction: &Restriction{
			Base: QName{Local: "BaseType"},
		},
	})

	if _, err := NewComplexTypeFromParsed(ct); err == nil {
		t.Fatal("expected error for complexContent with both extension and restriction")
	}
}