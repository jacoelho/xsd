package num

import "testing"

func TestParseInt(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		sign    int8
		digits  string
		errKind ParseErrKind
		wantErr bool
	}{
		{name: "zero", input: "0", sign: 0, digits: "0"},
		{name: "neg zero", input: "-0", sign: 0, digits: "0"},
		{name: "pos sign zero", input: "+000", sign: 0, digits: "0"},
		{name: "positive", input: "123", sign: 1, digits: "123"},
		{name: "negative", input: "-456", sign: -1, digits: "456"},
		{name: "leading zeros", input: "0007", sign: 1, digits: "7"},
		{name: "empty", input: "", wantErr: true, errKind: ParseEmpty},
		{name: "sign only", input: "+", wantErr: true, errKind: ParseNoDigits},
		{name: "bad char", input: "12a", wantErr: true, errKind: ParseBadChar},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseInt([]byte(tc.input))
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
			if string(got.Digits) != tc.digits {
				t.Fatalf("digits = %q, want %q", string(got.Digits), tc.digits)
			}
		})
	}
}

func TestIntCompare(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want int
	}{
		{name: "zero", a: "0", b: "0", want: 0},
		{name: "pos vs neg", a: "1", b: "-1", want: 1},
		{name: "neg vs zero", a: "-1", b: "0", want: -1},
		{name: "magnitude", a: "10", b: "2", want: 1},
		{name: "neg magnitude", a: "-2", b: "-3", want: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a, err := ParseInt([]byte(tc.a))
			if err != nil {
				t.Fatalf("parse a: %v", err)
			}
			b, err := ParseInt([]byte(tc.b))
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

func TestIntAddMulRender(t *testing.T) {
	addTests := []struct {
		name string
		a    string
		b    string
		want string
	}{
		{name: "simple", a: "1", b: "2", want: "3"},
		{name: "cancel", a: "-1", b: "1", want: "0"},
		{name: "negative", a: "-5", b: "-7", want: "-12"},
		{name: "borrow", a: "100", b: "-1", want: "99"},
	}

	for _, tc := range addTests {
		t.Run("add_"+tc.name, func(t *testing.T) {
			a, err := ParseInt([]byte(tc.a))
			if err != nil {
				t.Fatalf("parse a: %v", err)
			}
			b, err := ParseInt([]byte(tc.b))
			if err != nil {
				t.Fatalf("parse b: %v", err)
			}
			got := Add(a, b).RenderCanonical(nil)
			if string(got) != tc.want {
				t.Fatalf("add = %q, want %q", string(got), tc.want)
			}
		})
	}

	mulTests := []struct {
		name string
		a    string
		b    string
		want string
	}{
		{name: "zero", a: "0", b: "5", want: "0"},
		{name: "negative", a: "-2", b: "3", want: "-6"},
		{name: "square", a: "12", b: "12", want: "144"},
	}

	for _, tc := range mulTests {
		t.Run("mul_"+tc.name, func(t *testing.T) {
			a, err := ParseInt([]byte(tc.a))
			if err != nil {
				t.Fatalf("parse a: %v", err)
			}
			b, err := ParseInt([]byte(tc.b))
			if err != nil {
				t.Fatalf("parse b: %v", err)
			}
			got := Mul(a, b).RenderCanonical(nil)
			if string(got) != tc.want {
				t.Fatalf("mul = %q, want %q", string(got), tc.want)
			}
		})
	}
}
