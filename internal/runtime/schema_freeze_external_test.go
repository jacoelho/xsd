package runtime_test

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestSimpleTypeFreezeProjectionsCloneUnionSlices(t *testing.T) {
	t.Parallel()

	st := runtime.SimpleType{
		Union:   []runtime.SimpleTypeID{1},
		Variety: runtime.SimpleVarietyUnion,
	}
	restriction := runtime.NewSimpleTypeRestrictionValidationForSimpleType(st)
	restriction.Union[0] = 9
	if st.Union[0] != 1 {
		t.Fatalf("NewSimpleTypeRestrictionValidationForSimpleType returned table-backed union slice: %#v", st.Union)
	}
}

func TestValueConstraintIdentityClonesResolvedNames(t *testing.T) {
	t.Parallel()

	vc := &runtime.ValueConstraint{
		ResolvedNames: []runtime.ResolvedValueName{{Lexical: "p:item"}},
	}
	identity := runtime.NewValueConstraintIdentity(vc)
	identity.ResolvedNames[0].Lexical = "p:other"
	if vc.ResolvedNames[0].Lexical != "p:item" {
		t.Fatalf("NewValueConstraintIdentity returned table-backed resolved names: %#v", vc.ResolvedNames)
	}
}
