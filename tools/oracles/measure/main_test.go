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

const testDigest = "sha256:inputs"

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

	if err := record(&inv, results, false, testDigest); err != nil {
		t.Fatalf("record() error = %v", err)
	}

	if got, want := readTestTree(t, "oracles/area.tsv"), measured.String()+"\n"; got != want {
		t.Fatalf("oracles/area.tsv = %q, want %q", got, want)
	}
	if got := readTestTree(t, "oracles/top-level.tsv"); got != keptRow {
		t.Fatalf("oracles/top-level.tsv = %q, want %q", got, keptRow)
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

			if err := record(&inv, results, false, tt.digest); err == nil {
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
