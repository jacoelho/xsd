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
		want    AttributeUseMode
		wantErr string
	}{
		{name: "optional", mode: "optional", want: AttributeUseOptional},
		{name: "required", mode: "required", want: AttributeUseRequired},
		{name: "prohibited", mode: "prohibited", want: AttributeUseProhibited},
		{name: "invalid", mode: "bad", wantErr: "invalid attribute use bad"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseAttributeUseMode(tt.mode)
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

func TestApplyAttributeUseMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   AttributeUseModeApplication
		want AttributeUseModeState
		err  string
	}{
		{
			name: "optional",
			in:   AttributeUseModeApplication{Mode: AttributeUseOptional},
		},
		{
			name: "required",
			in:   AttributeUseModeApplication{Mode: AttributeUseRequired},
			want: AttributeUseModeState{Required: true},
		},
		{
			name: "prohibited without explicit fixed",
			in:   AttributeUseModeApplication{Mode: AttributeUseProhibited},
			want: AttributeUseModeState{Prohibited: true},
		},
		{
			name: "prohibited with explicit fixed",
			in:   AttributeUseModeApplication{Mode: AttributeUseProhibited, HasFixed: true},
		},
		{
			name: "invalid mode",
			in:   AttributeUseModeApplication{Mode: AttributeUseMode(99)},
			err:  "attribute use mode is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ApplyAttributeUseMode(tt.in)
			if tt.err == "" {
				if err != nil {
					t.Fatalf("ApplyAttributeUseMode() error = %v", err)
				}
				if got != tt.want {
					t.Fatalf("ApplyAttributeUseMode() = %+v, want %+v", got, tt.want)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.err) {
				t.Fatalf("ApplyAttributeUseMode() error = %v, want %q", err, tt.err)
			}
		})
	}
}

func TestValidateAttributeUseValueConstraintAdmission(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		admission AttributeUseValueConstraintAdmission
		wantErr   string
	}{
		{name: "optional unconstrained", admission: AttributeUseValueConstraintAdmission{Mode: AttributeUseOptional}},
		{
			name: "required default",
			admission: AttributeUseValueConstraintAdmission{
				Mode:       AttributeUseRequired,
				HasDefault: true,
			},
			wantErr: "required attribute cannot have default",
		},
		{
			name: "prohibited default",
			admission: AttributeUseValueConstraintAdmission{
				Mode:       AttributeUseProhibited,
				HasDefault: true,
			},
			wantErr: "prohibited attribute cannot have default",
		},
		{
			name: "default conflicts with declaration fixed",
			admission: AttributeUseValueConstraintAdmission{
				Mode:                   AttributeUseOptional,
				HasDefault:             true,
				ReferencedDeclHasFixed: true,
			},
			wantErr: "attribute use default conflicts with fixed attribute declaration",
		},
		{
			name: "default and fixed",
			admission: AttributeUseValueConstraintAdmission{
				Mode:       AttributeUseOptional,
				HasDefault: true,
				HasFixed:   true,
			},
			wantErr: "attribute cannot have both default and fixed",
		},
		{
			name:      "invalid mode",
			admission: AttributeUseValueConstraintAdmission{Mode: AttributeUseMode(99)},
			wantErr:   "attribute use mode is invalid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateAttributeUseValueConstraintAdmission(tt.admission)
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
		admission AttributeUseFixedValueAdmission
		wantErr   string
	}{
		{
			name: "no referenced fixed",
			admission: AttributeUseFixedValueAdmission{
				Fixed: valueConstraintIdentity(t, "string", "x"),
			},
		},
		{
			name: "matching referenced fixed",
			admission: AttributeUseFixedValueAdmission{
				Fixed:               valueConstraintIdentity(t, "integer", "5"),
				ReferencedDeclFixed: valueConstraintIdentity(t, "decimal", "5"),
			},
		},
		{
			name: "conflicting referenced fixed",
			admission: AttributeUseFixedValueAdmission{
				Fixed:               valueConstraintIdentity(t, "boolean", "true"),
				ReferencedDeclFixed: valueConstraintIdentity(t, "string", "true"),
			},
			wantErr: "attribute use fixed value conflicts with fixed attribute declaration",
		},
		{
			name: "inherits referenced fixed",
			admission: AttributeUseFixedValueAdmission{
				ReferencedDeclFixed: valueConstraintIdentity(t, "string", "x"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateAttributeUseFixedValueAdmission(tt.admission)
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
		fn         func(DeclarationValueConstraint) error
		wantErr    string
		constraint DeclarationValueConstraint
	}{
		{name: "element absent", fn: ValidateElementDeclValueConstraintAdmission},
		{name: "element default", fn: ValidateElementDeclValueConstraintAdmission, constraint: DeclarationValueConstraintDefault},
		{name: "element fixed", fn: ValidateElementDeclValueConstraintAdmission, constraint: DeclarationValueConstraintFixed},
		{
			name:       "element both",
			fn:         ValidateElementDeclValueConstraintAdmission,
			constraint: DeclarationValueConstraintConflict,
			wantErr:    "element cannot have both default and fixed",
		},
		{
			name:       "element unknown",
			fn:         ValidateElementDeclValueConstraintAdmission,
			constraint: DeclarationValueConstraint(99),
			wantErr:    "element declaration has unknown value constraint",
		},
		{name: "attribute absent", fn: ValidateAttributeDeclValueConstraintAdmission},
		{name: "attribute default", fn: ValidateAttributeDeclValueConstraintAdmission, constraint: DeclarationValueConstraintDefault},
		{name: "attribute fixed", fn: ValidateAttributeDeclValueConstraintAdmission, constraint: DeclarationValueConstraintFixed},
		{
			name:       "attribute both",
			fn:         ValidateAttributeDeclValueConstraintAdmission,
			constraint: DeclarationValueConstraintConflict,
			wantErr:    "attribute cannot have both default and fixed",
		},
		{
			name:       "attribute unknown",
			fn:         ValidateAttributeDeclValueConstraintAdmission,
			constraint: DeclarationValueConstraint(99),
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

func valueConstraintIdentity(tb testing.TB, typeName, lexical string) ValueConstraintIdentity {
	tb.Helper()
	validated := builtinValueForTest(tb, typeName, lexical)
	return ValueConstraintIdentity{Lexical: lexical, Canonical: validated.CanonicalText(), Value: validated, Present: true}
}
