package schema

import (
	"strings"
	"testing"
)

func TestParseAttributeUseMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mode    string
		want    attributeUseMode
		wantErr string
	}{
		{name: "optional", mode: "optional", want: attributeUseOptional},
		{name: "required", mode: "required", want: attributeUseRequired},
		{name: "prohibited", mode: "prohibited", want: attributeUseProhibited},
		{name: "invalid", mode: "bad", wantErr: "invalid attribute use bad"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseAttributeUseMode(tt.mode)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ParseAttributeUseMode() error = %v", err)
				}
				if got != tt.want {
					t.Fatalf("ParseAttributeUseMode() = %d, want %d", got, tt.want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ParseAttributeUseMode() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAttributeUseValueConstraintAdmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		admission attributeUseValueConstraintAdmission
		wantErr   string
	}{
		{name: "optional unconstrained", admission: attributeUseValueConstraintAdmission{Mode: attributeUseOptional}},
		{
			name: "required default",
			admission: attributeUseValueConstraintAdmission{
				Mode:       attributeUseRequired,
				HasDefault: true,
			},
			wantErr: "required attribute cannot have default",
		},
		{
			name: "prohibited default",
			admission: attributeUseValueConstraintAdmission{
				Mode:       attributeUseProhibited,
				HasDefault: true,
			},
			wantErr: "prohibited attribute cannot have default",
		},
		{
			name: "default conflicts with declaration fixed",
			admission: attributeUseValueConstraintAdmission{
				Mode:                   attributeUseOptional,
				HasDefault:             true,
				ReferencedDeclHasFixed: true,
			},
			wantErr: "attribute use default conflicts with fixed attribute declaration",
		},
		{
			name: "default and fixed",
			admission: attributeUseValueConstraintAdmission{
				Mode:       attributeUseOptional,
				HasDefault: true,
				HasFixed:   true,
			},
			wantErr: "attribute cannot have both default and fixed",
		},
		{
			name:      "invalid mode",
			admission: attributeUseValueConstraintAdmission{Mode: attributeUseMode(99)},
			wantErr:   "attribute use mode is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAttributeUseValueConstraintAdmission(tt.admission)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateAttributeUseValueConstraintAdmission() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAttributeUseValueConstraintAdmission() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateAttributeUseFixedValueAdmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		admission attributeUseFixedValueAdmission
		wantErr   string
	}{
		{
			name: "no referenced fixed",
			admission: attributeUseFixedValueAdmission{
				Fixed: valueConstraintIdentity(t, "string", "x"),
			},
		},
		{
			name: "matching referenced fixed",
			admission: attributeUseFixedValueAdmission{
				Fixed:               valueConstraintIdentity(t, "integer", "5"),
				ReferencedDeclFixed: valueConstraintIdentity(t, "decimal", "5"),
			},
		},
		{
			name: "conflicting referenced fixed",
			admission: attributeUseFixedValueAdmission{
				Fixed:               valueConstraintIdentity(t, "boolean", "true"),
				ReferencedDeclFixed: valueConstraintIdentity(t, "string", "true"),
			},
			wantErr: "attribute use fixed value conflicts with fixed attribute declaration",
		},
		{
			name: "inherits referenced fixed",
			admission: attributeUseFixedValueAdmission{
				ReferencedDeclFixed: valueConstraintIdentity(t, "string", "x"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateAttributeUseFixedValueAdmission(tt.admission)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateAttributeUseFixedValueAdmission() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("ValidateAttributeUseFixedValueAdmission() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestValidateDeclValueConstraintAdmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		fn         func(declarationValueConstraint) error
		wantErr    string
		constraint declarationValueConstraint
	}{
		{name: "element absent", fn: validateElementDeclValueConstraintAdmission},
		{name: "element default", fn: validateElementDeclValueConstraintAdmission, constraint: declarationValueConstraintDefault},
		{name: "element fixed", fn: validateElementDeclValueConstraintAdmission, constraint: declarationValueConstraintFixed},
		{
			name:       "element both",
			fn:         validateElementDeclValueConstraintAdmission,
			constraint: declarationValueConstraintConflict,
			wantErr:    "element cannot have both default and fixed",
		},
		{
			name:       "element unknown",
			fn:         validateElementDeclValueConstraintAdmission,
			constraint: declarationValueConstraint(99),
			wantErr:    "element declaration has unknown value constraint",
		},
		{name: "attribute absent", fn: validateAttributeDeclValueConstraintAdmission},
		{name: "attribute default", fn: validateAttributeDeclValueConstraintAdmission, constraint: declarationValueConstraintDefault},
		{name: "attribute fixed", fn: validateAttributeDeclValueConstraintAdmission, constraint: declarationValueConstraintFixed},
		{
			name:       "attribute both",
			fn:         validateAttributeDeclValueConstraintAdmission,
			constraint: declarationValueConstraintConflict,
			wantErr:    "attribute cannot have both default and fixed",
		},
		{
			name:       "attribute unknown",
			fn:         validateAttributeDeclValueConstraintAdmission,
			constraint: declarationValueConstraint(99),
			wantErr:    "attribute declaration has unknown value constraint",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.fn(tt.constraint)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("admission validator error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("admission validator error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func valueConstraintIdentity(tb testing.TB, typeName, lexical string) constraintIdentity {
	tb.Helper()
	validated := builtinValueForTest(tb, typeName, lexical)
	return constraintIdentity{Lexical: lexical, Canonical: validated.CanonicalText(), Value: validated, Present: true}
}
