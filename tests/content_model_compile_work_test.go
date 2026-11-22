//go:build !(386 || arm || mips || mipsle || wasm)

package tests_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	compileWorkDriver  = "github.com/jacoelho/xsd/tests/internal/compilework"
	compileWorkPackage = "github.com/jacoelho/xsd/internal/schema"
	// compileWorkGrowthLimit lies between the growth with every mapping-search
	// read charged (0.99) and the growth of the cheapest counted skip over
	// mapped base particles, one if statement per skipped particle (1.95).
	compileWorkGrowthLimit = 1.4
)

type compileWorkSample struct {
	work, steps int
}

func (s compileWorkSample) minus(o compileWorkSample) compileWorkSample {
	return compileWorkSample{work: s.work - o.work, steps: s.steps - o.steps}
}

func (s compileWorkSample) perStep() float64 {
	return float64(s.work) / float64(s.steps)
}

// sequenceRestrictingAllSchema returns a base type whose content is an all
// group of particles optional elements, and restrictors types that each
// restrict it by a sequence of the same elements, required, in base order.
func sequenceRestrictingAllSchema(restrictors, particles int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:t" targetNamespace="urn:t">`)
	b.WriteString(`<xs:complexType name="B"><xs:all>`)
	for i := range particles {
		fmt.Fprintf(&b, `<xs:element name="e%d" minOccurs="0"/>`, i)
	}
	b.WriteString(`</xs:all></xs:complexType>`)
	for r := range restrictors {
		fmt.Fprintf(&b, `<xs:complexType name="T%d"><xs:complexContent><xs:restriction base="t:B"><xs:sequence>`, r)
		for i := range particles {
			fmt.Fprintf(&b, `<xs:element name="e%d"/>`, i)
		}
		b.WriteString(`</xs:sequence></xs:restriction></xs:complexContent></xs:complexType>`)
	}
	b.WriteString(`</xs:schema>`)
	return b.String()
}

// TestSequenceRestrictionOfAnAllGroupDoesWorkInProportionToChargedSteps pins
// the mapping search of findMappedGroupParticle to the base particles it
// charges: the statements of internal/schema per charged step that each added
// restriction costs may grow by at most compileWorkGrowthLimit when the group
// grows sixteenfold. Whole-compilation work per step would not show a skip,
// because the base group's own charged checks are quadratic too.
//
// Work counts executed statements of internal/schema only, so it does not see
// a skip in a for-clause condition or post statement, inside a library or
// builtin call such as slices.Index or copy, or in another package. Only
// compilations that succeed are measured. The test does not build on 32-bit
// or WebAssembly targets: the counts do not depend on the architecture, so
// the 32-bit target would repeat it, and WebAssembly cannot run the go command.
func TestSequenceRestrictionOfAnAllGroupDoesWorkInProportionToChargedSteps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	driver := buildCompileWorkDriver(t, dir)
	const small, large = 128, 2048
	sizes := [4][2]int{{1, small}, {3, small}, {1, large}, {3, large}}
	var samples [4]compileWorkSample
	var errs [4]error
	var wg sync.WaitGroup
	for i, size := range sizes {
		wg.Go(func() {
			schema := sequenceRestrictingAllSchema(size[0], size[1])
			samples[i], errs[i] = measureCompileWork(t.Context(), driver, filepath.Join(dir, fmt.Sprintf("%d-%d", size[0], size[1])), schema)
		})
	}
	wg.Wait()
	if err := errors.Join(errs[:]...); err != nil {
		t.Fatal(err)
	}
	addedSmall := samples[1].minus(samples[0])
	addedLarge := samples[3].minus(samples[2])
	for _, added := range []compileWorkSample{addedSmall, addedLarge} {
		if added.work <= 0 || added.steps <= 0 {
			t.Fatalf("two added restrictions measured work %d and steps %d, want both positive", added.work, added.steps)
		}
	}
	growth := addedLarge.perStep() / addedSmall.perStep()
	if !(growth <= compileWorkGrowthLimit) {
		t.Errorf("two added restrictions: %d particles: work %d / steps %d = %.2f; %d particles: work %d / steps %d = %.2f; growth %.3f, want at most %.2f",
			small, addedSmall.work, addedSmall.steps, addedSmall.perStep(), large, addedLarge.work, addedLarge.steps, addedLarge.perStep(), growth, compileWorkGrowthLimit)
	}
}

func buildCompileWorkDriver(t *testing.T, dir string) string {
	t.Helper()
	out := filepath.Join(dir, "compilework")
	// The driver itself is instrumented because coverage.ClearCounters rejects
	// a program whose main package is not.
	//nolint:gosec // test-controlled go build arguments only.
	cmd := exec.CommandContext(t.Context(), "go", "build", "-cover", "-covermode=atomic",
		"-coverpkg="+compileWorkPackage+","+compileWorkDriver, "-o", out, compileWorkDriver)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "GOWORK=off")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v\n%s", compileWorkDriver, err, output)
	}
	return out
}

func measureCompileWork(ctx context.Context, driver, dir, schema string) (compileWorkSample, error) {
	counters := filepath.Join(dir, "counters")
	if err := os.MkdirAll(counters, 0o750); err != nil {
		return compileWorkSample{}, err
	}
	schemaPath := filepath.Join(dir, "schema.xsd")
	if err := os.WriteFile(schemaPath, []byte(schema), 0o600); err != nil {
		return compileWorkSample{}, err
	}
	//nolint:gosec // the driver is built by this test.
	cmd := exec.CommandContext(ctx, driver, schemaPath)
	cmd.Env = append(os.Environ(), "GOCOVERDIR="+counters)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, runErr := cmd.Output()
	if runErr != nil {
		return compileWorkSample{}, fmt.Errorf("compilework %s: %w\n%s", schemaPath, runErr, stderr.String())
	}
	var analysis, dependency int
	if _, err := fmt.Sscan(string(out), &analysis, &dependency); err != nil {
		return compileWorkSample{}, fmt.Errorf("compilework %s output %q: %w", schemaPath, out, err)
	}
	profile := filepath.Join(dir, "profile.txt")
	//nolint:gosec // test-controlled go tool arguments only.
	textfmt := exec.CommandContext(ctx, "go", "tool", "covdata", "textfmt", "-i", counters, "-o", profile, "-pkg", compileWorkPackage)
	if output, err := textfmt.CombinedOutput(); err != nil {
		return compileWorkSample{}, fmt.Errorf("covdata textfmt %s: %w\n%s", counters, err, output)
	}
	work, profileErr := executedStatements(profile)
	if profileErr != nil {
		return compileWorkSample{}, profileErr
	}
	return compileWorkSample{work: work, steps: analysis + dependency}, nil
}

func executedStatements(profile string) (int, error) {
	data, err := os.ReadFile(profile) //nolint:gosec // the profile is written by this test under its temporary directory.
	if err != nil {
		return 0, err
	}
	header, blocks, ok := strings.Cut(string(data), "\n")
	if !ok || header != "mode: atomic" {
		return 0, fmt.Errorf("%s: profile header %q, want %q", profile, header, "mode: atomic")
	}
	total := 0
	for line := range strings.Lines(blocks) {
		fields := strings.Split(strings.TrimSuffix(line, "\n"), " ")
		if len(fields) != 3 {
			return 0, fmt.Errorf("%s: malformed profile line %q", profile, line)
		}
		statements, err1 := strconv.Atoi(fields[1])
		count, err2 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil {
			return 0, fmt.Errorf("%s: malformed profile line %q", profile, line)
		}
		total += statements * count
	}
	return total, nil
}
