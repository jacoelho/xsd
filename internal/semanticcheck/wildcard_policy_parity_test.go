package semanticcheck

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
)

func TestValidateWildcardToWildcardRestrictionParityWithModelPolicy(t *testing.T) {
	tests := []struct {
		name        string
		base        *model.AnyElement
		restriction *model.AnyElement
	}{
		{
			name: "same any strict",
			base: &model.AnyElement{
				Namespace:       model.NSCAny,
				ProcessContents: model.Strict,
			},
			restriction: &model.AnyElement{
				Namespace:       model.NSCAny,
				ProcessContents: model.Strict,
			},
		},
		{
			name: "weakened process contents rejected",
			base: &model.AnyElement{
				Namespace:       model.NSCAny,
				ProcessContents: model.Strict,
			},
			restriction: &model.AnyElement{
				Namespace:       model.NSCAny,
				ProcessContents: model.Lax,
			},
		},
		{
			name: "subset list under any",
			base: &model.AnyElement{
				Namespace:       model.NSCAny,
				ProcessContents: model.Skip,
			},
			restriction: &model.AnyElement{
				Namespace:       model.NSCList,
				NamespaceList:   []model.NamespaceURI{"urn:a"},
				ProcessContents: model.Skip,
			},
		},
		{
			name: "non-subset any under list rejected",
			base: &model.AnyElement{
				Namespace:       model.NSCList,
				NamespaceList:   []model.NamespaceURI{"urn:a"},
				ProcessContents: model.Skip,
			},
			restriction: &model.AnyElement{
				Namespace:       model.NSCAny,
				ProcessContents: model.Skip,
			},
		},
		{
			name: "placeholder subset accepted",
			base: &model.AnyElement{
				Namespace:       model.NSCList,
				NamespaceList:   []model.NamespaceURI{"urn:target"},
				TargetNamespace: "urn:base",
				ProcessContents: model.Lax,
			},
			restriction: &model.AnyElement{
				Namespace:       model.NSCList,
				NamespaceList:   []model.NamespaceURI{model.NamespaceTargetPlaceholder},
				TargetNamespace: "urn:target",
				ProcessContents: model.Strict,
			},
		},
		{
			name: "local not subset not-absent rejected",
			base: &model.AnyElement{
				Namespace:       model.NSCNotAbsent,
				ProcessContents: model.Skip,
			},
			restriction: &model.AnyElement{
				Namespace:       model.NSCLocal,
				ProcessContents: model.Skip,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			want := model.ProcessContentsStrongerOrEqual(tc.restriction.ProcessContents, tc.base.ProcessContents) &&
				model.NamespaceConstraintSubset(
					tc.restriction.Namespace,
					tc.restriction.NamespaceList,
					tc.restriction.TargetNamespace,
					tc.base.Namespace,
					tc.base.NamespaceList,
					tc.base.TargetNamespace,
				)

			err := validateWildcardToWildcardRestriction(tc.base, tc.restriction)
			if want && err != nil {
				t.Fatalf("validateWildcardToWildcardRestriction() unexpected error: %v", err)
			}
			if !want && err == nil {
				t.Fatalf("validateWildcardToWildcardRestriction() expected error")
			}
		})
	}
}
