package model

import "testing"

func TestAllowsNamespacePolicy(t *testing.T) {
	tests := []struct {
		name       string
		constraint NamespaceConstraint
		list       []NamespaceURI
		targetNS   NamespaceURI
		ns         NamespaceURI
		want       bool
	}{
		{name: "any", constraint: NSCAny, targetNS: "urn:t", ns: "urn:x", want: true},
		{name: "local accepts empty", constraint: NSCLocal, targetNS: "urn:t", ns: NamespaceEmpty, want: true},
		{name: "target matches target namespace", constraint: NSCTargetNamespace, targetNS: "urn:t", ns: "urn:t", want: true},
		{name: "other rejects target", constraint: NSCOther, targetNS: "urn:t", ns: "urn:t", want: false},
		{name: "other rejects local", constraint: NSCOther, targetNS: "urn:t", ns: NamespaceEmpty, want: false},
		{name: "not absent accepts namespace", constraint: NSCNotAbsent, targetNS: "urn:t", ns: "urn:x", want: true},
		{
			name:       "list supports target placeholder",
			constraint: NSCList,
			list:       []NamespaceURI{NamespaceTargetPlaceholder},
			targetNS:   "urn:t",
			ns:         "urn:t",
			want:       true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := AllowsNamespace(tc.constraint, tc.list, tc.targetNS, tc.ns)
			if got != tc.want {
				t.Fatalf("AllowsNamespace() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNamespaceConstraintSubsetPolicy(t *testing.T) {
	type constraintCase struct {
		kind   NamespaceConstraint
		list   []NamespaceURI
		target NamespaceURI
	}

	tests := []struct {
		name    string
		derived constraintCase
		base    constraintCase
		want    bool
	}{
		{
			name:    "target subset of any",
			derived: constraintCase{kind: NSCTargetNamespace, target: "urn:test"},
			base:    constraintCase{kind: NSCAny},
			want:    true,
		},
		{
			name:    "local not subset not-absent",
			derived: constraintCase{kind: NSCLocal},
			base:    constraintCase{kind: NSCNotAbsent},
			want:    false,
		},
		{
			name: "list placeholder subset target",
			derived: constraintCase{
				kind:   NSCList,
				list:   []NamespaceURI{NamespaceTargetPlaceholder},
				target: "urn:test",
			},
			base: constraintCase{
				kind:   NSCTargetNamespace,
				target: "urn:test",
			},
			want: true,
		},
		{
			name:    "other subset same other",
			derived: constraintCase{kind: NSCOther, target: "urn:test"},
			base:    constraintCase{kind: NSCOther, target: "urn:test"},
			want:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NamespaceConstraintSubset(
				tc.derived.kind,
				tc.derived.list,
				tc.derived.target,
				tc.base.kind,
				tc.base.list,
				tc.base.target,
			)
			if got != tc.want {
				t.Fatalf("NamespaceConstraintSubset() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProcessContentsStrongerOrEqualPolicy(t *testing.T) {
	if !ProcessContentsStrongerOrEqual(Strict, Lax) {
		t.Fatalf("strict should be stronger than lax")
	}
	if ProcessContentsStrongerOrEqual(Lax, Strict) {
		t.Fatalf("lax should not be stronger than strict")
	}
	if !ProcessContentsStrongerOrEqual(Skip, Skip) {
		t.Fatalf("skip should be stronger-or-equal to skip")
	}
}

func TestResolveSymbolByProcessContentsPolicy(t *testing.T) {
	resolved, strict, err := ResolveSymbolByProcessContents(Skip, true, func() bool { return true })
	if err != nil || resolved || strict {
		t.Fatalf("skip = (%v,%v,%v), want (false,false,nil)", resolved, strict, err)
	}

	calls := 0
	resolved, strict, err = ResolveSymbolByProcessContents(Strict, true, func() bool {
		calls++
		return false
	})
	if err != nil || resolved || !strict {
		t.Fatalf("strict unresolved = (%v,%v,%v), want (false,true,nil)", resolved, strict, err)
	}
	if calls != 1 {
		t.Fatalf("strict resolver calls = %d, want 1", calls)
	}

	resolved, strict, err = ResolveSymbolByProcessContents(Lax, true, func() bool { return true })
	if err != nil || !resolved || strict {
		t.Fatalf("lax resolved = (%v,%v,%v), want (true,false,nil)", resolved, strict, err)
	}
}

func TestAllowsRuntimeNamespacePolicy(t *testing.T) {
	enum := RuntimeNamespaceConstraint{
		Kind:      RuntimeNamespaceEnumeration,
		HasTarget: true,
		HasLocal:  true,
	}
	if !AllowsRuntimeNamespace(enum, true, false, false) {
		t.Fatalf("enumeration should accept target match")
	}
	if !AllowsRuntimeNamespace(enum, false, true, false) {
		t.Fatalf("enumeration should accept local match")
	}
	if !AllowsRuntimeNamespace(enum, false, false, true) {
		t.Fatalf("enumeration should accept explicit list match")
	}
	if AllowsRuntimeNamespace(RuntimeNamespaceConstraint{Kind: RuntimeNamespaceOther}, true, false, false) {
		t.Fatalf("other should reject target namespace")
	}
	if AllowsRuntimeNamespace(RuntimeNamespaceConstraint{Kind: RuntimeNamespaceNotAbsent}, false, true, false) {
		t.Fatalf("not-absent should reject local namespace")
	}
}
