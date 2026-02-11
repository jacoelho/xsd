package model

import "testing"

func TestProcessContentsStrongerOrEqual(t *testing.T) {
	tests := []struct {
		name    string
		derived ProcessContents
		base    ProcessContents
		want    bool
	}{
		{name: "strict over strict", derived: Strict, base: Strict, want: true},
		{name: "lax over strict", derived: Lax, base: Strict, want: false},
		{name: "skip over strict", derived: Skip, base: Strict, want: false},
		{name: "strict over lax", derived: Strict, base: Lax, want: true},
		{name: "lax over lax", derived: Lax, base: Lax, want: true},
		{name: "skip over lax", derived: Skip, base: Lax, want: false},
		{name: "strict over skip", derived: Strict, base: Skip, want: true},
		{name: "lax over skip", derived: Lax, base: Skip, want: true},
		{name: "skip over skip", derived: Skip, base: Skip, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ProcessContentsStrongerOrEqual(tt.derived, tt.base)
			if got != tt.want {
				t.Fatalf("ProcessContentsStrongerOrEqual(%v, %v) = %v, want %v", tt.derived, tt.base, got, tt.want)
			}
		})
	}
}

func TestNamespaceConstraintSubset(t *testing.T) {
	tests := []struct {
		name             string
		derived          NamespaceConstraint
		derivedList      []NamespaceURI
		derivedTargetNS  NamespaceURI
		base             NamespaceConstraint
		baseList         []NamespaceURI
		baseTargetNS     NamespaceURI
		wantSubsetResult bool
	}{
		{
			name:             "list subset any",
			derived:          NSCList,
			derivedList:      []NamespaceURI{"urn:a"},
			base:             NSCAny,
			wantSubsetResult: true,
		},
		{
			name:             "any not subset list",
			derived:          NSCAny,
			base:             NSCList,
			baseList:         []NamespaceURI{"urn:a"},
			wantSubsetResult: false,
		},
		{
			name:             "target namespace subset of list",
			derived:          NSCTargetNamespace,
			derivedTargetNS:  "urn:a",
			base:             NSCList,
			baseList:         []NamespaceURI{"urn:a"},
			wantSubsetResult: true,
		},
		{
			name:             "local not subset not-absent",
			derived:          NSCLocal,
			base:             NSCNotAbsent,
			wantSubsetResult: false,
		},
		{
			name:             "other subset same other target",
			derived:          NSCOther,
			derivedTargetNS:  "urn:a",
			base:             NSCOther,
			baseTargetNS:     "urn:a",
			wantSubsetResult: true,
		},
		{
			name:             "other not subset different other target",
			derived:          NSCOther,
			derivedTargetNS:  "urn:a",
			base:             NSCOther,
			baseTargetNS:     "urn:b",
			wantSubsetResult: false,
		},
		{
			name:             "placeholder resolution in list",
			derived:          NSCList,
			derivedList:      []NamespaceURI{NamespaceTargetPlaceholder},
			derivedTargetNS:  "urn:derived",
			base:             NSCList,
			baseList:         []NamespaceURI{"urn:derived"},
			wantSubsetResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NamespaceConstraintSubset(
				tt.derived,
				tt.derivedList,
				tt.derivedTargetNS,
				tt.base,
				tt.baseList,
				tt.baseTargetNS,
			)
			if got != tt.wantSubsetResult {
				t.Fatalf("NamespaceConstraintSubset() = %v, want %v", got, tt.wantSubsetResult)
			}
		})
	}
}
