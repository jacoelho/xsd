//go:build !(386 || arm || mips || mipsle || wasm)

package tests_test

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

const (
	compileWorkDriver  = "github.com/jacoelho/xsd/tests/internal/compilework"
	compileWorkPackage = "github.com/jacoelho/xsd/internal/schema"
)

// compileWorkSample is the work and charged steps of one compilation, and
// the executions of each block of internal/schema statements.
type compileWorkSample struct {
	work, steps int
	executions  map[coverageBlock]int
}

func (s compileWorkSample) minus(o compileWorkSample) compileWorkSample {
	return compileWorkSample{work: s.work - o.work, steps: s.steps - o.steps}
}

func (s compileWorkSample) perStep() float64 {
	return float64(s.work) / float64(s.steps)
}

// coverageBlock is a run of statements of one internal/schema file.
type coverageBlock struct {
	file               string
	startLine, endLine int
}

// contentModelWorkFamily is a shape of schema whose added components each
// read content-model particle lists in proportion to size. schema returns a
// document with the given number of components of the given size.
type contentModelWorkFamily struct {
	name         string
	schema       func(components, size int) string
	small, large int
	growthLimit  float64
}

const workSchemaOpen = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns:t="urn:t" targetNamespace="urn:t" elementFormDefault="qualified">`

func elementList(b *strings.Builder, size int, attrs string) {
	for i := range size {
		fmt.Fprintf(b, `<xs:element name="e%d"%s/>`, i, attrs)
	}
}

func groupTypes(b *strings.Builder, components, size int, group, attrs string) {
	for c := range components {
		fmt.Fprintf(b, `<xs:complexType name="T%d"><xs:%s>`, c, group)
		elementList(b, size, attrs)
		fmt.Fprintf(b, `</xs:%s></xs:complexType>`, group)
	}
}

func restrictionTypes(b *strings.Builder, components int, body func(*strings.Builder)) {
	for c := range components {
		fmt.Fprintf(b, `<xs:complexType name="T%d"><xs:complexContent><xs:restriction base="t:B">`, c)
		body(b)
		b.WriteString(`</xs:restriction></xs:complexContent></xs:complexType>`)
	}
}

func wrapped(prefix, suffix string, size int, attrs string) func(*strings.Builder) {
	return func(b *strings.Builder) {
		b.WriteString(prefix)
		elementList(b, size, attrs)
		b.WriteString(suffix)
	}
}

var contentModelWorkFamilies = []contentModelWorkFamily{
	{
		name: "sequence restricting all",
		schema: func(components, size int) string {
			var b strings.Builder
			b.WriteString(workSchemaOpen + `<xs:complexType name="B"><xs:all>`)
			elementList(&b, size, ` minOccurs="0"`)
			b.WriteString(`</xs:all></xs:complexType>`)
			restrictionTypes(&b, components, wrapped(`<xs:sequence>`, `</xs:sequence>`, size, ""))
			return b.String() + `</xs:schema>`
		},
		small: 64, large: 1024,
		// Measured growth 0.99, 1.47 with one uncharged if statement per mapped base particle in findMappedGroupParticle, and 1.92 with an uncharged loop quadratic in the list chargedParticles reads.
		growthLimit: 1.2,
	},
	{
		name: "all group",
		schema: func(components, size int) string {
			var b strings.Builder
			b.WriteString(workSchemaOpen)
			groupTypes(&b, components, size, "all", ` minOccurs="0"`)
			return b.String() + `</xs:schema>`
		},
		small: 32, large: 512,
		// Measured growth 0.65, and 2.82 with an uncharged loop cubic in the list compileAllModel reads.
		growthLimit: 1.3,
	},
	{
		name: "sequence restricting choice",
		schema: func(components, size int) string {
			var b strings.Builder
			b.WriteString(workSchemaOpen + `<xs:complexType name="B"><xs:choice maxOccurs="unbounded">`)
			elementList(&b, size, "")
			b.WriteString(`</xs:choice></xs:complexType>`)
			restrictionTypes(&b, components, wrapped(`<xs:sequence>`, `</xs:sequence>`, size, ""))
			return b.String() + `</xs:schema>`
		},
		small: 32, large: 512,
		// Measured growth 0.85, and 3.07 with an uncharged loop cubic in the base list validateSequenceRestrictsChoice reads.
		growthLimit: 1.6,
	},
	{
		name: "sequence restricting wildcard",
		schema: func(components, size int) string {
			var b strings.Builder
			b.WriteString(workSchemaOpen + `<xs:complexType name="B"><xs:sequence><xs:any processContents="lax" maxOccurs="unbounded"/></xs:sequence></xs:complexType>`)
			restrictionTypes(&b, components, wrapped(`<xs:sequence>`, `</xs:sequence>`, size, ""))
			return b.String() + `</xs:schema>`
		},
		small: 128, large: 2048,
		// Measured growth 0.99, and 3.07 with an uncharged loop quadratic in the list chargedParticles reads.
		growthLimit: 1.7,
	},
	{
		name: "extension",
		schema: func(components, size int) string {
			var b strings.Builder
			b.WriteString(workSchemaOpen + `<xs:complexType name="B"><xs:sequence>`)
			elementList(&b, size, "")
			b.WriteString(`</xs:sequence></xs:complexType>`)
			for c := range components {
				fmt.Fprintf(&b, `<xs:complexType name="T%d"><xs:complexContent><xs:extension base="t:B"><xs:sequence><xs:element name="x"/></xs:sequence></xs:extension></xs:complexContent></xs:complexType>`, c)
			}
			return b.String() + `</xs:schema>`
		},
		small: 128, large: 2048,
		// Measured growth 0.91, and 6.12 with an uncharged loop quadratic in the base list modelParticlePreservesBase reads.
		growthLimit: 2.3,
	},
	{
		name: "wildcards in sequence",
		schema: func(components, size int) string {
			var b strings.Builder
			b.WriteString(workSchemaOpen)
			for c := range components {
				fmt.Fprintf(&b, `<xs:complexType name="T%d"><xs:sequence>`, c)
				for i := range size {
					fmt.Fprintf(&b, `<xs:any namespace="urn:n%d" maxOccurs="2"/>`, i)
				}
				b.WriteString(`</xs:sequence></xs:complexType>`)
			}
			return b.String() + `</xs:schema>`
		},
		small: 32, large: 512,
		// Measured growth 0.97, and 1.72 with an uncharged loop quadratic in the list chargedParticles reads.
		growthLimit: 1.25,
	},
	{
		name: "group references",
		schema: func(components, size int) string {
			var b strings.Builder
			b.WriteString(workSchemaOpen + `<xs:group name="G"><xs:sequence>`)
			elementList(&b, size, ` type="xs:string"`)
			b.WriteString(`</xs:sequence></xs:group>`)
			for c := range components {
				fmt.Fprintf(&b, `<xs:complexType name="T%d"><xs:choice><xs:group ref="t:G"/><xs:sequence><xs:element name="y"/><xs:group ref="t:G"/></xs:sequence></xs:choice></xs:complexType>`, c)
			}
			return b.String() + `</xs:schema>`
		},
		small: 128, large: 2048,
		// Measured growth 0.98, and 3.42 with an uncharged loop quadratic in the list dfaBuilder.groupNode reads.
		growthLimit: 1.8,
	},
	{
		name: "spliced sequence restricting sequence",
		schema: func(components, size int) string {
			var b strings.Builder
			b.WriteString(workSchemaOpen + `<xs:complexType name="B"><xs:sequence>`)
			elementList(&b, size, "")
			b.WriteString(`</xs:sequence></xs:complexType>`)
			restrictionTypes(&b, components, func(b *strings.Builder) {
				b.WriteString(`<xs:sequence><xs:element name="e0"/><xs:sequence>`)
				for i := 1; i < size; i++ {
					fmt.Fprintf(b, `<xs:element name="e%d"/>`, i)
				}
				b.WriteString(`</xs:sequence></xs:sequence>`)
			})
			return b.String() + `</xs:schema>`
		},
		small: 64, large: 1024,
		// Measured growth 0.99, and 2.58 with uncharged loops quadratic in the lists validateOrderedGroupRestriction and chargedParticles read.
		growthLimit: 1.5,
	},
	{
		name: "spliced choice restricting choice",
		schema: func(components, size int) string {
			var b strings.Builder
			b.WriteString(workSchemaOpen + `<xs:complexType name="B"><xs:choice>`)
			elementList(&b, size, "")
			b.WriteString(`</xs:choice></xs:complexType>`)
			restrictionTypes(&b, components, func(b *strings.Builder) {
				b.WriteString(`<xs:choice><xs:element name="e0"/><xs:choice>`)
				for i := 1; i < size; i++ {
					fmt.Fprintf(b, `<xs:element name="e%d"/>`, i)
				}
				b.WriteString(`</xs:choice></xs:choice>`)
			})
			return b.String() + `</xs:schema>`
		},
		small: 32, large: 512,
		// Measured growth 0.88, and 4.74 with an uncharged loop cubic in the base list validateChoiceRestriction reads.
		growthLimit: 2.0,
	},
	{
		name: "empty restriction of an emptiable choice",
		schema: func(components, size int) string {
			var b strings.Builder
			b.WriteString(workSchemaOpen)
			for c := range components {
				fmt.Fprintf(&b, `<xs:complexType name="B%d"><xs:sequence><xs:choice>`, c)
				elementList(&b, size-1, "")
				fmt.Fprintf(&b, `<xs:element name="e%d" minOccurs="0"/></xs:choice><xs:element name="z" minOccurs="0"/></xs:sequence></xs:complexType>`, size-1)
				fmt.Fprintf(&b, `<xs:complexType name="T%d"><xs:complexContent><xs:restriction base="t:B%d"/></xs:complexContent></xs:complexType>`, c, c)
			}
			return b.String() + `</xs:schema>`
		},
		small: 32, large: 512,
		// Measured growth 0.81, and 3.55 with an uncharged loop cubic in the list modelEmptiableValue reads.
		growthLimit: 1.7,
	},
	{
		name: "substitution group restricting in a choice",
		schema: func(components, size int) string {
			var b strings.Builder
			b.WriteString(workSchemaOpen + `<xs:element name="h" type="xs:string"/>`)
			for i := range size {
				fmt.Fprintf(&b, `<xs:element name="m%d" type="xs:string" substitutionGroup="t:h"/>`, i)
			}
			b.WriteString(`<xs:complexType name="B"><xs:choice><xs:element ref="t:h"/><xs:element name="x"/></xs:choice></xs:complexType>`)
			restrictionTypes(&b, components, func(b *strings.Builder) {
				b.WriteString(`<xs:choice><xs:element ref="t:h"/><xs:element name="x"/></xs:choice>`)
			})
			return b.String() + `</xs:schema>`
		},
		small: 32, large: 512,
		// Measured growth 1.00, and 3.59 with an uncharged loop cubic in the base list validateChoiceRestriction reads.
		growthLimit: 1.9,
	},
	{
		name: "repeated sequence group",
		schema: func(components, size int) string {
			var b strings.Builder
			b.WriteString(workSchemaOpen)
			for c := range components {
				fmt.Fprintf(&b, `<xs:complexType name="T%d"><xs:sequence maxOccurs="unbounded"><xs:sequence maxOccurs="unbounded">`, c)
				elementList(&b, size-2, "")
				fmt.Fprintf(&b, `<xs:element name="e%d" maxOccurs="2"/><xs:element name="e%d" minOccurs="0" maxOccurs="2"/></xs:sequence>`, size-2, size-1)
				b.WriteString(`<xs:element name="z" minOccurs="0"/></xs:sequence></xs:complexType>`)
			}
			return b.String() + `</xs:schema>`
		},
		small: 64, large: 1024,
		// Measured growth 1.06, and 2.72 with uncharged loops quadratic in the lists dfaBuilder.groupNode and chargedParticles read.
		growthLimit: 1.7,
	},
}

// particleListReadersOnlyRejecting are the allowed particle-list readers
// that execute only in a compilation that fails, which no content-model work
// family measures.
var particleListReadersOnlyRejecting = map[string]string{
	"contentRestrictionValidator.modelContainsWildcard": "called only by validateFallbackContentRestriction, which rejects the restriction",
}

// TestContentModelCompileWorkGrowsWithChargedSteps requires, for each
// content-model work family, that the statements of internal/schema per
// charged step that two added components cost grow by at most the family's
// growth limit when the components grow sixteenfold. Work per step of a whole
// compilation would hide added work behind the fixed part of the document.
// It also requires every function of particleListReaders, except those in
// particleListReadersOnlyRejecting, to execute more often with the added
// components of some family at the large size, so each allowed reader runs
// under a measured budget; that check runs only when every family is measured.
//
// The test catches only uncharged work that grows faster than the charged
// steps of the component that does it: a loop over a list whose particles are
// charged once each is invisible to it, and so is a quadratic loop in a
// family whose charged steps are already quadratic. Work counts executed
// statements of internal/schema only, so it does not see work in a for-clause
// condition or post statement, inside a library or builtin call such as
// slices.Index or copy, or in another package. Only compilations that succeed
// are measured. The test does not build on 32-bit or WebAssembly targets: the
// counts do not depend on the architecture, so the 32-bit target would repeat
// it, and WebAssembly cannot run the go command.
func TestContentModelCompileWorkGrowsWithChargedSteps(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	driver := buildCompileWorkDriver(t, dir)
	functions := newSchemaFunctionLines(t)
	var mu sync.Mutex
	exercised := map[string]bool{}
	measured := 0
	t.Cleanup(func() {
		if t.Failed() || measured != len(contentModelWorkFamilies) {
			return
		}
		for _, name := range slices.Sorted(maps.Keys(particleListReaders)) {
			_, rejecting := particleListReadersOnlyRejecting[name]
			switch {
			case rejecting && exercised[name]:
				t.Errorf("allowed particle-list reader %s is listed as executing only in failing compilations but a content-model work family exercises it", name)
			case !rejecting && !exercised[name]:
				t.Errorf("allowed particle-list reader %s is exercised by no content-model work family", name)
			}
		}
		for _, name := range slices.Sorted(maps.Keys(particleListReadersOnlyRejecting)) {
			if _, ok := particleListReaders[name]; !ok {
				t.Errorf("%s is listed as executing only in failing compilations but is not an allowed particle-list reader", name)
			}
		}
	})
	for _, family := range contentModelWorkFamilies {
		t.Run(family.name, func(t *testing.T) {
			t.Parallel()
			added := measureFamily(t, driver, filepath.Join(dir, strings.ReplaceAll(family.name, " ", "-")), family)
			mu.Lock()
			measured++
			defer mu.Unlock()
			for _, block := range added {
				exercised[functions.at(block)] = true
			}
		})
	}
}

// measureFamily checks the growth of the work per charged step of family's
// added components and returns the blocks they execute at the large size.
func measureFamily(t *testing.T, driver, dir string, family contentModelWorkFamily) []coverageBlock {
	t.Helper()
	sizes := [4][2]int{{1, family.small}, {3, family.small}, {1, family.large}, {3, family.large}}
	var samples [4]compileWorkSample
	var errs [4]error
	var wg sync.WaitGroup
	for i, size := range sizes {
		wg.Go(func() {
			schema := family.schema(size[0], size[1])
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
			t.Fatalf("two added components measured work %d and steps %d, want both positive", added.work, added.steps)
		}
	}
	growth := addedLarge.perStep() / addedSmall.perStep()
	if !(growth <= family.growthLimit) {
		t.Errorf("two added components: size %d: work %d / steps %d = %.2f; size %d: work %d / steps %d = %.2f; growth %.3f, want at most %.2f",
			family.small, addedSmall.work, addedSmall.steps, addedSmall.perStep(), family.large, addedLarge.work, addedLarge.steps, addedLarge.perStep(), growth, family.growthLimit)
	}
	var added []coverageBlock
	for block, executions := range samples[3].executions {
		if executions > samples[2].executions[block] {
			added = append(added, block)
		}
	}
	return added
}

// schemaFunctionLines maps each non-test file of internal/schema to the line
// ranges of its functions, keyed as particleListReaders keys them.
type schemaFunctionLines map[string][]functionLines

type functionLines struct {
	key        string
	start, end int
}

func newSchemaFunctionLines(t *testing.T) schemaFunctionLines {
	t.Helper()
	dir := filepath.Join(repoRoot(t), "internal", "schema")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	lines := schemaFunctionLines{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", entry.Name(), parseErr)
		}
		for _, fn := range packageFunctions([]*ast.File{file}) {
			lines[entry.Name()] = append(lines[entry.Name()], functionLines{
				key:   fn.key,
				start: fset.Position(fn.node.Pos()).Line,
				end:   fset.Position(fn.node.End()).Line,
			})
		}
	}
	return lines
}

// at returns the key of the function holding block, or "" for none.
func (l schemaFunctionLines) at(block coverageBlock) string {
	for _, fn := range l[block.file] {
		if fn.start <= block.startLine && block.endLine <= fn.end {
			return fn.key
		}
	}
	return ""
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
	sample, profileErr := readCoverageProfile(profile)
	if profileErr != nil {
		return compileWorkSample{}, profileErr
	}
	sample.steps = analysis + dependency
	return sample, nil
}

// readCoverageProfile returns the statements a profile counts as executed,
// one per execution, and the executions of each block.
func readCoverageProfile(profile string) (compileWorkSample, error) {
	data, err := os.ReadFile(profile) //nolint:gosec // the profile is written by this test under its temporary directory.
	if err != nil {
		return compileWorkSample{}, err
	}
	header, blocks, ok := strings.Cut(string(data), "\n")
	if !ok || header != "mode: atomic" {
		return compileWorkSample{}, fmt.Errorf("%s: profile header %q, want %q", profile, header, "mode: atomic")
	}
	sample := compileWorkSample{executions: map[coverageBlock]int{}}
	for line := range strings.Lines(blocks) {
		counted, ok := parseCoverageLine(strings.TrimSuffix(line, "\n"))
		if !ok {
			return compileWorkSample{}, fmt.Errorf("%s: malformed profile line %q", profile, line)
		}
		sample.work += counted.statements * counted.executions
		sample.executions[counted.block] += counted.executions
	}
	return sample, nil
}

// countedBlock is one line of a coverage profile: a block, the statements
// it holds, and its executions.
type countedBlock struct {
	block                  coverageBlock
	statements, executions int
}

// parseCoverageLine parses "path:startLine.startCol,endLine.endCol statements count".
func parseCoverageLine(line string) (countedBlock, bool) {
	fields := strings.Split(line, " ")
	if len(fields) != 3 {
		return countedBlock{}, false
	}
	statements, err1 := strconv.Atoi(fields[1])
	count, err2 := strconv.Atoi(fields[2])
	path, span, ok := strings.Cut(fields[0], ":")
	start, end, ok2 := strings.Cut(span, ",")
	startLine, _, ok3 := strings.Cut(start, ".")
	endLine, _, ok4 := strings.Cut(end, ".")
	first, err3 := strconv.Atoi(startLine)
	last, err4 := strconv.Atoi(endLine)
	if err := errors.Join(err1, err2, err3, err4); err != nil || !ok || !ok2 || !ok3 || !ok4 {
		return countedBlock{}, false
	}
	return countedBlock{block: coverageBlock{file: filepath.Base(path), startLine: first, endLine: last}, statements: statements, executions: count}, true
}
