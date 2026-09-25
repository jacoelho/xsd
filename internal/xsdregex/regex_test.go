package xsdregex

import (
	"slices"
	"strings"
	"testing"
)

func mustCompile(t *testing.T, source string) *Pattern {
	t.Helper()
	pattern, err := Compile(source, CompileOptions{})
	if err != nil {
		t.Fatalf("Compile(%q) error = %v", source, err)
	}
	return pattern
}

func TestWholeStringAndLiteralAnchors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		{"abc", "abc", true},
		{"abc", "xabc", false},
		{"abc", "abcx", false},
		{"^abc$", "^abc$", true},
		{"^abc$", "abc", false},
		{`a.b`, "a\nb", false},
		{`a.b`, "a\rb", false},
		{`a.b`, "axb", true},
		{`a\.b`, "a.b", true},
		{`a\.b`, "axb", false},
	}
	for _, test := range tests {
		t.Run(test.pattern+"/"+test.input, func(t *testing.T) {
			t.Parallel()
			got, err := mustCompile(t, test.pattern).MatchString(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("MatchString(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestEscapesAndShorthands(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		{`\d+`, "१२३", true},
		{`\d+`, "12a", false},
		{`\s+`, " \t\n\r", true},
		{`\s+`, "\u00a0", false},
		{`\i\c*`, "Name_2", true},
		{`\i\c*`, "2Name", false},
		{`\c+`, "a-b.c", true},
		{`\c+`, "a b", false},
		{`\w+`, "abc123", true},
		{`\w+`, "a-b", false},
		{`\W+`, "!", true},
		{`\p{Lu}+`, "ABC", true},
		{`\p{Lu}+`, "Abc", false},
		{`\P{Lu}+`, "abc!", true},
		{`\P{Lu}+`, "ABC", false},
	}
	for _, test := range tests {
		t.Run(test.pattern+"/"+test.input, func(t *testing.T) {
			t.Parallel()
			got, err := mustCompile(t, test.pattern).MatchString(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("MatchString(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestUnicodeVersionForDecimalDigitEscapes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		{`\d`, "፱", true},     // U+1371 was Nd in the XSD Unicode data.
		{`\d`, "௦", false},    // U+0BE6 was not assigned in that data.
		{`\p{Nd}`, "𝟿", true}, // U+1D7FF is part of the same Nd table.
		{`\D`, "௦", true},
		{`\D`, "፱", false},
	}
	for _, test := range tests {
		p := mustCompile(t, test.pattern)
		got, err := p.MatchString(test.input)
		if err != nil {
			t.Fatalf("%s.MatchString(%q): %v", test.pattern, test.input, err)
		}
		if got != test.want {
			t.Errorf("%s.MatchString(%q) = %v, want %v", test.pattern, test.input, got, test.want)
		}
	}
}

func TestXMLCharactersExtendThroughUnicodeMaxRune(t *testing.T) {
	t.Parallel()
	for _, pattern := range []string{`[\I]+`, `[\C]+`} {
		p := mustCompile(t, pattern)
		got, err := p.MatchString("\U000F0000\U0010FFFE")
		if err != nil || !got {
			t.Errorf("%s.MatchString(high XML characters) = %v, %v; want true", pattern, got, err)
		}
	}
}

func TestBlocksAndSubtraction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		{`\p{IsBasicLatin}+`, "abc123", true},
		{`\p{IsBasicLatin}+`, "é", false},
		{`\p{IsGreek}+`, "Ωλ", true},
		{`[a-z-[aeiou]]+`, "rhythm", true},
		{`[a-z-[aeiou]]+`, "reader", false},
		{`[\d-[357]]+`, "124689", true},
		{`[\d-[357]]+`, "123", false},
		{`[^a-z-[aeiou]]+`, "123!", true},
		{`[^a-z-[aeiou]]+`, "rhythm", false},
		{`[a-z-[a-z-[aeiou]]]+`, "aeiou", true},
	}
	for _, test := range tests {
		t.Run(test.pattern+"/"+test.input, func(t *testing.T) {
			t.Parallel()
			got, err := mustCompile(t, test.pattern).MatchString(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("MatchString(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestUnicodeBlocksAndXMLNameBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		{`\p{IsDeseret}+`, "𐐀𐐁", true},
		{`\p{IsDeseret}+`, "abc", false},
		{`\p{IsCJKUnifiedIdeographsExtensionB}`, "𠀀", true},
		{`\p{IsCJKUnifiedIdeographsExtensionB}`, "一", false},
		{`\i\c*`, ":prefix-·name", true},
		{`\i\c*`, "prefix͆", true},
		{`\i\c*`, "prefix‿", true},
		{`\i\c*`, ".prefix", false},
		{`\i\c*`, "prefix space", false},
	}
	for _, test := range tests {
		t.Run(test.pattern+"/"+test.input, func(t *testing.T) {
			t.Parallel()
			got, err := mustCompile(t, test.pattern).MatchString(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("MatchString(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestCountedRepeatsBeyondGoLimit(t *testing.T) {
	t.Parallel()
	p := mustCompile(t, `0{1001}`)
	got, err := p.MatchString(strings.Repeat("0", 1001))
	if err != nil || !got {
		t.Fatalf("exact repeat error = %v, matched = %v", err, got)
	}
	got, err = p.MatchString(strings.Repeat("0", 1000))
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Fatal("exact repeat did not match")
	}
	p = mustCompile(t, `a{1001,}`)
	got, err = p.MatchString(strings.Repeat("a", 2_000))
	if err != nil || !got {
		t.Fatalf("open repeat error = %v, matched = %v", err, got)
	}
}

func TestCountedRepeatIsNotExpanded(t *testing.T) {
	t.Parallel()
	if _, err := Compile(`a{18446744073709551615}`, CompileOptions{}); !IsLimit(err) {
		t.Fatalf("huge exact repeat error = %v, want limit", err)
	}
	if _, err := Compile(`a{18446744073709551616}`, CompileOptions{}); !IsLimit(err) {
		t.Fatalf("overflowing repeat error = %v, want limit", err)
	}
}

func TestExactMaxUint64RepeatIsNotTheUnboundedForm(t *testing.T) {
	t.Parallel()
	parseRepeat := func(t *testing.T, source string) *node {
		t.Helper()
		p := parser{source: []rune(source), limits: normalizeCompileOptions(CompileOptions{})}
		root, err := p.parse()
		if err != nil {
			t.Fatal(err)
		}
		if root.kind != nodeRepeat {
			t.Fatalf("%q root kind = %d, want repeat", source, root.kind)
		}
		return root
	}
	exact := parseRepeat(t, `a{18446744073709551615}`)
	open := parseRepeat(t, `a{18446744073709551615,}`)
	if exact.max != maxRepeat || exact.unbounded {
		t.Fatalf("exact max repeat = (%d, unbounded=%v), want max uint64 and bounded", exact.max, exact.unbounded)
	}
	if open.max != maxRepeat || !open.unbounded {
		t.Fatalf("open max repeat = (%d, unbounded=%v), want max uint64 and unbounded", open.max, open.unbounded)
	}
	child := &node{kind: nodeSet, set: singletonSet('a')}
	exactNode := &node{kind: nodeRepeat, children: []*node{child}, min: 1, max: maxRepeat}
	if _, err := compileNFA(exactNode, CompileOptions{MaxStates: 3}); !IsLimit(err) {
		t.Fatalf("exact max repeat compilation error = %v, want limit", err)
	}
	openNode := &node{kind: nodeRepeat, children: []*node{child}, min: 1, max: maxRepeat, unbounded: true}
	if _, err := compileNFA(openNode, CompileOptions{MaxStates: 4}); err != nil {
		t.Fatalf("unbounded repeat compilation error = %v", err)
	}
}

func TestSyntaxErrors(t *testing.T) {
	t.Parallel()
	for _, source := range []string{
		`a{,2}`, `a{2,1}`, `a{2`, `a{2,x}`, `a\`, `(`, `a)`, `a]`, `[`, `[]`, `[^]`, `[a-z`, `[a-z-[ae]`, `[a-\d]`, `[\d-z]`, `\p{Nope}`, `\p{IsNope}`, `*a`, `a**`,
	} {
		if _, err := Compile(source, CompileOptions{}); !IsSyntax(err) {
			t.Errorf("Compile(%q) error = %v, want syntax", source, err)
		}
	}
}

func TestCharacterClassLiteralCaretAndDashSubtraction(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		{`[a^]`, "a", true},
		{`[a^]`, "^", true},
		{`[a^]`, "b", false},
		{`[^a-z^]`, "^", false},
		{`[^a-z^]`, "!", true},
		{`[a-z--[b-z]]`, "a", true},
		{`[a-z--[b-z]]`, "b", false},
		{`[a-z--[b-z]]`, "-", true},
	}
	for _, test := range tests {
		p := mustCompile(t, test.pattern)
		got, err := p.MatchString(test.input)
		if err != nil {
			t.Fatalf("%s.MatchString(%q): %v", test.pattern, test.input, err)
		}
		if got != test.want {
			t.Errorf("%s.MatchString(%q) = %v, want %v", test.pattern, test.input, got, test.want)
		}
	}
}

func TestCharacterClassCaretRangeEndpoints(t *testing.T) {
	t.Parallel()
	tests := []struct {
		pattern string
		input   string
		want    bool
	}{
		{`[A-^]`, "A", true},
		{`[A-^]`, "^", true},
		{`[A-^]`, "@", false},
		{`[A-^]`, "_", false},
		{`[A-\^]`, "A", true},
		{`[A-\^]`, "^", true},
		{`[A-\^]`, "@", false},
		{`[A-\^]`, "_", false},
		{`[^A-^]`, "@", true},
		{`[^A-^]`, "A", false},
		{`[^A-^]`, "^", false},
		{`[A-^-[M]]`, "A", true},
		{`[A-^-[M]]`, "M", false},
		{`[A-^-[M]]`, "^", true},
	}
	for _, test := range tests {
		t.Run(test.pattern+"/"+test.input, func(t *testing.T) {
			t.Parallel()
			got, err := mustCompile(t, test.pattern).MatchString(test.input)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("MatchString(%q) = %v, want %v", test.input, got, test.want)
			}
		})
	}
}

func TestSurrogateBlockEscapesAreExcluded(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"HighSurrogates", "LowSurrogates", "HighPrivateUseSurrogates"} {
		for _, polarity := range []string{"p", "P"} {
			for _, pattern := range []string{
				"\\" + polarity + "{Is" + name + "}",
				"[\\" + polarity + "{Is" + name + "}]",
			} {
				t.Run(pattern, func(t *testing.T) {
					t.Parallel()
					if _, err := Compile(pattern, CompileOptions{}); !IsSyntax(err) {
						t.Fatalf("Compile(%q) error = %v, want syntax error", pattern, err)
					}
				})
			}
		}
	}
}

func TestMatchLimits(t *testing.T) {
	t.Parallel()
	linear := mustCompile(t, `a*`)
	if _, err := linear.MatchStringWithOptions("aa", MatchOptions{MaxStates: 2}); !IsLimit(err) {
		t.Fatalf("linear state limit error = %v, want limit", err)
	}
	if _, err := mustCompile(t, `abc`).MatchStringWithOptions("abc", MatchOptions{MaxWork: 2}); !IsLimit(err) {
		t.Fatalf("literal work limit error = %v, want limit", err)
	}
	p := mustCompile(t, `(a|aa)*b`)
	if _, err := p.MatchStringWithOptions(strings.Repeat("a", 2_000)+"b", MatchOptions{MaxWork: 10, MaxStates: 10}); !IsLimit(err) {
		t.Fatalf("bounded match error = %v, want limit", err)
	}
}

func TestCompileRangeLimitAppliesToEveryCharacterSet(t *testing.T) {
	t.Parallel()
	if _, err := Compile("a", CompileOptions{MaxRanges: 1}); err != nil {
		t.Fatalf("single-range literal rejected at limit: %v", err)
	}
	if _, err := Compile("[a-z]", CompileOptions{MaxRanges: 1}); err != nil {
		t.Fatalf("single-range class rejected at limit: %v", err)
	}
	for _, source := range []string{"\\d", `\p{L}`, ".", "[a-zA-Z]"} {
		if _, err := Compile(source, CompileOptions{MaxRanges: 1}); !IsLimit(err) {
			t.Fatalf("Compile(%q) error = %v, want range limit", source, err)
		}
	}
}

func TestParserCachesRepeatedSetEscapes(t *testing.T) {
	t.Parallel()
	p := parser{source: []rune(`\p{L}\p{L}\w\w`), limits: normalizeCompileOptions(CompileOptions{})}
	root, err := p.parse()
	if err != nil {
		t.Fatal(err)
	}
	if root.kind != nodeConcat || len(root.children) != 4 {
		t.Fatalf("parsed root = kind %d with %d children, want four concatenated sets", root.kind, len(root.children))
	}
	if !sharesRangeStorage(root.children[0].set, root.children[1].set) {
		t.Fatal("repeated category escapes did not share immutable ranges")
	}
	if !sharesRangeStorage(root.children[2].set, root.children[3].set) {
		t.Fatal("repeated shorthand escapes did not share immutable ranges")
	}
	if len(p.categorySets.values) != 1 || len(p.simpleSets) != 1 {
		t.Fatalf("cache entries = categories %d, shorthands %d; want one each", len(p.categorySets.values), len(p.simpleSets))
	}
}

func TestParserCategoryCacheCoversClosedVocabulary(t *testing.T) {
	t.Parallel()
	names := make([]string, 0, len(xsdCategoryNames)+len(xsdBlocks))
	for name := range xsdCategoryNames {
		names = append(names, name)
	}
	for name := range xsdBlocks {
		names = append(names, "Is"+name)
	}
	slices.Sort(names)
	var source strings.Builder
	source.WriteString(`\P{L}`)
	firstPositiveL := -1
	for i, name := range names {
		if name == "L" {
			firstPositiveL = i + 1
		}
		source.WriteString(`\p{`)
		source.WriteString(name)
		source.WriteByte('}')
	}
	source.WriteString(`\P{L}\p{L}`)
	p := parser{source: []rune(source.String()), limits: normalizeCompileOptions(CompileOptions{})}
	root, err := p.parse()
	if err != nil {
		t.Fatal(err)
	}
	if root.kind != nodeConcat || len(root.children) != len(names)+3 {
		t.Fatalf("parsed root = kind %d with %d children, want %d concatenated sets", root.kind, len(root.children), len(names)+3)
	}
	if firstPositiveL < 0 {
		t.Fatal("category catalog does not contain L")
	}
	if !sharesRangeStorage(root.children[0].set, root.children[len(names)+1].set) {
		t.Fatal("negative category cache evicted a valid key")
	}
	if !sharesRangeStorage(root.children[firstPositiveL].set, root.children[len(names)+2].set) {
		t.Fatal("positive category cache evicted a valid key")
	}
	if len(p.categorySets.values) != len(names)+1 {
		t.Fatalf("category cache entries = %d, want %d", len(p.categorySets.values), len(names)+1)
	}
}

func sharesRangeStorage(a, b rangeSet) bool {
	return len(a.ranges) != 0 && len(a.ranges) == len(b.ranges) && &a.ranges[0] == &b.ranges[0]
}

func TestMatchLimitsBoundScratchAndAdversarialClosure(t *testing.T) {
	t.Parallel()
	p := mustCompile(t, `(a|aa)*b`)
	var scratch Scratch
	if _, err := p.MatchStringWithScratch("", MatchOptions{MaxWork: 1}, &scratch); !IsLimit(err) {
		t.Fatalf("initial closure with tiny work budget = %v, want limit", err)
	}
	if _, err := p.MatchStringWithScratch(strings.Repeat("a", 2_000)+"b", MatchOptions{MaxStates: 1}, &scratch); !IsLimit(err) {
		t.Fatalf("active state set with tiny state budget = %v, want limit", err)
	}
}

func TestScratchResetClearsNFAGenerationMarkers(t *testing.T) {
	p := mustCompile(t, `\d{1,5}\s([A-Z][a-z]{1,20}\s){4}Street\n([A-Z][a-z]{1,20}\s){2},\s[A-Z]{2}\s12848`)
	inputs := []string{
		"27951 Frameworks Library Them Objects Street\nRegard As , DC 12848",
		"7 Of Typical To Original Street\nIn Prominent , AK 12848",
		"5367 Bandwidth To Oasis Based Street\nCommunication Popular , NM 12848",
		"4 Many Different Means Of Street\nFiles In , OR 12848",
		"64 Those Soc Full Software Street\nChain Is , MT 12848",
	}
	var scratch Scratch
	for i, input := range inputs {
		if i != 0 {
			scratch.Reset(4096)
		}
		matched, err := p.MatchStringWithScratch(input, MatchOptions{}, &scratch)
		if err != nil || !matched {
			t.Fatalf("input %d match = %t, %v; want true, nil", i+1, matched, err)
		}
	}
}

func TestMatchBytes(t *testing.T) {
	t.Parallel()
	p := mustCompile(t, `\p{L}+`)
	got, err := p.MatchBytes([]byte("héllo"))
	if err != nil || !got {
		t.Fatalf("MatchBytes valid input error = %v, matched = %v", err, got)
	}
}

func TestLinearMatchStateLimitCountsRunes(t *testing.T) {
	t.Parallel()
	p := mustCompile(t, `é*`)
	input := strings.Repeat("é", 129)
	options := MatchOptions{MaxStates: 130}
	for name, match := range map[string]func() (bool, error){
		"string": func() (bool, error) { return p.MatchStringWithOptions(input, options) },
		"bytes":  func() (bool, error) { return p.MatchBytesWithOptions([]byte(input), options) },
	} {
		got, err := match()
		if err != nil || !got {
			t.Errorf("%s match at rune state limit = %v, %v; want true, nil", name, got, err)
		}
	}
	tooMany := strings.Repeat("é", 130)
	if _, err := p.MatchStringWithOptions(tooMany, options); !IsLimit(err) {
		t.Fatalf("string match over rune state limit error = %v, want limit", err)
	}
	if _, err := p.MatchBytesWithOptions([]byte(tooMany), options); !IsLimit(err) {
		t.Fatalf("bytes match over rune state limit error = %v, want limit", err)
	}
}

func TestShortLinearMatchesDoNotAllocate(t *testing.T) {
	pattern := mustCompile(t, `[A-Z]{2}\d{4}`)
	for _, input := range []string{"AB1234", "AB12345"} {
		t.Run(input, func(t *testing.T) {
			raw := []byte(input)
			want := input == "AB1234"
			var matched bool
			var err error
			allocations := testing.AllocsPerRun(100, func() {
				var scratch Scratch
				matched, err = pattern.MatchStringWithScratch(input, MatchOptions{}, &scratch)
			})
			if err != nil || matched != want {
				t.Fatalf("string match = %t, %v; want %t, nil", matched, err, want)
			}
			if allocations != 0 {
				t.Fatalf("short string match allocated %v times, want 0", allocations)
			}
			allocations = testing.AllocsPerRun(100, func() {
				var scratch Scratch
				matched, err = pattern.MatchBytesWithScratch(raw, MatchOptions{}, &scratch)
			})
			if err != nil || matched != want {
				t.Fatalf("byte match = %t, %v; want %t, nil", matched, err, want)
			}
			if allocations != 0 {
				t.Fatalf("short byte match allocated %v times, want 0", allocations)
			}
		})
	}
}
