package num

import "testing"

func TestParseDec(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		sign    int8
		coef    string
		scale   uint32
		errKind ParseErrKind
		wantErr bool
	}{
		{name: "zero", input: "0", sign: 0, coef: "0", scale: 0},
		{name: "neg zero", input: "-0.0", sign: 0, coef: "0", scale: 0},
		{name: "integer", input: "12", sign: 1, coef: "12", scale: 0},
		{name: "leading zero decimal", input: "0.1", sign: 1, coef: "1", scale: 1},
		{name: "trailing zero decimal", input: "1.0", sign: 1, coef: "1", scale: 0},
		{name: "trim trailing zeros", input: "12.3400", sign: 1, coef: "1234", scale: 2},
		{name: "leading dot", input: ".5", sign: 1, coef: "5", scale: 1},
		{name: "trailing dot", input: "5.", sign: 1, coef: "5", scale: 0},
		{name: "leading zeros", input: "-001.2300", sign: -1, coef: "123", scale: 2},
		{name: "empty", input: "", wantErr: true, errKind: ParseEmpty},
		{name: "sign only", input: "+", wantErr: true, errKind: ParseNoDigits},
		{name: "dot only", input: ".", wantErr: true, errKind: ParseNoDigits},
		{name: "double dot", input: "1..2", wantErr: true, errKind: ParseMultipleDots},
		{name: "bad char", input: "1a", wantErr: true, errKind: ParseBadChar},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseDec([]byte(tc.input))
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
			if got.Sign != tc.sign {
				t.Fatalf("sign = %d, want %d", got.Sign, tc.sign)
			}
			if string(got.Coef) != tc.coef {
				t.Fatalf("coef = %q, want %q", string(got.Coef), tc.coef)
			}
			if got.Scale != tc.scale {
				t.Fatalf("scale = %d, want %d", got.Scale, tc.scale)
			}
		})
	}
}

func TestDecCompare(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "equal scale", a: "1.2", b: "1.20", want: 0},
		{name: "greater", a: "1.2", b: "1.19", want: 1},
		{name: "neg vs pos", a: "-1.2", b: "1.2", want: -1},
		{name: "neg compare", a: "-1.2", b: "-1.3", want: 1},
		{name: "scale exceeds coef", a: "0.001", b: "0.01", want: -1},
		{name: "scale exceeds coef equal", a: "0.0001", b: "0.0001", want: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, err := ParseDec([]byte(tc.a))
			if err != nil {
				t.Fatalf("parse a: %v", err)
			}
			b, err := ParseDec([]byte(tc.b))
			if err != nil {
				t.Fatalf("parse b: %v", err)
			}
			got := a.Compare(b)
			if got != tc.want {
				t.Fatalf("compare = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestDecRenderCanonical(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "zero", in: "0", want: "0.0"},
		{name: "integer", in: "12", want: "12.0"},
		{name: "negative", in: "-12.34", want: "-12.34"},
		{name: "fraction", in: "0.5", want: "0.5"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dec, err := ParseDec([]byte(tc.in))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got := dec.RenderCanonical(nil)
			if string(got) != tc.want {
				t.Fatalf("render = %q, want %q", string(got), tc.want)
			}
		})
	}
}
