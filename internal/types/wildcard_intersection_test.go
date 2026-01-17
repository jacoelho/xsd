package types

import (
	"reflect"
	"testing"
)

func TestUnionAnyAttribute_TargetNamespace(t *testing.T) {
	tests := []struct {
		w1             *AnyAttribute
		w2             *AnyAttribute
		name           string
		wantList       []NamespaceURI
		wantConstraint NamespaceConstraint
		wantNil        bool
	}{
		{
			name: "list_with_target_and_other_same_target_is_any",
			w1: &AnyAttribute{
				Namespace:       NSCList,
				NamespaceList:   []NamespaceURI{"a", "b", "c"},
				TargetNamespace: "a",
			},
			w2: &AnyAttribute{
				Namespace:       NSCOther,
				TargetNamespace: "a",
			},
			wantConstraint: NSCAny,
		},
		{
			name: "list_without_target_and_other_same_target_is_other",
			w1: &AnyAttribute{
				Namespace:       NSCList,
				NamespaceList:   []NamespaceURI{"b", "c"},
				TargetNamespace: "a",
			},
			w2: &AnyAttribute{
				Namespace:       NSCOther,
				TargetNamespace: "a",
			},
			wantConstraint: NSCOther,
		},
		{
			name: "any_overrides_list",
			w1: &AnyAttribute{
				Namespace: NSCAny,
			},
			w2: &AnyAttribute{
				Namespace:     NSCList,
				NamespaceList: []NamespaceURI{"a"},
			},
			wantConstraint: NSCAny,
		},
		{
			name: "list_union_dedupes",
			w1: &AnyAttribute{
				Namespace:     NSCList,
				NamespaceList: []NamespaceURI{"a", "b"},
			},
			w2: &AnyAttribute{
				Namespace:     NSCList,
				NamespaceList: []NamespaceURI{"b", "c"},
			},
			wantConstraint: NSCList,
			wantList:       []NamespaceURI{"a", "b", "c"},
		},
		{
			name: "list_with_other_target_namespace_is_invalid",
			w1: &AnyAttribute{
				Namespace:       NSCList,
				NamespaceList:   []NamespaceURI{"a"},
				TargetNamespace: "a",
			},
			w2: &AnyAttribute{
				Namespace:       NSCOther,
				TargetNamespace: "b",
			},
			wantNil: true,
		},
		{
			name: "list_with_empty_and_other_same_target_is_invalid",
			w1: &AnyAttribute{
				Namespace:       NSCList,
				NamespaceList:   []NamespaceURI{NamespaceEmpty, "b"},
				TargetNamespace: "a",
			},
			w2: &AnyAttribute{
				Namespace:       NSCOther,
				TargetNamespace: "a",
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := UnionAnyAttribute(tt.w1, tt.w2)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil union, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected union, got nil")
			}
			if got.Namespace != tt.wantConstraint {
				t.Fatalf("union constraint = %v, want %v", got.Namespace, tt.wantConstraint)
			}
			if tt.wantList != nil {
				assertNamespaceList(t, got.NamespaceList, tt.wantList)
			}
		})
	}
}

func TestIntersectAnyAttribute_Table(t *testing.T) {
	tests := []struct {
		w1             *AnyAttribute
		w2             *AnyAttribute
		name           string
		wantList       []NamespaceURI
		wantConstraint NamespaceConstraint
		wantProcess    ProcessContents
		checkProcess   bool
		wantNil        bool
	}{
		{
			name: "any_and_list_returns_list",
			w1: &AnyAttribute{
				Namespace:       NSCAny,
				ProcessContents: Skip,
			},
			w2: &AnyAttribute{
				Namespace:       NSCList,
				NamespaceList:   []NamespaceURI{"a"},
				ProcessContents: Lax,
			},
			wantConstraint: NSCList,
			wantList:       []NamespaceURI{"a"},
			wantProcess:    Lax,
			checkProcess:   true,
		},
		{
			name: "target_in_list_returns_target_namespace",
			w1: &AnyAttribute{
				Namespace:       NSCTargetNamespace,
				TargetNamespace: "a",
			},
			w2: &AnyAttribute{
				Namespace:     NSCList,
				NamespaceList: []NamespaceURI{"a", "b"},
			},
			wantConstraint: NSCTargetNamespace,
		},
		{
			name: "target_not_in_list_returns_nil",
			w1: &AnyAttribute{
				Namespace:       NSCTargetNamespace,
				TargetNamespace: "a",
			},
			w2: &AnyAttribute{
				Namespace:     NSCList,
				NamespaceList: []NamespaceURI{"b"},
			},
			wantNil: true,
		},
		{
			name: "local_and_list_returns_local",
			w1: &AnyAttribute{
				Namespace: NSCLocal,
			},
			w2: &AnyAttribute{
				Namespace:     NSCList,
				NamespaceList: []NamespaceURI{NamespaceEmpty, "b"},
			},
			wantConstraint: NSCLocal,
		},
		{
			name: "other_and_list_filters",
			w1: &AnyAttribute{
				Namespace:       NSCOther,
				TargetNamespace: "a",
			},
			w2: &AnyAttribute{
				Namespace:     NSCList,
				NamespaceList: []NamespaceURI{"a", "b", NamespaceEmpty},
			},
			wantConstraint: NSCList,
			wantList:       []NamespaceURI{"b"},
		},
		{
			name: "other_and_other_different_target_is_nil",
			w1: &AnyAttribute{
				Namespace:       NSCOther,
				TargetNamespace: "a",
			},
			w2: &AnyAttribute{
				Namespace:       NSCOther,
				TargetNamespace: "b",
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IntersectAnyAttribute(tt.w1, tt.w2)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil intersection, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected intersection, got nil")
			}
			if got.Namespace != tt.wantConstraint {
				t.Fatalf("intersection constraint = %v, want %v", got.Namespace, tt.wantConstraint)
			}
			if tt.wantList != nil {
				assertNamespaceList(t, got.NamespaceList, tt.wantList)
			}
			if tt.checkProcess && got.ProcessContents != tt.wantProcess {
				t.Fatalf("intersection processContents = %v, want %v", got.ProcessContents, tt.wantProcess)
			}
		})
	}
}

func TestIntersectAnyElement_Table(t *testing.T) {
	tests := []struct {
		w1             *AnyElement
		w2             *AnyElement
		name           string
		wantList       []NamespaceURI
		wantMin        Occurs
		wantMax        Occurs
		wantConstraint NamespaceConstraint
		wantProcess    ProcessContents
		checkMin       bool
		checkMax       bool
		checkProcess   bool
		wantNil        bool
	}{
		{
			name: "min_max_uses_restrictive_values",
			w1: &AnyElement{
				Namespace: NSCAny,
				MinOccurs: OccursFromInt(1),
				MaxOccurs: OccursFromInt(5),
			},
			w2: &AnyElement{
				Namespace: NSCAny,
				MinOccurs: OccursFromInt(2),
				MaxOccurs: OccursFromInt(3),
			},
			wantConstraint: NSCAny,
			wantMin:        OccursFromInt(2),
			wantMax:        OccursFromInt(3),
			checkMin:       true,
			checkMax:       true,
		},
		{
			name: "unbounded_max_occurs",
			w1: &AnyElement{
				Namespace: NSCAny,
				MinOccurs: OccursFromInt(0),
				MaxOccurs: OccursUnbounded,
			},
			w2: &AnyElement{
				Namespace: NSCAny,
				MinOccurs: OccursFromInt(1),
				MaxOccurs: OccursFromInt(4),
			},
			wantConstraint: NSCAny,
			wantMin:        OccursFromInt(1),
			wantMax:        OccursFromInt(4),
			checkMin:       true,
			checkMax:       true,
		},
		{
			name: "process_contents_is_most_restrictive",
			w1: &AnyElement{
				Namespace:       NSCAny,
				ProcessContents: Skip,
			},
			w2: &AnyElement{
				Namespace:       NSCAny,
				ProcessContents: Strict,
			},
			wantConstraint: NSCAny,
			wantProcess:    Strict,
			checkProcess:   true,
		},
		{
			name: "other_and_local_empty_is_nil",
			w1: &AnyElement{
				Namespace:       NSCOther,
				TargetNamespace: "a",
			},
			w2: &AnyElement{
				Namespace: NSCLocal,
			},
			wantNil: true,
		},
		{
			name: "list_intersection_returns_common",
			w1: &AnyElement{
				Namespace:     NSCList,
				NamespaceList: []NamespaceURI{"a", "b"},
			},
			w2: &AnyElement{
				Namespace:     NSCList,
				NamespaceList: []NamespaceURI{"b", "c"},
			},
			wantConstraint: NSCList,
			wantList:       []NamespaceURI{"b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IntersectAnyElement(tt.w1, tt.w2)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("expected nil intersection, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected intersection, got nil")
			}
			if got.Namespace != tt.wantConstraint {
				t.Fatalf("intersection constraint = %v, want %v", got.Namespace, tt.wantConstraint)
			}
			if tt.wantList != nil {
				assertNamespaceList(t, got.NamespaceList, tt.wantList)
			}
			if tt.checkMin && !got.MinOccurs.Equal(tt.wantMin) {
				t.Fatalf("intersection minOccurs = %s, want %s", got.MinOccurs, tt.wantMin)
			}
			if tt.checkMax && !got.MaxOccurs.Equal(tt.wantMax) {
				t.Fatalf("intersection maxOccurs = %s, want %s", got.MaxOccurs, tt.wantMax)
			}
			if tt.checkProcess && got.ProcessContents != tt.wantProcess {
				t.Fatalf("intersection processContents = %v, want %v", got.ProcessContents, tt.wantProcess)
			}
		})
	}
}

func assertNamespaceList(t *testing.T, got, want []NamespaceURI) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("namespace list = %v, want %v", got, want)
	}
}
