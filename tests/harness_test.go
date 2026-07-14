package tests_test

import (
	"context"
	"encoding/json"
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
	"github.com/jacoelho/xsd/xsderrors"
)

type manifest struct {
	Totals manifestTotals `json:"totals"`
	Cases  []manifestCase `json:"cases"`
}

type manifestTotals struct {
	Cases        int `json:"cases"`
	SchemaCases  int `json:"schemaCases"`
	InstanceRuns int `json:"instanceRuns"`
	W3CCases     int `json:"w3cCases"`
	InternalRuns int `json:"internalRuns"`
	XercesJCases int `json:"xercesJCases"`
}

type manifestCase struct {
	ID             string             `json:"id"`
	ExpectedSource string             `json:"expectedSource"`
	Schema         *manifestSchema    `json:"schema"`
	Instances      []manifestInstance `json:"instances"`
	Files          []manifestFile     `json:"files"`
}

type manifestSchema struct {
	Expected  string             `json:"expected"`
	ErrorCode string             `json:"errorCode"`
	Documents []manifestDocument `json:"documents"`
}

type manifestDocument struct {
	File string `json:"file"`
}

type manifestInstance struct {
	TestName  string `json:"testName"`
	File      string `json:"file"`
	Expected  string `json:"expected"`
	ErrorCode string `json:"errorCode"`
}

type manifestFile struct {
	File string `json:"file"`
	Role string `json:"role"`
}

func (m *manifest) UnmarshalJSON(data []byte) error {
	var decoded struct {
		Totals manifestTotals `json:"totals"`
		Cases  []manifestCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*m = manifest{Totals: decoded.Totals, Cases: decoded.Cases}
	return m.validate()
}

func (m *manifest) validate() error {
	caseIdentities := make(map[string]bool, len(m.Cases))
	observed := manifestTotals{Cases: len(m.Cases)}
	for i, tc := range m.Cases {
		if tc.ExpectedSource == "" || tc.ID == "" {
			return fmt.Errorf("manifest case %d has empty expectedSource or id", i)
		}
		identity := tc.ExpectedSource + "\x00" + tc.ID
		if caseIdentities[identity] {
			return fmt.Errorf("manifest duplicates case identity %q/%q", tc.ExpectedSource, tc.ID)
		}
		caseIdentities[identity] = true

		if tc.Schema != nil {
			observed.SchemaCases++
			documents := make(map[string]bool, len(tc.Schema.Documents))
			for _, doc := range tc.Schema.Documents {
				if documents[doc.File] {
					return fmt.Errorf("manifest case %q/%q duplicates schema document %q", tc.ExpectedSource, tc.ID, doc.File)
				}
				documents[doc.File] = true
			}
		}

		instances := make(map[string]bool, len(tc.Instances))
		for _, inst := range tc.Instances {
			name := instanceName(inst)
			if instances[name] {
				return fmt.Errorf("manifest case %q/%q duplicates instance name %q", tc.ExpectedSource, tc.ID, name)
			}
			instances[name] = true
		}
		observed.InstanceRuns += len(tc.Instances)
		switch tc.ExpectedSource {
		case "w3c":
			observed.W3CCases++
		case "project":
			observed.InternalRuns++
		case "xerces-j":
			observed.XercesJCases++
		default:
			return fmt.Errorf("manifest case %q has unknown expectedSource %q", tc.ID, tc.ExpectedSource)
		}
	}
	if m.Totals != observed {
		return fmt.Errorf("manifest totals = %+v, want %+v", m.Totals, observed)
	}
	return nil
}

func (c *manifestCase) UnmarshalJSON(data []byte) error {
	var decoded struct {
		ID             string             `json:"id"`
		ExpectedSource string             `json:"expectedSource"`
		Schema         json.RawMessage    `json:"schema"`
		Instances      []manifestInstance `json:"instances"`
		Files          []manifestFile     `json:"files"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var schema *manifestSchema
	if decoded.Schema != nil {
		if err := json.Unmarshal(decoded.Schema, &schema); err != nil {
			return fmt.Errorf("manifest case %q schema: %w", decoded.ID, err)
		}
	}
	if schema != nil {
		switch schema.Expected {
		case "valid", "invalid":
		default:
			return fmt.Errorf("manifest case %q has unknown schema expected value %q", decoded.ID, schema.Expected)
		}
	}
	for _, inst := range decoded.Instances {
		switch inst.Expected {
		case "valid", "invalid":
		default:
			return fmt.Errorf("manifest case %q instance %q has unknown expected value %q", decoded.ID, instanceName(inst), inst.Expected)
		}
	}
	*c = manifestCase{
		ID:             decoded.ID,
		ExpectedSource: decoded.ExpectedSource,
		Schema:         schema,
		Instances:      decoded.Instances,
		Files:          decoded.Files,
	}
	return nil
}

func TestHarness(t *testing.T) {
	dir := testDir(t)
	m := readManifest(t, filepath.Join(dir, "manifest.json"))
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

func (r harnessRunCoverage) complete(m manifest) bool {
	schemaCases, schemaLessCases, instanceRuns := manifestRunCounts(m)
	return r.schemaCases == schemaCases &&
		r.schemaLessCases == schemaLessCases &&
		r.instanceRuns+r.blockedInstanceRuns == instanceRuns
}

func manifestRunCounts(m manifest) (schemaCases, schemaLessCases, instanceRuns int) {
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

func readManifest(t *testing.T, path string) manifest {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // Test harness reads repository-owned manifest path.
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var m manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return m
}

func manifestSources(m manifest) []string {
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

func runCase(t *testing.T, dir string, unsupported unsupportedAllowlist, tc manifestCase, run *harnessRunCoverage) {
	t.Helper()
	if tc.Schema == nil {
		run.schemaLessCases++
		for _, inst := range tc.Instances {
			t.Run(instanceName(inst), func(t *testing.T) {
				run.instanceRuns++
				t.Skip("schema-less assessment is unsupported by the precompiled Engine harness")
			})
		}
		return
	}
	run.schemaCases++
	engine, err := xsd.Compile(context.Background(), schemaSources(dir, tc)...)
	switch tc.Schema.Expected {
	case "valid":
		if err != nil {
			if xsderrors.IsUnsupported(err) {
				run.blockedInstanceRuns += len(tc.Instances)
			}
			skipUnsupported(t, unsupported, unsupportedSchemaKey(tc), err)
			t.Fatalf("Compile() error = %v", err)
		}
	case "invalid":
		if err == nil {
			t.Fatalf("Compile() expected invalid schema")
		}
		if tc.Schema.ErrorCode != "" {
			expectErrorCode(t, err, tc.Schema.ErrorCode)
		} else {
			if xsderrors.IsUnsupported(err) {
				run.blockedInstanceRuns += len(tc.Instances)
			}
			skipUnsupported(t, unsupported, unsupportedSchemaKey(tc), err)
		}
		for _, inst := range tc.Instances {
			t.Run(instanceName(inst), func(t *testing.T) {
				run.blockedInstanceRuns++
				t.Skip("instance assessment is blocked by the expected-invalid schema")
			})
		}
		return
	default:
		t.Fatalf("unknown schema expected value %q", tc.Schema.Expected)
	}
	for _, inst := range tc.Instances {
		t.Run(instanceName(inst), func(t *testing.T) {
			run.instanceRuns++
			validateInstance(t, dir, engine, unsupported, tc, inst)
		})
	}
}

func validateInstance(t *testing.T, dir string, engine *xsd.Engine, unsupported unsupportedAllowlist, tc manifestCase, inst manifestInstance) {
	t.Helper()
	f, err := os.Open(harnessFile(dir, inst.File))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	err = engine.Validate(context.Background(), f)
	closeErr := f.Close()
	if closeErr != nil {
		t.Fatalf("Close() error = %v", closeErr)
	}
	switch inst.Expected {
	case "valid":
		if err != nil {
			skipUnsupported(t, unsupported, unsupportedInstanceKey(tc, inst), err)
			t.Fatalf("Validate() error = %v", err)
		}
	case "invalid":
		if err == nil {
			t.Fatalf("Validate() expected invalid")
		}
		if inst.ErrorCode != "" {
			expectErrorCode(t, err, inst.ErrorCode)
			return
		}
		skipUnsupported(t, unsupported, unsupportedInstanceKey(tc, inst), err)
	default:
		t.Fatalf("unknown instance expected value %q", inst.Expected)
	}
}

func expectErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	xerr, ok := errors.AsType[*xsderrors.Error](err)
	if !ok {
		t.Fatalf("error %v is not *xsderrors.Error", err)
	}
	if string(xerr.Code) != code {
		t.Fatalf("error code = %s, want %s; err=%v", xerr.Code, code, err)
	}
}

func skipUnsupported(t *testing.T, unsupported unsupportedAllowlist, key unsupportedKey, err error) {
	t.Helper()
	if !xsderrors.IsUnsupported(err) {
		return
	}
	code := unsupportedErrorCode(err)
	if useErr := unsupported.use(key, code); useErr != nil {
		t.Fatal(useErr)
	}
	if xerr, ok := errors.AsType[*xsderrors.Error](err); ok {
		t.Skipf("unsupported feature %s: %s", xerr.Code, xerr.Message)
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

func unsupportedSchemaKey(tc manifestCase) unsupportedKey {
	return unsupportedKey{kind: unsupportedSchema, source: tc.ExpectedSource, caseID: tc.ID}
}

func unsupportedInstanceKey(tc manifestCase, inst manifestInstance) unsupportedKey {
	return unsupportedKey{kind: unsupportedInstance, source: tc.ExpectedSource, caseID: tc.ID, instance: instanceName(inst)}
}

func unsupportedErrorCode(err error) string {
	if xerr, ok := errors.AsType[*xsderrors.Error](err); ok {
		return string(xerr.Code)
	}
	return "unsupported"
}

func schemaSources(dir string, tc manifestCase) []xsd.SchemaSource {
	files := schemaDocumentFiles(tc)
	sources := make([]xsd.SchemaSource, 0, len(files))
	for _, file := range files {
		sources = append(sources, xsd.File(harnessFile(dir, file)))
	}
	return sources
}

func schemaDocumentFiles(tc manifestCase) []string {
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

func instanceName(inst manifestInstance) string {
	if inst.TestName != "" {
		return inst.TestName
	}
	if inst.File != "" {
		return filepath.Base(inst.File)
	}
	return fmt.Sprintf("instance-%s", inst.Expected)
}
