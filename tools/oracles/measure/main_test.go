package main

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jacoelho/xsd/internal/corpus"
)

const (
	testDigest  = "sha256:inputs"
	testDate    = "2026-11-24"
	testProfile = `{"providers":{"xerces":"Xerces-J 2.12.2","dotnet":".NET 10.0.14","xmlschema":"xmlschema 4.3.2","libxml2":"libxml2 2.9.13"},"date":"2026-11-19","image":"` + testDigest + `"}`
)

func writeTestTree(t *testing.T, files map[string]string) {
	t.Helper()
	for name, data := range files {
		path := filepath.Join("tests", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func readTestTree(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("tests", filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRecordMeasuresACaseOfAShardWithNoObservationFile(t *testing.T) {
	t.Chdir(t.TempDir())
	const keptRow = "project/a\tschema\txerces\tschema-valid\t\n"
	writeTestTree(t, map[string]string{
		corpus.IndexPath:                  `{"version":"2.0","specification":"XSD 1.0","shards":["project/top-level","project/area"]}`,
		"manifest/project/top-level.json": `{"cases":[{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"a.xsd"}]}}]}`,
		"manifest/project/area.json":      `{"cases":[{"id":"project/area/b","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"b.xsd"}]}}]}`,
		corpus.ProfilePath:                `{"providers":{"xerces":"Xerces-J 2.12.2","dotnet":".NET 10.0.14","xmlschema":"xmlschema 4.3.2","libxml2":"libxml2 2.9.13"},"date":"2026-11-19","image":"` + testDigest + `"}`,
		"oracles/top-level.tsv":           keptRow,
	})
	inv, err := corpus.LoadInventory(os.DirFS("tests"))
	if err != nil {
		t.Fatal(err)
	}
	measured := corpus.Row{CaseID: "project/area/b", Phase: corpus.SchemaPhase, Provider: corpus.Xerces, Verdict: corpus.SchemaInvalid, Message: "b.xsd:1: bad"}
	results := map[string]caseResult{"project/area/b": {
		versions: map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.2"},
		rows:     []corpus.Row{measured},
	}}

	if err := record(&inv, results, false, testDigest, testDate); err != nil {
		t.Fatalf("record() error = %v", err)
	}

	if got, want := readTestTree(t, "oracles/area.tsv"), "project/area/b\tschema\txerces\tschema-invalid\tb.xsd:1: bad\n"; got != want {
		t.Fatalf("oracles/area.tsv = %q, want %q", got, want)
	}
	if got := readTestTree(t, "oracles/top-level.tsv"); got != keptRow {
		t.Fatalf("oracles/top-level.tsv = %q, want %q", got, keptRow)
	}
}

func TestRecordPartialRunDropsRowsOfRemovedCases(t *testing.T) {
	t.Chdir(t.TempDir())
	const keptRow = "project/a\tschema\txerces\tschema-valid\t\n"
	writeTestTree(t, map[string]string{
		corpus.IndexPath:                  `{"version":"2.0","specification":"XSD 1.0","shards":["project/top-level"]}`,
		"manifest/project/top-level.json": `{"cases":[{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"a.xsd"}]}},{"id":"project/renamed","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"r.xsd"}]}}]}`,
		corpus.ProfilePath:                `{"providers":{"xerces":"Xerces-J 2.12.2","dotnet":".NET 10.0.14","xmlschema":"xmlschema 4.3.2","libxml2":"libxml2 2.9.13"},"date":"2026-11-19","image":"` + testDigest + `"}`,
		"oracles/top-level.tsv":           keptRow + "project/old-name\tschema\txerces\tschema-valid\t\n",
	})
	inv, err := corpus.LoadInventory(os.DirFS("tests"))
	if err != nil {
		t.Fatal(err)
	}
	measured := corpus.Row{CaseID: "project/renamed", Phase: corpus.SchemaPhase, Provider: corpus.Xerces, Verdict: corpus.SchemaValid}
	results := map[string]caseResult{"project/renamed": {
		versions: map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.2"},
		rows:     []corpus.Row{measured},
	}}

	if err := record(&inv, results, false, testDigest, testDate); err != nil {
		t.Fatalf("record() error = %v", err)
	}

	if got, want := readTestTree(t, "oracles/top-level.tsv"), keptRow+"project/renamed\tschema\txerces\tschema-valid\t\n"; got != want {
		t.Fatalf("oracles/top-level.tsv = %q, want %q", got, want)
	}
}

func TestRecordPartialRunKeepsW3COverrideRowsAndDropsRowsOfUnmeasuredW3CCases(t *testing.T) {
	t.Chdir(t.TempDir())
	const (
		overrideRow = "w3c/msMeta/Regex_w3c.xml/overridden\tschema\txerces\tschema-valid\t\n"
		upstreamRow = "w3c/msMeta/Regex_w3c.xml/upstream\tschema\txerces\tschema-valid\t\n"
	)
	writeTestTree(t, map[string]string{
		corpus.IndexPath:                  `{"version":"2.0","specification":"XSD 1.0","shards":["project/top-level","w3c/msMeta/Regex_w3c"]}`,
		"manifest/project/top-level.json": `{"cases":[{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"a.xsd"}]}}]}`,
		"manifest/w3c/msMeta/Regex_w3c.json": `{"cases":[` +
			`{"id":"w3c/msMeta/Regex_w3c.xml/overridden","expectedSource":"w3c","schema":{"expected":"invalid","documents":[{"file":"o.xsd"}],"oracle":{"w3c":{"expected":"valid","reason":"r"}}}},` +
			`{"id":"w3c/msMeta/Regex_w3c.xml/upstream","expectedSource":"w3c","schema":{"expected":"valid","documents":[{"file":"u.xsd"}]}}]}`,
		corpus.ProfilePath:      `{"providers":{"xerces":"Xerces-J 2.12.2","dotnet":".NET 10.0.14","xmlschema":"xmlschema 4.3.2","libxml2":"libxml2 2.9.13"},"date":"2026-11-19","image":"` + testDigest + `"}`,
		"oracles/top-level.tsv": "",
		"oracles/w3c.tsv":       overrideRow + upstreamRow,
	})
	inv, err := corpus.LoadInventory(os.DirFS("tests"))
	if err != nil {
		t.Fatal(err)
	}
	measured := corpus.Row{CaseID: "project/a", Phase: corpus.SchemaPhase, Provider: corpus.Xerces, Verdict: corpus.SchemaValid}
	results := map[string]caseResult{"project/a": {
		versions: map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.2"},
		rows:     []corpus.Row{measured},
	}}

	if err := record(&inv, results, false, testDigest, testDate); err != nil {
		t.Fatalf("record() error = %v", err)
	}

	if got := readTestTree(t, "oracles/w3c.tsv"); got != overrideRow {
		t.Fatalf("oracles/w3c.tsv = %q, want only the overridden case's row %q", got, overrideRow)
	}
	if got, want := readTestTree(t, "oracles/top-level.tsv"), "project/a\tschema\txerces\tschema-valid\t\n"; got != want {
		t.Fatalf("oracles/top-level.tsv = %q, want %q", got, want)
	}
}

func TestSelectCasesAcceptsOnlyMeasuredCases(t *testing.T) {
	inv := corpus.Inventory{Cases: []corpus.Case{
		{ID: "project/a", ExpectedSource: corpus.SourceProject},
		{ID: "w3c/s/t.xml/overridden", ExpectedSource: corpus.SourceW3C, Schema: &corpus.Schema{Expected: corpus.ExpectedInvalid, Oracle: map[string]corpus.Provenance{corpus.SourceW3C: {Expected: corpus.ExpectedValid, Reason: "r"}}}},
		{ID: "w3c/s/t.xml/upstream", ExpectedSource: corpus.SourceW3C, Schema: &corpus.Schema{Expected: corpus.ExpectedValid}},
	}}

	every, all, err := selectCases(&inv, nil)
	if err != nil || !all || len(every) != 2 || every[0].ID != "project/a" || every[1].ID != "w3c/s/t.xml/overridden" {
		t.Fatalf("selectCases(nil) = %v, %v, %v; want project/a and the overridden W3C case, all", every, all, err)
	}
	if _, all, err := selectCases(&inv, []string{"w3c/s/t.xml/overridden", "project/a"}); err != nil || !all {
		t.Fatalf("selectCases(every member) all = %v, error = %v; want true, nil", all, err)
	}
	if _, _, err := selectCases(&inv, []string{"w3c/s/t.xml/upstream"}); err == nil || !strings.Contains(err.Error(), "is not a measured case") {
		t.Fatalf("selectCases(upstream W3C case) error = %v, want not a measured case", err)
	}
}

func TestRecordRefusesAPartialRunAgainstAnotherProfileWithoutWriting(t *testing.T) {
	tests := []struct {
		name     string
		digest   string
		versions map[corpus.Provider]string
	}{
		{"image digest", "sha256:other", map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.2"}},
		{"provider version", testDigest, map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.3"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			writeTestTree(t, map[string]string{
				corpus.IndexPath:                  `{"version":"2.0","specification":"XSD 1.0","shards":["project/top-level","project/area"]}`,
				"manifest/project/top-level.json": `{"cases":[{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"a.xsd"}]}}]}`,
				"manifest/project/area.json":      `{"cases":[{"id":"project/area/b","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"b.xsd"}]}}]}`,
				corpus.ProfilePath:                `{"providers":{"xerces":"Xerces-J 2.12.2","dotnet":".NET 10.0.14","xmlschema":"xmlschema 4.3.2","libxml2":"libxml2 2.9.13"},"date":"2026-11-19","image":"` + testDigest + `"}`,
				"oracles/top-level.tsv":           "project/a\tschema\txerces\tschema-valid\t\n",
			})
			inv, err := corpus.LoadInventory(os.DirFS("tests"))
			if err != nil {
				t.Fatal(err)
			}
			before := snapshotTestTree(t)
			results := map[string]caseResult{"project/area/b": {
				versions: tt.versions,
				rows:     []corpus.Row{{CaseID: "project/area/b", Phase: corpus.SchemaPhase, Provider: corpus.Xerces, Verdict: corpus.SchemaValid}},
			}}

			if err := record(&inv, results, false, tt.digest, testDate); err == nil {
				t.Fatal("record() error = nil, want a profile mismatch")
			}

			if after := snapshotTestTree(t); !maps.Equal(after, before) {
				t.Fatalf("tree after refused run = %q, want %q", after, before)
			}
		})
	}
}

func snapshotTestTree(t *testing.T) map[string]string {
	t.Helper()
	files := make(map[string]string)
	err := filepath.WalkDir("tests", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path) //nolint:gosec // The path comes from walking the test's own temporary tree.
		files[path] = string(data)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestBuildContextHoldsExactlyTheInputsRelativeToTheContextRoot(t *testing.T) {
	inputs := []imageInput{
		{path: "tools/oracles/Dockerfile", data: []byte("FROM scratch\n")},
		{path: "tools/oracles/libxml2/oracle.sh", data: []byte("#!/bin/sh\n")},
	}
	data, err := buildContext("tools/oracles", inputs)
	if err != nil {
		t.Fatal(err)
	}
	type entry struct {
		name string
		mode int64
		data string
	}
	var got []entry
	tr := tar.NewReader(bytes.NewReader(data))
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, entry{hdr.Name, hdr.Mode, string(body)})
	}
	want := []entry{{"Dockerfile", 0o755, "FROM scratch\n"}, {"libxml2/oracle.sh", 0o755, "#!/bin/sh\n"}}
	if !slices.Equal(got, want) {
		t.Fatalf("build context = %+v, want %+v", got, want)
	}
}

func TestCacheKeyChangesWithTheBytesOfEveryRunFile(t *testing.T) {
	t.Chdir(t.TempDir())
	files := map[string]string{"a.xsd": "<a/>", "b.xsd": "<b/>", "i.xml": "<i/>"}
	writeTestTree(t, files)
	c := &corpus.Case{ID: "project/a", Schema: &corpus.Schema{Documents: []corpus.Document{{File: "a.xsd"}, {File: "b.xsd"}}}}
	in := phaseInputs(c, "i.xml")
	base, err := cacheKey(testDigest, c, in)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range files {
		writeTestTree(t, map[string]string{name: data + " "})
		got, err := cacheKey(testDigest, c, in)
		if err != nil {
			t.Fatal(err)
		}
		if got == base {
			t.Errorf("cacheKey() after changing %s = the key before", name)
		}
		writeTestTree(t, map[string]string{name: data})
	}
}

func installFakeDocker(t *testing.T, runScript string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake run.sh and docker are shell scripts")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	calls := filepath.Join(dir, "docker-calls")
	bin := filepath.Join(dir, "bin")
	scripts := map[string]string{
		filepath.Join(bin, "docker"):                "#!/bin/sh\necho \"$*\" >> '" + calls + "'\n",
		filepath.Join("tools", "oracles", "run.sh"): runScript,
	}
	for name, body := range scripts {
		if err := os.MkdirAll(filepath.Dir(name), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0o700); err != nil { //nolint:gosec // The fake commands must be executable.
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	return calls
}

func dockerCalls(t *testing.T, calls string) []string {
	t.Helper()
	data, err := os.ReadFile(calls) //nolint:gosec // The path is the test's own temporary file.
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func TestRunOraclesInterruptedRemovesTheSlotContainersOnce(t *testing.T) {
	calls := installFakeDocker(t, "#!/bin/sh\necho started\nexec sleep 60\n")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := runOracles(ctx, "slot", []string{"a.xsd"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("runOracles() error = %v, want a deadline", err)
	}

	if got, want := dockerCalls(t, calls), []string{"ps --all --no-trunc --format {{.ID}}\t{{.Image}}"}; !slices.Equal(got, want) {
		t.Fatalf("docker calls = %q, want %q", got, want)
	}
}

func TestRunOraclesThatNeverStartedRemovesNoContainers(t *testing.T) {
	calls := installFakeDocker(t, "#!/bin/sh\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := runOracles(ctx, "slot", []string{"a.xsd"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("runOracles() error = %v, want a cancellation", err)
	}

	if got := dockerCalls(t, calls); got != nil {
		t.Fatalf("docker calls = %q, want none", got)
	}
}

func TestRecordAllRewritesEveryFileAndTheProfile(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestTree(t, map[string]string{
		corpus.IndexPath:                  `{"version":"2.0","specification":"XSD 1.0","shards":["project/top-level","project/area"]}`,
		"manifest/project/top-level.json": `{"cases":[{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"a.xsd"}]}}]}`,
		"manifest/project/area.json":      `{"cases":[{"id":"project/area/b","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"b.xsd"}]}}]}`,
		corpus.ProfilePath:                testProfile,
		"oracles/top-level.tsv":           "project/a\tschema\txerces\tschema-invalid\told\nproject/removed\tschema\txerces\tschema-valid\t\n",
		"oracles/area.tsv":                "project/area/b\tschema\txerces\tschema-invalid\told\n",
	})
	inv, err := corpus.LoadInventory(os.DirFS("tests"))
	if err != nil {
		t.Fatal(err)
	}
	results := map[string]caseResult{
		"project/a": {
			versions: map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.3", corpus.DotNet: ".NET 10.0.15"},
			rows:     []corpus.Row{{CaseID: "project/a", Phase: corpus.SchemaPhase, Provider: corpus.Xerces, Verdict: corpus.SchemaValid}},
		},
		"project/area/b": {
			versions: map[corpus.Provider]string{corpus.XMLSchema: "xmlschema 4.3.3", corpus.LibXML2: "libxml2 2.9.14"},
			rows:     []corpus.Row{{CaseID: "project/area/b", Phase: corpus.SchemaPhase, Provider: corpus.LibXML2, Verdict: corpus.SchemaInvalid, Message: "b.xsd:2: bad"}},
		},
	}

	if err := record(&inv, results, true, "sha256:new", testDate); err != nil {
		t.Fatalf("record() error = %v", err)
	}

	want := map[string]string{
		"oracles/top-level.tsv": "project/a\tschema\txerces\tschema-valid\t\n",
		"oracles/area.tsv":      "project/area/b\tschema\tlibxml2\tschema-invalid\tb.xsd:2: bad\n",
		corpus.ProfilePath: `{
  "providers": {
    "dotnet": ".NET 10.0.15",
    "libxml2": "libxml2 2.9.14",
    "xerces": "Xerces-J 2.12.3",
    "xmlschema": "xmlschema 4.3.3"
  },
  "date": "2026-11-24",
  "image": "sha256:new"
}
`,
	}
	for name, data := range want {
		if got := readTestTree(t, name); got != data {
			t.Errorf("%s = %q, want %q", name, got, data)
		}
	}
}

func TestRecordWritesNothingWhenAnyOutputFileDoesNotValidate(t *testing.T) {
	tests := []struct {
		name     string
		all      bool
		versions map[corpus.Provider]string
		message  string
		want     string
	}{
		{"row of the last file", false, map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.2"}, "b.xsd:1:\tbad", "a tab or a line break in its message"},
		{"profile without a provider version", true, map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.2"}, "", "has no version for dotnet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			writeTestTree(t, map[string]string{
				corpus.IndexPath:                  `{"version":"2.0","specification":"XSD 1.0","shards":["project/top-level","project/zz"]}`,
				"manifest/project/top-level.json": `{"cases":[{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"a.xsd"}]}}]}`,
				"manifest/project/zz.json":        `{"cases":[{"id":"project/zz/b","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"b.xsd"}]}}]}`,
				corpus.ProfilePath:                testProfile,
				"oracles/top-level.tsv":           "project/a\tschema\txerces\tschema-invalid\told\n",
				"oracles/zz.tsv":                  "",
			})
			inv, err := corpus.LoadInventory(os.DirFS("tests"))
			if err != nil {
				t.Fatal(err)
			}
			before := snapshotTestTree(t)
			results := map[string]caseResult{
				"project/a":    {versions: tt.versions, rows: []corpus.Row{{CaseID: "project/a", Phase: corpus.SchemaPhase, Provider: corpus.Xerces, Verdict: corpus.SchemaValid}}},
				"project/zz/b": {versions: tt.versions, rows: []corpus.Row{{CaseID: "project/zz/b", Phase: corpus.SchemaPhase, Provider: corpus.Xerces, Verdict: corpus.SchemaInvalid, Message: tt.message}}},
			}

			if err := record(&inv, results, tt.all, testDigest, testDate); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("record() error = %v, want containing %q", err, tt.want)
			}

			if after := snapshotTestTree(t); !maps.Equal(after, before) {
				t.Fatalf("tree after refused run = %q, want %q", after, before)
			}
		})
	}
}

func TestParseRunReturnsRowsAndReportedVersions(t *testing.T) {
	out := "oracle\tversion\tverdict\tmessage\n" +
		"xerces\tXerces-J 2.12.2\tvalid\t\n" +
		"dotnet\t.NET 10.0.14\tinvalid\ti.xml:3: bad value\n" +
		"libxml2\t?\terror\tno output\n" +
		"xmlschema\t4.3.2\tblocked\tnon-file reference\n"

	rows, versions, err := parseRun("project/a", "i", []byte(out))
	if err != nil {
		t.Fatalf("parseRun() error = %v", err)
	}

	wantRows := []corpus.Row{
		{CaseID: "project/a", Phase: "i", Provider: corpus.Xerces, Verdict: corpus.Valid},
		{CaseID: "project/a", Phase: "i", Provider: corpus.DotNet, Verdict: corpus.Invalid, Message: "i.xml:3: bad value"},
		{CaseID: "project/a", Phase: "i", Provider: corpus.LibXML2, Verdict: corpus.Error, Message: "no output"},
		{CaseID: "project/a", Phase: "i", Provider: corpus.XMLSchema, Verdict: corpus.Blocked, Message: "non-file reference"},
	}
	if !slices.Equal(rows, wantRows) {
		t.Errorf("rows = %+v, want %+v", rows, wantRows)
	}
	wantVersions := map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.2", corpus.DotNet: ".NET 10.0.14", corpus.XMLSchema: "4.3.2"}
	if !maps.Equal(versions, wantVersions) {
		t.Errorf("versions = %v, want %v", versions, wantVersions)
	}
}

func TestParseRunRejectsMalformedOutput(t *testing.T) {
	const header = "oracle\tversion\tverdict\tmessage\n"
	const rest = "dotnet\tv\tvalid\t\nlibxml2\tv\tvalid\t\nxmlschema\tv\tvalid\t\n"
	for _, tt := range []struct {
		name, out, want string
	}{
		{name: "other header", out: "oracle\tverdict\tmessage\nxerces\tv\tvalid\t\n" + rest, want: "unexpected run.sh output"},
		{name: "missing provider", out: header + rest, want: "unexpected run.sh output"},
		{name: "tab in message", out: header + "xerces\tv\tinvalid\ta\tb\n" + rest, want: "has 5 fields, want 4"},
		{name: "unknown provider", out: header + "saxon\tv\tvalid\t\n" + rest, want: `unknown provider "saxon"`},
		{name: "unknown verdict", out: header + "xerces\tv\taccepted\t\n" + rest, want: `unknown verdict "accepted"`},
		{name: "substituted provider", out: header + "xerces\tv\tvalid\t\nxerces\tv\tvalid\t\nlibxml2\tv\tvalid\t\nxmlschema\tv\tvalid\t\n", want: "run.sh reported xerces twice"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, err := parseRun("project/a", corpus.SchemaPhase, []byte(tt.out)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("parseRun() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestProviderVersionsMergesAgreeingRuns(t *testing.T) {
	results := map[string]caseResult{
		"project/a": {versions: map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.2", corpus.DotNet: ".NET 10.0.14"}},
		"project/b": {versions: map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.2", corpus.LibXML2: "libxml2 2.9.13"}},
		"project/c": {},
	}

	got, err := providerVersions(results)

	want := map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.2", corpus.DotNet: ".NET 10.0.14", corpus.LibXML2: "libxml2 2.9.13"}
	if err != nil || !maps.Equal(got, want) {
		t.Fatalf("providerVersions() = %v, %v; want %v", got, err, want)
	}
}

func TestProviderVersionsRejectsTwoVersionsOfOneProvider(t *testing.T) {
	results := map[string]caseResult{
		"project/a": {versions: map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.2"}},
		"project/b": {versions: map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.3"}},
	}

	if got, err := providerVersions(results); err == nil || !strings.Contains(err.Error(), "reported xerces version") {
		t.Fatalf("providerVersions() = %v, %v; want two xerces versions refused", got, err)
	}
}

// countingRun returns a fake run.sh that appends a line to run-calls in the
// working directory on each invocation and then runs body.
func countingRun(body string) string {
	return "#!/bin/sh\necho \"$*\" >> run-calls\n" + body
}

func runCalls(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile("run-calls")
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func schemaCase(t *testing.T, id string) *corpus.Case {
	t.Helper()
	file := strings.ReplaceAll(id, "/", "-") + ".xsd"
	writeTestTree(t, map[string]string{file: "<schema/>"})
	return &corpus.Case{ID: id, ExpectedSource: corpus.SourceProject, Schema: &corpus.Schema{Expected: corpus.ExpectedValid, Documents: []corpus.Document{{File: file}}}}
}

func TestMeasureAllStartsNoCaseAfterTheFirstFailure(t *testing.T) {
	installFakeDocker(t, countingRun("echo broken >&2\nexit 1\n"))
	cases := []*corpus.Case{schemaCase(t, "project/a"), schemaCase(t, "project/b"), schemaCase(t, "project/c")}
	m := &measurer{digest: testDigest, cache: t.TempDir()}

	_, err := m.measureAll(context.Background(), []string{"slot"}, cases)

	if err == nil || !strings.Contains(err.Error(), "project/a: phase schema: run.sh") {
		t.Fatalf("measureAll() error = %v, want the failure of project/a", err)
	}
	if got := runCalls(t); len(got) != 1 {
		t.Fatalf("run.sh calls = %q, want only the first case's", got)
	}
}

// passingRun is a fake run.sh that prints a schema-valid verdict for every
// provider.
const passingRun = "printf 'oracle\\tversion\\tverdict\\tmessage\\n" +
	"xerces\\tX\\tschema-valid\\t\\n" +
	"dotnet\\tD\\tschema-valid\\t\\n" +
	"libxml2\\tL\\tschema-valid\\t\\n" +
	"xmlschema\\tP\\tschema-valid\\t\\n'\n"

func TestMeasurePhaseRunsAndCachesOnlyOnACacheMiss(t *testing.T) {
	installFakeDocker(t, countingRun(passingRun))
	c := schemaCase(t, "project/a")
	m := &measurer{digest: testDigest, cache: t.TempDir()}
	key, err := cacheKey(testDigest, c, phaseInputs(c, ""))
	if err != nil {
		t.Fatal(err)
	}
	cached := filepath.Join(m.cache, key[:2], key+".tsv")
	measure := func() os.FileInfo {
		t.Helper()
		res := caseResult{versions: make(map[corpus.Provider]string)}
		if _, err := m.measurePhase(context.Background(), "slot", c, corpus.SchemaPhase, "", &res); err != nil {
			t.Fatalf("measurePhase() error = %v", err)
		}
		info, err := os.Stat(cached)
		if err != nil {
			t.Fatalf("cache file: %v", err)
		}
		return info
	}

	miss := measure()
	hit := measure()

	if got := runCalls(t); len(got) != 1 {
		t.Fatalf("run.sh calls = %q, want one, for the miss", got)
	}
	if !os.SameFile(miss, hit) {
		t.Fatal("a cache hit replaced the cache file")
	}
}

func TestMeasurePhaseFailsOnAnUnreadableCacheWithoutRunning(t *testing.T) {
	installFakeDocker(t, countingRun(passingRun))
	c := schemaCase(t, "project/a")
	m := &measurer{digest: testDigest, cache: t.TempDir()}
	key, err := cacheKey(testDigest, c, phaseInputs(c, ""))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(m.cache, key[:2], key+".tsv"), 0o750); err != nil {
		t.Fatal(err)
	}
	res := caseResult{versions: make(map[corpus.Provider]string)}

	if _, err := m.measurePhase(context.Background(), "slot", c, corpus.SchemaPhase, "", &res); err == nil || !strings.Contains(err.Error(), "read cache") {
		t.Fatalf("measurePhase() error = %v, want a cache read failure", err)
	}
	if got := runCalls(t); got != nil {
		t.Fatalf("run.sh calls = %q, want none", got)
	}
}

func TestRecordAllWritesTheProfileOnlyAfterEveryObservationFile(t *testing.T) {
	t.Chdir(t.TempDir())
	writeTestTree(t, map[string]string{
		corpus.IndexPath:                  `{"version":"2.0","specification":"XSD 1.0","shards":["project/top-level","project/zz"]}`,
		"manifest/project/top-level.json": `{"cases":[{"id":"project/a","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"a.xsd"}]}}]}`,
		"manifest/project/zz.json":        `{"cases":[{"id":"project/zz/b","expectedSource":"project","schema":{"expected":"valid","documents":[{"file":"b.xsd"}]}}]}`,
		corpus.ProfilePath:                testProfile,
		"oracles/top-level.tsv":           "",
		"oracles/zz.tsv":                  "",
	})
	if err := os.Mkdir(filepath.Join("tests", "oracles", "zz.tsv.tmp"), 0o750); err != nil {
		t.Fatal(err)
	}
	inv, err := corpus.LoadInventory(os.DirFS("tests"))
	if err != nil {
		t.Fatal(err)
	}
	versions := map[corpus.Provider]string{corpus.Xerces: "Xerces-J 2.12.3", corpus.DotNet: ".NET 10.0.15", corpus.XMLSchema: "xmlschema 4.3.3", corpus.LibXML2: "libxml2 2.9.14"}
	results := map[string]caseResult{
		"project/a":    {versions: versions, rows: []corpus.Row{{CaseID: "project/a", Phase: corpus.SchemaPhase, Provider: corpus.Xerces, Verdict: corpus.SchemaValid}}},
		"project/zz/b": {versions: versions, rows: []corpus.Row{{CaseID: "project/zz/b", Phase: corpus.SchemaPhase, Provider: corpus.Xerces, Verdict: corpus.SchemaValid}}},
	}

	if err := record(&inv, results, true, "sha256:new", testDate); err == nil {
		t.Fatal("record() error = nil, want the failed write of oracles/zz.tsv")
	}

	if got := readTestTree(t, corpus.ProfilePath); got != testProfile {
		t.Fatalf("profile after a failed observation write = %q, want the earlier %q", got, testProfile)
	}
}
