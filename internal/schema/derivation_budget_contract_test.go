package schema

import (
	"errors"
	"reflect"
	"slices"
	"testing"

	valuepkg "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestDerivationTraversalWorkFailureCanBeRetried(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name    string
		derived TypeID
		base    TypeID
		want    DerivationMask
	}{
		{name: "complex to simple", derived: ComplexRef(4), base: SimpleRef(0), want: DerivationExtension},
		{name: "complex to anyType", derived: ComplexRef(4), base: ComplexRef(0), want: DerivationExtension | DerivationRestriction},
		{name: "complex to complex", derived: ComplexRef(3), base: ComplexRef(1), want: DerivationExtension | DerivationRestriction},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			simple := []SimpleType{{ValueSpec: valuepkg.TypeSpec{Base: NoSimpleType, Variety: valuepkg.Atomic}}}
			complexTypes := []ComplexType{
				{Derivation: DerivationKindNone},
				{Base: ComplexRef(0), Derivation: DerivationKindExtension},
				{Base: ComplexRef(1), Derivation: DerivationKindExtension},
				{Base: ComplexRef(2), Derivation: DerivationKindRestriction},
				{Base: SimpleRef(0), Derivation: DerivationKindExtension},
			}
			build := &schemaBuild{SimpleTypes: slices.Clone(simple), ComplexTypes: slices.Clone(complexTypes), AnyType: 0}
			budget := newWorkBudget(dependencyWorkBudget, 1)
			mask, found, err := deriveTypeMask(build, tc.derived, tc.base, budget.spend)
			if mask != 0 || found {
				t.Fatalf("failed traversal published result: mask=%v found=%v", mask, found)
			}
			diagnostic, ok := errors.AsType[*xsderrors.Error](err)
			if !ok || diagnostic.Category() != xsderrors.CategorySchemaCompile || diagnostic.Code() != xsderrors.CodeSchemaLimit || diagnostic.Message() != "compiler work exceeds MaxSchemaDependencySteps (limit 1)" {
				t.Fatalf("nested work failure diagnostic: %v", err)
			}
			if !reflect.DeepEqual(build.SimpleTypes, simple) || !slices.Equal(build.ComplexTypes, complexTypes) {
				t.Fatal("failed traversal changed canonical type records")
			}
			budget = newWorkBudget(dependencyWorkBudget, 16)
			mask, found, err = deriveTypeMask(build, tc.derived, tc.base, budget.spend)
			if err != nil || !found || mask != tc.want {
				t.Fatalf("retry: mask=%v found=%v err=%v, want mask=%v", mask, found, err, tc.want)
			}
			if !reflect.DeepEqual(build.SimpleTypes, simple) || !slices.Equal(build.ComplexTypes, complexTypes) {
				t.Fatal("retry changed canonical type records")
			}
		})
	}
}
