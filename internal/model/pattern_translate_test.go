package model

import "testing"

func TestTranslateXSDPatternToGoAnchorsOutput(t *testing.T) {
	got, err := TranslateXSDPatternToGo(`abc`)
	if err != nil {
		t.Fatalf("TranslateXSDPatternToGo() error = %v", err)
	}
	if got != `^(?:abc)$` {
		t.Fatalf("TranslateXSDPatternToGo() = %q, want %q", got, `^(?:abc)$`)
	}
}

func TestTranslateXSDPatternToGoRejectsLazyQuantifier(t *testing.T) {
	if _, err := TranslateXSDPatternToGo(`a+?`); err == nil {
		t.Fatal("TranslateXSDPatternToGo() error = nil, want lazy-quantifier rejection")
	}
}

func TestTranslateXSDPatternToGoSupportsNameEscape(t *testing.T) {
	got, err := TranslateXSDPatternToGo(`\i\c*`)
	if err != nil {
		t.Fatalf("TranslateXSDPatternToGo() error = %v", err)
	}
	if got == "" {
		t.Fatal("TranslateXSDPatternToGo() returned empty pattern")
	}
}
