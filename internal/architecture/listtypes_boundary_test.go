package architecture_test

import (
	"go/ast"
	"go/parser"
	"testing"
)

func TestListTypesImportBoundary(t *testing.T) {
	t.Parallel()

	const importPath = "github.com/jacoelho/xsd/internal/listtypes"

	forEachParsedRepoProductionGoFile(t, parser.ParseComments, func(file repoGoFile, parsed *ast.File) {
		if len(importAliasesForPath(parsed, importPath, "listtypes")) == 0 {
			return
		}
		t.Fatalf("legacy listtypes import is forbidden: %s imports %s", file.relPath, importPath)
	})
}
