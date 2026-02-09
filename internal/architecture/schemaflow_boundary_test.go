package architecture_test

import (
	"go/ast"
	"go/parser"
	"strings"
	"testing"
)

const schemaflowImportPath = "github.com/jacoelho/xsd/internal/schemaflow"

func TestSchemaflowImportsScopedToPipeline(t *testing.T) {
	t.Parallel()

	forEachParsedRepoProductionGoFile(t, parser.ImportsOnly, func(file repoGoFile, parsed *ast.File) {
		if withinScope(file.relPath, "internal/pipeline") || withinScope(file.relPath, "internal/schemaflow") {
			return
		}
		for _, imp := range parsed.Imports {
			importPath := strings.Trim(imp.Path.Value, "\"")
			if importPath == schemaflowImportPath {
				t.Fatalf("schemaflow boundary violation: %s imports %s", file.relPath, importPath)
			}
		}
	})
}
