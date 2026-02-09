package architecture_test

import (
	"go/ast"
	"go/parser"
	"strings"
	"testing"
)

const semanticResolveImportPath = "github.com/jacoelho/xsd/internal/semanticresolve"

func TestSemanticResolveImportsScopedToSchemaflow(t *testing.T) {
	t.Parallel()

	forEachParsedRepoProductionGoFile(t, parser.ImportsOnly, func(file repoGoFile, parsed *ast.File) {
		if withinScope(file.relPath, "internal/schemaflow") || withinScope(file.relPath, "internal/semanticresolve") {
			return
		}
		for _, imp := range parsed.Imports {
			importPath := strings.Trim(imp.Path.Value, "\"")
			if importPath == semanticResolveImportPath {
				t.Fatalf("semanticresolve boundary violation: %s imports %s", file.relPath, importPath)
			}
		}
	})
}
