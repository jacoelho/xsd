package runtime

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// StringPatternGroup is one pattern-facet derivation step. Patterns inside a
// group are OR-ed; groups are AND-ed in derivation order.
type StringPatternGroup struct {
	Patterns []StringPattern
}

// StringPattern is a compiled string pattern matcher used during validation.
type StringPattern struct {
	re        *regexp.Regexp
	fast      *SimplePattern
	xsdSource string
	goSource  string
}

// NewFastStringPattern returns a pattern backed by the runtime fast matcher.
func NewFastStringPattern(source string, fast *SimplePattern) StringPattern {
	return StringPattern{xsdSource: source, fast: fast}
}

// NewRegexpStringPattern returns a pattern backed by a Go regexp.
func NewRegexpStringPattern(xsdSource, goSource string, re *regexp.Regexp) StringPattern {
	return StringPattern{xsdSource: xsdSource, goSource: goSource, re: re}
}

// MatchString reports whether s matches p.
func (p StringPattern) MatchString(s string) bool {
	if p.fast != nil {
		return p.fast.MatchString(s)
	}
	return p.re.MatchString(s)
}

// MatchBytes reports whether s matches p.
func (p StringPattern) MatchBytes(s []byte) bool {
	if p.fast != nil {
		return p.fast.MatchBytes(s)
	}
	return p.re.Match(s)
}

// FacetProjection returns the freeze-comparable projection of p.
func (p StringPattern) FacetProjection() PatternFacet {
	out := PatternFacet{
		XSDSource: p.xsdSource,
		GoSource:  p.goSource,
	}
	if p.re != nil {
		out.HasRegexp = true
		out.Regexp = p.re.String()
	}
	if p.fast != nil {
		out.HasFast = true
		out.FastSignature = p.fast.signature()
	}
	return out
}

// CloneStringPatternGroups clones pattern groups and mutable fast matchers.
func CloneStringPatternGroups(in []StringPatternGroup) []StringPatternGroup {
	out := slices.Clone(in)
	for i, group := range in {
		out[i].Patterns = cloneStringPatterns(group.Patterns)
	}
	return out
}

func cloneStringPatterns(in []StringPattern) []StringPattern {
	out := slices.Clone(in)
	for i := range out {
		out[i].fast = cloneSimplePattern(in[i].fast)
	}
	return out
}

func cloneSimplePattern(in *SimplePattern) *SimplePattern {
	if in == nil {
		return nil
	}
	out := *in
	out.atoms = slices.Clone(in.atoms)
	for i := range out.atoms {
		out.atoms[i].class.ranges = slices.Clone(in.atoms[i].class.ranges)
	}
	return &out
}

func (p *SimplePattern) signature() string {
	var b strings.Builder
	b.WriteString("variable=")
	if p.variable {
		b.WriteString("true")
	} else {
		b.WriteString("false")
	}
	for _, atom := range p.atoms {
		b.WriteString("|min=")
		b.WriteString(strconv.Itoa(atom.min))
		b.WriteString("|max=")
		b.WriteString(strconv.Itoa(atom.max))
		b.WriteString("|digit=")
		if atom.class.digit {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
		for _, r := range atom.class.ranges {
			b.WriteString("|range=")
			b.WriteString(strconv.Itoa(int(r.lo)))
			b.WriteByte('-')
			b.WriteString(strconv.Itoa(int(r.hi)))
		}
	}
	return b.String()
}

// SimplePattern is a small compiled subset of XSD regex syntax.
type SimplePattern struct {
	atoms    []simplePatternAtom
	variable bool
}

type simplePatternAtom struct {
	class simplePatternClass
	min   int
	max   int
}

const simplePatternUnbounded = -1

type simplePatternClass struct {
	ranges []runeRange
	digit  bool
}

type runeRange struct {
	lo rune
	hi rune
}

// CompileSimpleStringPattern compiles the fast runtime subset of XSD regex
// syntax. It returns nil when source requires the general regexp path.
func CompileSimpleStringPattern(source string) *SimplePattern {
	var out SimplePattern
	for i := 0; i < len(source); {
		class, next, ok := parseSimplePatternAtom(source, i)
		if !ok {
			return nil
		}
		repeatMin := 1
		repeatMax := 1
		if next < len(source) && source[next] == '{' {
			parsedMin, parsedMax, after, ok := parseSimplePatternRepeat(source, next)
			if !ok {
				return nil
			}
			repeatMin = parsedMin
			repeatMax = parsedMax
			next = after
		}
		if repeatMin != repeatMax {
			out.variable = true
		}
		out.atoms = append(out.atoms, simplePatternAtom{class: class, min: repeatMin, max: repeatMax})
		i = next
	}
	return &out
}

func parseSimplePatternAtom(source string, i int) (simplePatternClass, int, bool) {
	switch source[i] {
	case '[':
		return parseSimplePatternClass(source, i)
	case '\\':
		return parseSimplePatternEscape(source, i)
	case '.', '|', '(', ')', '?', '*', '+', '{', '}', '^', '$':
		return simplePatternClass{}, 0, false
	default:
		r, size := utf8.DecodeRuneInString(source[i:])
		if r == utf8.RuneError && size == 0 {
			return simplePatternClass{}, 0, false
		}
		return simplePatternClass{ranges: []runeRange{{lo: r, hi: r}}}, i + size, true
	}
}

func parseSimplePatternEscape(source string, i int) (simplePatternClass, int, bool) {
	if i+1 >= len(source) {
		return simplePatternClass{}, 0, false
	}
	switch source[i+1] {
	case 'd':
		return simplePatternClass{digit: true}, i + 2, true
	case 'n':
		return simplePatternClass{ranges: []runeRange{{lo: '\n', hi: '\n'}}}, i + 2, true
	case 'r':
		return simplePatternClass{ranges: []runeRange{{lo: '\r', hi: '\r'}}}, i + 2, true
	case 't':
		return simplePatternClass{ranges: []runeRange{{lo: '\t', hi: '\t'}}}, i + 2, true
	case '\\', '|', '-', '.', '?', '*', '+', '{', '}', '(', ')', '[', ']', '^':
		r := rune(source[i+1])
		return simplePatternClass{ranges: []runeRange{{lo: r, hi: r}}}, i + 2, true
	default:
		return simplePatternClass{}, 0, false
	}
}

func parseSimplePatternClass(source string, i int) (simplePatternClass, int, bool) {
	i++
	if i >= len(source) || source[i] == '^' {
		return simplePatternClass{}, 0, false
	}
	var class simplePatternClass
	for i < len(source) {
		if source[i] == ']' {
			if len(class.ranges) == 0 {
				return simplePatternClass{}, 0, false
			}
			return class, i + 1, true
		}
		lo, next, ok := parseSimplePatternClassRune(source, i)
		if !ok {
			return simplePatternClass{}, 0, false
		}
		if next < len(source) && source[next] == '-' && next+1 < len(source) && source[next+1] != ']' {
			hi, after, ok := parseSimplePatternClassRune(source, next+1)
			if !ok || hi < lo {
				return simplePatternClass{}, 0, false
			}
			class.ranges = append(class.ranges, runeRange{lo: lo, hi: hi})
			i = after
			continue
		}
		class.ranges = append(class.ranges, runeRange{lo: lo, hi: lo})
		i = next
	}
	return simplePatternClass{}, 0, false
}

func parseSimplePatternClassRune(source string, i int) (rune, int, bool) {
	if source[i] == '[' {
		return 0, 0, false
	}
	if source[i] == '\\' {
		if i+1 >= len(source) {
			return 0, 0, false
		}
		switch source[i+1] {
		case 'n':
			return '\n', i + 2, true
		case 'r':
			return '\r', i + 2, true
		case 't':
			return '\t', i + 2, true
		case '\\', '|', '-', '.', '?', '*', '+', '{', '}', '(', ')', '[', ']', '^':
			return rune(source[i+1]), i + 2, true
		default:
			return 0, 0, false
		}
	}
	r, size := utf8.DecodeRuneInString(source[i:])
	if r == utf8.RuneError && size == 0 {
		return 0, 0, false
	}
	return r, i + size, true
}

func parseSimplePatternRepeat(source string, i int) (int, int, int, bool) {
	end := strings.IndexByte(source[i:], '}')
	if end < 0 {
		return 0, 0, 0, false
	}
	end += i
	body := source[i+1 : end]
	if body == "" {
		return 0, 0, 0, false
	}
	lower, upper, found := strings.Cut(body, ",")
	if lower == "" {
		return 0, 0, 0, false
	}
	repeatMin, err := strconv.Atoi(lower)
	if err != nil || repeatMin < 0 {
		return 0, 0, 0, false
	}
	if !found {
		return repeatMin, repeatMin, end + 1, true
	}
	if upper == "" {
		return repeatMin, simplePatternUnbounded, end + 1, true
	}
	repeatMax, err := strconv.Atoi(upper)
	if err != nil || repeatMax < repeatMin {
		return 0, 0, 0, false
	}
	return repeatMin, repeatMax, end + 1, true
}

// MatchString reports whether s matches p.
func (p *SimplePattern) MatchString(s string) bool {
	if p.variable {
		return p.matchVariableString(s)
	}
	i := 0
	for _, atom := range p.atoms {
		for range atom.min {
			if i >= len(s) {
				return false
			}
			r, size := utf8.DecodeRuneInString(s[i:])
			if r == utf8.RuneError && size == 0 {
				return false
			}
			if !atom.class.matches(r) {
				return false
			}
			i += size
		}
	}
	return i == len(s)
}

// MatchBytes reports whether s matches p.
func (p *SimplePattern) MatchBytes(s []byte) bool {
	if p.variable {
		return p.matchVariableBytes(s)
	}
	i := 0
	for _, atom := range p.atoms {
		for range atom.min {
			if i >= len(s) {
				return false
			}
			r, size := utf8.DecodeRune(s[i:])
			if r == utf8.RuneError && size == 0 {
				return false
			}
			if !atom.class.matches(r) {
				return false
			}
			i += size
		}
	}
	return i == len(s)
}

// smallPatternRunes is the input length up to which variable-length matching
// runs on stack buffers; longer inputs fall back to heap allocations.
const smallPatternRunes = 128

func (p *SimplePattern) matchVariableString(s string) bool {
	if len(s) <= smallPatternRunes {
		var stack [smallPatternRunes]rune
		runes := stack[:0]
		for _, r := range s {
			runes = append(runes, r)
		}
		return p.matchVariableRunes(runes)
	}
	return p.matchVariableRunes([]rune(s))
}

func (p *SimplePattern) matchVariableBytes(s []byte) bool {
	var stack [smallPatternRunes]rune
	runes := stack[:0]
	if n := utf8.RuneCount(s); n > len(stack) {
		runes = make([]rune, 0, n)
	}
	for len(s) > 0 {
		r, size := utf8.DecodeRune(s)
		runes = append(runes, r)
		s = s[size:]
	}
	return p.matchVariableRunes(runes)
}

func (p *SimplePattern) matchVariableRunes(runes []rune) bool {
	runeCount := len(runes)
	var stack [2 * (smallPatternRunes + 1)]bool
	var buf []bool
	if size := 2 * (runeCount + 1); size <= len(stack) {
		buf = stack[:size]
	} else {
		buf = make([]bool, size)
	}
	prev, next := buf[:runeCount+1], buf[runeCount+1:]
	prev[0] = true
	for _, atom := range p.atoms {
		clear(next)
		if atom.min == 0 {
			copy(next, prev)
		}
		if atom.max != 0 {
			minRepeat := atom.min
			if minRepeat == 0 {
				minRepeat = 1
			}
			start := 0
			for start < runeCount {
				for start < runeCount && !atom.class.matches(runes[start]) {
					start++
				}
				runStart := start
				for start < runeCount && atom.class.matches(runes[start]) {
					start++
				}
				markRepeatedRun(prev, next, runStart, start, minRepeat, atom.max)
			}
		}
		if !hasReachableOffset(next) {
			return false
		}
		prev, next = next, prev
	}
	return prev[runeCount]
}

func markRepeatedRun(prev, next []bool, start, end, minRepeat, maxRepeat int) {
	if end-start < minRepeat {
		return
	}
	active := 0
	for pos := start + minRepeat; pos <= end; pos++ {
		if prev[pos-minRepeat] {
			active++
		}
		if maxRepeat != simplePatternUnbounded {
			remove := pos - maxRepeat - 1
			if remove >= start && prev[remove] {
				active--
			}
		}
		if active > 0 {
			next[pos] = true
		}
	}
}

func hasReachableOffset(offsets []bool) bool {
	for _, ok := range offsets {
		if ok {
			return true
		}
	}
	return false
}

func (c simplePatternClass) matches(r rune) bool {
	if c.digit && isXSDDigitRune(r) {
		return true
	}
	for _, rr := range c.ranges {
		if r >= rr.lo && r <= rr.hi {
			return true
		}
	}
	return false
}

func isXSDDigitRune(r rune) bool {
	switch {
	case r >= 0x0030 && r <= 0x0039,
		r >= 0x0660 && r <= 0x0669,
		r >= 0x06F0 && r <= 0x06F9,
		r >= 0x0966 && r <= 0x096F,
		r >= 0x09E6 && r <= 0x09EF,
		r >= 0x0A66 && r <= 0x0A6F,
		r >= 0x0AE6 && r <= 0x0AEF,
		r >= 0x0B66 && r <= 0x0B6F,
		r >= 0x0BE7 && r <= 0x0BEF,
		r >= 0x0C66 && r <= 0x0C6F,
		r >= 0x0CE6 && r <= 0x0CEF,
		r >= 0x0D66 && r <= 0x0D6F,
		r >= 0x0E50 && r <= 0x0E59,
		r >= 0x0ED0 && r <= 0x0ED9,
		r >= 0x0F20 && r <= 0x0F29,
		r >= 0x1040 && r <= 0x1049,
		r >= 0x1369 && r <= 0x1371,
		r >= 0x17E0 && r <= 0x17E9,
		r >= 0x1810 && r <= 0x1819,
		r >= 0x1D7CE && r <= 0x1D7FF,
		r >= 0xFF10 && r <= 0xFF19:
		return true
	default:
		return false
	}
}
