package schemafacet

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/builtins"
)

func TestValidateRangeValuesFacetNamesInErrors(t *testing.T) {
	t.Parallel()

	intType := builtins.Get(builtins.TypeNameInt)
	if intType == nil {
		t.Fatal("missing builtin int type")
	}

	tests := []struct {
		name         string
		minExclusive *string
		maxExclusive *string
		minInclusive *string
		maxInclusive *string
		wantSubstr   string
	}{
		{
			name:         "minExclusive",
			minExclusive: ptr("not-an-int"),
			wantSubstr:   `minExclusive value "not-an-int" is not valid for base type`,
		},
		{
			name:         "maxExclusive",
			maxExclusive: ptr("not-an-int"),
			wantSubstr:   `maxExclusive value "not-an-int" is not valid for base type`,
		},
		{
			name:         "minInclusive",
			minInclusive: ptr("not-an-int"),
			wantSubstr:   `minInclusive value "not-an-int" is not valid for base type`,
		},
		{
			name:         "maxInclusive",
			maxInclusive: ptr("not-an-int"),
			wantSubstr:   `maxInclusive value "not-an-int" is not valid for base type`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateRangeValues(tc.minExclusive, tc.maxExclusive, tc.minInclusive, tc.maxInclusive, nil, intType)
			if err == nil {
				t.Fatal("ValidateRangeValues() error = nil, want non-nil")
			}
			if !strings.Contains(err.Error(), tc.wantSubstr) {
				t.Fatalf("ValidateRangeValues() error = %q, want substring %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

func ptr(value string) *string {
	return &value
}
