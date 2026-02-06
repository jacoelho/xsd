package architecture_test

import (
	"os/exec"
	"strings"
	"testing"
)

func TestInternalPackageBoundaries(t *testing.T) {
	t.Parallel()

	graph := listInternalImports(t)

	for pkg, imports := range graph {
		if isUtilityPackage(pkg) {
			for _, imp := range imports {
				if isOrchestrationPackage(imp) {
					t.Fatalf("utility package %s imports orchestration package %s", pkg, imp)
				}
			}
		}

		if pkg == "github.com/jacoelho/xsd/internal/runtime" {
			for _, imp := range imports {
				if isUpperLayerPackage(imp) {
					t.Fatalf("runtime package imports upper-layer package %s", imp)
				}
			}
		}
	}

	for pkg, imports := range graph {
		for _, imp := range imports {
			if isLegacyPath(imp) {
				t.Fatalf("legacy internal path still imported in %s: %s", pkg, imp)
			}
		}
	}
}

func listInternalImports(t *testing.T) map[string][]string {
	t.Helper()

	cmd := exec.Command("go", "list", "-f", "{{.ImportPath}} {{join .Imports \" \"}}", "github.com/jacoelho/xsd/internal/...")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list failed: %v\n%s", err, string(out))
	}

	graph := make(map[string][]string)
	lines := strings.SplitSeq(strings.TrimSpace(string(out)), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg := fields[0]
		if !strings.HasPrefix(pkg, "github.com/jacoelho/xsd/internal/") {
			continue
		}
		imports := make([]string, 0, len(fields)-1)
		for _, imp := range fields[1:] {
			if strings.HasPrefix(imp, "github.com/jacoelho/xsd/internal/") {
				imports = append(imports, imp)
			}
		}
		graph[pkg] = imports
	}
	if len(graph) == 0 {
		t.Fatal("no internal packages found")
	}
	return graph
}

func isUtilityPackage(pkg string) bool {
	switch pkg {
	case "github.com/jacoelho/xsd/internal/ids":
		return true
	case "github.com/jacoelho/xsd/internal/num":
		return true
	case "github.com/jacoelho/xsd/internal/traversal":
		return true
	case "github.com/jacoelho/xsd/internal/typegraph":
		return true
	case "github.com/jacoelho/xsd/internal/typeops":
		return true
	case "github.com/jacoelho/xsd/internal/value":
		return true
	case "github.com/jacoelho/xsd/internal/value/datetime":
		return true
	case "github.com/jacoelho/xsd/internal/value/temporal":
		return true
	case "github.com/jacoelho/xsd/internal/valuekey":
		return true
	case "github.com/jacoelho/xsd/internal/whitespace":
		return true
	case "github.com/jacoelho/xsd/internal/xmlnames":
		return true
	case "github.com/jacoelho/xsd/internal/xpath":
		return true
	case "github.com/jacoelho/xsd/internal/xsdxml":
		return true
	default:
		return false
	}
}

func isOrchestrationPackage(pkg string) bool {
	switch pkg {
	case "github.com/jacoelho/xsd/internal/source":
		return true
	case "github.com/jacoelho/xsd/internal/pipeline":
		return true
	case "github.com/jacoelho/xsd/internal/semantic":
		return true
	case "github.com/jacoelho/xsd/internal/semanticcheck":
		return true
	case "github.com/jacoelho/xsd/internal/semanticresolve":
		return true
	case "github.com/jacoelho/xsd/internal/runtimecompile":
		return true
	case "github.com/jacoelho/xsd/internal/validator":
		return true
	default:
		return false
	}
}

func isUpperLayerPackage(pkg string) bool {
	switch pkg {
	case "github.com/jacoelho/xsd/internal/source":
		return true
	case "github.com/jacoelho/xsd/internal/pipeline":
		return true
	case "github.com/jacoelho/xsd/internal/semantic":
		return true
	case "github.com/jacoelho/xsd/internal/semanticcheck":
		return true
	case "github.com/jacoelho/xsd/internal/semanticresolve":
		return true
	case "github.com/jacoelho/xsd/internal/runtimecompile":
		return true
	}
	return false
}

func isLegacyPath(path string) bool {
	legacy := []string{
		"github.com/jacoelho/xsd/internal/loader",
		"github.com/jacoelho/xsd/internal/schemacheck",
		"github.com/jacoelho/xsd/internal/resolver",
		"github.com/jacoelho/xsd/internal/schema",
		"github.com/jacoelho/xsd/internal/runtimebuild",
		"github.com/jacoelho/xsd/internal/models",
		"github.com/jacoelho/xsd/internal/ic",
		"github.com/jacoelho/xsd/internal/wsmode",
		"github.com/jacoelho/xsd/internal/xml",
	}
	for _, prefix := range legacy {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
