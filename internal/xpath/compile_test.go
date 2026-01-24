package xpath

import "testing"

func TestParseRestrictedXPath(t *testing.T) {
	ns := map[string]string{
		"imp": "importNS",
	}

	tests := []struct {
		name           string
		expr           string
		policy         AttributePolicy
		wantErr        bool
		verifySteps    bool
		wantStepCount  int
		wantFirstAxis  Axis
		wantSecondAxis Axis
	}{
		{
			name:          "child axis with space before ::",
			expr:          "child ::imp:iid",
			policy:        AttributesDisallowed,
			wantErr:       false,
			verifySteps:   true,
			wantStepCount: 1,
			wantFirstAxis: AxisChild,
		},
		{
			name:          "child axis with space after ::",
			expr:          "child:: imp:iid",
			policy:        AttributesDisallowed,
			wantErr:       false,
			verifySteps:   true,
			wantStepCount: 1,
			wantFirstAxis: AxisChild,
		},
		{
			name:          "attribute axis with space before ::",
			expr:          "attribute ::imp:sid",
			policy:        AttributesAllowed,
			wantErr:       false,
			verifySteps:   true,
			wantStepCount: 0,
		},
		{
			name:           "descendant prefix with dot",
			expr:           ".//.",
			policy:         AttributesDisallowed,
			wantErr:        false,
			verifySteps:    true,
			wantStepCount:  2,
			wantFirstAxis:  AxisDescendantOrSelf,
			wantSecondAxis: AxisSelf,
		},
		{
			name:    "descendant prefix without node test",
			expr:    ".//",
			policy:  AttributesDisallowed,
			wantErr: true,
		},
		{
			name:    "descendant prefix mid-path",
			expr:    ".//imp:iid1/.//imp:iid2",
			policy:  AttributesDisallowed,
			wantErr: true,
		},
		{
			name:    "explicit self axis disallowed",
			expr:    "self::*",
			policy:  AttributesDisallowed,
			wantErr: true,
		},
		{
			name:    "explicit descendant axis disallowed",
			expr:    "descendant::*",
			policy:  AttributesDisallowed,
			wantErr: true,
		},
		{
			name:    "relative path required",
			expr:    "//",
			policy:  AttributesDisallowed,
			wantErr: true,
		},
		{
			name:    "invalid axis separator",
			expr:    "child: :imp:iid",
			policy:  AttributesDisallowed,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := Parse(tt.expr, ns, tt.policy)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse(%q) expected error, got nil", tt.expr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", tt.expr, err)
			}
			if !tt.verifySteps {
				return
			}
			if len(parsed.Paths) != 1 {
				t.Fatalf("Parse(%q) expected 1 path, got %d", tt.expr, len(parsed.Paths))
			}
			path := parsed.Paths[0]
			if len(path.Steps) != tt.wantStepCount {
				t.Fatalf("Parse(%q) expected %d steps, got %d", tt.expr, tt.wantStepCount, len(path.Steps))
			}
			if tt.wantStepCount > 0 && path.Steps[0].Axis != tt.wantFirstAxis {
				t.Fatalf("Parse(%q) step[0] axis = %v, want %v", tt.expr, path.Steps[0].Axis, tt.wantFirstAxis)
			}
			if tt.wantStepCount > 1 && path.Steps[1].Axis != tt.wantSecondAxis {
				t.Fatalf("Parse(%q) step[1] axis = %v, want %v", tt.expr, path.Steps[1].Axis, tt.wantSecondAxis)
			}
		})
	}
}

func TestParseAttributeDefaultNamespaceBehavior(t *testing.T) {
	tests := []string{"@id", "@ id"}
	for _, expr := range tests {
		parsed, err := Parse(expr, nil, AttributesAllowed)
		if err != nil {
			t.Fatalf("Parse(%q) unexpected error: %v", expr, err)
		}
		if len(parsed.Paths) != 1 {
			t.Fatalf("Parse(%q) expected 1 path, got %d", expr, len(parsed.Paths))
		}
		attr := parsed.Paths[0].Attribute
		if attr == nil {
			t.Fatalf("Parse(%q) expected attribute node test", expr)
		}
		if !attr.NamespaceSpecified {
			t.Fatalf("Parse(%q) expected NamespaceSpecified to be true for no-namespace attributes", expr)
		}
		if attr.Namespace != "" {
			t.Fatalf("Parse(%q) expected empty namespace, got %q", expr, attr.Namespace)
		}
	}
}
