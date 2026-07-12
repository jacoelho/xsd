package compile

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestCheckComplexTypeFinalAllows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		final      runtime.DerivationMask
		derivation runtime.DerivationMask
		role       ComplexTypeFinalRole
		msg        string
	}{
		{
			name:       "extension",
			final:      runtime.DerivationExtension,
			derivation: runtime.DerivationExtension,
			role:       ComplexTypeFinalBaseExtension,
			msg:        "base complex type final blocks extension",
		},
		{
			name:       "restriction",
			final:      runtime.DerivationRestriction,
			derivation: runtime.DerivationRestriction,
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
	err := CheckSimpleBaseComplexExtensionFinalAllows(runtime.DerivationExtension)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaReference, "base simple type final blocks extension")
}

func TestCheckComplexContentRestrictionBase(t *testing.T) {
	t.Parallel()

	if err := CheckComplexContentRestrictionBase(runtime.ComplexType{ContentKind: runtime.ContentElementOnly}); err != nil {
		t.Fatalf("CheckComplexContentRestrictionBase(element content) error = %v", err)
	}
	err := CheckComplexContentRestrictionBase(runtime.ComplexType{ContentKind: runtime.ContentSimple})
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

func TestCheckSimpleContentComplexBaseExists(t *testing.T) {
	t.Parallel()

	if err := CheckSimpleContentComplexBaseExists(true); err != nil {
		t.Fatalf("CheckSimpleContentComplexBaseExists(true) error = %v", err)
	}
	err := CheckSimpleContentComplexBaseExists(false)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaReference, "simpleContent base must be simple or simple-content complex type")
}

func TestCheckSimpleContentDerivationBase(t *testing.T) {
	t.Parallel()

	if err := CheckSimpleContentDerivationBase(nil, runtime.ComplexType{ContentKind: runtime.ContentSimple}, false); err != nil {
		t.Fatalf("CheckSimpleContentDerivationBase(simple content) error = %v", err)
	}
	err := CheckSimpleContentDerivationBase(nil, runtime.ComplexType{ContentKind: runtime.ContentElementOnly}, false)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "simpleContent base must have simple content")
}

func TestCheckSimpleContentRestrictionTextTypePresent(t *testing.T) {
	t.Parallel()

	if err := CheckSimpleContentRestrictionTextTypePresent(1); err != nil {
		t.Fatalf("CheckSimpleContentRestrictionTextTypePresent(present) error = %v", err)
	}
	err := CheckSimpleContentRestrictionTextTypePresent(runtime.NoSimpleType)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "simpleContent restriction of mixed content requires simpleType")
}

func TestCheckSimpleContentRestrictionTextType(t *testing.T) {
	t.Parallel()

	rt := emptyTypeDerivationRuntime{}
	if err := CheckSimpleContentRestrictionTextType(rt, 1, runtime.NoSimpleType); err != nil {
		t.Fatalf("CheckSimpleContentRestrictionTextType(no base) error = %v", err)
	}
	err := CheckSimpleContentRestrictionTextType(rt, 2, 1)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "simpleContent restriction type is not derived from base")
}

func TestCheckComplexContentMixedDerivationBase(t *testing.T) {
	t.Parallel()

	if err := CheckComplexContentMixedDerivationBase(nil, runtime.ComplexType{ContentKind: runtime.ContentMixed}, true, true); err != nil {
		t.Fatalf("CheckComplexContentMixedDerivationBase(mixed base) error = %v", err)
	}
	err := CheckComplexContentMixedDerivationBase(nil, runtime.ComplexType{ContentKind: runtime.ContentElementOnly}, false, true)
	expectCompileDiagnostic(t, err, xsderrors.CodeSchemaContentModel, "complexContent mixed derivation requires mixed base")
}

type emptyTypeDerivationRuntime struct{}

func (emptyTypeDerivationRuntime) AnyTypeID() runtime.ComplexTypeID { return 0 }

func (emptyTypeDerivationRuntime) ComplexTypeCount() int { return 0 }

func (emptyTypeDerivationRuntime) SimpleTypeCount() int { return 0 }

func (emptyTypeDerivationRuntime) SimpleTypeDerivation(runtime.SimpleTypeID) (runtime.SimpleTypeDerivation, bool) {
	return runtime.SimpleTypeDerivation{}, false
}

func (emptyTypeDerivationRuntime) ComplexTypeDerivation(runtime.ComplexTypeID) (runtime.ComplexTypeDerivation, bool) {
	return runtime.ComplexTypeDerivation{}, false
}
