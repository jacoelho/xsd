package stream

import "testing"

func TestUTF8BOMHelpers(t *testing.T) {
	input := []byte{0xEF, 0xBB, 0xBF, '<'}
	if !HasUTF8BOM(input) {
		t.Fatal("HasUTF8BOM() = false for UTF-8 BOM")
	}
	trimmed := TrimUTF8BOM(input)
	if string(trimmed) != "<" {
		t.Fatalf("TrimUTF8BOM() = %q, want '<'", trimmed)
	}
	if got := TrimUTF8BOM([]byte("<")); string(got) != "<" {
		t.Fatalf("TrimUTF8BOM(no BOM) = %q, want '<'", got)
	}
}

func TestDeclaredXMLVersion(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{`<root/>`, ""},
		{`<?xml version="1.0" encoding="UTF-8"?><root/>`, "1.0"},
		{`<?xml version='1.0'?><root/>`, "1.0"},
		{`<?xml encoding="UTF-8" version="1.1"?><root/>`, "1.1"},
		{`<?xml-stylesheet href="x"?><root/>`, ""},
		{`<?xml-stylesheet version="1.0"?><root/>`, ""},
	}
	for _, tt := range tests {
		if got := DeclaredXMLVersion([]byte(tt.in)); got != tt.want {
			t.Fatalf("DeclaredXMLVersion(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestDeclaredEncoding(t *testing.T) {
	if got := DeclaredEncoding([]byte(`<?xml version="1.0" encoding="UTF-8"?><root/>`)); got != "UTF-8" {
		t.Fatalf("DeclaredEncoding() = %q, want UTF-8", got)
	}
}

func TestValidateXMLDeclContentUsesXMLWhitespace(t *testing.T) {
	if err := ValidateXMLDeclContent([]byte("version=\"1.0\"\t")); err != nil {
		t.Fatalf("ValidateXMLDeclContent() XML whitespace error = %v", err)
	}
	if err := ValidateXMLDeclContent([]byte("version=\"1.0\"\u00a0")); err == nil {
		t.Fatal("ValidateXMLDeclContent() error = nil for NBSP")
	}
}
