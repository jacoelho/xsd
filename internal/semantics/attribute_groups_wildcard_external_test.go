package semantics_test

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/semantics"
)

func TestAttributeWildcardIntersectAndUnion(t *testing.T) {
	base := &model.AnyAttribute{Namespace: model.NSCAny}
	derived := &model.AnyAttribute{Namespace: model.NSCOther}

	unioned, err := semantics.UnionAttributeWildcards(derived, base)
	if err != nil {
		t.Fatalf("UnionAttributeWildcards() error = %v", err)
	}
	if unioned == nil {
		t.Fatal("UnionAttributeWildcards() returned nil")
	}

	intersected, err := semantics.IntersectAttributeWildcards(derived, base)
	if err != nil {
		t.Fatalf("IntersectAttributeWildcards() error = %v", err)
	}
	if intersected == nil {
		t.Fatal("IntersectAttributeWildcards() returned nil")
	}
}

func TestRestrictAttributeWildcardRequiresBase(t *testing.T) {
	derived := &model.AnyAttribute{Namespace: model.NSCAny}
	if _, err := semantics.RestrictAttributeWildcard(nil, derived); !errors.Is(err, semantics.ErrAttributeWildcardRestrictionAddsWildcard) {
		t.Fatalf("RestrictAttributeWildcard(nil, derived) error = %v", err)
	}
}
