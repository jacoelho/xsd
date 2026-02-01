package num

import (
	"testing"
)

func TestDecToScaledInt(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		scale  uint32
		expect string
		sign   int8
	}{
		{name: "scale up", input: "12.34", scale: 4, expect: "123400", sign: 1},
		{name: "scale up small", input: "0.1", scale: 3, expect: "100", sign: 1},
		{name: "scale down truncate", input: "12.34", scale: 1, expect: "123", sign: 1},
		{name: "scale down to zero", input: "0.004", scale: 2, expect: "0", sign: 0},
		{name: "zero", input: "0", scale: 5, expect: "0", sign: 0},
		{name: "negative", input: "-0.1", scale: 3, expect: "100", sign: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := mustParseDec(t, tt.input)
			got := DecToScaledInt(dec, tt.scale)
			if got.Sign != tt.sign {
				t.Fatalf("sign = %d, want %d", got.Sign, tt.sign)
			}
			if string(got.Digits) != tt.expect {
				t.Fatalf("digits = %q, want %q", string(got.Digits), tt.expect)
			}
		})
	}
}

func TestDecToScaledIntExact(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		scale   uint32
		expect  string
		sign    int8
		wantErr bool
	}{
		{name: "exact keep", input: "12.30", scale: 1, expect: "123", sign: 1},
		{name: "exact zero", input: "0.000", scale: 2, expect: "0", sign: 0},
		{name: "error fractional loss", input: "12.34", scale: 1, wantErr: true},
		{name: "scale up", input: "-2.5", scale: 4, expect: "25000", sign: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dec := mustParseDec(t, tt.input)
			got, err := DecToScaledIntExact(dec, tt.scale)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Sign != tt.sign {
				t.Fatalf("sign = %d, want %d", got.Sign, tt.sign)
			}
			if string(got.Digits) != tt.expect {
				t.Fatalf("digits = %q, want %q", string(got.Digits), tt.expect)
			}
		})
	}
}

func TestDecFromScaledInt(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		sign   int8
		scale  uint32
		expect string
	}{
		{name: "scale two", input: "1234", sign: 1, scale: 2, expect: "12.34"},
		{name: "scale leading zeros", input: "5", sign: 1, scale: 3, expect: "0.005"},
		{name: "negative", input: "50", sign: -1, scale: 1, expect: "-5.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intVal := Int{Sign: tt.sign, Digits: []byte(tt.input)}
			got := DecFromScaledInt(intVal, tt.scale)
			want := mustParseDec(t, tt.expect)
			if got.Compare(want) != 0 {
				t.Fatalf("dec = %s, want %s", got.RenderCanonical(nil), want.RenderCanonical(nil))
			}
		})
	}
}

func TestDecScaledRoundTrip(t *testing.T) {
	tests := []struct {
		input string
		scale uint32
	}{
		{input: "12.34", scale: 4},
		{input: "-0.125", scale: 5},
		{input: "0", scale: 3},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			dec := mustParseDec(t, tt.input)
			intVal, err := DecToScaledIntExact(dec, tt.scale)
			if err != nil {
				t.Fatalf("DecToScaledIntExact error: %v", err)
			}
			round := DecFromScaledInt(intVal, tt.scale)
			if round.Compare(dec) != 0 {
				t.Fatalf("roundtrip = %s, want %s", round.RenderCanonical(nil), dec.RenderCanonical(nil))
			}
		})
	}
}

func mustParseDec(t *testing.T, input string) Dec {
	t.Helper()
	dec, err := ParseDec([]byte(input))
	if err != nil {
		t.Fatalf("ParseDec(%q) error: %v", input, err)
	}
	return dec
}
