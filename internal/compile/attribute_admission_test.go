package compile

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
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
				Fixed: valueConstraintIdentity("x", 1),
			},
		},
		{
			name: "matching referenced fixed",
			admission: AttributeUseFixedValueAdmission{
				Fixed:               valueConstraintIdentity("5", 2, "decimal:5"),
				ReferencedDeclFixed: valueConstraintIdentity("5", 1, "decimal:5"),
			},
		},
		{
			name: "conflicting referenced fixed",
			admission: AttributeUseFixedValueAdmission{
				Fixed:               valueConstraintIdentity("true", 2, "boolean:true"),
				ReferencedDeclFixed: valueConstraintIdentity("true", 1, "string:true"),
			},
			wantErr: "attribute use fixed value conflicts with fixed attribute declaration",
		},
		{
			name: "inherits referenced fixed",
			admission: AttributeUseFixedValueAdmission{
				ReferencedDeclFixed: valueConstraintIdentity("x", 1),
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
		fn         func(bool, bool) error
		wantErr    string
		hasDefault bool
		hasFixed   bool
	}{
		{name: "element absent", fn: ValidateElementDeclValueConstraintAdmission},
		{name: "element default", fn: ValidateElementDeclValueConstraintAdmission, hasDefault: true},
		{name: "element fixed", fn: ValidateElementDeclValueConstraintAdmission, hasFixed: true},
		{
			name:       "element both",
			fn:         ValidateElementDeclValueConstraintAdmission,
			hasDefault: true,
			hasFixed:   true,
			wantErr:    "element cannot have both default and fixed",
		},
		{name: "attribute absent", fn: ValidateAttributeDeclValueConstraintAdmission},
		{name: "attribute default", fn: ValidateAttributeDeclValueConstraintAdmission, hasDefault: true},
		{name: "attribute fixed", fn: ValidateAttributeDeclValueConstraintAdmission, hasFixed: true},
		{
			name:       "attribute both",
			fn:         ValidateAttributeDeclValueConstraintAdmission,
			hasDefault: true,
			hasFixed:   true,
			wantErr:    "attribute cannot have both default and fixed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.fn(tt.hasDefault, tt.hasFixed)
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

func valueConstraintIdentity(canonical string, typ runtime.SimpleTypeID, identity ...string) runtime.ValueConstraintIdentity {
	id := ""
	if len(identity) != 0 {
		id = identity[0]
	}
	return runtime.ValueConstraintIdentity{
		Canonical: canonical,
		Value: runtime.SimpleValue{
			Canonical: canonical,
			Identity:  id,
			Type:      typ,
		},
		Present: true,
	}
}
