// Package xsdregex implements the regular-expression language defined by
// XML Schema 1.0 Part 2, Appendix F.
//
// It deliberately does not translate XSD syntax into a host regexp dialect.
// XSD character sets (including subtraction) are normalized to immutable rune
// ranges. Patterns compile to a bounded linear matcher or Thompson NFA, and
// matching evaluates the chosen form with caller-owned scratch storage and
// explicit work limits.
package xsdregex

import (
	"errors"
	"fmt"
	"math"
	"unicode/utf8"
)

// ErrorKind identifies a pattern compilation or matching failure.
type ErrorKind uint8

const (
	// ErrorSyntax indicates that a pattern is not valid XSD 1.0 syntax.
	ErrorSyntax ErrorKind = iota + 1
	// ErrorLimit indicates that an explicit resource limit was exceeded.
	ErrorLimit
)

// Error is returned for invalid patterns and bounded resource failures.
type Error struct {
	What   string
	Offset int
	Kind   ErrorKind
}

const (
	nilPatternError     = "nil pattern"
	matchWorkLimitError = "match work limit exceeded"
)

func (e *Error) Error() string {
	if e.Offset >= 0 {
		return fmt.Sprintf("xsdregex: %s at rune %d", e.What, e.Offset)
	}
	return "xsdregex: " + e.What
}

// IsLimit reports whether err is a compile or match resource-limit error.
func IsLimit(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Kind == ErrorLimit
}

// compileLimits are the fixed structural bounds of one compiled pattern. They
// are not per-value budgets: the NFA expands counted repeats and precomputes
// closures, so its size is capped when the schema is compiled. states bounds
// len(states), which bounds every reachable match-time state set; it never
// limits input length. Repetitions are never expanded into one AST node per
// occurrence. The aggregate over patterns is admitted by the caller from
// Pattern.RetainedBytes.
type compileLimits struct {
	patternBytes   uint64
	nodes          uint64
	states         uint64
	retainedRanges uint64
	depth          uint64
}

// fixedCompileLimits are the bounds Compile applies; docs/RESOURCE_LIMITS.md
// publishes them.
var fixedCompileLimits = compileLimits{
	patternBytes:   1 << 20,
	nodes:          100_000,
	states:         100_000,
	retainedRanges: 1_000_000,
	depth:          1_024,
}

// MatchOptions bounds one match. MaxWork counts:
//   - one unit per input byte, charged first;
//   - nothing further for literal patterns;
//   - for linear patterns, 4*(runes+1)+1 per atom, every atom including
//     literal atoms of a non-literal pattern and exact-count atoms;
//   - for NFA patterns, one unit per set-state test and per split or jump
//     followed during epsilon closure (bitset runner: the popcount of active
//     states per rune), so NFA work is at most 72*(runes+1)*compiled states.
//
// Zero admits no work: every match returns an ErrorLimit error. Work depends
// only on the pattern and the input.
type MatchOptions struct {
	MaxWork uint64
}

// Pattern is an immutable compiled XSD regular expression. Concatenations of
// character sets use the bounded run-length matcher; general expressions use
// the Thompson machine. Pattern is immutable after Compile; all match state
// lives in the caller's Scratch, so one Pattern may be shared.
//
// The fields stay flat. Rejected: embedding compiler return records
// (compiledNFA, bitsetNFA), because the aggregate copies introduce bulk write
// barriers and enlarge frames active during graph and bitset allocations.
type Pattern struct {
	linear        *linearPattern
	source        string
	states        []instruction
	closures      [][]int
	startClosure  []int
	closureBits   []uint64
	start         int
	startBits     uint64
	acceptBits    uint64
	deterministic bool
	bitset        bool

	// retainedRanges counts runeRange elements in distinct backing arrays.
	retainedRanges uint64
}

// Compile parses and compiles one XSD 1.0 regular expression under the fixed
// per-pattern bounds. XSD patterns match the complete input; callers do not
// add host-language anchors.
func Compile(source string) (*Pattern, error) {
	return compileWithLimits(source, fixedCompileLimits)
}

func compileWithLimits(source string, options compileLimits) (*Pattern, error) {
	if uint64(len(source)) > options.patternBytes {
		return nil, &Error{Kind: ErrorLimit, Offset: -1, What: "pattern exceeds byte limit"}
	}
	if !utf8.ValidString(source) {
		return nil, &Error{Kind: ErrorSyntax, Offset: -1, What: "pattern is not valid UTF-8"}
	}
	p := parser{
		source: []rune(source),
		limits: options,
	}
	root, err := p.parse()
	if err != nil {
		return nil, err
	}
	if linear, ok := compileLinear(root, options); ok {
		return &Pattern{source: source, linear: linear, retainedRanges: p.retainedRanges}, nil
	}
	nfa, err := compileNFA(root, options)
	if err != nil {
		return nil, err
	}
	deterministic := isDeterministicNFA(nfa.states, nfa.closures, nfa.startClosure)
	bitset := buildBitsetNFA(nfa.states, nfa.closures, nfa.startClosure)
	return &Pattern{
		source:        source,
		start:         nfa.start,
		states:        nfa.states,
		closures:      nfa.closures,
		startClosure:  nfa.startClosure,
		deterministic: deterministic,
		bitset:        bitset.enabled,
		closureBits:   bitset.closureBits,
		startBits:     bitset.startBits,
		acceptBits:    bitset.acceptBits,

		retainedRanges: p.retainedRanges,
	}, nil
}

// Source returns the original pattern text.
func (p *Pattern) Source() string {
	if p == nil {
		return ""
	}
	return p.source
}

// MatchStringWithScratch matches complete valid UTF-8 XML text using
// caller-owned bounded scratch. Nil scratch keeps storage local to this call.
// Scratch is not safe for concurrent use by multiple goroutines.
//
// work is the number of units consumed, at most options.MaxWork, and is
// meaningful only when err is nil. Equal inputs report equal work.
func (p *Pattern) MatchStringWithScratch(input string, options MatchOptions, scratch *Scratch) (matched bool, work uint64, err error) {
	if p == nil {
		return false, 0, &Error{Kind: ErrorSyntax, Offset: -1, What: nilPatternError}
	}
	if options.MaxWork == 0 {
		return false, 0, &Error{Kind: ErrorLimit, Offset: -1, What: matchWorkLimitError}
	}
	return matchString(p, input, options, scratch)
}

// MatchBytesWithScratch matches validated XML bytes using caller-owned bounded
// scratch. Callers must validate the byte stream before matching. Nil scratch
// keeps storage local to this call. work has the meaning documented on
// MatchStringWithScratch.
func (p *Pattern) MatchBytesWithScratch(input []byte, options MatchOptions, scratch *Scratch) (matched bool, work uint64, err error) {
	if p == nil {
		return false, 0, &Error{Kind: ErrorSyntax, Offset: -1, What: nilPatternError}
	}
	if options.MaxWork == 0 {
		return false, 0, &Error{Kind: ErrorLimit, Offset: -1, What: matchWorkLimitError}
	}
	return matchBytes(p, input, options, scratch)
}

type nodeKind uint8

const (
	nodeEmpty nodeKind = iota
	nodeSet
	nodeConcat
	nodeAlt
	nodeRepeat
)

type node struct {
	set       rangeSet
	children  []*node
	min       uint64
	max       uint64
	kind      nodeKind
	unbounded bool
}

const maxRepeat = math.MaxUint64

// parser is local to one Compile call, and its set caches die with it. Only
// the fixed category, block and name catalogs are package-level; no
// process-wide cache retains compiled patterns or per-compile sets.
type parser struct {
	simpleSets   map[rune]rangeSet
	seenRanges   map[*runeRange]struct{}
	categorySets categorySetCache
	source       []rune
	limits       compileLimits
	pos          int
	depth        uint64
	nodes        uint64
	// retainedRanges counts runeRange elements (by cap) of distinct backing arrays.
	retainedRanges uint64
}

const simpleSetCacheCapacity = 10 // d, D, s, S, w, W, i, I, c, and C.

// namedCategorySet accepts the closed category and block catalogs. Both p/P
// polarities are cached, so this is the exact maximum number of valid keys.
func categorySetCacheCapacity() int {
	return 2 * (len(xsdCategoryNames) + len(xsdBlocks))
}

type categoryCacheKey struct {
	name string
	kind rune
}

type categorySetCache struct {
	values map[categoryCacheKey]rangeSet
}

func (c *categorySetCache) lookup(kind rune, name string) (rangeSet, bool) {
	if c.values == nil {
		return rangeSet{}, false
	}
	set, ok := c.values[categoryCacheKey{kind: kind, name: name}]
	return set, ok
}

func (c *categorySetCache) store(kind rune, name string, set rangeSet) {
	if c.values == nil {
		c.values = make(map[categoryCacheKey]rangeSet, categorySetCacheCapacity())
	}
	c.values[categoryCacheKey{kind: kind, name: name}] = set
}

func (p *parser) parse() (*node, error) {
	root, err := p.parseRegexp(0)
	if err != nil {
		return nil, err
	}
	if p.pos != len(p.source) {
		return nil, p.syntax("unexpected character")
	}
	return root, nil
}

func (p *parser) parseRegexp(end rune) (*node, error) {
	branches := make([]*node, 0, 1)
	for {
		branch, err := p.parseBranch(end)
		if err != nil {
			return nil, err
		}
		branches = append(branches, branch)
		if !p.take('|') {
			break
		}
	}
	return p.alt(branches...)
}

func (p *parser) parseBranch(end rune) (*node, error) {
	pieces := make([]*node, 0, 4)
	for p.pos < len(p.source) {
		r := p.source[p.pos]
		if r == '|' || (end != 0 && r == end) {
			break
		}
		piece, err := p.parsePiece()
		if err != nil {
			return nil, err
		}
		pieces = append(pieces, piece)
	}
	return p.concat(pieces...)
}

func (p *parser) parsePiece() (*node, error) {
	atom, err := p.parseAtom()
	if err != nil {
		return nil, err
	}
	if p.pos == len(p.source) {
		return atom, nil
	}
	switch p.source[p.pos] {
	case '?':
		p.pos++
		return p.repeat(atom, 0, 1, false)
	case '*':
		p.pos++
		return p.repeat(atom, 0, maxRepeat, true)
	case '+':
		p.pos++
		return p.repeat(atom, 1, maxRepeat, true)
	case '{':
		quantifier, err := p.parseCountedQuantifier()
		if err != nil {
			return nil, err
		}
		return p.repeat(atom, quantifier.min, quantifier.max, quantifier.unbounded)
	default:
		return atom, nil
	}
}

type countedQuantifier struct {
	min       uint64
	max       uint64
	unbounded bool
}

func (p *parser) parseCountedQuantifier() (countedQuantifier, error) {
	p.pos++ // {
	minCount, ok, err := p.parseDigits()
	if err != nil {
		return countedQuantifier{}, err
	}
	if !ok {
		return countedQuantifier{}, p.syntax("counted quantifier requires a lower bound")
	}
	if p.take('}') {
		return countedQuantifier{min: minCount, max: minCount}, nil
	}
	if !p.take(',') {
		return countedQuantifier{}, p.syntax("invalid counted quantifier")
	}
	if p.take('}') {
		return countedQuantifier{min: minCount, max: maxRepeat, unbounded: true}, nil
	}
	maxCount, ok, err := p.parseDigits()
	if err != nil {
		return countedQuantifier{}, err
	}
	if !ok || !p.take('}') {
		return countedQuantifier{}, p.syntax("invalid counted quantifier")
	}
	if maxCount < minCount {
		return countedQuantifier{}, p.syntax("counted quantifier upper bound is less than lower bound")
	}
	return countedQuantifier{min: minCount, max: maxCount}, nil
}

func (p *parser) parseDigits() (uint64, bool, error) {
	start := p.pos
	var value uint64
	for p.pos < len(p.source) {
		r := p.source[p.pos]
		if r < '0' || r > '9' {
			break
		}
		digit := uint64(r - '0')
		if value > (math.MaxUint64-digit)/10 {
			return 0, false, p.limit("counted quantifier overflows uint64")
		}
		value = value*10 + digit
		p.pos++
	}
	if p.pos == start {
		return 0, false, nil
	}
	return value, true, nil
}

func (p *parser) parseAtom() (*node, error) {
	if p.pos >= len(p.source) {
		return nil, p.syntax("missing atom")
	}
	r := p.source[p.pos]
	switch r {
	case '(':
		return p.parseGroupAtom()
	case '[':
		return p.parseClassAtom()
	case '.':
		return p.parseDotAtom()
	case '\\':
		return p.parseEscapeAtom()
	case '|', ')', '?', '*', '+', '{', '}', ']':
		return nil, p.syntax("unexpected metacharacter")
	default:
		return p.parseLiteralAtom()
	}
}

func (p *parser) parseGroupAtom() (*node, error) {
	p.pos++
	p.depth++
	if p.depth > p.limits.depth {
		return nil, p.limit("group nesting exceeds limit")
	}
	group, err := p.parseRegexp(')')
	if err != nil {
		return nil, err
	}
	if !p.take(')') {
		return nil, p.syntax("unclosed group")
	}
	p.depth--
	return group, nil
}

func (p *parser) parseClassAtom() (*node, error) {
	set, err := p.parseClass()
	if err != nil {
		return nil, err
	}
	return p.setNode(set)
}

func (p *parser) parseDotAtom() (*node, error) {
	p.pos++
	return p.setNode(subtractSets(xmlCharacters, setFromRanges([]runeRange{{lo: '\n', hi: '\n'}, {lo: '\r', hi: '\r'}})))
}

func (p *parser) parseEscapeAtom() (*node, error) {
	set, literal, err := p.parseEscape()
	if err != nil {
		return nil, err
	}
	if literal {
		return p.setNode(singletonSet(set.ranges[0].lo))
	}
	return p.setNode(set)
}

func (p *parser) parseLiteralAtom() (*node, error) {
	r := p.source[p.pos]
	p.pos++
	if !isXMLChar(r) {
		return nil, p.syntax("pattern contains a non-XML character")
	}
	return p.setNode(singletonSet(r))
}

func (p *parser) parseEscape() (rangeSet, bool, error) {
	p.pos++ // backslash
	if p.pos >= len(p.source) {
		return rangeSet{}, false, p.syntax("trailing escape")
	}
	r := p.source[p.pos]
	p.pos++
	if r == 'p' || r == 'P' {
		return p.parseCategoryEscape(r)
	}
	if set, ok := p.simpleSets[r]; ok {
		return set, false, nil
	}
	escape := parseSimpleEscape(r)
	if !escape.ok {
		return rangeSet{}, false, p.syntax("invalid escape")
	}
	if !escape.literal {
		if p.simpleSets == nil {
			p.simpleSets = make(map[rune]rangeSet, simpleSetCacheCapacity)
		}
		p.simpleSets[r] = escape.set
	}
	return escape.set, escape.literal, nil
}

type simpleEscape struct {
	set     rangeSet
	literal bool
	ok      bool
}

func parseSimpleEscape(r rune) (result simpleEscape) {
	switch r {
	case 'n':
		return simpleEscape{set: singletonSet('\n'), literal: true, ok: true}
	case 'r':
		return simpleEscape{set: singletonSet('\r'), literal: true, ok: true}
	case 't':
		return simpleEscape{set: singletonSet('\t'), literal: true, ok: true}
	case '\\', '|', '.', '?', '*', '+', '{', '}', '(', ')', '[', ']', '^', '-':
		return simpleEscape{set: singletonSet(r), literal: true, ok: true}
	case 'd':
		return simpleEscape{set: intersectSets(xsdDigitSet(), xmlCharacters), ok: true}
	case 'D':
		return simpleEscape{set: complementXML(xsdDigitSet()), ok: true}
	case 's':
		return simpleEscape{set: xsdSpaceSet(), ok: true}
	case 'S':
		return simpleEscape{set: complementXML(xsdSpaceSet()), ok: true}
	case 'w':
		return simpleEscape{set: xsdWordSet(), ok: true}
	case 'W':
		return simpleEscape{set: complementXML(xsdWordSet()), ok: true}
	case 'i':
		return simpleEscape{set: xmlNameStartChars, ok: true}
	case 'I':
		return simpleEscape{set: complementXML(xmlNameStartChars), ok: true}
	case 'c':
		return simpleEscape{set: intersectSets(xmlNameChars, xmlCharacters), ok: true}
	case 'C':
		return simpleEscape{set: complementXML(xmlNameChars), ok: true}
	default:
		return simpleEscape{}
	}
}

func (p *parser) parseCategoryEscape(kind rune) (rangeSet, bool, error) {
	if !p.take('{') {
		return rangeSet{}, false, p.syntax("category escape requires braces")
	}
	start := p.pos
	for p.pos < len(p.source) && p.source[p.pos] != '}' {
		p.pos++
	}
	if p.pos >= len(p.source) {
		return rangeSet{}, false, p.syntax("unclosed category escape")
	}
	name := string(p.source[start:p.pos])
	p.pos++
	if set, ok := p.categorySets.lookup(kind, name); ok {
		return set, false, nil
	}
	set, ok := namedCategorySet(name)
	if !ok {
		return rangeSet{}, false, p.syntax("unknown category or block " + name)
	}
	if kind == 'P' {
		set = complementXML(set)
	}
	set = intersectSets(set, xmlCharacters)
	p.categorySets.store(kind, name, set)
	return set, false, nil
}

func namedCategorySet(name string) (rangeSet, bool) {
	if len(name) >= 2 && name[:2] == "Is" {
		set, ok := xsdBlocks[name[2:]]
		return set, ok
	}
	return categorySet(name)
}

func (p *parser) parseClass() (rangeSet, error) {
	p.pos++ // [
	p.depth++
	if p.depth > p.limits.depth {
		return rangeSet{}, p.limit("character-class nesting exceeds limit")
	}
	polarity := classPositive
	if p.take('^') {
		polarity = classNegated
	}
	return p.parseClassBody(polarity, make([]classTerm, 0, 4))
}

func (p *parser) parseClassBody(polarity classPolarity, terms []classTerm) (rangeSet, error) {
	for {
		step, err := p.parseClassBodyStep(polarity, terms)
		if err != nil {
			return rangeSet{}, err
		}
		if step.done {
			return step.result, nil
		}
		terms = step.terms
	}
}

type classBodyStep struct {
	terms  []classTerm
	result rangeSet
	done   bool
}

func (p *parser) parseClassBodyStep(polarity classPolarity, terms []classTerm) (classBodyStep, error) {
	if p.pos >= len(p.source) {
		return classBodyStep{}, p.syntax("unclosed character class")
	}
	if p.source[p.pos] == ']' {
		result, err := p.finishClass(polarity, terms)
		return classBodyStep{result: result, done: true}, err
	}
	if p.source[p.pos] == '-' {
		return p.parseClassDash(polarity, terms)
	}
	position := classTermSubsequent
	if len(terms) == 0 {
		position = classTermFirst
	}
	term, err := p.parseClassTermAndRange(position)
	if err != nil {
		return classBodyStep{}, err
	}
	return classBodyStep{terms: append(terms, term)}, nil
}

func (p *parser) parseClassDash(polarity classPolarity, terms []classTerm) (classBodyStep, error) {
	if len(terms) != 0 && p.peek(1) == '[' {
		result, err := p.parseClassSubtraction(polarity, terms)
		return classBodyStep{result: result, done: true}, err
	}
	// I-011: an unescaped dash is a literal only as the first term of the
	// positive group, its last term, or its last term before `-[`. It never
	// starts a range, so `[--a]` is rejected rather than read as `-` to `a`.
	if len(terms) != 0 && p.peek(1) != ']' && !p.dashBeforeSubtraction(1) {
		return classBodyStep{}, p.syntax("unescaped dash must begin or end a character group")
	}
	p.pos++
	return classBodyStep{terms: append(terms, classTerm{set: singletonSet('-'), single: true})}, nil
}

func (p *parser) finishClass(polarity classPolarity, terms []classTerm) (rangeSet, error) {
	if len(terms) == 0 {
		return rangeSet{}, p.syntax("empty character class")
	}
	p.pos++
	p.depth--
	base := classTermsSet(terms)
	if polarity == classNegated {
		base = complementXML(base)
	}
	return base, nil
}

func (p *parser) parseClassSubtraction(polarity classPolarity, terms []classTerm) (rangeSet, error) {
	p.pos++
	subtracted, err := p.parseClass()
	if err != nil {
		return rangeSet{}, err
	}
	if p.pos >= len(p.source) || p.source[p.pos] != ']' {
		return rangeSet{}, p.syntax("character-class subtraction is not closed")
	}
	p.pos++
	p.depth--
	base := classTermsSet(terms)
	if polarity == classNegated {
		base = complementXML(base)
	}
	base = subtractSets(base, subtracted)
	return base, nil
}

// peek returns the rune offset positions past the cursor, or 0 past the end.
// A pattern rune is never 0 because 0 is not an XML character.
func (p *parser) peek(offset int) rune {
	if p.pos+offset >= len(p.source) {
		return 0
	}
	return p.source[p.pos+offset]
}

// dashBeforeSubtraction reports whether the runes at offset spell `-[`.
func (p *parser) dashBeforeSubtraction(offset int) bool {
	return p.peek(offset) == '-' && p.peek(offset+1) == '['
}

func (p *parser) parseClassTermAndRange(position classTermPosition) (classTerm, error) {
	term, err := p.parseClassTerm(position)
	if err != nil {
		return classTerm{}, err
	}
	// A dash followed by `]`, `[` or `-[` belongs to the group, not a range.
	if !term.single || p.peek(0) != '-' {
		return term, nil
	}
	if next := p.peek(1); next == 0 || next == ']' || next == '[' || p.dashBeforeSubtraction(1) {
		return term, nil
	}
	p.pos++
	end, err := p.parseClassRangeEnd()
	if err != nil {
		return classTerm{}, err
	}
	if end.set.ranges[0].lo < term.set.ranges[0].lo {
		return classTerm{}, p.syntax("descending character range")
	}
	term.set = setFromRanges([]runeRange{{lo: term.set.ranges[0].lo, hi: end.set.ranges[0].lo}})
	term.single = false
	return term, nil
}

type classTerm struct {
	set    rangeSet
	single bool
}

type classTermPosition uint8

const (
	classTermFirst classTermPosition = iota
	classTermSubsequent
)

type classPolarity uint8

const (
	classPositive classPolarity = iota
	classNegated
)

func (p *parser) parseClassTerm(position classTermPosition) (classTerm, error) {
	if p.pos >= len(p.source) {
		return classTerm{}, p.syntax("unclosed character class")
	}
	r := p.source[p.pos]
	switch r {
	case '^':
		if position == classTermFirst {
			return classTerm{}, p.syntax("invalid character-class negation")
		}
		p.pos++
		return classTerm{set: singletonSet('^'), single: true}, nil
	case '[':
		return classTerm{}, p.syntax("nested character class requires subtraction")
	case '\\':
		return p.parseEscapedClassTerm()
	case ']':
		return classTerm{}, p.syntax("empty character class")
	}
	return p.parseLiteralClassTerm(r)
}

func (p *parser) parseEscapedClassTerm() (classTerm, error) {
	set, single, err := p.parseEscape()
	if err != nil {
		return classTerm{}, err
	}
	return classTerm{set: set, single: single}, nil
}

func (p *parser) parseLiteralClassTerm(r rune) (classTerm, error) {
	p.pos++
	if !isXMLChar(r) {
		return classTerm{}, p.syntax("character class contains a non-XML character")
	}
	return classTerm{set: singletonSet(r), single: true}, nil
}

func (p *parser) parseClassRangeEnd() (classTerm, error) {
	if p.pos >= len(p.source) {
		return classTerm{}, p.syntax("unclosed character range")
	}
	if p.source[p.pos] == '\\' {
		term, err := p.parseClassTerm(classTermSubsequent)
		if err != nil {
			return classTerm{}, err
		}
		if !term.single {
			return classTerm{}, p.syntax("character-class range endpoint is not a single character")
		}
		return term, nil
	}
	r := p.source[p.pos]
	if r == '[' || r == ']' || r == '-' {
		return classTerm{}, p.syntax("invalid character-class range endpoint")
	}
	p.pos++
	if !isXMLChar(r) {
		return classTerm{}, p.syntax("character-class range contains a non-XML character")
	}
	return classTerm{set: singletonSet(r), single: true}, nil
}

func classTermsSet(terms []classTerm) rangeSet {
	sets := make([]rangeSet, len(terms))
	for i, term := range terms {
		sets[i] = term.set
	}
	return intersectSets(unionMany(sets...), xmlCharacters)
}

// setNode is the only entry of sets into the AST. Each backing array is
// charged once, by cap, because the compiled machine retains whole arrays:
// counted repeats and cached categories share one array, and NFA instructions
// copy the slice header. Subslices of one array at different offsets, and
// package-level sets such as xsdDecimalDigits, may be charged more than once;
// that over-count is deliberate and safe. Do not change it into an under-count.
func (p *parser) setNode(set rangeSet) (*node, error) {
	if cap(set.ranges) > 0 {
		key := &set.ranges[:1][0]
		if _, seen := p.seenRanges[key]; !seen {
			if p.seenRanges == nil {
				p.seenRanges = make(map[*runeRange]struct{})
			}
			p.seenRanges[key] = struct{}{}
			p.retainedRanges += uint64(cap(set.ranges))
			if p.retainedRanges > p.limits.retainedRanges {
				return nil, p.limit("pattern retained character-range count exceeds limit")
			}
		}
	}
	return p.newNode(nodeSet, set, nil, 0, 0)
}

func (p *parser) concat(children ...*node) (*node, error) {
	flat := make([]*node, 0, len(children))
	for _, child := range children {
		if child == nil || child.kind == nodeEmpty {
			continue
		}
		if child.kind == nodeConcat {
			flat = append(flat, child.children...)
		} else {
			flat = append(flat, child)
		}
	}
	if len(flat) == 0 {
		return p.newNode(nodeEmpty, rangeSet{}, nil, 0, 0)
	}
	if len(flat) == 1 {
		return flat[0], nil
	}
	return p.newNode(nodeConcat, rangeSet{}, flat, 0, 0)
}

func (p *parser) alt(children ...*node) (*node, error) {
	flat := make([]*node, 0, len(children))
	for _, child := range children {
		if child == nil {
			continue
		}
		if child.kind == nodeAlt {
			flat = append(flat, child.children...)
		} else {
			flat = append(flat, child)
		}
	}
	if len(flat) == 0 {
		return p.newNode(nodeEmpty, rangeSet{}, nil, 0, 0)
	}
	if len(flat) == 1 {
		return flat[0], nil
	}
	return p.newNode(nodeAlt, rangeSet{}, flat, 0, 0)
}

func (p *parser) repeat(child *node, minRepeat, maxRepeat uint64, unbounded bool) (*node, error) {
	if maxRepeat == 0 || child.kind == nodeEmpty {
		return p.newNode(nodeEmpty, rangeSet{}, nil, 0, 0)
	}
	if minRepeat == 1 && maxRepeat == 1 {
		return child, nil
	}
	repeat, err := p.newNode(nodeRepeat, rangeSet{}, nil, minRepeat, maxRepeat)
	if err != nil {
		return nil, err
	}
	repeat.children = []*node{child}
	repeat.unbounded = unbounded
	return repeat, nil
}

// newNode admits every AST constructor before allocating it, including nodes
// that concat or alt later flatten away, so MaxNodes bounds parser
// allocation. A failed admission stops parsing at the current rune offset.
func (p *parser) newNode(kind nodeKind, set rangeSet, children []*node, minRepeat, maxRepeat uint64) (*node, error) {
	if p.nodes >= p.limits.nodes {
		return nil, p.limit("compiled node count exceeds limit")
	}
	p.nodes++
	return &node{kind: kind, set: set, children: children, min: minRepeat, max: maxRepeat}, nil
}

func (p *parser) take(want rune) bool {
	if p.pos < len(p.source) && p.source[p.pos] == want {
		p.pos++
		return true
	}
	return false
}

func (p *parser) syntax(message string) error {
	return &Error{Kind: ErrorSyntax, Offset: p.pos, What: message}
}

func (p *parser) limit(message string) error {
	return &Error{Kind: ErrorLimit, Offset: p.pos, What: message}
}
