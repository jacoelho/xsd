package xsdregex

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
)

var repeatLanguageBodies = []string{
	`a`, `ab`, `a|aa`, `a?`, `a*`, `[ab]+`, `a?b?`, `a|ab`, `b|a?`, `[ab]{1,2}`, `a{0,2}`, `ab|a`, `(a|b)(a|b)?`,
}

var repeatLanguageQuantifiers = []string{
	`{0,1}`, `{0,3}`, `{1,2}`, `{1,6}`, `{0,40}`, `{1,70}`, `{2}`, `{3}`, `{5}`, `{2,4}`, `{3,40}`, `{0,}`, `{2,}`, `{4,}`, `{5,9}`,
}

var repeatLanguageWrappers = []string{`%s`, `(%s)*`, `a(%s)b?`, `(%s){1,3}`, `((%s)b){0,4}a?`, `(%s)|b{2,30}`, `((%s)b?){2,6}`}

func repeatLanguageSources(body string) []string {
	var sources []string
	for _, quantifier := range repeatLanguageQuantifiers {
		for _, wrapper := range repeatLanguageWrappers {
			sources = append(sources, fmt.Sprintf(wrapper, "("+body+")"+quantifier))
		}
	}
	return sources
}

func mustParse(t *testing.T, source string) *node {
	t.Helper()
	p := parser{source: []rune(source), limits: fixedCompileLimits}
	root, err := p.parse()
	if err != nil {
		t.Fatalf("parse %q: %v", source, err)
	}
	return root
}

// repeatLanguageInputs holds every string over {a,b} up to length 8 and runs
// of a, optionally framed by b, long enough to cross every bound above.
func repeatLanguageInputs() []string {
	inputs := []string{""}
	for length := 1; length <= 8; length++ {
		for bits := range 1 << length {
			var b strings.Builder
			for i := range length {
				b.WriteByte("ab"[bits>>i&1])
			}
			inputs = append(inputs, b.String())
		}
	}
	for n := 9; n <= 150; n += 1 + n/40 {
		run := strings.Repeat("a", n)
		inputs = append(inputs, run, "b"+run, run+"b", run+"ab")
	}
	return inputs
}

func TestCountedRepeatsAcceptSameLanguageAsGoRegexp(t *testing.T) {
	t.Parallel()
	inputs := repeatLanguageInputs()
	for _, body := range repeatLanguageBodies {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			assertRepeatLanguage(t, body, inputs)
		})
	}
}

func assertRepeatLanguage(t *testing.T, body string, inputs []string) {
	t.Helper()
	runners := map[string]int{}
	for _, source := range repeatLanguageSources(body) {
		oracle, err := regexp.Compile(`^(?:` + source + `)$`)
		if err != nil {
			t.Fatalf("regexp.Compile(%q): %v", source, err)
		}
		for label, pattern := range compilations(t, source) {
			runners[modeName(pattern)]++
			if pattern.counted != nil {
				runners["counted"]++
			}
			for name, match := range matchers() {
				for _, input := range inputs {
					matched, _, err := match(pattern, input, unlimitedWork, nil)
					if err != nil {
						t.Fatalf("%s %s %q on %q: %v", label, name, source, input, err)
					}
					if want := oracle.MatchString(input); matched != want {
						t.Fatalf("%s %s %q (runner %s) on %q: matched = %v, want %v", label, name, source, modeName(pattern), input, matched, want)
					}
				}
			}
		}
	}
	for _, runner := range []string{"dfa", "bitset", "general", "counted"} {
		if runners[runner] == 0 {
			t.Errorf("body %q: no pattern ran on the %s runner", body, runner)
		}
	}
}

func TestCountedMachineTracksAtMostThreeTimesUnrolledStates(t *testing.T) {
	t.Parallel()
	large := []string{`(a|aa){1,19000}`, `([a-z]+[0-9]){1,19000}`, `(a?){2,24000}`, `((ab){2}c?){1,9000}`, `(a|b)*a(a|b){1,24000}`}
	for _, source := range large {
		if mustCompileWithoutDFA(t, source).counted == nil {
			t.Fatalf("%q: no counter", source)
		}
	}
	sources := large
	for _, body := range repeatLanguageBodies {
		sources = append(sources, repeatLanguageSources(body)...)
	}
	for _, source := range sources {
		unrolled := newRepeatPlan(mustParse(t, source)).unrolled
		if tracked := uint64(mustCompileWithoutDFA(t, source).trackedSlots()); tracked > 3*unrolled { //nolint:gosec // tracked slots are bounded by the compile limit.
			t.Errorf("%q: %d tracked states, want at most 3 * %d unrolled", source, tracked, unrolled)
		}
	}
}

func TestRepeatPlanUnrolledCountEqualsUnrolledMachine(t *testing.T) {
	t.Parallel()
	sources := []string{`a`, `(a|b){33000}`, `((ab){2}c){1,40}`, `(a{2,5}|b?){0,7}c*`, `()`, `(|a)`, `a{0}`, `(a{3}){2,}`}
	for _, body := range repeatLanguageBodies {
		sources = append(sources, repeatLanguageSources(body)...)
	}
	for _, source := range sources {
		root := mustParse(t, source)
		c := nfaCompiler{limit: fixedCompileLimits.states}
		if _, err := c.compileRoot(root); err != nil {
			t.Fatalf("compile %q: %v", source, err)
		}
		if got, want := newRepeatPlan(root).unrolled, uint64(len(c.states)); got != want {
			t.Errorf("%q: plan counts %d unrolled states, compiler emits %d", source, got, want)
		}
	}
}
