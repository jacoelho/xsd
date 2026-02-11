package wildcardpolicy

import "testing"

func TestAllowsNamespace(t *testing.T) {
	tests := []struct {
		name       string
		constraint NamespaceConstraintKind
		list       []string
		targetNS   string
		ns         string
		want       bool
	}{
		{name: "any", constraint: NamespaceAny, targetNS: "urn:t", ns: "urn:x", want: true},
		{name: "local accepts empty", constraint: NamespaceLocal, targetNS: "urn:t", ns: "", want: true},
		{name: "target matches target namespace", constraint: NamespaceTargetNamespace, targetNS: "urn:t", ns: "urn:t", want: true},
		{name: "other rejects target", constraint: NamespaceOther, targetNS: "urn:t", ns: "urn:t", want: false},
		{name: "other rejects local", constraint: NamespaceOther, targetNS: "urn:t", ns: "", want: false},
		{name: "not absent accepts namespace", constraint: NamespaceNotAbsent, targetNS: "urn:t", ns: "urn:x", want: true},
		{name: "list supports target placeholder", constraint: NamespaceList, list: []string{NamespaceTargetPlaceholder}, targetNS: "urn:t", ns: "urn:t", want: true},
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

func TestNamespaceConstraintSubset(t *testing.T) {
	tests := []struct {
		name    string
		derived NamespaceConstraint
		base    NamespaceConstraint
		want    bool
	}{
		{
			name: "target subset of any",
			derived: NamespaceConstraint{
				Kind:     NamespaceTargetNamespace,
				TargetNS: "urn:test",
			},
			base: NamespaceConstraint{
				Kind: NamespaceAny,
			},
			want: true,
		},
		{
			name: "local not subset not-absent",
			derived: NamespaceConstraint{
				Kind: NamespaceLocal,
			},
			base: NamespaceConstraint{
				Kind: NamespaceNotAbsent,
			},
			want: false,
		},
		{
			name: "list placeholder subset target",
			derived: NamespaceConstraint{
				Kind:     NamespaceList,
				List:     []string{NamespaceTargetPlaceholder},
				TargetNS: "urn:test",
			},
			base: NamespaceConstraint{
				Kind:     NamespaceTargetNamespace,
				TargetNS: "urn:test",
			},
			want: true,
		},
		{
			name: "other subset same other",
			derived: NamespaceConstraint{
				Kind:     NamespaceOther,
				TargetNS: "urn:test",
			},
			base: NamespaceConstraint{
				Kind:     NamespaceOther,
				TargetNS: "urn:test",
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NamespaceConstraintSubset(tc.derived, tc.base)
			if got != tc.want {
				t.Fatalf("NamespaceConstraintSubset() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProcessContentsStrongerOrEqual(t *testing.T) {
	if !ProcessContentsStrongerOrEqual(ProcessStrict, ProcessLax) {
		t.Fatalf("strict should be stronger than lax")
	}
	if ProcessContentsStrongerOrEqual(ProcessLax, ProcessStrict) {
		t.Fatalf("lax should not be stronger than strict")
	}
	if !ProcessContentsStrongerOrEqual(ProcessSkip, ProcessSkip) {
		t.Fatalf("skip should be stronger-or-equal to skip")
	}
}

func TestResolveSymbolByProcessContents(t *testing.T) {
	resolved, strict, err := ResolveSymbolByProcessContents(ProcessSkip, true, func() bool { return true })
	if err != nil || resolved || strict {
		t.Fatalf("skip = (%v,%v,%v), want (false,false,nil)", resolved, strict, err)
	}

	calls := 0
	resolved, strict, err = ResolveSymbolByProcessContents(ProcessStrict, true, func() bool {
		calls++
		return false
	})
	if err != nil || resolved || !strict {
		t.Fatalf("strict unresolved = (%v,%v,%v), want (false,true,nil)", resolved, strict, err)
	}
	if calls != 1 {
		t.Fatalf("strict resolver calls = %d, want 1", calls)
	}

	resolved, strict, err = ResolveSymbolByProcessContents(ProcessLax, true, func() bool { return true })
	if err != nil || !resolved || strict {
		t.Fatalf("lax resolved = (%v,%v,%v), want (true,false,nil)", resolved, strict, err)
	}
}

func TestAllowsRuntimeNamespace(t *testing.T) {
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
