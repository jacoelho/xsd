package types

import "testing"

func TestParseIntegerRejectsUnicodeWhitespace(t *testing.T) {
	if _, err := ParseInteger("\u00A0123"); err == nil {
		t.Fatalf("expected ParseInteger to reject NBSP")
	}
}

func TestParseBooleanRejectsUnicodeWhitespace(t *testing.T) {
	if _, err := ParseBoolean("true\u00A0"); err == nil {
		t.Fatalf("expected ParseBoolean to reject NBSP")
	}
}

func TestParseDecimalRejectsUnicodeWhitespace(t *testing.T) {
	if _, err := ParseDecimal("1\u00A02"); err == nil {
		t.Fatalf("expected ParseDecimal to reject NBSP")
	}
}

func TestParseHexBinaryRejectsUnicodeWhitespace(t *testing.T) {
	if _, err := ParseHexBinary("AA\u00A0"); err == nil {
		t.Fatalf("expected ParseHexBinary to reject NBSP")
	}
}
