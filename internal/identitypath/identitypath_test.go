package identitypath

import "testing"

func TestParseSelectorRejectsAttribute(t *testing.T) {
	if _, err := ParseSelector("@id", nil); err == nil {
		t.Fatalf("ParseSelector should reject attribute selection")
	}
}

func TestParseFieldAllowsAttribute(t *testing.T) {
	if _, err := ParseField("@id", nil); err != nil {
		t.Fatalf("ParseField rejected valid attribute selection: %v", err)
	}
}

func TestParseSelectorAllowsDescendantPrefix(t *testing.T) {
	if _, err := ParseSelector(".//item", nil); err != nil {
		t.Fatalf("ParseSelector rejected .// prefix: %v", err)
	}
}
