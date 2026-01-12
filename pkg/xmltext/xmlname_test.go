package xmltext

import "testing"

func TestNameRuneClassification(t *testing.T) {
	if !isNameStartRune('\u00e9') {
		t.Fatalf("isNameStartRune(\\u00e9) = false, want true")
	}
	if !isNameRune('\u03c0') {
		t.Fatalf("isNameRune(\\u03c0) = false, want true")
	}
	if isNameStartRune('0') {
		t.Fatalf("isNameStartRune('0') = true, want false")
	}
	if !isNameRune('0') {
		t.Fatalf("isNameRune('0') = false, want true")
	}
	if isNameRune('\u2603') {
		t.Fatalf("isNameRune(\\u2603) = true, want false")
	}
}
