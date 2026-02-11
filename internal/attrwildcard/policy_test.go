package attrwildcard

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func anyAttr(ns model.NamespaceConstraint, list []model.NamespaceURI) *model.AnyAttribute {
	return &model.AnyAttribute{
		Namespace:       ns,
		NamespaceList:   list,
		ProcessContents: model.Lax,
	}
}

func TestIntersectAndUnion(t *testing.T) {
	base := anyAttr(model.NSCAny, nil)
	derived := anyAttr(model.NSCList, []model.NamespaceURI{"urn:a"})

	unioned, err := Union(derived, base)
	if err != nil {
		t.Fatalf("Union() error = %v", err)
	}
	if unioned == nil {
		t.Fatalf("Union() returned nil")
	}

	intersected, err := Intersect(derived, base)
	if err != nil {
		t.Fatalf("Intersect() error = %v", err)
	}
	if intersected == nil {
		t.Fatalf("Intersect() returned nil")
	}
}

func TestRestrictErrors(t *testing.T) {
	derived := anyAttr(model.NSCAny, nil)
	if _, err := Restrict(nil, derived); !errors.Is(err, ErrRestrictionAddsWildcard) {
		t.Fatalf("Restrict(nil, derived) error = %v, want ErrRestrictionAddsWildcard", err)
	}
}

func TestRestrictRejectsWeakerProcessContents(t *testing.T) {
	base := &model.AnyAttribute{
		Namespace:       model.NSCAny,
		ProcessContents: model.Strict,
	}
	derived := &model.AnyAttribute{
		Namespace:       model.NSCAny,
		ProcessContents: model.Skip,
	}
	if _, err := Restrict(base, derived); !errors.Is(err, ErrRestrictionNotExpressible) {
		t.Fatalf("Restrict(base, derived) error = %v, want ErrRestrictionNotExpressible", err)
	}
}

func TestRestrictRejectsNonSubsetNamespace(t *testing.T) {
	base := &model.AnyAttribute{
		Namespace:       model.NSCList,
		NamespaceList:   []model.NamespaceURI{"urn:a"},
		ProcessContents: model.Lax,
	}
	derived := &model.AnyAttribute{
		Namespace:       model.NSCAny,
		ProcessContents: model.Lax,
	}
	if _, err := Restrict(base, derived); !errors.Is(err, ErrRestrictionNotExpressible) {
		t.Fatalf("Restrict(base, derived) error = %v, want ErrRestrictionNotExpressible", err)
	}
}

func TestRestrictAcceptsValidSubset(t *testing.T) {
	base := &model.AnyAttribute{
		Namespace:       model.NSCAny,
		ProcessContents: model.Lax,
	}
	derived := &model.AnyAttribute{
		Namespace:       model.NSCList,
		NamespaceList:   []model.NamespaceURI{"urn:a"},
		ProcessContents: model.Strict,
	}
	out, err := Restrict(base, derived)
	if err != nil {
		t.Fatalf("Restrict(base, derived) error = %v", err)
	}
	if out == nil {
		t.Fatalf("Restrict(base, derived) returned nil")
	}
	if out.ProcessContents != model.Strict {
		t.Fatalf("ProcessContents = %v, want %v", out.ProcessContents, model.Strict)
	}
	if out.Namespace != model.NSCList {
		t.Fatalf("Namespace = %v, want %v", out.Namespace, model.NSCList)
	}
}
