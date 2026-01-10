package types

import (
	"testing"
)

func TestComplexType_DerivationMethod_Extension(t *testing.T) {
	// test derivation method for extension
	ct := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		// content set via SetContent
		DerivationMethod: DerivationExtension,
	}
	ct.SetContent(&ComplexContent{
		Extension: &Extension{
			Base: QName{Namespace: "http://example.com", Local: "BaseType"},
		},
	})

	if !ct.IsExtension() {
		t.Error("IsExtension() should return true for extension")
	}
	if ct.IsRestriction() {
		t.Error("IsRestriction() should return false for extension")
	}
	if !ct.IsDerived() {
		t.Error("IsDerived() should return true for extension")
	}
}

func TestComplexType_DerivationMethod_Restriction(t *testing.T) {
	// test derivation method for restriction
	ct := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "DerivedType",
		},
		// content set via SetContent
		DerivationMethod: DerivationRestriction,
	}
	ct.SetContent(&ComplexContent{
		Restriction: &Restriction{
			Base: QName{Namespace: "http://example.com", Local: "BaseType"},
		},
	})

	if ct.IsExtension() {
		t.Error("IsExtension() should return false for restriction")
	}
	if !ct.IsRestriction() {
		t.Error("IsRestriction() should return true for restriction")
	}
	if !ct.IsDerived() {
		t.Error("IsDerived() should return true for restriction")
	}
}

func TestComplexType_DerivationMethod_NoDerivation(t *testing.T) {
	// test derivation method for non-derived types
	ct := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "BaseType",
		},
		// content set via SetContent
		DerivationMethod: 0, // no derivation
	}
	ct.SetContent(&ElementContent{})

	if ct.IsExtension() {
		t.Error("IsExtension() should return false for non-derived type")
	}
	if ct.IsRestriction() {
		t.Error("IsRestriction() should return false for non-derived type")
	}
	if ct.IsDerived() {
		t.Error("IsDerived() should return false for non-derived type")
	}
}

func TestComplexType_DerivationMethod_SimpleContent(t *testing.T) {
	// test derivation method for simpleContent
	ct := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "StringWithAttr",
		},
		// content set via SetContent
		DerivationMethod: DerivationExtension,
	}
	ct.SetContent(&SimpleContent{
		Extension: &Extension{
			Base: QName{Namespace: "http://www.w3.org/2001/XMLSchema", Local: "string"},
		},
	})

	if !ct.IsExtension() {
		t.Error("IsExtension() should return true for simpleContent extension")
	}
	if !ct.IsDerived() {
		t.Error("IsDerived() should return true for simpleContent extension")
	}
}

func TestComplexType_DerivationMethod_MultipleFlags(t *testing.T) {
	ct := &ComplexType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "MixedDerivation",
		},
		DerivationMethod: DerivationExtension | DerivationRestriction,
	}
	ct.SetContent(&ComplexContent{
		Base: ct.QName,
		Extension: &Extension{
			Base: ct.QName,
		},
		Restriction: &Restriction{
			Base: ct.QName,
		},
	})

	if ct.IsExtension() {
		t.Error("IsExtension() should return false when multiple flags are set")
	}
	if ct.IsRestriction() {
		t.Error("IsRestriction() should return false when multiple flags are set")
	}
	if !ct.IsDerived() {
		t.Error("IsDerived() should return true when any derivation flag is set")
	}
}
