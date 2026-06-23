package validate

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

type attributeRuntimeStub struct {
	globalAttrs    map[runtime.QName]runtime.AttributeID
	invalidGlobals map[runtime.QName]bool
	wildcards      map[runtime.WildcardID]runtime.Wildcard
}

func (s attributeRuntimeStub) WildcardView(id runtime.WildcardID) (runtime.WildcardView, bool) {
	w, ok := s.wildcards[id]
	if !ok {
		return runtime.WildcardView{}, false
	}
	return runtime.NewWildcardView(nil, &w), true
}

func (s attributeRuntimeStub) GlobalAttribute(name runtime.QName) (runtime.AttributeID, bool, bool) {
	id, ok := s.globalAttrs[name]
	if !ok {
		return 0, false, true
	}
	if s.invalidGlobals[name] {
		return 0, false, false
	}
	return id, true, true
}

func TestAttributeSeenTracksBitsetAndSliceSlots(t *testing.T) {
	t.Parallel()

	bitset := NewAttributeSeen(2)
	if !bitset.mark(1) {
		t.Fatal("first bitset mark failed")
	}
	if bitset.mark(1) {
		t.Fatal("duplicate bitset mark succeeded")
	}
	if !bitset.has(1) || bitset.has(0) {
		t.Fatalf("bitset has states = slot0:%v slot1:%v", bitset.has(0), bitset.has(1))
	}

	const slot = 65
	slice := NewAttributeSeen(70)
	if !slice.mark(slot) {
		t.Fatal("first slice mark failed")
	}
	if slice.mark(slot) {
		t.Fatal("duplicate slice mark succeeded")
	}
	if !slice.has(slot) || slice.has(1) {
		t.Fatalf("slice has states = slot1:%v slot%d:%v", slice.has(1), slot, slice.has(slot))
	}
}

func TestMatchAttributeWildcard(t *testing.T) {
	t.Parallel()

	const wildcard = runtime.WildcardID(1)
	const attr = runtime.AttributeID(2)
	name := runtime.QName{Namespace: runtime.EmptyNamespaceID, Local: 1}
	rn := runtime.RuntimeName{Known: true, Name: name, Local: "known"}
	tests := []struct {
		name  string
		rt    attributeRuntimeStub
		id    runtime.WildcardID
		rn    runtime.RuntimeName
		want  AttributeWildcardMatch
		valid bool
	}{
		{
			name:  "no wildcard",
			id:    runtime.NoWildcard,
			valid: true,
		},
		{
			name: "invalid wildcard",
			id:   wildcard,
		},
		{
			name: "namespace not allowed",
			id:   wildcard,
			rt: attributeRuntimeStub{
				wildcards: map[runtime.WildcardID]runtime.Wildcard{wildcard: {Mode: runtime.WildcardLocal, Process: runtime.ProcessStrict}},
			},
			rn:    runtime.RuntimeName{NS: "urn:not-local", Local: "x"},
			valid: true,
		},
		{
			name: "skip",
			id:   wildcard,
			rt: attributeRuntimeStub{
				wildcards: map[runtime.WildcardID]runtime.Wildcard{wildcard: {Mode: runtime.WildcardAny, Process: runtime.ProcessSkip}},
			},
			rn:    runtime.RuntimeName{NS: "urn:any", Local: "x"},
			want:  AttributeWildcardMatch{Matched: true, Skip: true},
			valid: true,
		},
		{
			name: "lax missing",
			id:   wildcard,
			rt: attributeRuntimeStub{
				wildcards: map[runtime.WildcardID]runtime.Wildcard{wildcard: {Mode: runtime.WildcardAny, Process: runtime.ProcessLax}},
			},
			rn:    runtime.RuntimeName{NS: "urn:any", Local: "x"},
			want:  AttributeWildcardMatch{Matched: true, LaxMissing: true},
			valid: true,
		},
		{
			name: "lax known missing global",
			id:   wildcard,
			rt: attributeRuntimeStub{
				wildcards: map[runtime.WildcardID]runtime.Wildcard{wildcard: {Mode: runtime.WildcardAny, Process: runtime.ProcessLax}},
			},
			rn:    rn,
			want:  AttributeWildcardMatch{Matched: true, LaxMissing: true},
			valid: true,
		},
		{
			name: "strict missing",
			id:   wildcard,
			rt: attributeRuntimeStub{
				wildcards: map[runtime.WildcardID]runtime.Wildcard{wildcard: {Mode: runtime.WildcardAny, Process: runtime.ProcessStrict}},
			},
			rn:    runtime.RuntimeName{NS: "urn:any", Local: "x"},
			want:  AttributeWildcardMatch{Matched: true},
			valid: true,
		},
		{
			name: "known global",
			id:   wildcard,
			rt: attributeRuntimeStub{
				globalAttrs: map[runtime.QName]runtime.AttributeID{name: attr},
				wildcards:   map[runtime.WildcardID]runtime.Wildcard{wildcard: {Mode: runtime.WildcardAny, Process: runtime.ProcessStrict}},
			},
			rn:    rn,
			want:  AttributeWildcardMatch{Attribute: attr, Matched: true, HasAttribute: true},
			valid: true,
		},
		{
			name: "known global invalid metadata",
			id:   wildcard,
			rt: attributeRuntimeStub{
				globalAttrs:    map[runtime.QName]runtime.AttributeID{name: attr},
				invalidGlobals: map[runtime.QName]bool{name: true},
				wildcards:      map[runtime.WildcardID]runtime.Wildcard{wildcard: {Mode: runtime.WildcardAny, Process: runtime.ProcessStrict}},
			},
			rn:    rn,
			valid: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, valid := MatchAttributeWildcard(tc.rt, tc.id, tc.rn)
			if valid != tc.valid || got != tc.want {
				t.Fatalf("MatchAttributeWildcard() = %+v/%v, want %+v/%v", got, valid, tc.want, tc.valid)
			}
		})
	}
}
