package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestSliceAttrUsesOverflowReturnsNil(t *testing.T) {
	uses := []runtime.AttrUse{{}}
	ref := runtime.AttrIndexRef{Off: ^uint32(0), Len: 2}
	got := Uses(uses, ref)
	if got != nil {
		t.Fatalf("Uses() = %#v, want nil", got)
	}
}

func TestValidateAttributesOutOfRangeComplexID(t *testing.T) {
	schema, ids := buildAttrFixtureNoRequired(t)
	schema.TypeTable()[ids.typeBase] = runtime.Type{
		Kind: runtime.TypeComplex,
		Complex: runtime.ComplexTypeRef{
			ID: uint32(len(schema.ComplexTypeTable()) + 1),
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
