package schemaast

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

func TestTranslateXSDPatternToGoRejectsLeadingQuantifier(t *testing.T) {
	if _, err := TranslateXSDPatternToGo(`?a`); err == nil {
		t.Fatal("TranslateXSDPatternToGo() error = nil, want leading-quantifier rejection")
	}
}

func TestTranslateXSDPatternToGoRejectsCategoryRangeEndpoint(t *testing.T) {
	if _, err := TranslateXSDPatternToGo(`foo([a-\W]*)bar`); err == nil {
		t.Fatal("TranslateXSDPatternToGo() error = nil, want category-range rejection")
	}
}

func TestTranslateXSDPatternToGoRejectsConsecutiveQuantifiers(t *testing.T) {
	if _, err := TranslateXSDPatternToGo(`(foo)(\c?*)(bar)`); err == nil {
		t.Fatal("TranslateXSDPatternToGo() error = nil, want consecutive-quantifier rejection")
	}
}

func TestTranslateXSDPatternToGoRejectsQuantifierAfterGroupStart(t *testing.T) {
	if _, err := TranslateXSDPatternToGo(`(*)b`); err == nil {
		t.Fatal("TranslateXSDPatternToGo() error = nil, want group-start quantifier rejection")
	}
}

func TestTranslateXSDPatternToGoAllowsRepeatAfterCharacterClass(t *testing.T) {
	if _, err := TranslateXSDPatternToGo(`\-?[0-3]{3}`); err != nil {
		t.Fatalf("TranslateXSDPatternToGo() error = %v", err)
	}
}

func TestTranslateXSDPatternToGoAllowsQuantifierAfterEscapedAlternation(t *testing.T) {
	if _, err := TranslateXSDPatternToGo(`(foo)(\c\|*)(bar)`); err != nil {
		t.Fatalf("TranslateXSDPatternToGo() error = %v", err)
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
