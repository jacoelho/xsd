package analysis

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestAttributeWildcardIntersectAndUnion(t *testing.T) {
	base := &model.AnyAttribute{Namespace: model.NSCAny}
	derived := &model.AnyAttribute{Namespace: model.NSCOther}

	unioned, err := UnionAttributeWildcards(derived, base)
	if err != nil {
		t.Fatalf("UnionAttributeWildcards() error = %v", err)
	}
	if unioned == nil {
		t.Fatal("UnionAttributeWildcards() returned nil")
	}

	intersected, err := IntersectAttributeWildcards(derived, base)
	if err != nil {
		t.Fatalf("IntersectAttributeWildcards() error = %v", err)
	}
	if intersected == nil {
		t.Fatal("IntersectAttributeWildcards() returned nil")
	}
}

func TestRestrictAttributeWildcardRequiresBase(t *testing.T) {
	derived := &model.AnyAttribute{Namespace: model.NSCAny}
	if _, err := RestrictAttributeWildcard(nil, derived); !errors.Is(err, ErrAttributeWildcardRestrictionAddsWildcard) {
		t.Fatalf("RestrictAttributeWildcard(nil, derived) error = %v", err)
	}
}
