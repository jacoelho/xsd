package compile

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestTranslateXSDRegexToGoDotAndLiteralAnchors(t *testing.T) {
	re := regexp.MustCompile("^(?:" + TranslateXSDRegexToGo("a.b") + ")$")
	if re.MatchString("a\rb") {
		t.Fatal(`translated "a.b" matches carriage return`)
	}
	if re.MatchString("a\nb") {
		t.Fatal(`translated "a.b" matches newline`)
	}
	if !re.MatchString("axb") {
		t.Fatal(`translated "a.b" does not match "axb"`)
	}

	literalDot := regexp.MustCompile("^(?:" + TranslateXSDRegexToGo(`a\.b`) + ")$")
	if !literalDot.MatchString("a.b") || literalDot.MatchString("axb") {
		t.Fatal(`translated "a\.b" must match a literal dot only`)
	}

	classDot := regexp.MustCompile("^(?:" + TranslateXSDRegexToGo(`[.]`) + ")$")
	if !classDot.MatchString(".") || classDot.MatchString("x") {
		t.Fatal(`translated "[.]" must match a literal dot only`)
	}

	anchors := regexp.MustCompile("^(?:" + TranslateXSDRegexToGo("^abc$") + ")$")
	if !anchors.MatchString("^abc$") || anchors.MatchString("abc") {
		t.Fatal(`translated "^abc$" must treat anchors as literals`)
	}
}

func TestValidateXSDRegexSyntaxReportsSchemaAndUnsupportedErrors(t *testing.T) {
	err := ValidateXSDRegexSyntax(`a{,2}`, nil)
	var diag *xsderrors.Error
	if !errors.As(err, &diag) || diag.Code != xsderrors.CodeSchemaFacet {
		t.Fatalf("ValidateXSDRegexSyntax invalid quantifier error = %v, want %s", err, xsderrors.CodeSchemaFacet)
	}

	err = ValidateXSDRegexSyntax(`\p{IsBasicLatin}`, nil)
	if !xsderrors.IsUnsupported(err) {
		t.Fatalf("ValidateXSDRegexSyntax(IsBasicLatin) error = %v, want unsupported", err)
	}
}

func TestCheckXSDRegexSyntaxCachesGoCategorySupport(t *testing.T) {
	cache := RegexCategoryCache{}
	unsupported, err := CheckXSDRegexSyntax(`\p{Lu}+`, cache)
	if err != nil || unsupported {
		t.Fatalf("CheckXSDRegexSyntax() = (%v, %v), want supported nil", unsupported, err)
	}
	if got, ok := cache["Lu"]; !ok || !got {
		t.Fatalf("category cache[Lu] = (%v, %v), want (true, true)", got, ok)
	}
}

func TestCompilePatternFacetUsesFastMatcherBeforeGoUnsupportedRepeatLimit(t *testing.T) {
	pattern, err := CompilePatternFacet(`0{1001}`, nil)
	if err != nil {
		t.Fatalf("CompilePatternFacet() error = %v", err)
	}
	projection := pattern.FacetProjection()
	if !projection.HasFast || projection.HasRegexp {
		t.Fatalf("projection = %+v, want fast-only matcher", projection)
	}
	if !pattern.MatchString(strings.Repeat("0", 1001)) {
		t.Fatal("fast pattern does not match exact repeat count")
	}
	if pattern.MatchString(strings.Repeat("0", 1000)) {
		t.Fatal("fast pattern matches below exact repeat count")
	}
}

func TestCompilePatternFacetRejectsSubtractionAsUnsupported(t *testing.T) {
	_, err := CompilePatternFacet(`[a-z-[aeiou]]`, nil)
	if !xsderrors.IsUnsupported(err) {
		t.Fatalf("CompilePatternFacet(subtraction) error = %v, want unsupported", err)
	}
}

func TestCompilePatternFacetFastDigitClassMatchesXSDDigits(t *testing.T) {
	pattern, err := CompilePatternFacet(`[A-Z]{2}\d{4}`, nil)
	if err != nil {
		t.Fatalf("CompilePatternFacet() error = %v", err)
	}
	tests := []struct {
		value string
		want  bool
	}{
		{"AB1234", true},
		{"AB" + "\u0661\u0662\u0663\u0664", true},
		{"AB" + "\uff11\uff12\uff13\uff14", true},
		{"AB123", false},
		{"ab1234", false},
		{"AB12A4", false},
	}
	for _, test := range tests {
		if got := pattern.MatchString(test.value); got != test.want {
			t.Fatalf("pattern.MatchString(%q) = %v, want %v", test.value, got, test.want)
		}
	}
}
