package attrs

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestValidateTypeSimplePath(t *testing.T) {
	t.Parallel()

	rt := &runtime.Schema{}
	calledComplex := false
	result, err := ValidateType(
		rt,
		runtime.Type{Kind: runtime.TypeSimple},
		Classification{Classes: []Class{ClassXML}},
		[]Start{{}},
		true,
		TypeCallbacks{
			PrepareValidated: func(store bool, size int) []Start {
				if !store || size != 1 {
					t.Fatalf("PrepareValidated(store=%v, size=%d)", store, size)
				}
				return []Start{{}}
			},
			PreparePresent: func(int) []bool {
				t.Fatal("PreparePresent should not be called on simple path")
				return nil
			},
			ValidateSimple: func(input []Start, classes []Class, store bool, validated []Start) ([]Start, error) {
				return append(validated, Start{Value: []byte("ok")}), nil
			},
			ValidateComplex: func(*runtime.ComplexType, []bool, []Start, []Class, bool, []Start) ([]Start, bool, error) {
				calledComplex = true
				return nil, false, nil
			},
			ApplyDefaults: func([]runtime.AttrUse, []bool, bool, bool) ([]Applied, error) {
				t.Fatal("ApplyDefaults should not be called on simple path")
				return nil, nil
			},
		},
	)
	if err != nil {
		t.Fatalf("ValidateType() error = %v", err)
	}
	if calledComplex {
		t.Fatal("ValidateType() called complex path on simple type")
	}
	if len(result.Attrs) != 2 || len(result.Applied) != 0 {
		t.Fatalf("ValidateType() = %+v, want simple attrs only", result)
	}
}

func TestValidateTypeComplexPath(t *testing.T) {
	t.Parallel()

	rt := &runtime.Schema{
		ComplexTypes: []runtime.ComplexType{
			{},
			{Attrs: runtime.AttrIndexRef{Off: 0, Len: 2}},
		},
		AttrIndex: runtime.ComplexAttrIndex{
			Uses: []runtime.AttrUse{{}, {}},
		},
	}
	result, err := ValidateType(
		rt,
		runtime.Type{Kind: runtime.TypeComplex, Complex: runtime.ComplexTypeRef{ID: 1}},
		Classification{Classes: []Class{ClassOther}},
		[]Start{{}},
		false,
		TypeCallbacks{
			PrepareValidated: func(store bool, size int) []Start {
				if store || size != 1 {
					t.Fatalf("PrepareValidated(store=%v, size=%d)", store, size)
				}
				return nil
			},
			PreparePresent: func(size int) []bool {
				if size != 2 {
					t.Fatalf("PreparePresent(size=%d), want 2", size)
				}
				return make([]bool, size)
			},
			ValidateSimple: func([]Start, []Class, bool, []Start) ([]Start, error) {
				t.Fatal("ValidateSimple should not be called on complex path")
				return nil, nil
			},
			ValidateComplex: func(ct *runtime.ComplexType, present []bool, input []Start, classes []Class, store bool, validated []Start) ([]Start, bool, error) {
				if ct.Attrs.Len != 2 {
					t.Fatalf("ValidateComplex() got attrs len %d", ct.Attrs.Len)
				}
				present[1] = true
				return []Start{{Value: []byte("validated")}}, true, nil
			},
			ApplyDefaults: func(uses []runtime.AttrUse, present []bool, storeAttrs, seenID bool) ([]Applied, error) {
				if len(uses) != 2 || !present[1] || !seenID || storeAttrs {
					t.Fatalf("ApplyDefaults() got uses=%d present=%v store=%v seenID=%v", len(uses), present, storeAttrs, seenID)
				}
				return []Applied{{}}, nil
			},
		},
	)
	if err != nil {
		t.Fatalf("ValidateType() error = %v", err)
	}
	if len(result.Attrs) != 1 || len(result.Applied) != 1 {
		t.Fatalf("ValidateType() = %+v, want validated attrs plus defaults", result)
	}
}

func TestValidateTypeDuplicateErrorShortCircuits(t *testing.T) {
	t.Parallel()

	dupErr := errors.New("duplicate")
	_, err := ValidateType(
		&runtime.Schema{},
		runtime.Type{Kind: runtime.TypeSimple},
		Classification{DuplicateErr: dupErr},
		nil,
		false,
		TypeCallbacks{},
	)
	if !errors.Is(err, dupErr) {
		t.Fatalf("ValidateType() error = %v, want %v", err, dupErr)
	}
}
