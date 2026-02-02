package parser

import "testing"

func TestImportContextKeyRoundTrip(t *testing.T) {
	cases := []struct {
		fsKey    string
		location string
	}{
		{fsKey: "", location: "dir||file.xsd"},
		{fsKey: "fs||key", location: "schema.xsd"},
		{fsKey: "fs", location: "path||file.xsd"},
	}

	for _, tc := range cases {
		key := ImportContextKey(tc.fsKey, tc.location)
		got := ImportContextLocation(key)
		if got != tc.location {
			t.Fatalf("ImportContextLocation(%q) = %q, want %q", key, got, tc.location)
		}
	}
}

func TestImportContextKeyAvoidsCollisions(t *testing.T) {
	keyA := ImportContextKey("a||b", "c||d")
	keyB := ImportContextKey("a", "b||c||d")
	if keyA == keyB {
		t.Fatalf("ImportContextKey produced collision: %q", keyA)
	}
}
