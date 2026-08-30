package runtime

import (
	"errors"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// StringPattern is a compiled string pattern matcher used during validation.
type StringPattern struct {
	re   *regexp.Regexp
	fast *SimplePattern
}

type stringPatternSteps struct {
	tail *stringPatternStep
}

type stringPatternStep struct {
	parent   *stringPatternStep
	patterns []StringPattern
	count    uint32
}

type stringPatternStepRead struct {
	parent   *stringPatternStepRead
	patterns []stringPatternRead
	count    uint32
}

type stringPatternRead struct {
	re   *regexp.Regexp
	fast *SimplePattern
}

func appendStringPatternStep(steps stringPatternSteps, patterns []StringPattern) stringPatternSteps {
	count := uint32(1)
	if steps.tail != nil {
		count = steps.tail.count + 1
	}
	return stringPatternSteps{tail: &stringPatternStep{parent: steps.tail, patterns: patterns, count: count}}
}

// AppendPatternFacetGroup appends one immutable pattern derivation step.
func AppendPatternFacetGroup(f *FacetSet, patterns []StringPattern) {
	f.patterns = appendStringPatternStep(f.patterns, patterns)
	SetFacetPresent(f, FacetPattern)
}

func newStringPatternReadPoolForSimpleTypes(types []SimpleType) map[*stringPatternStep]*stringPatternStepRead {
	hint := stringPatternReadPoolHint(types)
	if hint == 0 {
		return nil
	}
	sources, patternCount, pool := collectStringPatternReadSources(types, hint)
	return buildStringPatternReadPool(sources, patternCount, pool)
}

func stringPatternReadPoolHint(types []SimpleType) int {
	hint := 0
	for i := range types {
		tail := types[i].Facets.patterns.tail
		if tail == nil {
			continue
		}
		if uint64(tail.count) > uint64(math.MaxInt) {
			panic("string pattern read step count exceeds int capacity")
		}
		hint = max(hint, int(tail.count))
	}
	return hint
}

func collectStringPatternReadSources(
	types []SimpleType,
	hint int,
) ([]*stringPatternStep, int, map[*stringPatternStep]*stringPatternStepRead) {
	sources := make([]*stringPatternStep, 0, hint)
	missing := make([]*stringPatternStep, 0, min(hint, 1_024))
	pool := make(map[*stringPatternStep]*stringPatternStepRead, hint)
	patternCount := 0
	for i := range types {
		missing = collectMissingStringPatternSteps(types[i].Facets.patterns.tail, pool, missing[:0])
		addStringPatternReadCount(len(sources), len(missing))
		for _, step := range slices.Backward(missing) {
			patternCount = addStringPatternReadCount(patternCount, len(step.patterns))
			sources = append(sources, step)
		}
	}
	return sources, patternCount, pool
}

func collectMissingStringPatternSteps(
	step *stringPatternStep,
	pool map[*stringPatternStep]*stringPatternStepRead,
	missing []*stringPatternStep,
) []*stringPatternStep {
	for ; step != nil; step = step.parent {
		if _, ok := pool[step]; ok {
			break
		}
		pool[step] = nil
		missing = append(missing, step)
	}
	return missing
}

func buildStringPatternReadPool(
	sources []*stringPatternStep,
	patternCount int,
	pool map[*stringPatternStep]*stringPatternStepRead,
) map[*stringPatternStep]*stringPatternStepRead {
	reads := make([]stringPatternStepRead, len(sources))
	patterns := make([]stringPatternRead, patternCount)
	fastCopies := make(map[*SimplePattern]*SimplePattern)
	regexpCopies := make(map[*regexp.Regexp]*regexp.Regexp)
	patternOffset := 0
	for i, source := range sources {
		stepPatterns := projectStringPatternReads(source.patterns, patterns, &patternOffset, fastCopies, regexpCopies)
		reads[i] = stringPatternStepRead{
			parent:   pool[source.parent],
			patterns: stepPatterns,
			count:    source.count,
		}
		pool[source] = &reads[i]
	}
	return pool
}

func projectStringPatternReads(
	source []StringPattern,
	patterns []stringPatternRead,
	offset *int,
	fastCopies map[*SimplePattern]*SimplePattern,
	regexpCopies map[*regexp.Regexp]*regexp.Regexp,
) []stringPatternRead {
	if len(source) == 0 {
		return nil
	}
	end := *offset + len(source)
	reads := patterns[*offset:end:end]
	*offset = end
	for i, pattern := range source {
		reads[i] = newStringPatternRead(pattern, fastCopies, regexpCopies)
	}
	return reads
}

func newStringPatternRead(
	pattern StringPattern,
	fastCopies map[*SimplePattern]*SimplePattern,
	regexpCopies map[*regexp.Regexp]*regexp.Regexp,
) stringPatternRead {
	if pattern.fast != nil {
		if fast := fastCopies[pattern.fast]; fast != nil {
			return stringPatternRead{fast: fast}
		}
		fast := &SimplePattern{
			atoms:    slices.Clone(pattern.fast.atoms),
			variable: pattern.fast.variable,
		}
		for i := range fast.atoms {
			fast.atoms[i].class.ranges = slices.Clone(pattern.fast.atoms[i].class.ranges)
		}
		fastCopies[pattern.fast] = fast
		return stringPatternRead{fast: fast}
	}
	if re := regexpCopies[pattern.re]; re != nil {
		return stringPatternRead{re: re}
	}
	// Validation only asks whether the expression matches, so recompiling the
	// same source preserves its language without sharing Longest's mutable flag.
	re := regexp.MustCompile(pattern.re.String())
	regexpCopies[pattern.re] = re
	return stringPatternRead{re: re}
}

func addStringPatternReadCount(total, count int) int {
	if count > math.MaxInt-total {
		panic("string pattern read projection size exceeds int capacity")
	}
	return total + count
}

func (s stringPatternSteps) count() uint32 {
	if s.tail == nil {
		return 0
	}
	return s.tail.count
}

func validateStringPatternSourcesForSimpleTypes(types []SimpleType) error {
	var validated map[*stringPatternStep]struct{}
	for i := range types {
		tail := types[i].Facets.patterns.tail
		if tail == nil {
			continue
		}
		var err error
		validated, err = validateStringPatternSource(tail, validated)
		if err != nil {
			return err
		}
	}
	return nil
}

func validateStringPatternSource(
	tail *stringPatternStep,
	validated map[*stringPatternStep]struct{},
) (map[*stringPatternStep]struct{}, error) {
	step := tail
	for walked := uint32(0); step != nil; walked++ {
		if _, ok := validated[step]; ok {
			break
		}
		if invalidStringPatternStepLink(tail, step, walked) {
			return nil, errors.New("simple type pattern facet chain is invalid")
		}
		if err := validateStringPatternStepShape(step); err != nil {
			return nil, err
		}
		if validated == nil {
			validated = make(map[*stringPatternStep]struct{})
		}
		validated[step] = struct{}{}
		step = step.parent
	}
	return validated, nil
}

func invalidStringPatternStepLink(tail, step *stringPatternStep, walked uint32) bool {
	return walked >= tail.count || step.count == 0 ||
		(step.parent == nil) != (step.count == 1) ||
		step.parent != nil && step.parent.count+1 != step.count
}

func validateStringPatternStepShape(step *stringPatternStep) error {
	if len(step.patterns) == 0 {
		return errors.New("simple type pattern facet group has no patterns")
	}
	for _, pattern := range step.patterns {
		if (pattern.fast == nil) == (pattern.re == nil) {
			return errors.New("simple type pattern facet has invalid matcher")
		}
	}
	return nil
}

func (p stringPatternRead) matchString(s string) bool {
	if p.fast != nil {
		return p.fast.MatchString(s)
	}
	return p.re.MatchString(s)
}

func (p stringPatternRead) matchStringWithScratch(s string, input *simplePatternInput, scratch *StringPatternScratch) bool {
	if scratch == nil {
		return p.matchString(s)
	}
	if p.fast != nil {
		if p.fast.variable && len(s) > smallPatternRunes {
			return p.fast.matchVariableRunesWithScratch(input.stringRunes(scratch), scratch)
		}
		return p.fast.MatchString(s)
	}
	return p.re.MatchString(s)
}

func (p stringPatternRead) matchBytes(s []byte) bool {
	if p.fast != nil {
		return p.fast.MatchBytes(s)
	}
	return p.re.Match(s)
}

func (p stringPatternRead) matchBytesWithScratch(s []byte, input *simplePatternInput, scratch *StringPatternScratch) bool {
	if scratch == nil {
		return p.matchBytes(s)
	}
	if p.fast != nil {
		if p.fast.variable && input.byteNeedsScratch() {
			return p.fast.matchVariableRunesWithScratch(input.byteRunes(scratch), scratch)
		}
		return p.fast.MatchBytes(s)
	}
	return p.re.Match(s)
}

// NewFastStringPattern returns a pattern backed by the runtime fast matcher.
func NewFastStringPattern(fast *SimplePattern) StringPattern {
	return StringPattern{fast: fast}
}

// NewRegexpStringPattern returns a pattern backed by a Go regexp.
func NewRegexpStringPattern(re *regexp.Regexp) StringPattern {
	return StringPattern{re: re}
}

// MatchString reports whether s matches p.
func (p StringPattern) MatchString(s string) bool {
	if p.fast != nil {
		return p.fast.MatchString(s)
	}
	return p.re.MatchString(s)
}

func equalSimplePattern(a, b *SimplePattern) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.variable != b.variable || len(a.atoms) != len(b.atoms) {
		return false
	}
	for i := range a.atoms {
		aa, ba := a.atoms[i], b.atoms[i]
		if aa.min != ba.min || aa.max != ba.max || aa.class.digit != ba.class.digit ||
			!slices.Equal(aa.class.ranges, ba.class.ranges) {
			return false
		}
	}
	return true
}

// SimplePattern is a small compiled subset of XSD regex syntax.
type SimplePattern struct {
	atoms    []simplePatternAtom
	variable bool
}

// StringPatternScratch owns reusable scalar buffers for published string
// pattern validation. It must not be shared by concurrent validations.
type StringPatternScratch struct {
	runes  []rune
	states []bool
}

// Reset clears scratch state and drops buffers larger than maxRetainedRunes.
func (s *StringPatternScratch) Reset(maxRetainedRunes int) {
	if s == nil {
		return
	}
	if maxRetainedRunes < 0 || cap(s.runes) > maxRetainedRunes {
		s.runes = nil
	} else {
		s.runes = s.runes[:0]
	}
	maxStates := 0
	if maxRetainedRunes >= 0 && maxRetainedRunes <= (math.MaxInt-2)/2 {
		maxStates = 2 * (maxRetainedRunes + 1)
	}
	if maxRetainedRunes < 0 || maxStates == 0 || cap(s.states) > maxStates {
		s.states = nil
	} else {
		s.states = s.states[:0]
	}
}

// simplePatternInput scopes decoded input reuse to one pattern facet chain.
// It never survives the validation call that owns the input.
type simplePatternInput struct {
	text      string
	bytes     []byte
	runes     []rune
	runeCount int
	prepared  bool
	counted   bool
}

func (i *simplePatternInput) stringRunes(scratch *StringPatternScratch) []rune {
	if i.prepared {
		return i.runes
	}
	if cap(scratch.runes) < len(i.text) {
		runeCount := utf8.RuneCountInString(i.text)
		if cap(scratch.runes) < runeCount {
			scratch.runes = make([]rune, 0, runeCount)
		} else {
			scratch.runes = scratch.runes[:0]
		}
	} else {
		scratch.runes = scratch.runes[:0]
	}
	text := i.text
	for text != "" {
		r, size := utf8.DecodeRuneInString(text)
		scratch.runes = append(scratch.runes, r)
		text = text[size:]
	}
	i.runes = scratch.runes
	i.prepared = true
	return i.runes
}

func (i *simplePatternInput) byteRuneCount() int {
	if !i.counted {
		i.runeCount = countUTF8Runes(i.bytes)
		i.counted = true
	}
	return i.runeCount
}

func (i *simplePatternInput) byteNeedsScratch() bool {
	if len(i.bytes) > smallPatternRunes*utf8.UTFMax {
		return true
	}
	return i.byteRuneCount() > smallPatternRunes
}

func (i *simplePatternInput) byteRunes(scratch *StringPatternScratch) []rune {
	if i.prepared {
		return i.runes
	}
	if cap(scratch.runes) < len(i.bytes) {
		runeCount := i.byteRuneCount()
		if cap(scratch.runes) < runeCount {
			scratch.runes = make([]rune, 0, runeCount)
		} else {
			scratch.runes = scratch.runes[:0]
		}
	} else {
		scratch.runes = scratch.runes[:0]
	}
	raw := i.bytes
	for len(raw) > 0 {
		r, size := utf8.DecodeRune(raw)
		scratch.runes = append(scratch.runes, r)
		raw = raw[size:]
	}
	i.runes = scratch.runes
	i.prepared = true
	return i.runes
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
		repeat := parseSimplePatternQuantifier(source, next)
		if !repeat.valid {
			return nil
		}
		if repeat.min != repeat.max {
			out.variable = true
		}
		out.atoms = append(out.atoms, simplePatternAtom{class: class, min: repeat.min, max: repeat.max})
		i = repeat.next
	}
	return &out
}

type simplePatternRepeat struct {
	min   int
	max   int
	next  int
	valid bool
}

func parseSimplePatternQuantifier(source string, position int) simplePatternRepeat {
	if position >= len(source) || source[position] != '{' {
		return simplePatternRepeat{min: 1, max: 1, next: position, valid: true}
	}
	return parseSimplePatternRepeat(source, position)
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
		parsedRange, next, ok := parseSimplePatternClassItem(source, i)
		if !ok {
			return simplePatternClass{}, 0, false
		}
		class.ranges = append(class.ranges, parsedRange)
		i = next
	}
	return simplePatternClass{}, 0, false
}

func parseSimplePatternClassItem(source string, position int) (runeRange, int, bool) {
	lo, next, ok := parseSimplePatternClassRune(source, position)
	if !ok {
		return runeRange{}, 0, false
	}
	if next >= len(source) || source[next] != '-' || next+1 >= len(source) || source[next+1] == ']' {
		return runeRange{lo: lo, hi: lo}, next, true
	}
	hi, after, ok := parseSimplePatternClassRune(source, next+1)
	if !ok || hi < lo {
		return runeRange{}, 0, false
	}
	return runeRange{lo: lo, hi: hi}, after, true
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

func parseSimplePatternRepeat(source string, i int) simplePatternRepeat {
	end := strings.IndexByte(source[i:], '}')
	if end < 0 {
		return simplePatternRepeat{}
	}
	end += i
	body := source[i+1 : end]
	if body == "" {
		return simplePatternRepeat{}
	}
	lower, upper, found := strings.Cut(body, ",")
	if lower == "" {
		return simplePatternRepeat{}
	}
	repeatMin, err := strconv.Atoi(lower)
	if err != nil || repeatMin < 0 {
		return simplePatternRepeat{}
	}
	if !found {
		return simplePatternRepeat{min: repeatMin, max: repeatMin, next: end + 1, valid: true}
	}
	if upper == "" {
		return simplePatternRepeat{min: repeatMin, max: simplePatternUnbounded, next: end + 1, valid: true}
	}
	repeatMax, err := strconv.Atoi(upper)
	if err != nil || repeatMax < repeatMin {
		return simplePatternRepeat{}
	}
	return simplePatternRepeat{min: repeatMin, max: repeatMax, next: end + 1, valid: true}
}

// MatchString reports whether s matches p.
func (p *SimplePattern) MatchString(s string) bool {
	if p.variable {
		return p.matchVariableString(s)
	}
	i := 0
	for _, atom := range p.atoms {
		if !matchFixedStringAtom(atom, s, &i) {
			return false
		}
	}
	return i == len(s)
}

func matchFixedStringAtom(atom simplePatternAtom, input string, position *int) bool {
	for range atom.min {
		if *position >= len(input) {
			return false
		}
		r, size := utf8.DecodeRuneInString(input[*position:])
		if r == utf8.RuneError && size == 0 {
			return false
		}
		if !atom.class.matches(r) {
			return false
		}
		*position += size
	}
	return true
}

// MatchBytes reports whether s matches p.
func (p *SimplePattern) MatchBytes(s []byte) bool {
	if p.variable {
		return p.matchVariableBytes(s)
	}
	i := 0
	for _, atom := range p.atoms {
		if !matchFixedByteAtom(atom, s, &i) {
			return false
		}
	}
	return i == len(s)
}

func matchFixedByteAtom(atom simplePatternAtom, input []byte, position *int) bool {
	for range atom.min {
		if *position >= len(input) {
			return false
		}
		r, size := utf8.DecodeRune(input[*position:])
		if r == utf8.RuneError && size == 0 {
			return false
		}
		if !atom.class.matches(r) {
			return false
		}
		*position += size
	}
	return true
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
	if n := countUTF8Runes(s); n > len(stack) {
		runes = make([]rune, 0, n)
	}
	for len(s) > 0 {
		r, size := utf8.DecodeRune(s)
		runes = append(runes, r)
		s = s[size:]
	}
	return p.matchVariableRunes(runes)
}

func countUTF8Runes(s []byte) int {
	count := 0
	for len(s) > 0 {
		_, size := utf8.DecodeRune(s)
		s = s[size:]
		count++
	}
	return count
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
	return p.matchVariableRunesWithBuffer(runes, buf)
}

func (p *SimplePattern) matchVariableRunesWithBuffer(runes []rune, buf []bool) bool {
	runeCount := len(runes)
	prev, next := buf[:runeCount+1], buf[runeCount+1:]
	clear(prev)
	prev[0] = true
	for _, atom := range p.atoms {
		if !matchVariablePatternAtom(runes, prev, next, atom) {
			return false
		}
		prev, next = next, prev
	}
	return prev[runeCount]
}

func matchVariablePatternAtom(runes []rune, prev, next []bool, atom simplePatternAtom) bool {
	clear(next)
	if atom.min == 0 {
		copy(next, prev)
	}
	if atom.max != 0 {
		minRepeat := max(atom.min, 1)
		markMatchingRuneRuns(runes, prev, next, atom.class, minRepeat, atom.max)
	}
	return hasReachableOffset(next)
}

func markMatchingRuneRuns(
	runes []rune,
	prev, next []bool,
	class simplePatternClass,
	minRepeat, maxRepeat int,
) {
	start := 0
	for start < len(runes) {
		for start < len(runes) && !class.matches(runes[start]) {
			start++
		}
		runStart := start
		for start < len(runes) && class.matches(runes[start]) {
			start++
		}
		markRepeatedRun(prev, next, runStart, start, minRepeat, maxRepeat)
	}
}

func (p *SimplePattern) matchVariableRunesWithScratch(runes []rune, scratch *StringPatternScratch) bool {
	size := 2 * (len(runes) + 1)
	if cap(scratch.states) < size {
		scratch.states = make([]bool, size)
	} else {
		scratch.states = scratch.states[:size]
	}
	return p.matchVariableRunesWithBuffer(runes, scratch.states)
}

func markRepeatedRun(prev, next []bool, start, end, minRepeat, maxRepeat int) {
	if end-start < minRepeat {
		return
	}
	if maxRepeat == simplePatternUnbounded {
		markUnboundedRepeatedRun(prev, next, start, end, minRepeat)
		return
	}
	markBoundedRepeatedRun(prev, next, start, end, minRepeat, maxRepeat)
}

func markUnboundedRepeatedRun(prev, next []bool, start, end, minRepeat int) {
	active := 0
	for pos := start + minRepeat; pos <= end; pos++ {
		if prev[pos-minRepeat] {
			active++
		}
		if active > 0 {
			next[pos] = true
		}
	}
}

func markBoundedRepeatedRun(prev, next []bool, start, end, minRepeat, maxRepeat int) {
	active := 0
	for pos := start + minRepeat; pos <= end; pos++ {
		if prev[pos-minRepeat] {
			active++
		}
		remove := pos - maxRepeat - 1
		if remove >= start && prev[remove] {
			active--
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
	case runeInRange(r, 0x0030, 0x0039),
		runeInRange(r, 0x0660, 0x0669),
		runeInRange(r, 0x06F0, 0x06F9),
		runeInRange(r, 0x0966, 0x096F),
		runeInRange(r, 0x09E6, 0x09EF),
		runeInRange(r, 0x0A66, 0x0A6F),
		runeInRange(r, 0x0AE6, 0x0AEF),
		runeInRange(r, 0x0B66, 0x0B6F),
		runeInRange(r, 0x0BE7, 0x0BEF),
		runeInRange(r, 0x0C66, 0x0C6F),
		runeInRange(r, 0x0CE6, 0x0CEF),
		runeInRange(r, 0x0D66, 0x0D6F),
		runeInRange(r, 0x0E50, 0x0E59),
		runeInRange(r, 0x0ED0, 0x0ED9),
		runeInRange(r, 0x0F20, 0x0F29),
		runeInRange(r, 0x1040, 0x1049),
		runeInRange(r, 0x1369, 0x1371),
		runeInRange(r, 0x17E0, 0x17E9),
		runeInRange(r, 0x1810, 0x1819),
		runeInRange(r, 0x1D7CE, 0x1D7FF),
		runeInRange(r, 0xFF10, 0xFF19):
		return true
	default:
		return false
	}
}

func runeInRange(r, lower, upper rune) bool {
	return r >= lower && r <= upper
}
