package tests_test

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/internal/corpus"
)

func TestManifestCorpusClosure(t *testing.T) {
	dir := testDir(t)
	m := loadInventory(t, dir)
	if err := validateManifestCorpus(dir, m); err != nil {
		t.Fatal(err)
	}
}

func TestValidateManifestCorpus(t *testing.T) {
	t.Run("duplicate reference", func(t *testing.T) {
		dir := t.TempDir()
		name := "corpus/schema.xsd"
		writeHarnessTestFile(t, filepath.Join(dir, filepath.FromSlash(name)), `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
		m := corpus.Inventory{Cases: []corpus.Case{{
			Schema: &corpus.Schema{Documents: []corpus.Document{{File: name}, {File: name}}},
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
			var m corpus.Inventory
			if tt.reference != "" {
				m.Cases = []corpus.Case{{
					Schema: &corpus.Schema{Documents: []corpus.Document{{File: tt.reference}}},
				}}
			}
			err := validateManifestCorpus(dir, m)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("validateManifestCorpus() error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestSchemaSourcesDoNotPromoteDependencyFiles(t *testing.T) {
	dir := t.TempDir()
	principal := filepath.Join(dir, "principal.xsd")
	dependency := filepath.Join(dir, "dependency.xsd")
	writeHarnessTestFile(t, principal, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`)
	writeHarnessTestFile(t, dependency, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element/></xs:schema>`)

	tc := corpus.Case{
		Schema: &corpus.Schema{Documents: []corpus.Document{{File: "principal.xsd"}}},
		Files:  []corpus.File{{File: "dependency.xsd", Role: "dependency"}},
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
	tc := corpus.Case{Schema: &corpus.Schema{Documents: []corpus.Document{
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

func validateManifestCorpus(dir string, m corpus.Inventory) error {
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

func manifestFileNames(m corpus.Inventory) []string {
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
