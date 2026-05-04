package tests_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/jacoelho/xsd"
)

type manifest struct {
	Cases []manifestCase `json:"cases"`
}

type manifestCase struct {
	ID             string             `json:"id"`
	ExpectedSource string             `json:"expectedSource"`
	Schema         manifestSchema     `json:"schema"`
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

func TestHarness(t *testing.T) {
	dir := testDir(t)
	m := readManifest(t, filepath.Join(dir, "manifest.json"))
	for _, source := range manifestSources(m) {
		t.Run(source, func(t *testing.T) {
			for _, tc := range m.Cases {
				if tc.ExpectedSource != source {
					continue
				}
				t.Run(tc.ID, func(t *testing.T) {
					runCase(t, dir, tc)
				})
			}
		})
	}
}

func testDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	return filepath.Dir(file)
}

func readManifest(t *testing.T, path string) manifest {
	t.Helper()
	data, err := os.ReadFile(path)
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

func runCase(t *testing.T, dir string, tc manifestCase) {
	t.Helper()
	engine, err := xsd.Compile(schemaSources(dir, tc)...)
	switch tc.Schema.Expected {
	case "valid":
		if err != nil {
			skipUnsupported(t, err)
			t.Fatalf("Compile() error = %v", err)
		}
	case "invalid":
		if err == nil {
			t.Fatalf("Compile() expected invalid schema")
		}
		if tc.Schema.ErrorCode != "" {
			expectErrorCode(t, err, tc.Schema.ErrorCode)
			return
		}
		skipUnsupported(t, err)
		return
	default:
		t.Skipf("schema expected value is %q", tc.Schema.Expected)
	}
	for _, inst := range tc.Instances {
		t.Run(instanceName(inst), func(t *testing.T) {
			validateInstance(t, dir, engine, inst)
		})
	}
}

func validateInstance(t *testing.T, dir string, engine *xsd.Engine, inst manifestInstance) {
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
	switch inst.Expected {
	case "valid":
		if err != nil {
			skipUnsupported(t, err)
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
		skipUnsupported(t, err)
	default:
		t.Skipf("instance expected value is %q", inst.Expected)
	}
}

func expectErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	xerr, ok := errors.AsType[*xsd.Error](err)
	if !ok {
		t.Fatalf("error %v is not *xsd.Error", err)
	}
	if string(xerr.Code) != code {
		t.Fatalf("error code = %s, want %s; err=%v", xerr.Code, code, err)
	}
}

func skipUnsupported(t *testing.T, err error) {
	t.Helper()
	if !xsd.IsUnsupported(err) {
		return
	}
	if xerr, ok := errors.AsType[*xsd.Error](err); ok {
		t.Skipf("unsupported feature %s: %s", xerr.Code, xerr.Message)
	}
	t.Skipf("unsupported feature: %v", err)
}

func schemaSources(dir string, tc manifestCase) []xsd.SchemaSource {
	seen := make(map[string]bool)
	sources := make([]xsd.SchemaSource, 0, len(tc.Schema.Documents))
	for _, doc := range tc.Schema.Documents {
		addSource(dir, &sources, seen, doc.File)
	}
	for _, file := range tc.Files {
		if file.Role == "principal" || file.Role == "dependency" {
			addSource(dir, &sources, seen, file.File)
		}
	}
	return sources
}

func addSource(dir string, sources *[]xsd.SchemaSource, seen map[string]bool, name string) {
	if seen[name] {
		return
	}
	seen[name] = true
	*sources = append(*sources, xsd.File(harnessFile(dir, name)))
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
