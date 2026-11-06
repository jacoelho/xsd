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
		role       complexTypeFinalRole
		msg        string
	}{
		{
			name:       "extension",
			final:      DerivationExtension,
			derivation: DerivationExtension,
			role:       complexTypeFinalBaseExtension,
			msg:        "base complex type final blocks extension",
		},
		{
			name:       "restriction",
			final:      DerivationRestriction,
			derivation: DerivationRestriction,
			role:       complexTypeFinalBaseRestriction,
			msg:        "base complex type final blocks restriction",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := checkComplexTypeFinalAllows(0, tt.derivation, tt.role); err != nil {
				t.Fatalf("CheckComplexTypeFinalAllows(allowed) error = %v", err)
			}
			err := checkComplexTypeFinalAllows(tt.final, tt.derivation, tt.role)
			expectCompileDiagnostic(t, err, xsderrors.CodeSchemaReference, tt.msg)
		})
	}
}

func TestCheckSimpleBaseComplexExtensionFinalAllows(t *testing.T) {
	t.Parallel()

	if err := checkSimpleBaseComplexExtensionFinalAllows(0); err != nil {
		t.Fatalf("CheckSimpleBaseComplexExtensionFinalAllows(allowed) error = %v", err)
	}
	err := checkSimpleBaseComplexExtensionFinalAllows(DerivationExtension)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaReference, "base simple type final blocks extension")
}

func TestCheckComplexContentRestrictionBase(t *testing.T) {
	t.Parallel()

	if err := checkComplexContentRestrictionBase(complexTypeMetadata{ContentKind: contentElementOnly}); err != nil {
		t.Fatalf("CheckComplexContentRestrictionBase(element content) error = %v", err)
	}
	err := checkComplexContentRestrictionBase(complexTypeMetadata{ContentKind: contentSimple})
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "complexContent restriction base cannot have simple content")
}

func TestCheckSimpleContentSimpleBase(t *testing.T) {
	t.Parallel()

	if err := checkSimpleContentSimpleBase(contentDerivationExtension); err != nil {
		t.Fatalf("CheckSimpleContentSimpleBase(extension) error = %v", err)
	}
	err := checkSimpleContentSimpleBase(contentDerivationRestriction)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "simpleContent restriction base must be complex type")
}

func TestSimpleContentComplexBaseMissingError(t *testing.T) {
	t.Parallel()

	err := simpleContentComplexBaseMissingError()
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaReference, "simpleContent base must be simple or simple-content complex type")
}

func TestCheckSimpleContentDerivationBase(t *testing.T) {
	t.Parallel()

	if err := checkSimpleContentDerivationBase(nil, complexTypeMetadata{ContentKind: contentSimple}, contentDerivationExtension); err != nil {
		t.Fatalf("CheckSimpleContentDerivationBase(simple content) error = %v", err)
	}
	err := checkSimpleContentDerivationBase(nil, complexTypeMetadata{ContentKind: contentElementOnly}, contentDerivationExtension)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "simpleContent base must have simple content")
}

func TestCheckSimpleContentRestrictionTextTypePresent(t *testing.T) {
	t.Parallel()

	if err := checkSimpleContentRestrictionTextTypePresent(1); err != nil {
		t.Fatalf("CheckSimpleContentRestrictionTextTypePresent(present) error = %v", err)
	}
	err := checkSimpleContentRestrictionTextTypePresent(NoSimpleType)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "simpleContent restriction of mixed content requires simpleType")
}

func TestCheckSimpleContentRestrictionTextType(t *testing.T) {
	t.Parallel()

	rt := emptyTypeDerivationFixture{}
	if err := checkSimpleContentRestrictionTextType(rt.build(), 1, NoSimpleType, unboundedTypeDerivationWork); err != nil {
		t.Fatalf("CheckSimpleContentRestrictionTextType(no base) error = %v", err)
	}
	err := checkSimpleContentRestrictionTextType(rt.build(), 2, 1, unboundedTypeDerivationWork)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "simpleContent restriction type is not derived from base")
}

func TestCheckComplexContentMixedDerivationBase(t *testing.T) {
	t.Parallel()

	if err := checkComplexContentMixedDerivationBase(complexTypeMetadata{ContentKind: contentMixed}, contentDerivationExtension, contentMixed); err != nil {
		t.Fatalf("CheckComplexContentMixedDerivationBase(mixed base) error = %v", err)
	}
	err := checkComplexContentMixedDerivationBase(complexTypeMetadata{ContentKind: contentElementOnly}, contentDerivationRestriction, contentMixed)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "complexContent mixed derivation requires mixed base")
}

type emptyTypeDerivationFixture struct{}
