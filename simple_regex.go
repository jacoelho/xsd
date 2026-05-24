package xsd

import (
	"regexp"
	"strings"
)

const maxGoRegexpRepeat = "1000"

func (c *compiler) compilePattern(source string) (pattern, error) {
	return compilePatternWithCompiler(source, c)
}

func compilePatternWithCompiler(source string, c *compiler) (pattern, error) {
	if err := validateXSDRegexSyntaxWithCompiler(source, c); err != nil {
		return pattern{}, err
	}
	goPattern := translateXSDRegexToGo(source)
	goSource := "^(?:" + goPattern + ")$"
	re, err := regexp.Compile(goSource)
	if err != nil {
		return pattern{}, unsupported(ErrUnsupportedRegex, "invalid or unsupported regex "+source)
	}
	return pattern{XSDSource: source, GoSource: goSource, RE: re}, nil
}

func validateXSDRegexSyntaxWithCompiler(source string, c *compiler) error {
	v := xsdRegexSyntaxValidator{compiler: c}
	for _, r := range source {
		if err := v.consume(r); err != nil {
			return err
		}
	}
	if err := v.finish(); err != nil {
		return err
	}
	if v.unsupported {
		return unsupported(ErrUnsupportedRegex, "XSD regex is not representable by Go regexp: "+source)
	}
	return nil
}

type xsdRegexSyntaxValidator struct {
	compiler           *compiler
	categoryName       string
	classTerms         []bool
	classFirst         []bool
	classTermCount     []int
	classUnsafeEscape  []bool
	classNegated       []bool
	quantifierMin      []byte
	quantifierMax      []byte
	groupDepth         int
	classRangeStart    rune
	classLastRune      rune
	escaped            bool
	inClass            bool
	classLastHyphen    bool
	classPendingRange  bool
	classHasLastRune   bool
	classJustRange     bool
	classHyphenAfter   bool
	inQuantifier       bool
	quantifierHasDigit bool
	quantifierSawComma bool
	pendingCategory    bool
	inCategory         bool
	canQuantify        bool
	prevQuantifier     bool
	unsupported        bool
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
		v.categoryName += string(r)
		return nil
	}
	switch {
	case v.categoryName == "":
		return schemaCompile(ErrSchemaFacet, "invalid regex category escape")
	case strings.HasPrefix(v.categoryName, "Is"):
		if len(v.categoryName) == len("Is") {
			return schemaCompile(ErrSchemaFacet, "invalid regex category escape")
		}
		v.unsupported = true
	case !v.validGoRegexCategory(v.categoryName):
		return schemaCompile(ErrSchemaFacet, "invalid regex category escape")
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
	last := len(v.classTerms) - 1
	v.classTerms[last] = true
	v.classFirst[last] = false
	v.markClassTerm(last)
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
	v.categoryName = ""
	return nil
}

func validGoRegexCategory(name string) bool {
	_, err := regexp.Compile(`\p{` + name + `}`)
	return err == nil
}

func (c *compiler) validGoRegexCategory(name string) bool {
	ok, found := c.regexCategories[name]
	if found {
		return ok
	}
	if c.regexCategories == nil {
		c.regexCategories = make(map[string]bool)
	}
	ok = validGoRegexCategory(name)
	c.regexCategories[name] = ok
	return ok
}

func (v *xsdRegexSyntaxValidator) validGoRegexCategory(name string) bool {
	if v.compiler == nil {
		return validGoRegexCategory(name)
	}
	return v.compiler.validGoRegexCategory(name)
}

func (v *xsdRegexSyntaxValidator) consumeQuantifier(r rune) error {
	switch {
	case r >= '0' && r <= '9':
		v.addQuantifierDigit(r)
	case r == ',':
		if v.quantifierSawComma || len(v.quantifierMin) == 0 {
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
	if v.quantifierSawComma {
		v.quantifierMax = append(v.quantifierMax, byte(r))
		return
	}
	v.quantifierMin = append(v.quantifierMin, byte(r))
}

func (v *xsdRegexSyntaxValidator) finishQuantifier() error {
	if !v.quantifierHasDigit {
		return schemaCompile(ErrSchemaFacet, "invalid regex quantifier")
	}
	if v.quantifierSawComma && len(v.quantifierMax) != 0 && compareRegexQuantity(v.quantifierMax, v.quantifierMin) < 0 {
		return schemaCompile(ErrSchemaFacet, "invalid regex quantifier")
	}
	if regexQuantityExceedsGoLimit(v.quantifierMin) ||
		v.quantifierSawComma && len(v.quantifierMax) != 0 && regexQuantityExceedsGoLimit(v.quantifierMax) {
		v.unsupported = true
	}
	v.inQuantifier = false
	v.canQuantify = false
	v.prevQuantifier = true
	return nil
}

func regexQuantityExceedsGoLimit(s []byte) bool {
	return compareRegexQuantityText(s, maxGoRegexpRepeat) > 0
}

func compareRegexQuantity(a, b []byte) int {
	a = trimRegexQuantityBytes(a)
	b = trimRegexQuantityBytes(b)
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func compareRegexQuantityText(a []byte, b string) int {
	a = trimRegexQuantityBytes(a)
	b = trimRegexQuantityText(b)
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func trimRegexQuantityBytes(s []byte) []byte {
	for len(s) > 1 && s[0] == '0' {
		s = s[1:]
	}
	return s
}

func trimRegexQuantityText(s string) string {
	s = strings.TrimLeft(s, "0")
	if s == "" {
		return "0"
	}
	return s
}

func (v *xsdRegexSyntaxValidator) consumeEscaped(r rune) error {
	if !isXSDRegexEscape(r) {
		return schemaCompile(ErrSchemaFacet, "invalid regex escape")
	}
	if err := v.checkEscapedClassRange(r); err != nil {
		return err
	}
	if r == 'i' || r == 'I' || r == 'c' || r == 'C' {
		v.unsupported = true
	}
	if r == 'p' || r == 'P' {
		v.pendingCategory = true
		v.escaped = false
		return nil
	}
	if v.inClass {
		if isUnsafeXSDRegexClassEscape(r) {
			v.classUnsafeEscape[len(v.classUnsafeEscape)-1] = true
		}
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
		v.classNegated[last] = true
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
	v.unsupported = true
	v.classPendingRange = false
	v.classTerms = append(v.classTerms, false)
	v.classFirst = append(v.classFirst, true)
	v.classTermCount = append(v.classTermCount, 0)
	v.classUnsafeEscape = append(v.classUnsafeEscape, false)
	v.classNegated = append(v.classNegated, false)
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
	if v.classPendingRange {
		v.markClassTerm(last)
	}
	if v.classUnsafeEscape[last] && (v.classNegated[last] || v.classTermCount[last] != 1) {
		v.unsupported = true
	}
	v.classTerms = v.classTerms[:last]
	v.classFirst = v.classFirst[:last]
	v.classTermCount = v.classTermCount[:last]
	v.classUnsafeEscape = v.classUnsafeEscape[:last]
	v.classNegated = v.classNegated[:last]
	if len(v.classTerms) == 0 {
		v.inClass = false
		v.canQuantify = true
		v.prevQuantifier = false
	} else {
		parent := len(v.classTerms) - 1
		v.classTerms[parent] = true
		v.classFirst[parent] = false
		v.markClassTerm(parent)
	}
	v.classLastHyphen = false
	v.classPendingRange = false
	v.classHasLastRune = false
	v.classJustRange = false
	v.classHyphenAfter = false
	return nil
}

func (v *xsdRegexSyntaxValidator) consumeClassHyphen(last int) error {
	if v.classPendingRange {
		return schemaCompile(ErrSchemaFacet, "invalid regex character range")
	}
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
		v.markClassTerm(last)
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
	v.markClassTerm(last)
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
	v.classTermCount = []int{0}
	v.classUnsafeEscape = []bool{false}
	v.classNegated = []bool{false}
	v.classLastHyphen = false
	v.classPendingRange = false
	v.classHasLastRune = false
	v.classJustRange = false
	v.classHyphenAfter = false
	v.canQuantify = false
	v.prevQuantifier = false
}

func (v *xsdRegexSyntaxValidator) markClassTerm(last int) {
	v.classTermCount[last]++
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
	v.quantifierMin = v.quantifierMin[:0]
	v.quantifierMax = v.quantifierMax[:0]
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
	for i := 0; i < len(source); i++ {
		c := source[i]
		if escaped {
			switch c {
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
			case 's':
				if inClass {
					b.WriteString(xsdSpaceClassInner)
				} else {
					b.WriteString(`[` + xsdSpaceClassInner + `]`)
				}
			case 'S':
				if inClass {
					b.WriteString(`^` + xsdSpaceClassInner)
				} else {
					b.WriteString(`[^` + xsdSpaceClassInner + `]`)
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
				b.WriteByte(c)
			}
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		switch {
		case c == '[':
			inClass = true
		case c == ']':
			inClass = false
		case !inClass && c == '{':
			end := strings.IndexByte(source[i:], '}')
			if end >= 0 {
				end += i
				b.WriteString(normalizeXSDRegexQuantifier(source[i : end+1]))
				i = end
				continue
			}
		case !inClass && (c == '^' || c == '$'):
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	if escaped {
		b.WriteByte('\\')
	}
	return b.String()
}

func normalizeXSDRegexQuantifier(s string) string {
	if len(s) < 3 || s[0] != '{' || s[len(s)-1] != '}' {
		return s
	}
	body := s[1 : len(s)-1]
	lower, upper, found := strings.Cut(body, ",")
	if !found {
		return "{" + trimRegexQuantityText(lower) + "}"
	}
	if upper == "" {
		return "{" + trimRegexQuantityText(lower) + ",}"
	}
	return "{" + trimRegexQuantityText(lower) + "," + trimRegexQuantityText(upper) + "}"
}

const xsdDigitClassInner = `\x{0030}-\x{0039}\x{0660}-\x{0669}\x{06F0}-\x{06F9}\x{0966}-\x{096F}\x{09E6}-\x{09EF}\x{0A66}-\x{0A6F}\x{0AE6}-\x{0AEF}\x{0B66}-\x{0B6F}\x{0BE7}-\x{0BEF}\x{0C66}-\x{0C6F}\x{0CE6}-\x{0CEF}\x{0D66}-\x{0D6F}\x{0E50}-\x{0E59}\x{0ED0}-\x{0ED9}\x{0F20}-\x{0F29}\x{1040}-\x{1049}\x{1369}-\x{1371}\x{17E0}-\x{17E9}\x{1810}-\x{1819}\x{1D7CE}-\x{1D7FF}\x{FF10}-\x{FF19}`

const xsdSpaceClassInner = `\x{0009}\x{000A}\x{000D}\x{0020}`

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

func isUnsafeXSDRegexClassEscape(r rune) bool {
	switch r {
	case 'D', 'S', 'w', 'W':
		return true
	default:
		return false
	}
}
