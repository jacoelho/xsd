package validator

import (
	"bytes"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestSelectAttrDefaultFixed(t *testing.T) {
	defaultRef := runtime.ValueRef{Off: 1, Len: 2, Present: true}
	fixedRef := runtime.ValueRef{Off: 3, Len: 4, Present: true}
	defaultKey := runtime.ValueKeyRef{Kind: runtime.VKString, Ref: runtime.ValueRef{Off: 7, Len: 3, Present: true}}
	fixedKey := runtime.ValueKeyRef{Kind: runtime.VKQName, Ref: runtime.ValueRef{Off: 10, Len: 5, Present: true}}

	tests := []struct {
		name   string
		use    *runtime.AttrUse
		policy defaultFixedPolicy
	}{
		{
			name:   "nil use",
			use:    nil,
			policy: defaultFixedPolicy{},
		},
		{
			name:   "empty use",
			use:    &runtime.AttrUse{},
			policy: defaultFixedPolicy{},
		},
		{
			name: "default only",
			use: &runtime.AttrUse{
				Default:       defaultRef,
				DefaultKey:    defaultKey,
				DefaultMember: 9,
			},
			policy: defaultFixedPolicy{
				value:   defaultRef,
				key:     defaultKey,
				member:  9,
				present: true,
			},
		},
		{
			name: "fixed wins over default",
			use: &runtime.AttrUse{
				Default:       defaultRef,
				DefaultKey:    defaultKey,
				DefaultMember: 9,
				Fixed:         fixedRef,
				FixedKey:      fixedKey,
				FixedMember:   11,
			},
			policy: defaultFixedPolicy{
				value:   fixedRef,
				key:     fixedKey,
				member:  11,
				fixed:   true,
				present: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := selectAttrDefaultFixed(tc.use)
			if got != tc.policy {
				t.Fatalf("policy = %+v, want %+v", got, tc.policy)
			}
		})
	}
}

func TestSelectTextDefaultFixedPrecedence(t *testing.T) {
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

	cases := []struct {
		name           string
		hasContent     bool
		elemOK         bool
		hasComplexText bool
		want           defaultFixedPolicy
	}{
		{
			name:       "has content disables fallback",
			hasContent: true,
			want:       defaultFixedPolicy{},
		},
		{
			name:           "element fixed first",
			hasContent:     false,
			elemOK:         true,
			hasComplexText: true,
			want: defaultFixedPolicy{
				value:   elem.Fixed,
				key:     elem.FixedKey,
				member:  elem.FixedMember,
				fixed:   true,
				present: true,
			},
		},
		{
			name:           "complex fixed when element unavailable",
			hasContent:     false,
			elemOK:         false,
			hasComplexText: true,
			want: defaultFixedPolicy{
				value:   ct.TextFixed,
				member:  ct.TextFixedMember,
				fixed:   true,
				present: true,
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := selectTextDefaultFixed(tc.hasContent, elem, tc.elemOK, ct, tc.hasComplexText)
			if got != tc.want {
				t.Fatalf("policy = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestSelectTextFixedConstraintPrecedence(t *testing.T) {
	elem := runtime.Element{
		Fixed:       runtime.ValueRef{Present: true, Off: 1, Len: 1},
		FixedKey:    runtime.ValueKeyRef{Kind: runtime.VKQName, Ref: runtime.ValueRef{Present: true, Off: 2, Len: 1}},
		FixedMember: 3,
	}
	ct := runtime.ComplexType{
		TextFixed:       runtime.ValueRef{Present: true, Off: 4, Len: 1},
		TextFixedMember: 5,
	}

	got := selectTextFixedConstraint(elem, true, ct, true)
	want := defaultFixedPolicy{
		value:   elem.Fixed,
		key:     elem.FixedKey,
		member:  elem.FixedMember,
		fixed:   true,
		present: true,
	}
	if got != want {
		t.Fatalf("element fixed policy = %+v, want %+v", got, want)
	}

	got = selectTextFixedConstraint(elem, false, ct, true)
	want = defaultFixedPolicy{
		value:   ct.TextFixed,
		member:  ct.TextFixedMember,
		fixed:   true,
		present: true,
	}
	if got != want {
		t.Fatalf("complex fixed policy = %+v, want %+v", got, want)
	}
}

func TestMaterializePolicyKeyStoredAndDerived(t *testing.T) {
	schema, _ := buildAttrFixtureNoRequired(t)
	schema.Values.Blob = []byte("stored")
	sess := NewSession(schema)

	storedKey := runtime.ValueKeyRef{
		Kind: runtime.VKQName,
		Ref: runtime.ValueRef{
			Off:     0,
			Len:     uint32(len("stored")),
			Present: true,
		},
	}
	kind, key, err := sess.materializePolicyKey(1, []byte("ignored"), 0, storedKey)
	if err != nil {
		t.Fatalf("materializePolicyKey(stored): %v", err)
	}
	if kind != runtime.VKQName {
		t.Fatalf("stored kind = %v, want %v", kind, runtime.VKQName)
	}
	if !bytes.Equal(key, []byte("stored")) {
		t.Fatalf("stored key = %q, want %q", key, "stored")
	}

	kind, key, err = sess.materializePolicyKey(1, []byte("abc"), 0, runtime.ValueKeyRef{})
	if err != nil {
		t.Fatalf("materializePolicyKey(derived): %v", err)
	}
	if kind != runtime.VKString {
		t.Fatalf("derived kind = %v, want %v", kind, runtime.VKString)
	}
	if len(key) == 0 {
		t.Fatalf("derived key = empty, want non-empty")
	}
}

func TestMaterializeObservedKeyUsesMetricsAndFallback(t *testing.T) {
	schema, _ := buildAttrFixtureNoRequired(t)
	sess := NewSession(schema)

	kind, key, err := sess.materializeObservedKey(
		1,
		[]byte("ignored"),
		nil,
		0,
		ValueMetrics{keyKind: runtime.VKQName, keyBytes: []byte("provided")},
	)
	if err != nil {
		t.Fatalf("materializeObservedKey(metrics): %v", err)
	}
	if kind != runtime.VKQName {
		t.Fatalf("metrics kind = %v, want %v", kind, runtime.VKQName)
	}
	if !bytes.Equal(key, []byte("provided")) {
		t.Fatalf("metrics key = %q, want %q", key, "provided")
	}

	kind, key, err = sess.materializeObservedKey(
		1,
		[]byte("abc"),
		nil,
		0,
		ValueMetrics{},
	)
	if err != nil {
		t.Fatalf("materializeObservedKey(derived): %v", err)
	}
	if kind != runtime.VKString {
		t.Fatalf("derived kind = %v, want %v", kind, runtime.VKString)
	}
	if len(key) == 0 {
		t.Fatalf("derived key = empty, want non-empty")
	}
}

func TestFixedValueMatchesDerivesObservedKeyFromPolicyKernel(t *testing.T) {
	schema, _ := buildAttrFixtureNoRequired(t)
	sess := NewSession(schema)
	expectedKind, expectedKey, err := sess.keyForCanonicalValue(1, []byte("abc"), nil, 0)
	if err != nil {
		t.Fatalf("keyForCanonicalValue() error = %v", err)
	}
	schema.Values.Blob = slices.Clone(expectedKey)

	match, err := sess.fixedValueMatches(
		1,
		0,
		[]byte("abc"),
		ValueMetrics{},
		nil,
		runtime.ValueRef{},
		runtime.ValueKeyRef{
			Kind: expectedKind,
			Ref: runtime.ValueRef{
				Off:     0,
				Len:     uint32(len(expectedKey)),
				Present: true,
			},
		},
	)
	if err != nil {
		t.Fatalf("fixedValueMatches() error = %v", err)
	}
	if !match {
		t.Fatalf("fixedValueMatches() = false, want true")
	}
}

func TestFixedValueMatchesUsesProvidedMetricsWithoutDerivation(t *testing.T) {
	schema, _ := buildAttrFixtureNoRequired(t)
	schema.Values.Blob = []byte("provided")
	sess := NewSession(schema)

	match, err := sess.fixedValueMatches(
		999, // invalid validator id: this would fail if derivation were attempted
		0,
		[]byte("ignored"),
		ValueMetrics{keyKind: runtime.VKString, keyBytes: []byte("provided")},
		nil,
		runtime.ValueRef{},
		runtime.ValueKeyRef{
			Kind: runtime.VKString,
			Ref: runtime.ValueRef{
				Off:     0,
				Len:     uint32(len("provided")),
				Present: true,
			},
		},
	)
	if err != nil {
		t.Fatalf("fixedValueMatches() unexpected error = %v", err)
	}
	if !match {
		t.Fatalf("fixedValueMatches() = false, want true")
	}
}
