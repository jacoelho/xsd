package tests_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode"

	"github.com/jacoelho/xsd"
)

func TestManifestCorpusClosure(t *testing.T) {
	dir := testDir(t)
	m := readManifest(t, filepath.Join(dir, "manifest.json"))
	if err := validateManifestCorpus(dir, m); err != nil {
		t.Fatal(err)
	}
}

func TestValidateManifestCorpus(t *testing.T) {
	t.Run("duplicate reference", func(t *testing.T) {
		dir := t.TempDir()
		name := "corpus/schema.xsd"
		writeHarnessTestFile(t, filepath.Join(dir, filepath.FromSlash(name)), `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
		m := manifest{Cases: []manifestCase{{
			Schema: &manifestSchema{Documents: []manifestDocument{{File: name}, {File: name}}},
		}}}
		if err := validateManifestCorpus(dir, m); err != nil {
			t.Fatalf("validateManifestCorpus() error = %v", err)
		}
	})

	for _, tt := range []struct {
		name      string
		reference string
		file      string
		want      string
	}{
		{name: "missing", reference: "corpus/missing.xsd", want: "referenced file"},
		{name: "orphan", file: "corpus/orphan.xsd", want: "unreferenced corpus file"},
		{name: "traversal", reference: "corpus/../outside.xsd", want: "not a canonical corpus-relative path"},
		{name: "absolute", reference: "/corpus/schema.xsd", want: "not a canonical corpus-relative path"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "corpus"), 0o700); err != nil {
				t.Fatal(err)
			}
			if tt.file != "" {
				writeHarnessTestFile(t, filepath.Join(dir, filepath.FromSlash(tt.file)), "fixture")
			}
			var m manifest
			if tt.reference != "" {
				m.Cases = []manifestCase{{
					Schema: &manifestSchema{Documents: []manifestDocument{{File: tt.reference}}},
				}}
			}
			err := validateManifestCorpus(dir, m)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateManifestCorpus() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestManifestPreservesSchemaLessCases(t *testing.T) {
	var m manifest
	if err := json.Unmarshal([]byte(`{
		"totals":{"cases":1,"schemaCases":0,"instanceRuns":1,"w3cCases":0,"internalRuns":1,"xercesJCases":0},
		"cases":[{"id":"instance-only","expectedSource":"project","instances":[{"expected":"valid"}]}]
	}`), &m); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if len(m.Cases) != 1 || m.Cases[0].Schema != nil {
		t.Fatalf("manifest cases = %+v, want one explicit schema-less case", m.Cases)
	}
	if schemaCases, schemaLessCases, instanceRuns := manifestRunCounts(m); schemaCases != 0 || schemaLessCases != 1 || instanceRuns != 1 {
		t.Fatalf("manifestRunCounts() = %d, %d, %d; want 0, 1, 1", schemaCases, schemaLessCases, instanceRuns)
	}
}

func TestManifestRejectsIntegrityViolations(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
		want string
	}{
		{
			name: "totals",
			data: `{"totals":{},"cases":[{"id":"case","expectedSource":"project"}]}`,
			want: "manifest totals",
		},
		{
			name: "empty case identity",
			data: `{"totals":{},"cases":[{"id":"","expectedSource":"project"}]}`,
			want: "empty expectedSource or id",
		},
		{
			name: "unknown expected source",
			data: `{"totals":{"cases":1},"cases":[{"id":"case","expectedSource":"unknown"}]}`,
			want: `unknown expectedSource "unknown"`,
		},
		{
			name: "duplicate case identity",
			data: `{"totals":{},"cases":[
				{"id":"case","expectedSource":"project"},
				{"id":"case","expectedSource":"project"}
			]}`,
			want: "duplicates case identity",
		},
		{
			name: "duplicate instance name",
			data: `{"totals":{},"cases":[{"id":"case","expectedSource":"project","instances":[
				{"testName":"instance","expected":"valid"},
				{"testName":"instance","expected":"invalid"}
			]}]}`,
			want: "duplicates instance name",
		},
		{
			name: "duplicate schema document",
			data: `{"totals":{},"cases":[{"id":"case","expectedSource":"project","schema":{
				"expected":"valid","documents":[{"file":"schema.xsd"},{"file":"schema.xsd"}]
			}}]}`,
			want: "duplicates schema document",
		},
		{
			name: "unknown oracle key",
			data: `{"totals":{},"cases":[{"id":"case","expectedSource":"project","schema":{"expected":"valid"},"instances":[
				{"testName":"i","expected":"valid","oracle":{"dotNet":{"expected":"valid","reason":"r"}}}
			]}]}`,
			want: "manifest case \"case\" instance \"i\" has unknown oracle key \"dotNet\"",
		},
		{
			name: "oracle with expected and status",
			data: `{"totals":{},"cases":[{"id":"case","expectedSource":"project","schema":{"expected":"valid"},"instances":[
				{"testName":"i","expected":"valid","oracle":{"dotnet":{"expected":"valid","status":"blocked","reason":"r"}}}
			]}]}`,
			want: "manifest case \"case\" instance \"i\" oracle \"dotnet\" has both expected and status",
		},
		{
			name: "oracle without expected or status",
			data: `{"totals":{},"cases":[{"id":"case","expectedSource":"project","schema":{"expected":"valid"},"instances":[
				{"testName":"i","expected":"valid","oracle":{"dotnet":{"expcted":"valid","reason":"r"}}}
			]}]}`,
			want: "manifest case \"case\" instance \"i\" oracle \"dotnet\" has neither expected nor status",
		},
		{
			name: "unknown oracle expected value",
			data: `{"totals":{},"cases":[{"id":"case","expectedSource":"project","schema":{"expected":"valid"},"instances":[
				{"testName":"i","expected":"valid","oracle":{"xerces":{"expected":"error","reason":"r"}}}
			]}]}`,
			want: "manifest case \"case\" instance \"i\" oracle \"xerces\" has unknown expected value \"error\"",
		},
		{
			name: "unknown oracle status",
			data: `{"totals":{},"cases":[{"id":"case","expectedSource":"project","schema":{"expected":"valid"},"instances":[
				{"testName":"i","expected":"valid","oracle":{"libxml2":{"status":"skipped","reason":"r"}}}
			]}]}`,
			want: "manifest case \"case\" instance \"i\" oracle \"libxml2\" has unknown status \"skipped\"",
		},
		{
			name: "status under w3c",
			data: `{"totals":{},"cases":[{"id":"case","expectedSource":"project","schema":{"expected":"valid"},"instances":[
				{"testName":"i","expected":"valid","oracle":{"w3c":{"status":"blocked","reason":"r"}}}
			]}]}`,
			want: "manifest case \"case\" instance \"i\" oracle \"w3c\" has status, which only cross-validator providers record",
		},
		{
			name: "status under project",
			data: `{"totals":{},"cases":[{"id":"case","expectedSource":"project","schema":{"expected":"valid"},"instances":[
				{"testName":"i","expected":"valid","oracle":{"project":{"status":"unassessed","reason":"r"}}}
			]}]}`,
			want: "manifest case \"case\" instance \"i\" oracle \"project\" has status, which only cross-validator providers record",
		},
		{
			name: "blank oracle reason",
			data: `{"totals":{},"cases":[{"id":"case","expectedSource":"project","schema":{"expected":"valid"},"instances":[
				{"testName":"i","expected":"valid","oracle":{"xmlschema":{"status":"unassessed","reason":" "}}}
			]}]}`,
			want: "manifest case \"case\" instance \"i\" oracle \"xmlschema\" has no reason",
		},
		{
			name: "schema oracle",
			data: `{"totals":{},"cases":[{"id":"case","expectedSource":"project","schema":{
				"expected":"valid","oracle":{"dotnet":{"status":"absent","reason":"r"}}
			}}]}`,
			want: "manifest case \"case\" schema oracle \"dotnet\" has unknown status \"absent\"",
		},
		{
			name: "schema-less instance oracle",
			data: `{"totals":{},"cases":[{"id":"case","expectedSource":"project","instances":[
				{"testName":"i","expected":"valid","oracle":{"xerces":{"expected":"valid"}}}
			]}]}`,
			want: "manifest case \"case\" instance \"i\" oracle \"xerces\" has no reason",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var m manifest
			err := json.Unmarshal([]byte(tt.data), &m)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Unmarshal() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestManifestRejectsUnknownSchemaLessInstanceExpectedValue(t *testing.T) {
	var m manifest
	err := json.Unmarshal([]byte(`{"cases":[{"id":"instance-only","instances":[{"testName":"bad","expected":"unknown"}]}]}`), &m)
	if err == nil || err.Error() != `manifest case "instance-only" instance "bad" has unknown expected value "unknown"` {
		t.Fatalf("Unmarshal() error = %v, want unknown instance expected value", err)
	}
}

func TestSchemaSourcesDoNotPromoteDependencyFiles(t *testing.T) {
	dir := t.TempDir()
	principal := filepath.Join(dir, "principal.xsd")
	dependency := filepath.Join(dir, "dependency.xsd")
	writeHarnessTestFile(t, principal, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`)
	writeHarnessTestFile(t, dependency, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element/></xs:schema>`)

	tc := manifestCase{
		Schema: &manifestSchema{Documents: []manifestDocument{{File: "principal.xsd"}}},
		Files:  []manifestFile{{File: "dependency.xsd", Role: "dependency"}},
	}
	sources := schemaSources(dir, tc)
	if len(sources) != 1 {
		t.Fatalf("len(schemaSources()) = %d, want one manifest schema document", len(sources))
	}
	if _, err := xsd.Compile(sources...); err != nil {
		t.Fatalf("Compile() principal error = %v", err)
	}
	if _, err := xsd.Compile(append(sources, xsd.File(dependency))...); err == nil {
		t.Fatal("Compile() accepted an invalid dependency promoted to a root; fixture does not exercise dependency promotion")
	}
}

func TestSchemaSourcesPreserveManifestRootOrder(t *testing.T) {
	tc := manifestCase{Schema: &manifestSchema{Documents: []manifestDocument{
		{File: "z-first.xsd"},
		{File: "a-second.xsd"},
	}}}
	want := []string{"z-first.xsd", "a-second.xsd"}
	if got := schemaDocumentFiles(tc); !slices.Equal(got, want) {
		t.Fatalf("schemaDocumentFiles() = %v, want %v", got, want)
	}
	sources := schemaSources(t.TempDir(), tc)
	if len(sources) != 2 {
		t.Fatalf("len(schemaSources()) = %d, want both manifest roots", len(sources))
	}
}

func writeHarnessTestFile(t *testing.T, filename, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(filename), err)
	}
	if err := os.WriteFile(filename, []byte(data), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", filename, err)
	}
}

func validateManifestCorpus(dir string, m manifest) error {
	references := make(map[string]struct{})
	for _, name := range manifestFileNames(m) {
		if !isCanonicalCorpusPath(name) {
			return fmt.Errorf("manifest file %q is not a canonical corpus-relative path", name)
		}
		info, err := os.Lstat(filepath.Join(dir, filepath.FromSlash(name)))
		if err != nil {
			return fmt.Errorf("manifest referenced file %q: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("manifest referenced file %q is not regular", name)
		}
		references[name] = struct{}{}
	}

	corpusDir := filepath.Join(dir, "corpus")
	return filepath.WalkDir(corpusDir, func(file string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, file)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		if _, ok := references[name]; !ok {
			return fmt.Errorf("unreferenced corpus file %q", name)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("corpus file %q is not regular", name)
		}
		return nil
	})
}

func manifestFileNames(m manifest) []string {
	var names []string
	for _, tc := range m.Cases {
		if tc.Schema != nil {
			for _, doc := range tc.Schema.Documents {
				names = append(names, doc.File)
			}
		}
		for _, inst := range tc.Instances {
			names = append(names, inst.File)
		}
		for _, file := range tc.Files {
			names = append(names, file.File)
		}
	}
	return names
}

func isCanonicalCorpusPath(name string) bool {
	return name != "" &&
		!strings.ContainsRune(name, '\\') &&
		!path.IsAbs(name) &&
		path.Clean(name) == name &&
		strings.HasPrefix(name, "corpus/")
}

// oracleProviders are the cross-validator keys a measured phase records, as
// the tests/README.md Cross-validator Diagnosis table defines.
var oracleProviders = [...]string{"xerces", "dotnet", "xmlschema", "libxml2"}

func TestProjectCasesCarryOracleMetadata(t *testing.T) {
	dir := testDir(t)
	m := readManifest(t, filepath.Join(dir, "manifest.json"))
	data, err := os.ReadFile(filepath.Join(dir, "oracle-backlog.txt")) //nolint:gosec // Test harness reads repository-owned backlog path.
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	backlog, err := parseOracleBacklog(string(data))
	if err != nil {
		t.Fatalf("oracle backlog: %v", err)
	}
	if err := checkOracleBacklog(m, backlog); err != nil {
		t.Fatal(err)
	}
}

func TestOracleMeasured(t *testing.T) {
	all := func(expected string) map[string]manifestOracle {
		oracle := make(map[string]manifestOracle, len(oracleProviders))
		for _, key := range oracleProviders {
			oracle[key] = manifestOracle{Expected: expected, Reason: "r"}
		}
		return oracle
	}
	without := func(oracle map[string]manifestOracle, keys ...string) map[string]manifestOracle {
		out := maps.Clone(oracle)
		for _, key := range keys {
			delete(out, key)
		}
		return out
	}
	mixed := all("valid")
	mixed["xerces"] = manifestOracle{Expected: "invalid", Reason: "r"}
	mixed["libxml2"] = manifestOracle{Status: "blocked", Reason: "r"}
	for _, tt := range []struct {
		name string
		tc   manifestCase
		want bool
	}{
		{
			name: "every provider on every phase",
			tc:   manifestCase{Schema: &manifestSchema{Oracle: all("valid")}, Instances: []manifestInstance{{Oracle: all("invalid")}}},
			want: true,
		},
		{
			name: "schema phase without one provider",
			tc:   manifestCase{Schema: &manifestSchema{Oracle: without(all("valid"), "dotnet")}},
		},
		{
			name: "schema rejected by every provider needs no instance entries",
			tc:   manifestCase{Schema: &manifestSchema{Oracle: all("invalid")}, Instances: []manifestInstance{{}}},
			want: true,
		},
		{
			name: "instance needs only providers that accepted the schema",
			tc:   manifestCase{Schema: &manifestSchema{Oracle: mixed}, Instances: []manifestInstance{{Oracle: without(all("valid"), "xerces", "libxml2")}}},
			want: true,
		},
		{
			name: "instance without a provider that accepted the schema",
			tc:   manifestCase{Schema: &manifestSchema{Oracle: mixed}, Instances: []manifestInstance{{Oracle: without(all("valid"), "dotnet")}}},
		},
		{
			name: "instance status counts as an entry",
			tc: manifestCase{Schema: &manifestSchema{Oracle: all("valid")}, Instances: []manifestInstance{{Oracle: map[string]manifestOracle{
				"xerces": {Status: "unassessed", Reason: "r"}, "dotnet": {Status: "unassessed", Reason: "r"},
				"xmlschema": {Expected: "valid", Reason: "r"}, "libxml2": {Status: "blocked", Reason: "r"},
			}}}},
			want: true,
		},
		{
			name: "w3c and project entries are not provider entries",
			tc: manifestCase{Schema: &manifestSchema{Oracle: map[string]manifestOracle{
				"w3c": {Expected: "valid", Reason: "r"}, "project": {Expected: "valid", Reason: "r"},
				"xerces": {Expected: "valid", Reason: "r"}, "dotnet": {Expected: "valid", Reason: "r"}, "xmlschema": {Expected: "valid", Reason: "r"},
			}}},
		},
		{
			name: "schema-less case",
			tc:   manifestCase{Instances: []manifestInstance{{Oracle: all("valid")}}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := oracleMeasured(tt.tc); got != tt.want {
				t.Fatalf("oracleMeasured() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseOracleBacklogRejectsBadLines(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
		want string
	}{
		{name: "unsorted", data: "project/b\nproject/a\n", want: `line 2 is not sorted after "project/b"`},
		{name: "duplicate", data: strings.Repeat("project/a\n", 2), want: `line 2 is not sorted after "project/a"`},
		{name: "empty line", data: "project/a\n\nproject/b\n", want: "line 2 is empty"},
		{name: "surrounding space", data: "project/a \n", want: "line 1 has surrounding space"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseOracleBacklog(tt.data)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseOracleBacklog() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestCheckOracleBacklogRejectsStaleOrMissingEntries(t *testing.T) {
	measured := make(map[string]manifestOracle, len(oracleProviders))
	for _, key := range oracleProviders {
		measured[key] = manifestOracle{Expected: "invalid", Reason: "r"}
	}
	acceptedByAll := make(map[string]manifestOracle, len(oracleProviders))
	for _, key := range oracleProviders {
		acceptedByAll[key] = manifestOracle{Expected: "valid", Reason: "r"}
	}
	measuredCase := manifestCase{ID: "project/measured", ExpectedSource: "project", Schema: &manifestSchema{Oracle: measured}}
	unmeasuredCase := manifestCase{ID: "project/unmeasured", ExpectedSource: "project", Schema: &manifestSchema{Oracle: map[string]manifestOracle{"xerces": {Expected: "valid", Reason: "r"}}}}
	schemaOnlyCase := manifestCase{ID: "project/schema-measured", ExpectedSource: "project", Schema: &manifestSchema{Oracle: acceptedByAll}, Instances: []manifestInstance{{}}}
	w3cCase := manifestCase{ID: "w3c/unmeasured", ExpectedSource: "w3c", Schema: &manifestSchema{}}
	m := manifest{Cases: []manifestCase{measuredCase, unmeasuredCase, schemaOnlyCase, w3cCase}}
	if err := checkOracleBacklog(m, []string{"project/unmeasured"}); err == nil || !strings.Contains(err.Error(), `project case "project/schema-measured" lacks oracle metadata`) {
		t.Fatalf("checkOracleBacklog() error = %v, want the unlisted case with an unmeasured instance", err)
	}
	for _, tt := range []struct {
		name    string
		backlog []string
		want    string
	}{
		{name: "unlisted unmeasured case", want: `project case "project/unmeasured" lacks oracle metadata`},
		{name: "listed measured case", backlog: []string{"project/measured", "project/unmeasured"}, want: `oracle backlog lists "project/measured", which is measured`},
		{name: "listed case with a measured phase", backlog: []string{"project/schema-measured", "project/unmeasured"}, want: `oracle backlog lists "project/schema-measured", which has a phase with every provider`},
		{name: "unknown case", backlog: []string{"project/missing", "project/unmeasured"}, want: `oracle backlog lists "project/missing", which is not a project case`},
		{name: "w3c case", backlog: []string{"project/unmeasured", "w3c/unmeasured"}, want: `oracle backlog lists "w3c/unmeasured", which is not a project case`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := checkOracleBacklog(m, tt.backlog)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("checkOracleBacklog() error = %v, want containing %q", err, tt.want)
			}
		})
	}
	if err := checkOracleBacklog(manifest{Cases: []manifestCase{measuredCase, unmeasuredCase, w3cCase}}, []string{"project/unmeasured"}); err != nil {
		t.Fatalf("checkOracleBacklog() error = %v, want nil", err)
	}
}

// parseOracleBacklog reads tests/oracle-backlog.txt: one project case ID per
// line, sorted and unique.
func parseOracleBacklog(data string) ([]string, error) {
	if data == "" {
		return nil, nil
	}
	lines := strings.Split(strings.TrimSuffix(data, "\n"), "\n")
	for i, line := range lines {
		switch {
		case line == "":
			return nil, fmt.Errorf("line %d is empty", i+1)
		case strings.TrimFunc(line, unicode.IsSpace) != line:
			return nil, fmt.Errorf("line %d has surrounding space", i+1)
		case i > 0 && line <= lines[i-1]:
			return nil, fmt.Errorf("line %d is not sorted after %q", i+1, lines[i-1])
		}
	}
	return lines, nil
}

// checkOracleBacklog enforces the oracle rule of the tests/README.md Corpus
// Contract: every project case is measured unless backlog lists it, and
// backlog lists only project cases that are unmeasured and have no phase that
// every provider has measured.
func checkOracleBacklog(m manifest, backlog []string) error {
	listed := make(map[string]bool, len(backlog))
	for _, id := range backlog {
		listed[id] = true
	}
	var errs []error
	for _, tc := range m.Cases {
		if tc.ExpectedSource != "project" {
			continue
		}
		measured := oracleMeasured(tc)
		switch {
		case listed[tc.ID] && measured:
			errs = append(errs, fmt.Errorf("oracle backlog lists %q, which is measured", tc.ID))
		case listed[tc.ID] && hasFourProviderPhase(tc):
			errs = append(errs, fmt.Errorf("oracle backlog lists %q, which has a phase with every provider; measure its other phases", tc.ID))
		case !listed[tc.ID] && !measured:
			errs = append(errs, fmt.Errorf("project case %q lacks oracle metadata for every provider", tc.ID))
		}
		delete(listed, tc.ID)
	}
	for _, id := range slices.Sorted(maps.Keys(listed)) {
		errs = append(errs, fmt.Errorf("oracle backlog lists %q, which is not a project case", id))
	}
	return errors.Join(errs...)
}

// oracleMeasured reports whether every phase of tc is measured: the schema
// phase has an entry for every provider, and each instance phase has an entry
// for every provider whose schema-phase entry is valid.
func oracleMeasured(tc manifestCase) bool {
	if tc.Schema == nil || !hasEveryProvider(tc.Schema.Oracle) {
		return false
	}
	for _, inst := range tc.Instances {
		for _, key := range oracleProviders {
			if tc.Schema.Oracle[key].Expected != "valid" {
				continue
			}
			if _, ok := inst.Oracle[key]; !ok {
				return false
			}
		}
	}
	return true
}

func hasFourProviderPhase(tc manifestCase) bool {
	if tc.Schema != nil && hasEveryProvider(tc.Schema.Oracle) {
		return true
	}
	return slices.ContainsFunc(tc.Instances, func(inst manifestInstance) bool {
		return hasEveryProvider(inst.Oracle)
	})
}

func hasEveryProvider(oracle map[string]manifestOracle) bool {
	for _, key := range oracleProviders {
		if _, ok := oracle[key]; !ok {
			return false
		}
	}
	return true
}
