package xsdregex

import (
	"fmt"
	"strings"
	"testing"
)

// runLengthSets have no range or several, with members and non-members on
// both sides of U+0040 and of U+0080.
var runLengthSets = []string{
	"[a-[a]]",
	"[?A-Z]",
	"[@\u007f]",
	"[~\u0080-ÿ]",
	"[^<>]",
	"[\t\n a]",
	`[\p{L}\p{N}\p{Zs},]`,
	`[0-9a-zA-Z/\-\?:\(\)\.\n\r,'\+ ]`,
}

// runLengthFollower follows a set under test, so that where a run of the
// set's members ends inside a value decides the match.
const runLengthFollower = "[<>@~\u0080Ā]"

var runLengthProbes = []rune{'\t', '\n', ' ', ',', '0', '<', '=', '>', '?', '@', 'A', 'Z', 'a', '~', '\u007f', '\u0080', 'ÿ', 'Ā', 'é', '数', '\U00010000'}

// runLengthInputs returns runs of each probe, on both sides of the 128 bytes
// the matcher decodes on the stack, with every probe after and before the run.
func runLengthInputs() []string {
	var inputs []string
	for _, probe := range runLengthProbes {
		for _, count := range []int{0, 1, 62, 126, 127, 128, 199} {
			run := strings.Repeat(string(probe), count)
			for _, other := range runLengthProbes {
				inputs = append(inputs, run+string(other), string(other)+run, run+string(other)+string(other)+string(probe))
			}
		}
	}
	return inputs
}

func mustCompileRunLength(t *testing.T, source string) *Pattern {
	t.Helper()
	pattern := mustCompile(t, source)
	if pattern.mode != modeLinear || pattern.linear.literal {
		t.Fatalf("%q: runner %s, want the run-length matcher", source, modeName(pattern))
	}
	return pattern
}

// matchEveryWay matches input through both entry points, with scratch and
// without, fails the test unless all agree, and returns their answer.
func matchEveryWay(t *testing.T, pattern *Pattern, input string) bool {
	t.Helper()
	var answers []bool
	for name, match := range matchers() {
		for _, scratch := range []*Scratch{nil, {}} {
			matched, _, err := match(pattern, input, unlimitedWork, scratch)
			if err != nil {
				t.Fatalf("%s %q on %q: %v", name, pattern.Source(), input, err)
			}
			answers = append(answers, matched)
		}
	}
	for _, answer := range answers {
		if answer != answers[0] {
			t.Fatalf("%q on %q: entry points disagree: %v", pattern.Source(), input, answers)
		}
	}
	return answers[0]
}

// A bound of 9000 is over the table's state bound and above every input
// length here, so each pattern accepts what its unbounded twin's table does.
func TestRunLengthSetsAcceptWhatTheirTableTwinAccepts(t *testing.T) {
	t.Parallel()
	inputs := runLengthInputs()
	shapes := []struct{ name, runLength, table string }{
		{"alone", `%s{1,9000}`, `%s+`},
		{"followed", `%s{1,9000}` + runLengthFollower + `{0,9000}`, `%s+` + runLengthFollower + `*`},
		{"following", runLengthFollower + `{0,9000}%s{1,9000}`, runLengthFollower + `*%s+`},
	}
	for _, set := range runLengthSets {
		for _, shape := range shapes {
			t.Run(set+"/"+shape.name, func(t *testing.T) {
				t.Parallel()
				bounded := mustCompileRunLength(t, fmt.Sprintf(shape.runLength, set))
				if bounded.linear.fixed {
					t.Fatalf("%q: exact counts, want bounded ones", bounded.Source())
				}
				twin := mustCompileDFA(t, fmt.Sprintf(shape.table, set))
				for _, input := range inputs {
					want := matchEveryWay(t, twin, input)
					if got := matchEveryWay(t, bounded, input); got != want {
						t.Fatalf("%q on %+q = %v, the table of %q says %v", bounded.Source(), input, got, twin.Source(), want)
					}
				}
			})
		}
	}
}

func TestFixedRunLengthSetsAcceptWhatTheirTableTwinAccepts(t *testing.T) {
	t.Parallel()
	tail := strings.Repeat("a", 9000)
	for _, set := range runLengthSets {
		t.Run(set, func(t *testing.T) {
			t.Parallel()
			fixed := mustCompileRunLength(t, set+`{2}[ab]{9000}`)
			if !fixed.linear.fixed {
				t.Fatalf("%q: bounded counts, want exact ones", fixed.Source())
			}
			twin := mustCompileDFA(t, set+`{2}`)
			for _, first := range runLengthProbes {
				for _, second := range runLengthProbes {
					head := string(first) + string(second)
					want := matchEveryWay(t, twin, head)
					if got := matchEveryWay(t, fixed, head+tail); got != want {
						t.Fatalf("%q on %U %U and 9000 a = %v, the table of %q says %v", fixed.Source(), first, second, got, twin.Source(), want)
					}
				}
			}
		})
	}
}
