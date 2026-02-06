package source

import "testing"

func TestResolveSystemIDRejectsBaseWithBackslashes(t *testing.T) {
	_, err := resolveSystemID(`dir\schema.xsd`, "child.xsd")
	if err == nil {
		t.Fatalf("expected error for base system ID with backslashes")
	}
}
