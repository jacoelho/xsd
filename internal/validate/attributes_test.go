package validate

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestAttributeSeenTracksBitsetAndSliceSlots(t *testing.T) {
	t.Parallel()

	var scratch []bool
	bitset := newAttributeSeenWithScratch(2, &scratch)
	if scratch != nil {
		t.Fatalf("inline attribute presence allocated scratch: %v", scratch)
	}
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
	slice := newAttributeSeenWithScratch(70, &scratch)
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

func TestAttributeSeenScratchIsClearedBeforeReuse(t *testing.T) {
	var scratch []bool
	seen := newAttributeSeenWithScratch(70, &scratch)
	if !seen.mark(69) {
		t.Fatal("first mark failed")
	}
	retained := &scratch[0]

	seen = newAttributeSeenWithScratch(70, &scratch)
	if &scratch[0] != retained {
		t.Fatal("attribute presence scratch was not reused")
	}
	if !seen.mark(69) {
		t.Fatal("reused attribute presence scratch retained a mark")
	}

	retained = &scratch[0]
	seen = newAttributeSeenWithScratch(maxRetainedSliceCap+1, &scratch)
	if &scratch[0] != retained {
		t.Fatal("oversized attribute presence replaced retained scratch")
	}
	if !seen.mark(maxRetainedSliceCap) || !seen.has(maxRetainedSliceCap) {
		t.Fatal("oversized attribute presence did not track final slot")
	}
}

func TestMatchAttributeWildcard(t *testing.T) {
	t.Parallel()

	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:test" targetNamespace="urn:test">
  <xs:attribute name="known" type="xs:string"/>
  <xs:element name="strict"><xs:complexType><xs:anyAttribute processContents="strict"/></xs:complexType></xs:element>
  <xs:element name="lax"><xs:complexType><xs:anyAttribute processContents="lax"/></xs:complexType></xs:element>
  <xs:element name="skip"><xs:complexType><xs:anyAttribute processContents="skip"/></xs:complexType></xs:element>
  <xs:element name="local"><xs:complexType><xs:anyAttribute namespace="##local" processContents="strict"/></xs:complexType></xs:element>
</xs:schema>`)
	wildcard := func(local string) runtime.WildcardID {
		q, ok := rt.LookupQName("urn:test", local)
		if !ok {
			t.Fatalf("missing element name %s", local)
		}
		_, info, ok := rt.RootElement(runtime.RuntimeName{Known: true, Name: q, NS: "urn:test", Local: local})
		if !ok {
			t.Fatalf("missing element %s", local)
		}
		uses, has, valid := rt.AttributeUseSetForType(info.Type)
		if !has || !valid {
			t.Fatalf("attribute uses for %s = has %v, valid %v", local, has, valid)
		}
		return uses.Wildcard()
	}
	knownName, ok := rt.LookupQName("urn:test", "known")
	if !ok {
		t.Fatal("missing global attribute name")
	}
	knownID, found, valid := rt.GlobalAttribute(knownName)
	if !found || !valid {
		t.Fatalf("global attribute = found %v, valid %v", found, valid)
	}
	missingName, ok := rt.LookupQName("urn:test", "strict")
	if !ok {
		t.Fatal("missing known non-attribute name")
	}
	tests := []struct {
		name     string
		wildcard runtime.WildcardID
		rn       runtime.RuntimeName
		want     AttributeWildcardMatch
		valid    bool
	}{
		{
			name:     "no wildcard",
			wildcard: runtime.NoWildcard,
			valid:    true,
		},
		{
			name:     "invalid wildcard",
			wildcard: runtime.WildcardID(999),
		},
		{
			name:     "namespace not allowed",
			wildcard: wildcard("local"),
			rn:       runtime.RuntimeName{NS: "urn:not-local", Local: "x"},
			valid:    true,
		},
		{
			name:     "skip",
			wildcard: wildcard("skip"),
			rn:       runtime.RuntimeName{NS: "urn:any", Local: "x"},
			want:     AttributeWildcardMatch{Matched: true, Skip: true},
			valid:    true,
		},
		{
			name:     "lax missing",
			wildcard: wildcard("lax"),
			rn:       runtime.RuntimeName{NS: "urn:any", Local: "x"},
			want:     AttributeWildcardMatch{Matched: true, LaxMissing: true},
			valid:    true,
		},
		{
			name:     "lax known missing global",
			wildcard: wildcard("lax"),
			rn:       runtime.RuntimeName{Known: true, Name: missingName, NS: "urn:test", Local: "strict"},
			want:     AttributeWildcardMatch{Matched: true, LaxMissing: true},
			valid:    true,
		},
		{
			name:     "strict missing",
			wildcard: wildcard("strict"),
			rn:       runtime.RuntimeName{NS: "urn:any", Local: "x"},
			want:     AttributeWildcardMatch{Matched: true},
			valid:    true,
		},
		{
			name:     "known global",
			wildcard: wildcard("strict"),
			rn:       runtime.RuntimeName{Known: true, Name: knownName, NS: "urn:test", Local: "known"},
			want:     AttributeWildcardMatch{Attribute: knownID, Matched: true, HasAttribute: true},
			valid:    true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, valid := MatchAttributeWildcard(rt, tc.wildcard, tc.rn)
			if valid != tc.valid || got != tc.want {
				t.Fatalf("MatchAttributeWildcard() = %+v/%v, want %+v/%v", got, valid, tc.want, tc.valid)
			}
		})
	}
}
