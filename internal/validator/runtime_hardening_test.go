package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestSliceAttrUsesOverflowReturnsNil(t *testing.T) {
	uses := []runtime.AttrUse{{}}
	ref := runtime.AttrIndexRef{Off: ^uint32(0), Len: 2}
	got := sliceAttrUses(uses, ref)
	if got != nil {
		t.Fatalf("sliceAttrUses() = %#v, want nil", got)
	}
}

func TestUnionMemberInfoOverflowReturnsFalse(t *testing.T) {
	s := &Session{
		rt: &runtime.Schema{
			Validators: runtime.ValidatorsBundle{
				Union: []runtime.UnionValidator{
					{MemberOff: ^uint32(0), MemberLen: 2},
				},
				UnionMembers:      []runtime.ValidatorID{1},
				UnionMemberTypes:  []runtime.TypeID{1},
				UnionMemberSameWS: []uint8{1},
			},
		},
	}

	_, _, _, ok := s.unionMemberInfo(runtime.ValidatorMeta{Index: 0})
	if ok {
		t.Fatal("unionMemberInfo() ok = true, want false")
	}
}

func TestValidateAttributesOutOfRangeComplexID(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired(t)
	schema.Types[ids.typeBase] = runtime.Type{
		Kind: runtime.TypeComplex,
		Complex: runtime.ComplexTypeRef{
			ID: uint32(len(schema.ComplexTypes) + 1),
		},
	}

	sess := NewSession(schema)
	_, err := sess.ValidateAttributes(ids.typeBase, nil, nil)
	if err == nil {
		t.Fatal("ValidateAttributes() error = nil, want out-of-range complex type error")
	}
	if !strings.Contains(err.Error(), "complex type") {
		t.Fatalf("ValidateAttributes() error = %v, want complex type message", err)
	}
}
