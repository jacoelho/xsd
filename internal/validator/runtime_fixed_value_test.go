package validator

import (
	"bytes"
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestSelectTextDefaultOrFixed(t *testing.T) {
	elem := runtime.Element{
		Default:       runtime.ValueRef{Present: true, Off: 1, Len: 1},
		Fixed:         runtime.ValueRef{Present: true, Off: 2, Len: 1},
		DefaultKey:    runtime.ValueKeyRef{Kind: runtime.VKString, Ref: runtime.ValueRef{Present: true, Off: 3, Len: 1}},
		FixedKey:      runtime.ValueKeyRef{Kind: runtime.VKQName, Ref: runtime.ValueRef{Present: true, Off: 4, Len: 1}},
		DefaultMember: 5,
		FixedMember:   6,
	}
	ct := runtime.ComplexType{
		TextDefault:       runtime.ValueRef{Present: true, Off: 7, Len: 1},
		TextFixed:         runtime.ValueRef{Present: true, Off: 8, Len: 1},
		TextDefaultMember: 9,
		TextFixedMember:   10,
	}

	tests := []struct {
		name           string
		hasContent     bool
		elemOK         bool
		hasComplexText bool
		want           selectedValue
	}{
		{
			name:       "has content disables fallback",
			hasContent: true,
			want:       selectedValue{},
		},
		{
			name:           "element fixed first",
			elemOK:         true,
			hasComplexText: true,
			want: selectedValue{
				Value:   elem.Fixed,
				Key:     elem.FixedKey,
				Member:  elem.FixedMember,
				Fixed:   true,
				Present: true,
			},
		},
		{
			name:           "complex fixed when element unavailable",
			hasComplexText: true,
			want: selectedValue{
				Value:   ct.TextFixed,
				Member:  ct.TextFixedMember,
				Fixed:   true,
				Present: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := selectTextDefaultOrFixed(tc.hasContent, &elem, tc.elemOK, ct, tc.hasComplexText)
			if got != tc.want {
				t.Fatalf("selectTextDefaultOrFixed() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestSelectTextFixedConstraint(t *testing.T) {
	elem := runtime.Element{
		Fixed:       runtime.ValueRef{Present: true, Off: 1, Len: 1},
		FixedKey:    runtime.ValueKeyRef{Kind: runtime.VKQName, Ref: runtime.ValueRef{Present: true, Off: 2, Len: 1}},
		FixedMember: 3,
	}
	ct := runtime.ComplexType{
		TextFixed:       runtime.ValueRef{Present: true, Off: 4, Len: 1},
		TextFixedMember: 5,
	}

	got := selectTextFixedConstraint(&elem, true, ct, true)
	want := selectedValue{
		Value:   elem.Fixed,
		Key:     elem.FixedKey,
		Member:  elem.FixedMember,
		Fixed:   true,
		Present: true,
	}
	if got != want {
		t.Fatalf("selectTextFixedConstraint(element) = %+v, want %+v", got, want)
	}

	got = selectTextFixedConstraint(&elem, false, ct, true)
	want = selectedValue{
		Value:   ct.TextFixed,
		Member:  ct.TextFixedMember,
		Fixed:   true,
		Present: true,
	}
	if got != want {
		t.Fatalf("selectTextFixedConstraint(complex) = %+v, want %+v", got, want)
	}
}

func TestMaterializeValueKeyStoredAndDerived(t *testing.T) {
	storedRef := runtime.ValueRef{Off: 0, Len: 6, Present: true}
	storedKey := runtime.ValueKeyRef{Kind: runtime.VKQName, Ref: storedRef}
	storedValues := map[runtime.ValueRef][]byte{
		storedRef: []byte("stored"),
	}

	deriveCalls := 0
	deriveKey := func(validator runtime.ValidatorID, canonical []byte, member runtime.ValidatorID) (runtime.ValueKind, []byte, error) {
		deriveCalls++
		if validator != 11 {
			t.Fatalf("validator = %d, want 11", validator)
		}
		if member != 13 {
			t.Fatalf("member = %d, want 13", member)
		}
		if !bytes.Equal(canonical, []byte("abc")) {
			t.Fatalf("canonical = %q, want %q", canonical, "abc")
		}
		return runtime.VKString, []byte("derived"), nil
	}
	readValue := func(ref runtime.ValueRef) []byte { return storedValues[ref] }

	kind, key, err := materializeValueKey(1, []byte("ignored"), 0, storedKey, readValue, deriveKey)
	if err != nil {
		t.Fatalf("materializeValueKey(stored) error = %v", err)
	}
	if kind != runtime.VKQName {
		t.Fatalf("stored kind = %v, want %v", kind, runtime.VKQName)
	}
	if !bytes.Equal(key, []byte("stored")) {
		t.Fatalf("stored key = %q, want %q", key, "stored")
	}
	if deriveCalls != 0 {
		t.Fatalf("derive calls = %d, want 0", deriveCalls)
	}

	kind, key, err = materializeValueKey(11, []byte("abc"), 13, runtime.ValueKeyRef{}, readValue, deriveKey)
	if err != nil {
		t.Fatalf("materializeValueKey(derived) error = %v", err)
	}
	if kind != runtime.VKString {
		t.Fatalf("derived kind = %v, want %v", kind, runtime.VKString)
	}
	if !bytes.Equal(key, []byte("derived")) {
		t.Fatalf("derived key = %q, want %q", key, "derived")
	}
	if deriveCalls != 1 {
		t.Fatalf("derive calls = %d, want 1", deriveCalls)
	}
}

func TestMatchFixedValueDerivesObservedKey(t *testing.T) {
	fixedKeyRef := runtime.ValueRef{Off: 0, Len: 8, Present: true}
	fixedKey := runtime.ValueKeyRef{Kind: runtime.VKQName, Ref: fixedKeyRef}
	readValue := func(ref runtime.ValueRef) []byte {
		if ref != fixedKeyRef {
			t.Fatalf("readValue ref = %+v, want %+v", ref, fixedKeyRef)
		}
		return []byte("expected")
	}

	deriveCalls := 0
	match, err := matchFixedValue(
		17,
		19,
		[]byte("abc"),
		runtime.VKInvalid,
		nil,
		false,
		runtime.ValueRef{},
		fixedKey,
		readValue,
		func(validator runtime.ValidatorID, canonical []byte, member runtime.ValidatorID) (runtime.ValueKind, []byte, error) {
			deriveCalls++
			if validator != 17 {
				t.Fatalf("validator = %d, want 17", validator)
			}
			if member != 19 {
				t.Fatalf("member = %d, want 19", member)
			}
			if !bytes.Equal(canonical, []byte("abc")) {
				t.Fatalf("canonical = %q, want %q", canonical, "abc")
			}
			return runtime.VKQName, []byte("expected"), nil
		},
	)
	if err != nil {
		t.Fatalf("matchFixedValue() error = %v", err)
	}
	if !match {
		t.Fatalf("matchFixedValue() = false, want true")
	}
	if deriveCalls != 1 {
		t.Fatalf("derive calls = %d, want 1", deriveCalls)
	}
}

func TestMatchFixedValueUsesProvidedObservedKey(t *testing.T) {
	fixedKeyRef := runtime.ValueRef{Off: 0, Len: 8, Present: true}
	fixedKey := runtime.ValueKeyRef{Kind: runtime.VKString, Ref: fixedKeyRef}

	match, err := matchFixedValue(
		999,
		0,
		[]byte("ignored"),
		runtime.VKString,
		[]byte("provided"),
		true,
		runtime.ValueRef{},
		fixedKey,
		func(ref runtime.ValueRef) []byte {
			if ref != fixedKeyRef {
				t.Fatalf("readValue ref = %+v, want %+v", ref, fixedKeyRef)
			}
			return []byte("provided")
		},
		func(runtime.ValidatorID, []byte, runtime.ValidatorID) (runtime.ValueKind, []byte, error) {
			return runtime.VKInvalid, nil, errors.New("unexpected derivation")
		},
	)
	if err != nil {
		t.Fatalf("matchFixedValue() unexpected error = %v", err)
	}
	if !match {
		t.Fatalf("matchFixedValue() = false, want true")
	}
}

func TestMatchFixedValueUsesCanonicalBytesWithoutKey(t *testing.T) {
	fixedRef := runtime.ValueRef{Off: 0, Len: 3, Present: true}

	match, err := matchFixedValue(
		0,
		0,
		[]byte("abc"),
		runtime.VKInvalid,
		nil,
		false,
		fixedRef,
		runtime.ValueKeyRef{},
		func(ref runtime.ValueRef) []byte {
			if ref != fixedRef {
				t.Fatalf("readValue ref = %+v, want %+v", ref, fixedRef)
			}
			return []byte("abc")
		},
		func(runtime.ValidatorID, []byte, runtime.ValidatorID) (runtime.ValueKind, []byte, error) {
			return runtime.VKInvalid, nil, errors.New("unexpected derivation")
		},
	)
	if err != nil {
		t.Fatalf("matchFixedValue() unexpected error = %v", err)
	}
	if !match {
		t.Fatalf("matchFixedValue() = false, want true")
	}
}
