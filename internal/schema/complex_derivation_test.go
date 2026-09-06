package schema

import (
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestCheckComplexTypeFinalAllows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		final      DerivationMask
		derivation DerivationMask
		role       ComplexTypeFinalRole
		msg        string
	}{
		{
			name:       "extension",
			final:      DerivationExtension,
			derivation: DerivationExtension,
			role:       ComplexTypeFinalBaseExtension,
			msg:        "base complex type final blocks extension",
		},
		{
			name:       "restriction",
			final:      DerivationRestriction,
			derivation: DerivationRestriction,
			role:       ComplexTypeFinalBaseRestriction,
			msg:        "base complex type final blocks restriction",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := CheckComplexTypeFinalAllows(0, tt.derivation, tt.role); err != nil {
				t.Fatalf("CheckComplexTypeFinalAllows(allowed) error = %v", err)
			}
			err := CheckComplexTypeFinalAllows(tt.final, tt.derivation, tt.role)
			expectCompileDiagnostic(t, err, xsderrors.CodeSchemaReference, tt.msg)
		})
	}
}

func TestCheckSimpleBaseComplexExtensionFinalAllows(t *testing.T) {
	t.Parallel()

	if err := CheckSimpleBaseComplexExtensionFinalAllows(0); err != nil {
		t.Fatalf("CheckSimpleBaseComplexExtensionFinalAllows(allowed) error = %v", err)
	}
	err := CheckSimpleBaseComplexExtensionFinalAllows(DerivationExtension)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaReference, "base simple type final blocks extension")
}

func TestCheckComplexContentRestrictionBase(t *testing.T) {
	t.Parallel()

	if err := CheckComplexContentRestrictionBase(ComplexType{ContentKind: ContentElementOnly}); err != nil {
		t.Fatalf("CheckComplexContentRestrictionBase(element content) error = %v", err)
	}
	err := CheckComplexContentRestrictionBase(ComplexType{ContentKind: ContentSimple})
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "complexContent restriction base cannot have simple content")
}

func TestCheckSimpleContentSimpleBase(t *testing.T) {
	t.Parallel()

	if err := CheckSimpleContentSimpleBase(ContentDerivationExtension); err != nil {
		t.Fatalf("CheckSimpleContentSimpleBase(extension) error = %v", err)
	}
	err := CheckSimpleContentSimpleBase(ContentDerivationRestriction)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "simpleContent restriction base must be complex type")
}

func TestSimpleContentComplexBaseMissingError(t *testing.T) {
	t.Parallel()

	err := SimpleContentComplexBaseMissingError()
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaReference, "simpleContent base must be simple or simple-content complex type")
}

func TestCheckSimpleContentDerivationBase(t *testing.T) {
	t.Parallel()

	if err := CheckSimpleContentDerivationBase(nil, ComplexType{ContentKind: ContentSimple}, ContentDerivationExtension); err != nil {
		t.Fatalf("CheckSimpleContentDerivationBase(simple content) error = %v", err)
	}
	err := CheckSimpleContentDerivationBase(nil, ComplexType{ContentKind: ContentElementOnly}, ContentDerivationExtension)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "simpleContent base must have simple content")
}

func TestCheckSimpleContentRestrictionTextTypePresent(t *testing.T) {
	t.Parallel()

	if err := CheckSimpleContentRestrictionTextTypePresent(1); err != nil {
		t.Fatalf("CheckSimpleContentRestrictionTextTypePresent(present) error = %v", err)
	}
	err := CheckSimpleContentRestrictionTextTypePresent(NoSimpleType)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "simpleContent restriction of mixed content requires simpleType")
}

func TestCheckSimpleContentRestrictionTextType(t *testing.T) {
	t.Parallel()

	rt := emptyTypeDerivationRuntime{}
	if err := CheckSimpleContentRestrictionTextType(rt, 1, NoSimpleType, unboundedTypeDerivationWork); err != nil {
		t.Fatalf("CheckSimpleContentRestrictionTextType(no base) error = %v", err)
	}
	err := CheckSimpleContentRestrictionTextType(rt, 2, 1, unboundedTypeDerivationWork)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "simpleContent restriction type is not derived from base")
}

func TestCheckComplexContentMixedDerivationBase(t *testing.T) {
	t.Parallel()

	if err := CheckComplexContentMixedDerivationBase(nil, ComplexType{ContentKind: ContentMixed}, ContentDerivationExtension, ContentMixed); err != nil {
		t.Fatalf("CheckComplexContentMixedDerivationBase(mixed base) error = %v", err)
	}
	err := CheckComplexContentMixedDerivationBase(nil, ComplexType{ContentKind: ContentElementOnly}, ContentDerivationRestriction, ContentMixed)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "complexContent mixed derivation requires mixed base")
}

type emptyTypeDerivationRuntime struct{}

func (emptyTypeDerivationRuntime) AnyTypeID() ComplexTypeID { return 0 }

func (emptyTypeDerivationRuntime) ComplexTypeCount() int { return 0 }

func (emptyTypeDerivationRuntime) SimpleTypeCount() int { return 0 }

func (emptyTypeDerivationRuntime) SimpleTypeDerivation(SimpleTypeID) (SimpleTypeDerivation, bool) {
	return SimpleTypeDerivation{}, false
}

func (emptyTypeDerivationRuntime) ComplexTypeDerivation(ComplexTypeID) (ComplexTypeDerivation, bool) {
	return ComplexTypeDerivation{}, false
}
