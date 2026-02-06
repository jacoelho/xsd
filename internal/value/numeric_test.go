package value

import (
	"math"
	"testing"
)

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

func TestParseFloatLexicalVariants(t *testing.T) {
	valid := [][]byte{
		[]byte("0"),
		[]byte("+0"),
		[]byte("-0"),
		[]byte("1."),
		[]byte(".1"),
		[]byte("1.0"),
		[]byte("1e2"),
		[]byte("1E-2"),
	}
	for _, input := range valid {
		if _, err := ParseFloat(input); err != nil {
			t.Fatalf("ParseFloat(%q) unexpected error: %v", input, err)
		}
		if _, err := ParseDouble(input); err != nil {
			t.Fatalf("ParseDouble(%q) unexpected error: %v", input, err)
		}
	}

	invalid := [][]byte{
		[]byte(""),
		[]byte("."),
		[]byte("+"),
		[]byte("1e"),
		[]byte("1e+"),
	}
	for _, input := range invalid {
		if _, err := ParseFloat(input); err == nil {
			t.Fatalf("ParseFloat(%q) expected error", input)
		}
		if _, err := ParseDouble(input); err == nil {
			t.Fatalf("ParseDouble(%q) expected error", input)
		}
	}
}

func TestParseUnsignedRejectsSigns(t *testing.T) {
	tests := []struct {
		name string
		fn   func([]byte) (uint64, error)
	}{
		{
			name: "unsignedLong",
			fn: func(value []byte) (uint64, error) {
				return ParseUnsignedLong(value)
			},
		},
		{
			name: "unsignedInt",
			fn: func(value []byte) (uint64, error) {
				v, err := ParseUnsignedInt(value)
				return uint64(v), err
			},
		},
		{
			name: "unsignedShort",
			fn: func(value []byte) (uint64, error) {
				v, err := ParseUnsignedShort(value)
				return uint64(v), err
			},
		},
		{
			name: "unsignedByte",
			fn: func(value []byte) (uint64, error) {
				v, err := ParseUnsignedByte(value)
				return uint64(v), err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invalid := [][]byte{
				[]byte("+0"),
				[]byte("-0"),
				[]byte("+1"),
				[]byte("-1"),
			}
			for _, value := range invalid {
				if _, err := tt.fn(value); err == nil {
					t.Fatalf("Parse(%q) expected error", value)
				}
			}
			valid := map[string]uint64{
				"0": 0,
				"1": 1,
			}
			for value, want := range valid {
				got, err := tt.fn([]byte(value))
				if err != nil {
					t.Fatalf("Parse(%q) error = %v", value, err)
				}
				if got != want {
					t.Fatalf("Parse(%q) = %d, want %d", value, got, want)
				}
			}
		})
	}
}

func TestParseAnyURIWhitespaceCollapse(t *testing.T) {
	input := []byte(" \thttp://ex\tample.com \r\n")
	got, err := ParseAnyURI(input)
	if err != nil {
		t.Fatalf("ParseAnyURI() error = %v", err)
	}
	if got != "http://ex ample.com" {
		t.Fatalf("ParseAnyURI() = %q, want %q", got, "http://ex ample.com")
	}
}
