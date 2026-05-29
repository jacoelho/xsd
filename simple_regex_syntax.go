package xsd

import (
	"regexp"
	"strings"
)

const maxGoRegexpRepeat = "1000"

func validateXSDRegexSyntaxWithCompiler(source string, c *compiler) error {
	goUnsupported, err := checkXSDRegexSyntaxWithCompiler(source, c)
	if err != nil {
		return err
	}
	if goUnsupported {
		return unsupported(ErrUnsupportedRegex, "XSD regex is not representable by Go regexp: "+source)
	}
	return nil
}

func checkXSDRegexSyntaxWithCompiler(source string, c *compiler) (bool, error) {
	v := xsdRegexSyntaxValidator{compiler: c}
	for _, r := range source {
		if err := v.consume(r); err != nil {
			return false, err
		}
	}
	if err := v.finish(); err != nil {
		return false, err
	}
	return v.unsupported, nil
}

type xsdRegexSyntaxValidator struct {
	compiler           *compiler
	categoryName       string
	classStack         []regexClassState
	quantifierMin      []byte
	quantifierMax      []byte
	groupDepth         int
	classRangeStart    rune
	classLastRune      rune
	escaped            bool
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

type regexClassState struct {
	termCount    int
	hasTerm      bool
	first        bool
	unsafeEscape bool
	lastTermSet  bool
	negated      bool
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
	case v.insideClass():
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
	if v.insideClass() {
		return v.finishClassCategory()
	}
	v.canQuantify = true
	v.prevQuantifier = false
	return nil
}

func (v *xsdRegexSyntaxValidator) finishClassCategory() error {
	return v.acceptClassSet()
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
		v.addQuantifierDigit("0123456789"[r-'0'])
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

func (v *xsdRegexSyntaxValidator) addQuantifierDigit(b byte) {
	v.quantifierHasDigit = true
	if v.quantifierSawComma {
		v.quantifierMax = append(v.quantifierMax, b)
		return
	}
	v.quantifierMin = append(v.quantifierMin, b)
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
	return compareRegexQuantity(s, []byte(maxGoRegexpRepeat)) > 0
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

func trimRegexQuantityBytes(s []byte) []byte {
	for len(s) > 1 && s[0] == '0' {
		s = s[1:]
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
		if v.insideClass() && v.classPendingRange {
			return schemaCompile(ErrSchemaFacet, "invalid regex character range")
		}
		v.pendingCategory = true
		v.escaped = false
		return nil
	}
	if v.insideClass() {
		class := v.currentClass()
		if isUnsafeXSDRegexClassEscape(r) {
			class.unsafeEscape = true
		}
		if isXSDRegexMultiCharEscape(r) {
			if err := v.acceptClassSet(); err != nil {
				return err
			}
		} else if err := v.acceptClassRuneAt(r); err != nil {
			return err
		}
	}
	v.canQuantify = true
	v.prevQuantifier = false
	v.escaped = false
	return nil
}

func (v *xsdRegexSyntaxValidator) checkEscapedClassRange(r rune) error {
	if !v.insideClass() {
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
	class := v.currentClass()
	if class.first && r == '^' {
		class.first = false
		class.negated = true
		v.classLastHyphen = false
		return nil
	}
	if r == '-' {
		return v.consumeClassHyphen()
	}
	return v.acceptClassRuneAt(r)
}

func (v *xsdRegexSyntaxValidator) openNestedClass() error {
	if !v.classLastHyphen {
		return schemaCompile(ErrSchemaFacet, "invalid nested regex character class")
	}
	v.unsupported = true
	v.classPendingRange = false
	v.classStack = append(v.classStack, regexClassState{first: true})
	v.classLastHyphen = false
	v.classHasLastRune = false
	v.classJustRange = false
	v.classHyphenAfter = false
	return nil
}

func (v *xsdRegexSyntaxValidator) closeClass() error {
	last := len(v.classStack) - 1
	class := v.classStack[last]
	if !class.hasTerm {
		return schemaCompile(ErrSchemaFacet, "empty regex character class")
	}
	if v.classPendingRange {
		class.termCount++
	}
	if class.unsafeEscape && (class.negated || class.termCount != 1) {
		v.unsupported = true
	}
	v.classStack = v.classStack[:last]
	if len(v.classStack) == 0 {
		v.canQuantify = true
		v.prevQuantifier = false
	} else {
		parent := v.currentClass()
		parent.hasTerm = true
		parent.first = false
		parent.termCount++
		parent.lastTermSet = true
	}
	v.classLastHyphen = false
	v.classPendingRange = false
	v.classHasLastRune = false
	v.classJustRange = false
	v.classHyphenAfter = false
	return nil
}

func (v *xsdRegexSyntaxValidator) consumeClassHyphen() error {
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
	class := v.currentClass()
	if class.hasTerm {
		if class.lastTermSet {
			v.classLastHyphen = true
			v.classHyphenAfter = true
			v.classPendingRange = true
			v.classJustRange = false
			return nil
		}
		v.classLastHyphen = true
		v.classPendingRange = true
		v.classRangeStart = v.classLastRune
	} else {
		class.hasTerm = true
		class.first = false
		class.termCount++
		v.classLastHyphen = false
		v.classPendingRange = false
		v.classLastRune = '-'
		v.classHasLastRune = true
		class.lastTermSet = false
	}
	v.classHyphenAfter = false
	return nil
}

func (v *xsdRegexSyntaxValidator) acceptClassRuneAt(r rune) error {
	if v.classPendingRange {
		if v.classHasLastRune && r < v.classRangeStart {
			return schemaCompile(ErrSchemaFacet, "invalid regex character range")
		}
		v.classPendingRange = false
		v.classJustRange = true
	} else {
		v.classJustRange = false
	}
	class := v.currentClass()
	class.hasTerm = true
	class.first = false
	class.termCount++
	v.classLastHyphen = false
	v.classHyphenAfter = false
	v.classLastRune = r
	v.classHasLastRune = true
	class.lastTermSet = false
	return nil
}

func (v *xsdRegexSyntaxValidator) acceptClassSet() error {
	if v.classHyphenAfter || v.classPendingRange {
		return schemaCompile(ErrSchemaFacet, "invalid regex character range")
	}
	class := v.currentClass()
	class.hasTerm = true
	class.first = false
	class.termCount++
	v.classLastHyphen = false
	v.classHyphenAfter = false
	v.classHasLastRune = false
	v.classJustRange = false
	class.lastTermSet = true
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
	v.classStack = []regexClassState{{first: true}}
	v.classLastHyphen = false
	v.classPendingRange = false
	v.classHasLastRune = false
	v.classJustRange = false
	v.classHyphenAfter = false
	v.canQuantify = false
	v.prevQuantifier = false
}

func (v *xsdRegexSyntaxValidator) insideClass() bool {
	return len(v.classStack) != 0
}

func (v *xsdRegexSyntaxValidator) currentClass() *regexClassState {
	return &v.classStack[len(v.classStack)-1]
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
	if v.insideClass() {
		return schemaCompile(ErrSchemaFacet, "unclosed regex character class")
	}
	if v.groupDepth != 0 {
		return schemaCompile(ErrSchemaFacet, "unclosed regex group")
	}
	return nil
}

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
