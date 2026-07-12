package tests_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"
)

type listedPackage struct {
	ImportPath string   `json:"ImportPath"` //nolint:tagliatelle // go list -json uses exported field names.
	Imports    []string `json:"Imports"`    //nolint:tagliatelle // go list -json uses exported field names.
	Deps       []string `json:"Deps"`       //nolint:tagliatelle // go list -json uses exported field names.
}

func TestInternalPhasePackageImportGraph(t *testing.T) {
	packages := listPackages(t, "./internal/...")

	for _, path := range []string{
		"github.com/jacoelho/xsd/internal/compile",
		"github.com/jacoelho/xsd/internal/validate",
	} {
		if _, ok := packages[path]; !ok {
			t.Fatalf("phase package %s is missing", path)
		}
	}

	assertNoDeps(t, packages["github.com/jacoelho/xsd/internal/compile"],
		"github.com/jacoelho/xsd",
		"github.com/jacoelho/xsd/internal/validate",
	)
	assertNoDeps(t, packages["github.com/jacoelho/xsd/internal/validate"],
		"github.com/jacoelho/xsd",
		"github.com/jacoelho/xsd/internal/compile",
	)
}

func TestValidationInputPackageImportGraph(t *testing.T) {
	packages := listPackages(t, "./internal/validate", "./internal/stream")
	assertImports(t, packages["github.com/jacoelho/xsd/internal/validate"],
		"github.com/jacoelho/xsd/internal/stream",
	)
	assertNoDeps(t, packages["github.com/jacoelho/xsd/internal/stream"],
		"github.com/jacoelho/xsd/internal/validate",
		"github.com/jacoelho/xsd",
	)
}

func TestFormatPackageImportGraph(t *testing.T) {
	packages := listPackages(t, "./internal/format")
	assertNoDeps(t, packages["github.com/jacoelho/xsd/internal/format"],
		"github.com/jacoelho/xsd",
		"github.com/jacoelho/xsd/internal/compile",
		"github.com/jacoelho/xsd/internal/validate",
	)
}

func TestPublicLibraryPackageSurface(t *testing.T) {
	packages := listPackages(t, "./...")
	allowed := map[string]bool{
		"github.com/jacoelho/xsd":           true,
		"github.com/jacoelho/xsd/xsderrors": true,
	}
	for path := range packages {
		switch {
		case allowed[path]:
			continue
		case strings.Contains(path, "/internal/"):
			continue
		case strings.Contains(path, "/cmd/"):
			continue
		case path == "github.com/jacoelho/xsd/tests":
			continue
		default:
			t.Fatalf("unexpected public library package %s", path)
		}
	}
}

func TestXMLNamespacePackageImportGraph(t *testing.T) {
	packages := listPackages(t, ".", "./internal/xmlns")
	assertNoDeps(t, packages["github.com/jacoelho/xsd/internal/xmlns"],
		"github.com/jacoelho/xsd",
		"github.com/jacoelho/xsd/internal/compile",
		"github.com/jacoelho/xsd/internal/validate",
	)
}

func TestRuntimeVocabularyPackageImportGraph(t *testing.T) {
	packages := listPackages(t, ".", "./internal/runtime")
	assertNoDeps(t, packages["github.com/jacoelho/xsd/internal/runtime"],
		"github.com/jacoelho/xsd",
		"github.com/jacoelho/xsd/internal/compile",
		"github.com/jacoelho/xsd/internal/validate",
	)
}

func TestSourcePackageImportGraph(t *testing.T) {
	packages := listPackages(t, ".", "./internal/source")
	assertImports(t, packages["github.com/jacoelho/xsd"],
		"github.com/jacoelho/xsd/internal/source",
	)
	assertNoDeps(t, packages["github.com/jacoelho/xsd/internal/source"],
		"github.com/jacoelho/xsd",
		"github.com/jacoelho/xsd/internal/compile",
		"github.com/jacoelho/xsd/internal/validate",
		"github.com/jacoelho/xsd/internal/vocab",
	)
}

func listPackages(t *testing.T, patterns ...string) map[string]listedPackage {
	t.Helper()
	args := append([]string{"list", "-json"}, patterns...)
	//nolint:gosec // test-controlled go list patterns only.
	cmd := exec.CommandContext(t.Context(), "go", args...)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	packages := make(map[string]listedPackage)
	for dec.More() {
		var pkg listedPackage
		if err := dec.Decode(&pkg); err != nil {
			t.Fatalf("decode go list JSON: %v", err)
		}
		packages[pkg.ImportPath] = pkg
	}
	return packages
}

func assertImports(t *testing.T, pkg listedPackage, required ...string) {
	t.Helper()
	for _, want := range required {
		if !slices.Contains(pkg.Imports, want) {
			t.Fatalf("%s does not import required package %s", pkg.ImportPath, want)
		}
	}
}

func assertNoDeps(t *testing.T, pkg listedPackage, forbidden ...string) {
	t.Helper()
	for _, dep := range pkg.Deps {
		for _, bad := range forbidden {
			if forbiddenDependency(dep, bad) {
				t.Fatalf("%s depends on forbidden phase package %s via %s", pkg.ImportPath, bad, dep)
			}
		}
	}
}

func forbiddenDependency(dep, forbidden string) bool {
	if forbidden == "github.com/jacoelho/xsd" {
		return dep == forbidden
	}
	return dep == forbidden || strings.HasPrefix(dep, forbidden+"/")
}
