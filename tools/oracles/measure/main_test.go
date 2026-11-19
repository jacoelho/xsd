package main

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

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

func TestBuildContextHoldsExactlyTheInputsRelativeToTheContextRoot(t *testing.T) {
	inputs := []imageInput{
		{path: "tools/oracles/Dockerfile", data: []byte("FROM scratch\n"), mode: 0o644},
		{path: "tools/oracles/libxml2/oracle.sh", data: []byte("#!/bin/sh\n"), mode: 0o755},
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
	want := []entry{{"Dockerfile", 0o644, "FROM scratch\n"}, {"libxml2/oracle.sh", 0o755, "#!/bin/sh\n"}}
	if !slices.Equal(got, want) {
		t.Fatalf("build context = %+v, want %+v", got, want)
	}
}
