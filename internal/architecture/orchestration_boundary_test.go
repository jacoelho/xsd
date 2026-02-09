package architecture_test

import (
	"go/ast"
	"go/parser"
	"strings"
	"testing"
)

const (
	pipelineImportPath = "github.com/jacoelho/xsd/internal/pipeline"
	sourceImportPath   = "github.com/jacoelho/xsd/internal/source"
)

func TestPipelineImportsScopedToXSD(t *testing.T) {
	t.Parallel()
	assertImportScoped(t, pipelineImportPath, []string{"xsd.go", "internal/pipeline"})
}

func TestSourceImportsScopedToXSD(t *testing.T) {
	t.Parallel()
	assertImportScoped(t, sourceImportPath, []string{"xsd.go", "internal/source"})
}

func assertImportScoped(t *testing.T, importPath string, allowedScopes []string) {
	t.Helper()

	forEachParsedRepoProductionGoFile(t, parser.ImportsOnly, func(file repoGoFile, parsed *ast.File) {
		if withinAnyScope(file.relPath, allowedScopes) {
			return
		}
		for _, imp := range parsed.Imports {
			if strings.Trim(imp.Path.Value, "\"") == importPath {
				t.Fatalf("orchestration boundary violation: %s imports %s", file.relPath, importPath)
			}
		}
	})
}
