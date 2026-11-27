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

// runLengthFollower follows or precedes a set under test, so that where a run
// of the set's members ends inside a value decides the match.
const runLengthFollower = "[<>@~\u0080Ā]"

// runLengthShapes each take one set. All compile to a table and, without the
// table, to the run-length matcher.
var runLengthShapes = []struct {
	format string
	exact  bool
}{
	{`%s+`, false},
	{`%s+` + runLengthFollower + `*`, false},
	{runLengthFollower + `*%s+`, false},
	{`%s{2}`, true},
}

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

func TestRunLengthMatcherAcceptsWhatTheTableAccepts(t *testing.T) {
	t.Parallel()
	inputs := runLengthInputs()
	for _, set := range runLengthSets {
		for _, shape := range runLengthShapes {
			source := fmt.Sprintf(shape.format, set)
			t.Run(source, func(t *testing.T) {
				t.Parallel()
				runLength := mustCompileWithoutDFA(t, source)
				if runLength.mode != modeLinear || runLength.linear.fixed != shape.exact {
					t.Fatalf("precondition: %q without a table runs on the %s runner, want linear with exact counts %v", source, modeName(runLength), shape.exact)
				}
				patterns := map[string]*Pattern{"table": mustCompileDFA(t, source), "run-length": runLength}
				for _, input := range inputs {
					matchAgreed(t, source, input, patterns)
				}
			})
		}
	}
}
