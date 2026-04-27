package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestUsesOverflowReturnsNil(t *testing.T) {
	uses := []runtime.AttrUse{{}}
	ref := runtime.AttrIndexRef{Off: ^uint32(0), Len: 2}
	got := Uses(uses, ref)
	if got != nil {
		t.Fatalf("Uses() = %#v, want nil", got)
	}
}

func TestGlobalAttributeBySymbolOutOfRange(t *testing.T) {
	rt := newRuntimeSchema(t)
	setRuntimeGlobalAttributes(t, rt, []runtime.AttrID{0, 3})
	got, ok := GlobalAttributeBySymbol(rt, 2)
	if ok || got != 0 {
		t.Fatalf("GlobalAttributeBySymbol() = (%d, %v), want (0, false)", got, ok)
	}
}

func TestLookupUseFindsMatchingSymbol(t *testing.T) {
	rt := newRuntimeSchema(t)
	setRuntimeAttrIndex(t, rt, runtime.ComplexAttrIndex{Uses: []runtime.AttrUse{
		{Name: 7},
		{Name: 9},
	}})
	use, idx, ok := LookupUse(rt, runtime.AttrIndexRef{Off: 0, Len: 2}, 9)
	if !ok {
		t.Fatal("LookupUse() ok = false, want true")
	}
	if idx != 1 {
		t.Fatalf("LookupUse() idx = %d, want 1", idx)
	}
	if use.Name != 9 {
		t.Fatalf("LookupUse() name = %d, want 9", use.Name)
	}
}
