package num

import (
	"math"
	"testing"
)

func TestParseFloat32(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		class   FloatClass
		wantErr bool
		errKind ParseErrKind
	}{
		{name: "inf", input: "INF", class: FloatPosInf},
		{name: "neg inf", input: "-INF", class: FloatNegInf},
		{name: "nan", input: "NaN", class: FloatNaN},
		{name: "finite", input: "1.25", class: FloatFinite},
		{name: "plus inf invalid", input: "+INF", wantErr: true, errKind: ParseBadChar},
		{name: "bad char", input: "1e", wantErr: true, errKind: ParseBadChar},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, class, err := ParseFloat32([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if err.Kind != tc.errKind {
					t.Fatalf("error kind = %v, want %v", err.Kind, tc.errKind)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if class != tc.class {
				t.Fatalf("class = %v, want %v", class, tc.class)
			}
			switch class {
			case FloatNaN:
				if !math.IsNaN(float64(val)) {
					t.Fatalf("expected NaN")
				}
			case FloatPosInf:
				if !math.IsInf(float64(val), 1) {
					t.Fatalf("expected +Inf")
				}
			case FloatNegInf:
				if !math.IsInf(float64(val), -1) {
					t.Fatalf("expected -Inf")
				}
			}
		})
	}
}

func TestParseFloat64(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		class   FloatClass
		wantErr bool
		errKind ParseErrKind
	}{
		{name: "inf", input: "INF", class: FloatPosInf},
		{name: "neg inf", input: "-INF", class: FloatNegInf},
		{name: "nan", input: "NaN", class: FloatNaN},
		{name: "finite", input: "1.25", class: FloatFinite},
		{name: "plus inf invalid", input: "+INF", wantErr: true, errKind: ParseBadChar},
		{name: "bad char", input: "1e", wantErr: true, errKind: ParseBadChar},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			val, class, err := ParseFloat64([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				if err.Kind != tc.errKind {
					t.Fatalf("error kind = %v, want %v", err.Kind, tc.errKind)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if class != tc.class {
				t.Fatalf("class = %v, want %v", class, tc.class)
			}
			switch class {
			case FloatNaN:
				if !math.IsNaN(val) {
					t.Fatalf("expected NaN")
				}
			case FloatPosInf:
				if !math.IsInf(val, 1) {
					t.Fatalf("expected +Inf")
				}
			case FloatNegInf:
				if !math.IsInf(val, -1) {
					t.Fatalf("expected -Inf")
				}
			}
		})
	}
}

func TestValidateFloatLexical(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "inf", input: "INF"},
		{name: "neg inf", input: "-INF"},
		{name: "nan", input: "NaN"},
		{name: "finite", input: "3.14"},
		{name: "plus inf invalid", input: "+INF", wantErr: true},
		{name: "bad lexical", input: "1e", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateFloatLexical([]byte(tc.input))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCompareFloat(t *testing.T) {
	if _, ok := CompareFloat32(1, FloatFinite, float32(math.NaN()), FloatNaN); ok {
		t.Fatalf("expected FloatNaN comparison to be unordered")
	}
	if _, ok := CompareFloat64(math.NaN(), FloatNaN, 1, FloatFinite); ok {
		t.Fatalf("expected FloatNaN comparison to be unordered")
	}
}
