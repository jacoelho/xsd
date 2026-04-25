package xmltext

import (
	"errors"
	"testing"
)

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

func TestValidateXMLChars(t *testing.T) {
	if err := validateXMLChars([]byte("ok")); err != nil {
		t.Fatalf("validateXMLChars valid error = %v", err)
	}
	if err := validateXMLChars([]byte{0x00}); !errors.Is(err, errInvalidChar) {
		t.Fatalf("validateXMLChars control error = %v, want %v", err, errInvalidChar)
	}
	if err := validateXMLChars([]byte{0xff}); !errors.Is(err, errInvalidChar) {
		t.Fatalf("validateXMLChars utf8 error = %v, want %v", err, errInvalidChar)
	}
	if !isValidXMLChar('\n') {
		t.Fatalf("isValidXMLChar(\\n) = false, want true")
	}
	if isValidXMLChar(0x01) {
		t.Fatalf("isValidXMLChar(0x01) = true, want false")
	}
}

func TestValidateXMLText(t *testing.T) {
	if err := validateXMLText([]byte("a&amp;b"), nil); err != nil {
		t.Fatalf("validateXMLText valid error = %v", err)
	}
	if err := validateXMLText([]byte("a&bogus;"), nil); !errors.Is(err, errInvalidEntity) {
		t.Fatalf("validateXMLText entity error = %v, want %v", err, errInvalidEntity)
	}
	if err := validateXMLText([]byte("a\x00"), nil); !errors.Is(err, errInvalidChar) {
		t.Fatalf("validateXMLText char error = %v, want %v", err, errInvalidChar)
	}
	if err := validateXMLText([]byte("a&amp;b"), &entityResolver{}); err != nil {
		t.Fatalf("validateXMLText resolver error = %v", err)
	}
}
