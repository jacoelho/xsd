package xsdregex

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

func mustCompileDFA(tb testing.TB, source string) *Pattern {
	tb.Helper()
	pattern := mustCompile(tb, source)
	if pattern.mode != modeDFA {
		tb.Fatalf("precondition: %q runs on the %s runner, want dfa", source, modeName(pattern))
	}
	return pattern
}

func matcherClass(a *alphabet, r rune) uint8 {
	if r < utf8.RuneSelf {
		return a.ascii[r]
	}
	return a.class(r)
}

// randomSets returns count sets of up to maxRanges ranges each. Range ends
// cluster on the encoding-length and page boundaries the alphabet indexes by.
func randomSets(random *rand.Rand, count, maxRanges int) [][]runeRange {
	edges := []rune{0, 0x3f, 0x40, 0x7f, 0x80, 0xbf, 0xc0, 0x7ff, 0x800, 0xffbf, 0xffc0, 0xffff, 0x10000, 0x1003f, 0x10040, utf8.MaxRune}
	point := func() rune {
		switch random.IntN(4) {
		case 0:
			return edges[random.IntN(len(edges))]
		case 1:
			return random.Int32N(0x800)
		case 2:
			return random.Int32N(0x10000)
		}
		return random.Int32N(utf8.MaxRune + 1)
	}
	sets := make([][]runeRange, count)
	for index := range sets {
		points := make([]rune, 2*(1+random.IntN(maxRanges)))
		for i := range points {
			points[i] = point()
		}
		slices.Sort(points)
		for i := 0; i+1 < len(points); i += 2 {
			if last := len(sets[index]) - 1; last >= 0 && sets[index][last].hi+1 >= points[i] {
				sets[index][last].hi = points[i+1]
				continue
			}
			sets[index] = append(sets[index], runeRange{lo: points[i], hi: points[i+1]})
		}
	}
	return sets
}

func TestAlphabetClassesAreExactlyTheDistinctSetMemberships(t *testing.T) {
	t.Parallel()
	for _, shape := range []struct{ sets, maxRanges int }{{1, 1}, {3, 4}, {8, 40}, {70, 3}, {5, 400}} {
		for seed := range uint64(3) {
			t.Run(fmt.Sprintf("%d sets of %d ranges, seed %d", shape.sets, shape.maxRanges, seed), func(t *testing.T) {
				t.Parallel()
				ranges := randomSets(rand.New(rand.NewPCG(seed, uint64(shape.sets))), shape.sets, shape.maxRanges) //nolint:gosec // A seeded generator makes the failing sets reproducible.
				sets := make([]rangeSet, len(ranges))
				membership := make([]string, utf8.MaxRune+1)
				row := make([]byte, len(ranges))
				for index, set := range ranges {
					sets[index] = rangeSet{ranges: set}
				}
				distinct := map[string]bool{}
				previous := ""
				for r := range membership {
					for index, set := range ranges {
						row[index] = '0'
						for _, current := range set {
							if int(current.lo) <= r && r <= int(current.hi) {
								row[index] = '1'
							}
						}
					}
					if string(row) != previous {
						previous = string(row)
						distinct[previous] = true
					}
					membership[r] = previous
				}

				budget := buildBudget{remaining: 1 << 40}
				got, ok := newClasses(sets, &budget)
				if len(distinct) > maxClasses {
					if ok {
						t.Fatalf("%d memberships fit %d classes", len(distinct), maxClasses)
					}
					return
				}
				if !ok {
					t.Fatalf("no alphabet for %d memberships", len(distinct))
				}
				if got.count != len(distinct) {
					t.Fatalf("classes = %d, want the %d distinct memberships", got.count, len(distinct))
				}
				paged := len(got.alphabet.pages) != 0
				if want := len(got.alphabet.runs) >= pagedRunsFrom; paged != want {
					t.Fatalf("alphabet of %d runs has pages = %v, want %v", len(got.alphabet.runs), paged, want)
				}
				for r, members := range membership {
					class := matcherClass(&got.alphabet, rune(r))
					for index := range sets {
						if in, want := slices.Contains(got.bySet[index], class), members[index] == '1'; in != want {
							t.Fatalf("U+%04X has class %d, which set %d lists = %v; the set contains it = %v", r, class, index, in, want)
						}
					}
				}
			})
		}
	}
}

func TestAlphabetRefusesMoreClassesThanAClassIDNames(t *testing.T) {
	t.Parallel()
	singletons := func(count int) []rangeSet {
		sets := make([]rangeSet, count)
		for index := range sets {
			sets[index] = singletonSet(rune(0x100 + 2*index))
		}
		return sets
	}
	budget := buildBudget{remaining: 1 << 40}
	if got, ok := newClasses(singletons(maxClasses-1), &budget); !ok || got.count != maxClasses {
		t.Fatalf("%d singletons and the rest: classes = %d, %v; want %d", maxClasses-1, got.count, ok, maxClasses)
	}
	if _, ok := newClasses(singletons(maxClasses), &budget); ok {
		t.Fatalf("%d classes fit a class id", maxClasses+1)
	}
}

func TestDFAMatchesAcrossEncodingLengthBoundaries(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		source  string
		paged   bool
		accepts []string
		rejects []string
	}{
		{"[!-\u007f]+", false, []string{"!", "\u007f", "a\u007f"}, []string{"\u0080", " ", "", "a\u0080"}},
		{"[\u0080-߿]+", false, []string{"\u0080", "߿", "\u0080߿"}, []string{"\u007f", "ࠀ", "\u0080\u007f"}},
		{"[ࠀ-�]+", false, []string{"ࠀ", "퟿", "", "�"}, []string{"߿", "￾", "\U00010000"}},
		{"[\U00010000-\U0010ffff]+", false, []string{"\U00010000", "\U0010ffff", "\U000fffff\U00100000"}, []string{"￿", "a"}},
		{"[a-c]|[à-â]", false, []string{"a", "c", "à", "â"}, []string{"d", "ß", "ã", "\U0001f600", "aà"}},
		{"[^aé中\U0001f600]{2}", false, []string{"bè", "丬\U0001f601", "ê丮"}, []string{"aè", "bé", "中b", "b\U0001f600", "b"}},
		{`\p{Lu}\p{Ll}+(\s\p{Lu}\p{Ll}+)*`, true, []string{"Ærø", "Łódź Ćirić", "Ωμέγα"}, []string{"ærø", "Łódź  Ćirić", "Ł", "数据"}},
		{`[\p{L}\p{N}\p{Zs},\.\-']{1,5}`, true, []string{"é", "数据交换格", "a b", "٣"}, []string{"", "数据交换格式", "a_b", "a\tb"}},
	} {
		pattern := mustCompileDFA(t, test.source)
		if paged := len(pattern.dfa.alphabet.pages) != 0; paged != test.paged {
			t.Fatalf("precondition: %q has %d runs and pages = %v, want %v", test.source, len(pattern.dfa.alphabet.runs), paged, test.paged)
		}
		for _, input := range test.accepts {
			if !matchEvery(t, test.source, input) {
				t.Errorf("%q rejects %q", test.source, input)
			}
		}
		for _, input := range test.rejects {
			if matchEvery(t, test.source, input) {
				t.Errorf("%q accepts %q", test.source, input)
			}
		}
	}
}

func TestDFAWorkIsInputBytesPlusRunesStepped(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		source, input string
		matched       bool
		bytes, runes  uint64
	}{
		{`[a-c]+`, "", false, 0, 0},
		{`[a-c]+`, "abc", true, 3, 3},
		{`[a-c]+`, "abxabc", false, 6, 3},
		{`[a-c]+`, "xabcabc", false, 7, 1},
		{`[a-c]+`, "abcabcx", false, 7, 7},
		{`[a-c]+`, "é", false, 2, 1},
		{`[à-ÿ]+x?`, "éèê", true, 6, 3},
		{`[à-ÿ]+x?`, "éxè", false, 5, 3},
		{`[à-ÿ]+x?`, "数é", false, 5, 1},
		{`(a|b)*c`, "abab", false, 4, 4},
		{`(a|b)*c`, "ababcab", false, 7, 6},
		{`[^a]{2}`, "\U0001f600\U0001f601", true, 8, 2},
		{`[^a]{2}`, "\U0001f600\U0001f601\U0001f602\U0001f603", false, 16, 3},
	} {
		pattern := mustCompileDFA(t, test.source)
		want := test.bytes + test.runes
		for name, match := range matchers() {
			matched, work, err := match(pattern, test.input, unlimitedWork, nil)
			if err != nil || matched != test.matched || work != want {
				t.Errorf("%s %q on %q = %v, %d, %v; want %v, %d (%d bytes and %d runes)", name, test.source, test.input, matched, work, err, test.matched, want, test.bytes, test.runes)
			}
			if want == 0 {
				continue
			}
			matched, work, err = match(pattern, test.input, MatchOptions{MaxWork: want}, nil)
			if err != nil || matched != test.matched || work != want {
				t.Errorf("%s %q on %q with exactly its work = %v, %d, %v; want %v, %d", name, test.source, test.input, matched, work, err, test.matched, want)
			}
			if matched, _, err := match(pattern, test.input, MatchOptions{MaxWork: want - 1}, nil); !IsLimit(err) || matched {
				t.Errorf("%s %q on %q with one unit less = %v, %v; want false and a limit", name, test.source, test.input, matched, err)
			}
		}
	}
}

func TestDFAMatchesDoNotAllocate(t *testing.T) {
	for _, test := range []struct{ source, input string }{
		{`[A-Z]{2}\d{4}`, "AB1234"},
		{`[\p{L}\p{N}\p{Zs},\.\-']{1,70}`, "Müller Straße, Ærø Łódź 数据"},
		{`(a|b)*c`, strings.Repeat("ab", 400) + "x"},
	} {
		pattern := mustCompileDFA(t, test.source)
		raw := []byte(test.input)
		if allocations := testing.AllocsPerRun(100, func() {
			if _, _, err := pattern.MatchStringWithScratch(test.input, unlimitedWork, nil); err != nil {
				t.Fatal(err)
			}
		}); allocations != 0 {
			t.Errorf("%q: string match allocated %v times", test.source, allocations)
		}
		if allocations := testing.AllocsPerRun(100, func() {
			if _, _, err := pattern.MatchBytesWithScratch(raw, unlimitedWork, nil); err != nil {
				t.Fatal(err)
			}
		}); allocations != 0 {
			t.Errorf("%q: byte match allocated %v times", test.source, allocations)
		}
	}
}

// boundedCount is `[a-c]{3}`: four unrolled states, and a table of the dead
// row, one row per count from zero to three, and two classes.
const (
	boundedCount        = `[a-c]{3}`
	boundedCountStates  = 4
	boundedCountEntries = 5 * 2
)

func assertBoundedCountLanguage(t *testing.T, label string, pattern *Pattern) {
	t.Helper()
	for input, want := range map[string]bool{"": false, "ab": false, "abc": true, "cba": true, "abd": false, "abca": false, "ébc": false} {
		for name, match := range matchers() {
			if matched, _, err := match(pattern, input, unlimitedWork, nil); err != nil || matched != want {
				t.Errorf("%s: %s %q on %q (runner %s) = %v, %v; want %v", label, name, boundedCount, input, modeName(pattern), matched, err, want)
			}
		}
	}
}

func TestDFACapsSelectAnotherRunnerForTheSameLanguage(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		adjust func(*compileLimits)
		dfa    bool
	}{
		{"fixed bounds", func(*compileLimits) {}, true},
		{"states at the bound", func(l *compileLimits) { l.dfaStates = boundedCountStates }, true},
		{"states over the bound", func(l *compileLimits) { l.dfaStates = boundedCountStates - 1 }, false},
		{"entries at the bound", func(l *compileLimits) { l.dfaEntries = boundedCountEntries }, true},
		{"entries over the bound", func(l *compileLimits) { l.dfaEntries = boundedCountEntries - 1 }, false},
		{"no steps", func(l *compileLimits) { l.dfaSteps = 0 }, false},
		{"entries beyond what a row offset holds", func(l *compileLimits) { l.dfaEntries = maxDFAEntries + 1 }, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			limits := fixedCompileLimits
			test.adjust(&limits)
			pattern, err := compileWithLimits(boundedCount, limits)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			if got := pattern.mode == modeDFA; got != test.dfa {
				t.Fatalf("runner %s, want a DFA = %v", modeName(pattern), test.dfa)
			}
			if test.dfa && len(pattern.dfa.next) != boundedCountEntries {
				t.Errorf("table has %d entries, want %d", len(pattern.dfa.next), boundedCountEntries)
			}
			assertBoundedCountLanguage(t, test.name, pattern)
		})
	}
}

func TestDFAStepBudgetOnlyDecidesWhetherATableIsBuilt(t *testing.T) {
	t.Parallel()
	for _, source := range []string{boundedCount, `(a|ab)*c`, `[\p{L}-[a-z]]{1,3}`, `(a|b)*a(a|b){3}`} {
		want := compilations(t, source)["without DFA"]
		inputs := []string{"", "abc", "ab", "aababc", "c", "Éé", "ÉÀÜ", "abab", "aaaa", "baab", "abbb", "数"}
		built := false
		for steps := uint64(0); steps <= fixedCompileLimits.dfaSteps; steps += 1 + steps/16 {
			limits := fixedCompileLimits
			limits.dfaSteps = steps
			pattern, err := compileWithLimits(source, limits)
			if err != nil {
				t.Fatalf("%q with %d steps: %v", source, steps, err)
			}
			if pattern.mode != modeDFA {
				if built {
					t.Fatalf("%q: no table with %d steps, yet one with fewer", source, steps)
				}
				continue
			}
			built = true
			for _, input := range inputs {
				expected, _, err := want.MatchStringWithScratch(input, unlimitedWork, nil)
				if err != nil {
					t.Fatal(err)
				}
				if matched, _, err := pattern.MatchStringWithScratch(input, unlimitedWork, nil); err != nil || matched != expected {
					t.Fatalf("%q with %d steps on %q = %v, %v; the %s runner says %v", source, steps, input, matched, err, modeName(want), expected)
				}
			}
		}
		if !built {
			t.Errorf("%q: no table within the fixed %d steps", source, fixedCompileLimits.dfaSteps)
		}
	}
}

func TestAnalysisStepsAreTheSmallestStepBoundThatBuildsTheTable(t *testing.T) {
	t.Parallel()
	for _, source := range []string{`[a-c]{3}`, `(a|ab)*c`, `[\p{L}-[a-z]]{1,3}`, `[a-z]{1,2048}`} {
		steps := mustCompileDFA(t, source).AnalysisSteps()
		for range 8 {
			if again := mustCompile(t, source).AnalysisSteps(); again != steps {
				t.Fatalf("%q: AnalysisSteps() = %d, then %d", source, steps, again)
			}
		}
		limits := fixedCompileLimits
		limits.dfaSteps = steps
		exact, err := compileWithLimits(source, limits)
		if err != nil || exact.mode != modeDFA {
			t.Fatalf("%q under its own %d steps: runner %s, %v; want a table", source, steps, modeName(exact), err)
		}
		limits.dfaSteps = steps - 1
		short, err := compileWithLimits(source, limits)
		if err != nil || short.mode == modeDFA {
			t.Fatalf("%q under %d steps: runner %s, %v; want no table", source, steps-1, modeName(short), err)
		}
		if got := short.AnalysisSteps(); got >= steps {
			t.Errorf("%q abandoned under %d steps: AnalysisSteps() = %d, want fewer than %d", source, steps-1, got, steps)
		}
	}
}

func TestAnalysisStepsCountAbandonedAttemptsAndNoneWithoutAnAttempt(t *testing.T) {
	t.Parallel()
	for _, source := range []string{`abc`, ``, `[a-z]{60000}[a-z]{60000}`, `(a|b){9000}`} {
		pattern := mustCompile(t, source)
		if pattern.mode == modeDFA {
			t.Fatalf("%q compiled to a table", source)
		}
		if got := pattern.AnalysisSteps(); got != 0 {
			t.Errorf("%q: AnalysisSteps() = %d, want 0 for a pattern Compile attempts no table for", source, got)
		}
	}
	for _, source := range []string{`(a|b)*a(a|b){12}`, `(a?){1,2000}`, `[a-c]{8190}`} {
		pattern := mustCompile(t, source)
		if pattern.mode == modeDFA {
			t.Fatalf("%q compiled to a table", source)
		}
		if got := pattern.AnalysisSteps(); got == 0 || got > fixedCompileLimits.dfaSteps {
			t.Errorf("%q: AnalysisSteps() = %d, want between 1 and %d for an abandoned attempt", source, got, fixedCompileLimits.dfaSteps)
		}
	}
}

func TestDFARowOffsetsHoldAtTheLargestTable(t *testing.T) {
	t.Parallel()
	const count = maxDFAEntries/2 - 2
	limits := fixedCompileLimits
	limits.dfaStates, limits.dfaEntries, limits.dfaSteps = maxDFAEntries, maxDFAEntries, 1<<30
	pattern, err := compileWithLimits(fmt.Sprintf(`[a-c]{%d}`, count), limits)
	if err != nil {
		t.Fatal(err)
	}
	if pattern.mode != modeDFA || len(pattern.dfa.next) != maxDFAEntries {
		t.Fatalf("precondition: runner %s with %d entries, want a table of %d", modeName(pattern), len(pattern.dfa.next), maxDFAEntries)
	}
	for _, test := range []struct {
		runes int
		tail  string
		want  bool
	}{{count, "", true}, {count - 1, "", false}, {count + 1, "", false}, {count - 1, "x", false}, {count - 1, "c", true}} {
		input := strings.Repeat("abc", count/3+1)[:test.runes] + test.tail
		for name, match := range matchers() {
			if matched, _, err := match(pattern, input, unlimitedWork, nil); err != nil || matched != test.want {
				t.Errorf("%s: %d runes and %q = %v, %v; want %v", name, test.runes, test.tail, matched, err, test.want)
			}
		}
	}
}

func TestDFABuilderAbandonsRowsWhoseMembersHashAlike(t *testing.T) {
	t.Parallel()
	members, other := []uint32{1, 2}, []uint32{7}
	builder := func() *dfaBuilder {
		return &dfaBuilder{
			rows:        map[uint64]uint16{hashMembers(members): 1},
			memberFirst: []uint32{0, 0, 1},
			members:     slices.Clone(other),
			accepting:   []bool{false, false},
			classes:     classes{count: 2},
			budget:      buildBudget{remaining: 1 << 10},
			entries:     1 << 10,
		}
	}
	if row, ok := builder().row(members); ok {
		t.Fatalf("members %v took row %d, whose members are %v", members, row, other)
	}
	same := builder()
	same.members = slices.Clone(members)
	same.memberFirst[2] = 2
	if row, ok := same.row(members); !ok || row != 1 {
		t.Fatalf("members %v: row %d, %v; want their own row 1", members, row, ok)
	}
}

func TestExponentialDFAsCompileToAnotherRunner(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		format  string
		other   string
		classes uint64
	}{
		{`(a|b)*a(a|b){%d}`, "b", 3},
		{`([a-m]|[n-z]|\d)*[a-m]([a-m]|[n-z]|\d){%d}`, "7", 4},
	} {
		for _, tail := range []int{12, 40, 400} {
			source := fmt.Sprintf(test.format, tail)
			if tail < 60 && test.classes<<(tail+1) <= fixedCompileLimits.dfaEntries {
				t.Fatalf("precondition: %q fits the entry bound", source)
			}
			pattern := mustCompile(t, source)
			if pattern.mode == modeDFA {
				t.Fatalf("%q compiled to a table of %d entries", source, len(pattern.dfa.next))
			}
			for input, want := range map[string]bool{
				"a" + strings.Repeat(test.other, tail):                true,
				test.other + "a" + strings.Repeat("a", tail):          true,
				test.other + strings.Repeat("a", tail):                false,
				"a" + strings.Repeat(test.other, tail-1):              false,
				"a" + test.other + strings.Repeat(test.other, tail-1): true,
				"a" + strings.Repeat(test.other, tail) + "-":          false,
			} {
				if matched, _, err := pattern.MatchStringWithScratch(input, unlimitedWork, nil); err != nil || matched != want {
					t.Errorf("%q (runner %s) on %.12q... (%d bytes) = %v, %v; want %v", source, modeName(pattern), input, len(input), matched, err, want)
				}
			}
		}
	}
}
