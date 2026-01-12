package xmltext

import "testing"

func TestIsValidXMLCharRanges(t *testing.T) {
	tests := []struct {
		r     rune
		valid bool
	}{
		{0x9, true},
		{0xA, true},
		{0xD, true},
		{0x20, true},
		{0xD7FF, true},
		{0xE000, true},
		{0xFFFD, true},
		{0x10000, true},
		{0x10FFFF, true},
		{0x0, false},
		{0xD800, false},
		{0x110000, false},
	}
	for _, tt := range tests {
		if got := isValidXMLChar(tt.r); got != tt.valid {
			t.Fatalf("isValidXMLChar(%#x) = %v, want %v", tt.r, got, tt.valid)
		}
	}
}

func TestValidateXMLTextExtras(t *testing.T) {
	if err := validateXMLText([]byte("x&#x20;y"), nil); err != nil {
		t.Fatalf("validateXMLText numeric error = %v", err)
	}
	if err := validateXMLText([]byte("\u20ac"), nil); err != nil {
		t.Fatalf("validateXMLText unicode error = %v", err)
	}
}
