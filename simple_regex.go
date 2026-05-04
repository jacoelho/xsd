package xsd

import (
	"regexp"
	"strings"
)

func compilePattern(source string) (pattern, error) {
	if err := validateXSDRegexSyntax(source); err != nil {
		return pattern{}, err
	}
	if strings.Contains(source, `\i`) || strings.Contains(source, `\c`) ||
		strings.Contains(source, "&&") ||
		strings.Contains(source, `\p{Is`) || strings.Contains(source, `\P{Is`) {
		return pattern{}, unsupported(ErrUnsupportedRegex, "XSD regex is not representable by Go regexp: "+source)
	}
	goPattern := translateXSDRegexToGo(source)
	goSource := "^(?:" + goPattern + ")$"
	re, err := regexp.Compile(goSource)
	if err != nil {
		return pattern{}, unsupported(ErrUnsupportedRegex, "invalid or unsupported regex "+source)
	}
	return pattern{XSDSource: source, GoSource: goSource, RE: re}, nil
}

func validateXSDRegexSyntax(source string) error {
	var v xsdRegexSyntaxValidator
	for _, r := range source {
		if err := v.consume(r); err != nil {
			return err
		}
	}
	return v.finish()
}

type xsdRegexSyntaxValidator struct {
	classTerms          []bool
	classFirst          []bool
	quantifierMin       uint64
	quantifierMax       uint64
	quantifierMinDigits int
	quantifierMaxDigits int
	groupDepth          int
	classRangeStart     rune
	classLastRune       rune
	escaped             bool
	inClass             bool
	classLastHyphen     bool
	classPendingRange   bool
	classHasLastRune    bool
	classJustRange      bool
	classHyphenAfter    bool
	inQuantifier        bool
	quantifierHasDigit  bool
	quantifierSawComma  bool
	pendingCategory     bool
	inCategory          bool
	canQuantify         bool
	prevQuantifier      bool
}

func (v *xsdRegexSyntaxValidator) consume(r rune) error {
	switch {
	case v.inCategory:
		return v.consumeCategory(r)
	case v.pendingCategory:
		return v.consumePendingCategory(r)
	case v.inQuantifier:
		return v.consumeQuantifier(r)
	case v.escaped:
		return v.consumeEscaped(r)
	case r == '\\':
		v.escaped = true
		return nil
	case v.inClass:
		return v.consumeClass(r)
	default:
		return v.consumeAtom(r)
	}
}

func (v *xsdRegexSyntaxValidator) consumeCategory(r rune) error {
	if r != '}' {
		return nil
	}
	v.inCategory = false
	if v.inClass {
		return v.finishClassCategory()
	}
	v.canQuantify = true
	v.prevQuantifier = false
	return nil
}

func (v *xsdRegexSyntaxValidator) finishClassCategory() error {
	if v.classPendingRange {
		return schemaCompile(ErrSchemaFacet, "invalid regex character range")
	}
	v.classTerms[len(v.classTerms)-1] = true
	v.classFirst[len(v.classFirst)-1] = false
	v.classLastHyphen = false
	v.classHasLastRune = false
	v.classJustRange = false
	v.classHyphenAfter = false
	return nil
}

func (v *xsdRegexSyntaxValidator) consumePendingCategory(r rune) error {
	if r != '{' {
		return schemaCompile(ErrSchemaFacet, "invalid regex category escape")
	}
	v.pendingCategory = false
	v.inCategory = true
	return nil
}

func (v *xsdRegexSyntaxValidator) consumeQuantifier(r rune) error {
	switch {
	case r >= '0' && r <= '9':
		v.addQuantifierDigit(r)
	case r == ',':
		if v.quantifierSawComma || v.quantifierMinDigits == 0 {
			return schemaCompile(ErrSchemaFacet, "invalid regex quantifier")
		}
		v.quantifierSawComma = true
	case r == '}':
		return v.finishQuantifier()
	default:
		return schemaCompile(ErrSchemaFacet, "invalid regex quantifier")
	}
	return nil
}

func (v *xsdRegexSyntaxValidator) addQuantifierDigit(r rune) {
	v.quantifierHasDigit = true
	digit := uint64(r - '0')
	if v.quantifierSawComma {
		v.quantifierMaxDigits++
		v.quantifierMax = v.quantifierMax*10 + digit
		return
	}
	v.quantifierMinDigits++
	v.quantifierMin = v.quantifierMin*10 + digit
}

func (v *xsdRegexSyntaxValidator) finishQuantifier() error {
	if !v.quantifierHasDigit {
		return schemaCompile(ErrSchemaFacet, "invalid regex quantifier")
	}
	if v.quantifierSawComma && v.quantifierMaxDigits != 0 && v.quantifierMax < v.quantifierMin {
		return schemaCompile(ErrSchemaFacet, "invalid regex quantifier")
	}
	v.inQuantifier = false
	v.canQuantify = false
	v.prevQuantifier = true
	return nil
}

func (v *xsdRegexSyntaxValidator) consumeEscaped(r rune) error {
	if !isXSDRegexEscape(r) {
		return schemaCompile(ErrSchemaFacet, "invalid regex escape")
	}
	if err := v.checkEscapedClassRange(r); err != nil {
		return err
	}
	if r == 'p' || r == 'P' {
		v.pendingCategory = true
		v.escaped = false
		return nil
	}
	if v.inClass {
		if err := v.acceptClassRune(r); err != nil {
			return err
		}
	}
	v.canQuantify = true
	v.prevQuantifier = false
	v.escaped = false
	return nil
}

func (v *xsdRegexSyntaxValidator) checkEscapedClassRange(r rune) error {
	if !v.inClass {
		return nil
	}
	if v.classLastHyphen && isXSDRegexMultiCharEscape(r) {
		return schemaCompile(ErrSchemaFacet, "invalid regex character range")
	}
	if v.classHyphenAfter {
		return schemaCompile(ErrSchemaFacet, "invalid regex character range")
	}
	return nil
}

func (v *xsdRegexSyntaxValidator) consumeClass(r rune) error {
	switch {
	case r == '[':
		return v.openNestedClass()
	case r == ']':
		return v.closeClass()
	case v.classHyphenAfter && r == '-':
		v.classLastHyphen = true
		return nil
	case v.classHyphenAfter:
		return schemaCompile(ErrSchemaFacet, "invalid regex character range")
	}
	last := len(v.classTerms) - 1
	if v.classFirst[last] && r == '^' {
		v.classFirst[last] = false
		v.classLastHyphen = false
		return nil
	}
	if r == '-' {
		return v.consumeClassHyphen(last)
	}
	return v.acceptClassRuneAt(last, r)
}

func (v *xsdRegexSyntaxValidator) openNestedClass() error {
	if !v.classLastHyphen {
		return schemaCompile(ErrSchemaFacet, "invalid nested regex character class")
	}
	v.classPendingRange = false
	v.classTerms = append(v.classTerms, false)
	v.classFirst = append(v.classFirst, true)
	v.classLastHyphen = false
	v.classHasLastRune = false
	v.classJustRange = false
	v.classHyphenAfter = false
	return nil
}

func (v *xsdRegexSyntaxValidator) closeClass() error {
	last := len(v.classTerms) - 1
	if !v.classTerms[last] {
		return schemaCompile(ErrSchemaFacet, "empty regex character class")
	}
	v.classTerms = v.classTerms[:last]
	v.classFirst = v.classFirst[:last]
	if len(v.classTerms) == 0 {
		v.inClass = false
		v.canQuantify = true
		v.prevQuantifier = false
	} else {
		v.classTerms[len(v.classTerms)-1] = true
		v.classFirst[len(v.classFirst)-1] = false
	}
	v.classLastHyphen = false
	v.classPendingRange = false
	v.classHasLastRune = false
	v.classJustRange = false
	v.classHyphenAfter = false
	return nil
}

func (v *xsdRegexSyntaxValidator) consumeClassHyphen(last int) error {
	if v.classJustRange {
		v.classLastHyphen = true
		v.classHyphenAfter = true
		v.classPendingRange = false
		v.classJustRange = false
		return nil
	}
	if v.classTerms[last] {
		v.classLastHyphen = true
		v.classPendingRange = true
		v.classRangeStart = v.classLastRune
	} else {
		v.classTerms[last] = true
		v.classFirst[last] = false
		v.classLastHyphen = false
		v.classPendingRange = false
		v.classLastRune = '-'
		v.classHasLastRune = true
	}
	v.classJustRange = false
	v.classHyphenAfter = false
	return nil
}

func (v *xsdRegexSyntaxValidator) acceptClassRune(r rune) error {
	return v.acceptClassRuneAt(len(v.classTerms)-1, r)
}

func (v *xsdRegexSyntaxValidator) acceptClassRuneAt(last int, r rune) error {
	if v.classPendingRange {
		if v.classHasLastRune && r < v.classRangeStart {
			return schemaCompile(ErrSchemaFacet, "invalid regex character range")
		}
		v.classPendingRange = false
		v.classJustRange = true
	} else {
		v.classJustRange = false
	}
	v.classTerms[last] = true
	v.classFirst[last] = false
	v.classLastHyphen = false
	v.classHyphenAfter = false
	v.classLastRune = r
	v.classHasLastRune = true
	return nil
}

func (v *xsdRegexSyntaxValidator) consumeAtom(r rune) error {
	switch r {
	case '[':
		v.openClass()
	case ']':
		return schemaCompile(ErrSchemaFacet, "unmatched regex character class end")
	case '(':
		v.groupDepth++
		v.canQuantify = false
		v.prevQuantifier = false
	case ')':
		return v.closeGroup()
	case '|':
		v.canQuantify = false
		v.prevQuantifier = false
	case '?', '*', '+':
		return v.consumeSimpleQuantifier()
	case '{':
		return v.openQuantifier()
	default:
		v.canQuantify = true
		v.prevQuantifier = false
	}
	return nil
}

func (v *xsdRegexSyntaxValidator) openClass() {
	v.inClass = true
	v.classTerms = []bool{false}
	v.classFirst = []bool{true}
	v.classLastHyphen = false
	v.classPendingRange = false
	v.classHasLastRune = false
	v.classJustRange = false
	v.classHyphenAfter = false
	v.canQuantify = false
	v.prevQuantifier = false
}

func (v *xsdRegexSyntaxValidator) closeGroup() error {
	if v.groupDepth == 0 {
		return schemaCompile(ErrSchemaFacet, "unmatched regex group end")
	}
	v.groupDepth--
	v.canQuantify = true
	v.prevQuantifier = false
	return nil
}

func (v *xsdRegexSyntaxValidator) consumeSimpleQuantifier() error {
	if !v.canQuantify || v.prevQuantifier {
		return schemaCompile(ErrSchemaFacet, "invalid regex quantifier")
	}
	v.canQuantify = false
	v.prevQuantifier = true
	return nil
}

func (v *xsdRegexSyntaxValidator) openQuantifier() error {
	if !v.canQuantify || v.prevQuantifier {
		return schemaCompile(ErrSchemaFacet, "invalid regex quantifier")
	}
	v.inQuantifier = true
	v.quantifierHasDigit = false
	v.quantifierSawComma = false
	v.quantifierMinDigits = 0
	v.quantifierMaxDigits = 0
	v.quantifierMin = 0
	v.quantifierMax = 0
	return nil
}

func (v *xsdRegexSyntaxValidator) finish() error {
	if v.escaped {
		return schemaCompile(ErrSchemaFacet, "trailing regex escape")
	}
	if v.pendingCategory || v.inCategory {
		return schemaCompile(ErrSchemaFacet, "unclosed regex category escape")
	}
	if v.inQuantifier {
		return schemaCompile(ErrSchemaFacet, "unclosed regex quantifier")
	}
	if v.inClass {
		return schemaCompile(ErrSchemaFacet, "unclosed regex character class")
	}
	if v.groupDepth != 0 {
		return schemaCompile(ErrSchemaFacet, "unclosed regex group")
	}
	return nil
}

func translateXSDRegexToGo(source string) string {
	var b strings.Builder
	escaped := false
	inClass := false
	for _, r := range source {
		if escaped {
			switch r {
			case 'd':
				if inClass {
					b.WriteString(xsdDigitClassInner)
				} else {
					b.WriteString(`[` + xsdDigitClassInner + `]`)
				}
			case 'D':
				if inClass {
					b.WriteString(`^` + xsdDigitClassInner)
				} else {
					b.WriteString(`[^` + xsdDigitClassInner + `]`)
				}
			case 'w':
				if inClass {
					b.WriteString(xsdWordClassInner)
				} else {
					b.WriteString(`[` + xsdWordClassInner + `]`)
				}
			case 'W':
				if inClass {
					b.WriteString(xsdNotWordClassInner)
				} else {
					b.WriteString(`[` + xsdNotWordClassInner + `]`)
				}
			default:
				b.WriteByte('\\')
				b.WriteRune(r)
			}
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if r == '[' {
			inClass = true
		} else if r == ']' {
			inClass = false
		} else if !inClass && (r == '^' || r == '$') {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	if escaped {
		b.WriteByte('\\')
	}
	return b.String()
}

const xsdDigitClassInner = `\x{0030}-\x{0039}\x{0660}-\x{0669}\x{06F0}-\x{06F9}\x{0966}-\x{096F}\x{09E6}-\x{09EF}\x{0A66}-\x{0A6F}\x{0AE6}-\x{0AEF}\x{0B66}-\x{0B6F}\x{0BE7}-\x{0BEF}\x{0C66}-\x{0C6F}\x{0CE6}-\x{0CEF}\x{0D66}-\x{0D6F}\x{0E50}-\x{0E59}\x{0ED0}-\x{0ED9}\x{0F20}-\x{0F29}\x{1040}-\x{1049}\x{1369}-\x{1371}\x{17E0}-\x{17E9}\x{1810}-\x{1819}\x{1D7CE}-\x{1D7FF}\x{FF10}-\x{FF19}`

const xsdWordClassInner = `^\pP\pZ\pC\x{023F}`

const xsdNotWordClassInner = `\pP\pZ\pC\x{023F}`

func isXSDRegexEscape(r rune) bool {
	switch r {
	case 'n', 'r', 't', '\\', '|', '-', '.', '?', '*', '+', '{', '}', '(', ')', '[', ']', '^',
		'd', 'D', 'w', 'W', 's', 'S', 'i', 'I', 'c', 'C', 'p', 'P':
		return true
	default:
		return false
	}
}

func isXSDRegexMultiCharEscape(r rune) bool {
	switch r {
	case 'd', 'D', 'w', 'W', 's', 'S', 'i', 'I', 'c', 'C', 'p', 'P':
		return true
	default:
		return false
	}
}
