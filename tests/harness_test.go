package tests_test

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/internal/corpus"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestHarness(t *testing.T) {
	dir := testDir(t)
	m := loadInventory(t, dir)
	unsupported := readUnsupportedAllowlist(t, filepath.Join(dir, "unsupported.txt"))
	var run harnessRunCoverage
	for _, source := range manifestSources(m) {
		t.Run(source, func(t *testing.T) {
			for _, tc := range m.Cases {
				if tc.ExpectedSource != source {
					continue
				}
				t.Run(tc.ID, func(t *testing.T) {
					runCase(t, dir, unsupported, tc, &run)
				})
			}
		})
	}
	if !t.Failed() && run.complete(m) {
		unsupported.requireAllUsed(t)
	}
}

type harnessRunCoverage struct {
	schemaCases         int
	schemaLessCases     int
	instanceRuns        int
	blockedInstanceRuns int
}

func (r harnessRunCoverage) complete(m corpus.Inventory) bool {
	schemaCases, schemaLessCases, instanceRuns := manifestRunCounts(m)
	return r.schemaCases == schemaCases &&
		r.schemaLessCases == schemaLessCases &&
		r.instanceRuns+r.blockedInstanceRuns == instanceRuns
}

func manifestRunCounts(m corpus.Inventory) (schemaCases, schemaLessCases, instanceRuns int) {
	for _, tc := range m.Cases {
		instanceRuns += len(tc.Instances)
		if tc.Schema == nil {
			schemaLessCases++
			continue
		}
		schemaCases++
	}
	return schemaCases, schemaLessCases, instanceRuns
}

func testDir(t *testing.T) string {
	t.Helper()
	pc, file, line, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	if pc == 0 || line == 0 {
		t.Fatalf("runtime.Caller() = pc %v, line %d", pc, line)
	}
	return filepath.Dir(file)
}

func loadInventory(t *testing.T, dir string) corpus.Inventory {
	t.Helper()
	inv, err := corpus.LoadInventory(os.DirFS(dir))
	if err != nil {
		t.Fatalf("LoadInventory() error = %v", err)
	}
	return inv
}

func manifestSources(m corpus.Inventory) []string {
	seen := make(map[string]bool)
	for _, tc := range m.Cases {
		seen[tc.ExpectedSource] = true
	}
	sources := make([]string, 0, len(seen))
	for _, source := range []string{"project", "xerces-j", "w3c"} {
		if seen[source] {
			sources = append(sources, source)
			delete(seen, source)
		}
	}
	return append(sources, slices.Sorted(maps.Keys(seen))...)
}

func runCase(t *testing.T, dir string, unsupported unsupportedAllowlist, tc corpus.Case, run *harnessRunCoverage) {
	t.Helper()
	if tc.Schema == nil {
		run.schemaLessCases++
		for _, inst := range tc.Instances {
			t.Run(inst.TestName, func(t *testing.T) {
				run.instanceRuns++
				t.Skip("schema-less assessment is unsupported by the precompiled Engine harness")
			})
		}
		return
	}
	run.schemaCases++
	engine, err := xsd.Compile(schemaSources(dir, tc)...)
	if locErr := checkDiagnosticLocations(err, compileLocatedCategories); locErr != nil {
		t.Fatalf("case %s schema: %v", tc.ID, locErr)
	}
	switch tc.Schema.Expected {
	case corpus.ExpectedValid:
		if err != nil {
			if xsderrors.IsUnsupported(err) {
				run.blockedInstanceRuns += len(tc.Instances)
			}
			skipUnsupported(t, unsupported, unsupportedSchemaKey(tc), err)
			t.Fatalf("Compile() error = %v", err)
		}
	case corpus.ExpectedInvalid:
		if checkErr := checkExpectedFailure(err, tc.Schema.ErrorCode); checkErr != nil {
			t.Fatalf("Compile(): %v", checkErr)
		}
		if tc.Schema.ErrorCode == "" {
			if xsderrors.IsUnsupported(err) {
				run.blockedInstanceRuns += len(tc.Instances)
			}
			skipUnsupported(t, unsupported, unsupportedSchemaKey(tc), err)
		}
		for _, inst := range tc.Instances {
			t.Run(inst.TestName, func(t *testing.T) {
				run.blockedInstanceRuns++
				t.Skip("instance assessment is blocked by the expected-invalid schema")
			})
		}
		return
	default:
		t.Fatalf("unknown schema expected value %q", tc.Schema.Expected)
	}
	for _, inst := range tc.Instances {
		t.Run(inst.TestName, func(t *testing.T) {
			run.instanceRuns++
			validateInstance(t, dir, engine, unsupported, tc, inst)
		})
	}
}

func validateInstance(t *testing.T, dir string, engine *xsd.Engine, unsupported unsupportedAllowlist, tc corpus.Case, inst corpus.Instance) {
	t.Helper()
	f, err := os.Open(harnessFile(dir, inst.File))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	err = engine.Validate(f)
	closeErr := f.Close()
	if closeErr != nil {
		t.Fatalf("Close() error = %v", closeErr)
	}
	if locErr := checkDiagnosticLocations(err, instanceLocatedCategories); locErr != nil {
		t.Fatalf("case %s instance %s: %v", tc.ID, inst.File, locErr)
	}
	switch inst.Expected {
	case corpus.ExpectedValid:
		if err != nil {
			skipUnsupported(t, unsupported, unsupportedInstanceKey(tc, inst), err)
			t.Fatalf("Validate() error = %v", err)
		}
	case corpus.ExpectedInvalid:
		if checkErr := checkExpectedFailure(err, inst.ErrorCode); checkErr != nil {
			t.Fatalf("Validate(): %v", checkErr)
		}
		if inst.ErrorCode == "" {
			skipUnsupported(t, unsupported, unsupportedInstanceKey(tc, inst), err)
		}
	default:
		t.Fatalf("unknown instance expected value %q", inst.Expected)
	}
}

// compileLocatedCategories are the categories of every compile diagnostic the
// harness can produce: each is about schema document content.
var compileLocatedCategories = []xsderrors.Category{
	xsderrors.CategorySchemaParse,
	xsderrors.CategorySchemaCompile,
	xsderrors.CategoryUnsupported,
}

// instanceLocatedCategories are the categories of the diagnostics an instance
// run produces while reading a document.
var instanceLocatedCategories = []xsderrors.Category{
	xsderrors.CategoryValidation,
	xsderrors.CategoryUnsupported,
}

// checkDiagnosticLocations enforces that every diagnostic of one of
// categories in err carries a path and a line.
func checkDiagnosticLocations(err error, categories []xsderrors.Category) error {
	for _, leaf := range xsderrors.Flatten(err) {
		diagnostic, ok := errors.AsType[*xsderrors.Error](leaf)
		if !ok {
			continue
		}
		if !slices.Contains(categories, diagnostic.Category()) {
			continue
		}
		if diagnostic.Path() == "" || diagnostic.Line() <= 0 {
			return fmt.Errorf("diagnostic %s is unlocated: path=%q line=%d", diagnostic.Code(), diagnostic.Path(), diagnostic.Line())
		}
	}
	return nil
}

func checkExpectedFailure(err error, code string) error {
	if err == nil {
		return errors.New("expected failure, got success")
	}
	if internal := internalHarnessError(err); internal != nil {
		return fmt.Errorf("unexpected internal error: %w", internal)
	}
	if code == "" {
		return nil
	}
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		return fmt.Errorf("error is not *xsderrors.Error: %w", err)
	}
	if string(xerr.Code()) != code {
		return fmt.Errorf("error code = %s, want %s: %w", xerr.Code(), code, err)
	}
	return nil
}

func internalHarnessError(err error) error {
	pending := []error{err}
	for len(pending) != 0 {
		current := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if diagnostic, ok := current.(*xsderrors.Error); ok && diagnostic.Category() == xsderrors.CategoryInternal { //nolint:errorlint // Inspect each node rather than only the first matching diagnostic.
			return diagnostic
		}
		switch wrapped := current.(type) { //nolint:errorlint // Traverse every unwrap branch, including causes of structured diagnostics.
		case *xsderrors.Errors:
			if wrapped != nil {
				pending = append(pending, wrapped.Unwrap()...)
			}
		case interface{ Unwrap() []error }:
			pending = append(pending, wrapped.Unwrap()...)
		case interface{ Unwrap() error }:
			pending = append(pending, wrapped.Unwrap())
		}
	}
	return nil
}

func skipUnsupported(t *testing.T, unsupported unsupportedAllowlist, key unsupportedKey, err error) {
	t.Helper()
	if internal := internalHarnessError(err); internal != nil {
		t.Fatalf("unexpected internal error: %v", internal)
	}
	if !xsderrors.IsUnsupported(err) {
		return
	}
	code := unsupportedErrorCode(err)
	if useErr := unsupported.use(key, code); useErr != nil {
		t.Fatal(useErr)
	}
	if xerr, ok := errors.AsType[*xsderrors.Error](err); ok {
		t.Skipf("unsupported feature %s: %s", xerr.Code(), xerr.Message())
	}
	t.Skipf("unsupported feature: %v", err)
}

const (
	unsupportedSchema   = "schema"
	unsupportedInstance = "instance"
)

type unsupportedKey struct {
	kind     string
	source   string
	caseID   string
	instance string
}

func (k unsupportedKey) String() string {
	if k.kind == unsupportedInstance {
		return strings.Join([]string{k.kind, k.source, k.caseID, k.instance}, "\t")
	}
	return strings.Join([]string{k.kind, k.source, k.caseID}, "\t")
}

type unsupportedEntry struct {
	code string
	used bool
}

type unsupportedAllowlist map[unsupportedKey]unsupportedEntry

func readUnsupportedAllowlist(t *testing.T, path string) unsupportedAllowlist {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // Test harness reads repository-owned allowlist path.
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	unsupported, err := parseUnsupportedAllowlist(string(data))
	if err != nil {
		t.Fatalf("unsupported allowlist: %v", err)
	}
	return unsupported
}

func parseUnsupportedAllowlist(data string) (unsupportedAllowlist, error) {
	unsupported := make(unsupportedAllowlist)
	var prev string
	for lineNo, line := range strings.Split(strings.TrimSuffix(data, "\n"), "\n") {
		if line == "" {
			return nil, fmt.Errorf("line %d is empty", lineNo+1)
		}
		if prev != "" && line <= prev {
			return nil, fmt.Errorf("line %d is not sorted after %q", lineNo+1, prev)
		}
		prev = line
		fields := strings.Split(line, "\t")
		key, code, err := parseUnsupportedLine(fields)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo+1, err)
		}
		if _, ok := unsupported[key]; ok {
			return nil, fmt.Errorf("line %d duplicates %s", lineNo+1, key)
		}
		unsupported[key] = unsupportedEntry{code: code}
	}
	return unsupported, nil
}

func parseUnsupportedLine(fields []string) (unsupportedKey, string, error) {
	if len(fields) == 0 {
		return unsupportedKey{}, "", errors.New("missing kind")
	}
	switch fields[0] {
	case unsupportedSchema:
		if len(fields) != 4 {
			return unsupportedKey{}, "", fmt.Errorf("schema entry has %d fields, want 4", len(fields))
		}
		if fields[1] == "" || fields[2] == "" || fields[3] == "" {
			return unsupportedKey{}, "", errors.New("schema entry has empty field")
		}
		return unsupportedKey{kind: unsupportedSchema, source: fields[1], caseID: fields[2]}, fields[3], nil
	case unsupportedInstance:
		if len(fields) != 5 {
			return unsupportedKey{}, "", fmt.Errorf("instance entry has %d fields, want 5", len(fields))
		}
		if fields[1] == "" || fields[2] == "" || fields[3] == "" || fields[4] == "" {
			return unsupportedKey{}, "", errors.New("instance entry has empty field")
		}
		return unsupportedKey{kind: unsupportedInstance, source: fields[1], caseID: fields[2], instance: fields[3]}, fields[4], nil
	default:
		return unsupportedKey{}, "", fmt.Errorf("unknown kind %q", fields[0])
	}
}

func (unsupported unsupportedAllowlist) use(key unsupportedKey, code string) error {
	entry, ok := unsupported[key]
	if !ok {
		return fmt.Errorf("unsupported feature %s for unlisted %s", code, key)
	}
	if entry.code != code {
		return fmt.Errorf("unsupported feature %s for %s, want %s", code, key, entry.code)
	}
	entry.used = true
	unsupported[key] = entry
	return nil
}

func (unsupported unsupportedAllowlist) requireAllUsed(t *testing.T) {
	t.Helper()
	var unused []string
	for key, entry := range unsupported {
		if !entry.used {
			unused = append(unused, key.String()+"\t"+entry.code)
		}
	}
	if len(unused) == 0 {
		return
	}
	slices.Sort(unused)
	n := min(len(unused), 10)
	t.Fatalf("unsupported allowlist has %d unused entries; first unused:\n%s", len(unused), strings.Join(unused[:n], "\n"))
}

func unsupportedSchemaKey(tc corpus.Case) unsupportedKey {
	return unsupportedKey{kind: unsupportedSchema, source: tc.ExpectedSource, caseID: tc.ID}
}

func unsupportedInstanceKey(tc corpus.Case, inst corpus.Instance) unsupportedKey {
	return unsupportedKey{kind: unsupportedInstance, source: tc.ExpectedSource, caseID: tc.ID, instance: inst.TestName}
}

func unsupportedErrorCode(err error) string {
	if xerr, ok := errors.AsType[*xsderrors.Error](err); ok {
		return string(xerr.Code())
	}
	return "unsupported"
}

func schemaSources(dir string, tc corpus.Case) []xsd.SchemaSource {
	files := schemaDocumentFiles(tc)
	sources := make([]xsd.SchemaSource, 0, len(files))
	for _, file := range files {
		sources = append(sources, xsd.File(harnessFile(dir, file)))
	}
	return sources
}

func schemaDocumentFiles(tc corpus.Case) []string {
	if tc.Schema == nil {
		return nil
	}
	files := make([]string, len(tc.Schema.Documents))
	for i, doc := range tc.Schema.Documents {
		files[i] = doc.File
	}
	return files
}

func harnessFile(dir, name string) string {
	return filepath.Join(dir, filepath.FromSlash(name))
}
