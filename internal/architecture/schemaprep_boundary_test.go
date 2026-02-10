package architecture_test

import (
	"go/ast"
	"go/parser"
	"strings"
	"testing"
)

const schemaprepImportPath = "github.com/jacoelho/xsd/internal/schemaprep"

func TestSchemaprepImportsScopedToPipeline(t *testing.T) {
	t.Parallel()

	forEachParsedRepoProductionGoFile(t, parser.ImportsOnly, func(file repoGoFile, parsed *ast.File) {
		if withinScope(file.relPath, "internal/pipeline") || withinScope(file.relPath, "internal/schemaprep") {
			return
		}
		for _, imp := range parsed.Imports {
			importPath := strings.Trim(imp.Path.Value, "\"")
			if importPath == schemaprepImportPath {
				t.Fatalf("schemaprep boundary violation: %s imports %s", file.relPath, importPath)
			}
		}
	})
}
