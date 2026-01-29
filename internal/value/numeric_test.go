package value

import (
	"math"
	"math/big"
	"testing"
)

func TestParseDecimal(t *testing.T) {
	rat, err := ParseDecimal([]byte(" 123.456 "))
	if err != nil {
		t.Fatalf("ParseDecimal() error = %v", err)
	}
	want, _ := new(big.Rat).SetString("123.456")
	if rat.Cmp(want) != 0 {
		t.Fatalf("ParseDecimal() = %v, want %v", rat, want)
	}
}

func TestParseInteger(t *testing.T) {
	val, err := ParseInteger([]byte(" -42 "))
	if err != nil {
		t.Fatalf("ParseInteger() error = %v", err)
	}
	if val.String() != "-42" {
		t.Fatalf("ParseInteger() = %s, want -42", val.String())
	}
}

func TestParseBoolean(t *testing.T) {
	if got, err := ParseBoolean([]byte(" true ")); err != nil || got != true {
		t.Fatalf("ParseBoolean(true) = %v, %v", got, err)
	}
	if got, err := ParseBoolean([]byte("0")); err != nil || got != false {
		t.Fatalf("ParseBoolean(0) = %v, %v", got, err)
	}
	if _, err := ParseBoolean([]byte("yes")); err == nil {
		t.Fatalf("expected error for invalid boolean")
	}
}

func TestParseFloat(t *testing.T) {
	if got, err := ParseFloat([]byte("INF")); err != nil || !math.IsInf(float64(got), 1) {
		t.Fatalf("ParseFloat(INF) = %v, %v", got, err)
	}
	if _, err := ParseFloat([]byte("+INF")); err == nil {
		t.Fatalf("expected error for +INF")
	}
}

func TestParseDouble(t *testing.T) {
	if got, err := ParseDouble([]byte("NaN")); err != nil || !math.IsNaN(got) {
		t.Fatalf("ParseDouble(NaN) = %v, %v", got, err)
	}
}

func TestIsValidDecimalLexicalEmptyInput(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{name: "nil slice", input: nil},
		{name: "empty slice", input: []byte{}},
		{name: "sign only plus", input: []byte{'+'}},
		{name: "sign only minus", input: []byte{'-'}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("isValidDecimalLexical panicked: %v", r)
				}
			}()
			if isValidDecimalLexical(tt.input) {
				t.Fatalf("isValidDecimalLexical(%v) = true, want false", tt.input)
			}
		})
	}
}
