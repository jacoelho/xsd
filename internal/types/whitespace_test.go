package types

import (
	"testing"
)

func TestWhiteSpace_Inheritance(t *testing.T) {
	// Test that derived types inherit whitespace from base type
	baseType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     string(TypeNameString),
		},
		// Variety set via SetVariety
	}
	baseType.MarkBuiltin()
	baseType.SetVariety(AtomicVariety)
	baseType.SetWhiteSpace(WhiteSpacePreserve)

	derivedType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "MyString",
		},
		// Variety set via SetVariety
		Restriction: &Restriction{
			Base: baseType.QName,
		},
	}
	derivedType.ResolvedBase = baseType
	derivedType.SetVariety(AtomicVariety)
	derivedType.SetWhiteSpace(baseType.WhiteSpace()) // Inherit from base

	if derivedType.WhiteSpace() != WhiteSpacePreserve {
		t.Errorf("WhiteSpace = %v, want %v", derivedType.WhiteSpace(), WhiteSpacePreserve)
	}
}

func TestWhiteSpace_Override(t *testing.T) {
	// Test that whitespace can be overridden in restrictions
	baseType := &SimpleType{
		QName: QName{
			Namespace: "http://www.w3.org/2001/XMLSchema",
			Local:     string(TypeNameString),
		},
		// Variety set via SetVariety
	}
	baseType.MarkBuiltin()
	baseType.SetVariety(AtomicVariety)
	baseType.SetWhiteSpace(WhiteSpacePreserve)

	derivedType := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "MyString",
		},
		// Variety set via SetVariety
		Restriction: &Restriction{
			Base: baseType.QName,
		},
	}
	derivedType.ResolvedBase = baseType
	derivedType.SetVariety(AtomicVariety)
	derivedType.SetWhiteSpace(WhiteSpaceCollapse) // Override to collapse

	if derivedType.WhiteSpace() != WhiteSpaceCollapse {
		t.Errorf("WhiteSpace = %v, want %v", derivedType.WhiteSpace(), WhiteSpaceCollapse)
	}
}

func TestWhiteSpace_StricterOnly(t *testing.T) {
	// Test that whitespace can only be made stricter (preserve -> replace -> collapse)
	// This is validated during schema validation, not here, but we can test the values
	tests := []struct {
		name      string
		base      WhiteSpace
		derived   WhiteSpace
		shouldErr bool
	}{
		{"preserve to replace", WhiteSpacePreserve, WhiteSpaceReplace, false},
		{"preserve to collapse", WhiteSpacePreserve, WhiteSpaceCollapse, false},
		{"replace to collapse", WhiteSpaceReplace, WhiteSpaceCollapse, false},
		{"preserve to preserve", WhiteSpacePreserve, WhiteSpacePreserve, false},
		{"replace to replace", WhiteSpaceReplace, WhiteSpaceReplace, false},
		{"collapse to collapse", WhiteSpaceCollapse, WhiteSpaceCollapse, false},
		// These should fail validation (but we're just testing the values here)
		{"replace to preserve", WhiteSpaceReplace, WhiteSpacePreserve, true},
		{"collapse to preserve", WhiteSpaceCollapse, WhiteSpacePreserve, true},
		{"collapse to replace", WhiteSpaceCollapse, WhiteSpaceReplace, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseType := &SimpleType{
				QName: QName{
					Namespace: "http://example.com",
					Local:     "Base",
				},
				// Variety set via SetVariety
			}
			baseType.SetVariety(AtomicVariety)
			baseType.SetWhiteSpace(tt.base)

			derivedType := &SimpleType{
				QName: QName{
					Namespace: "http://example.com",
					Local:     "Derived",
				},
				// Variety set via SetVariety
			}
			derivedType.ResolvedBase = baseType
			derivedType.SetVariety(AtomicVariety)
			derivedType.SetWhiteSpace(tt.derived)

			// Check if the values are set correctly
			if derivedType.WhiteSpace() != tt.derived {
				t.Errorf("WhiteSpace = %v, want %v", derivedType.WhiteSpace(), tt.derived)
			}
			// The actual validation that it's stricter will be in schema validator
		})
	}
}

func TestNormalizeValue_WhiteSpace(t *testing.T) {
	typ := &SimpleType{
		QName: QName{
			Namespace: "http://example.com",
			Local:     "NormalizedString",
		},
	}
	typ.SetVariety(AtomicVariety)
	typ.SetWhiteSpace(WhiteSpaceCollapse)

	normalized, err := NormalizeValue(" \talpha \n  beta\r\n", typ)
	if err != nil {
		t.Fatalf("NormalizeValue() error = %v", err)
	}
	if normalized != "alpha beta" {
		t.Errorf("NormalizeValue() = %q, want %q", normalized, "alpha beta")
	}
}
