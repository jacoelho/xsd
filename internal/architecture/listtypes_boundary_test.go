package architecture_test

import (
	"go/ast"
	"go/parser"
	"testing"
)

func TestListTypesImportBoundary(t *testing.T) {
	t.Parallel()

	const importPath = "github.com/jacoelho/xsd/internal/listtypes"
	allowedScopes := []string{
		"internal/listtypes",
		"internal/builtins",
		"internal/model",
	}

	forEachParsedRepoProductionGoFile(t, parser.ParseComments, func(file repoGoFile, parsed *ast.File) {
		if withinAnyScope(file.relPath, allowedScopes) {
			return
		}
		if len(importAliasesForPath(parsed, importPath, "listtypes")) == 0 {
			return
		}
		t.Fatalf("listtypes boundary violation: %s imports %s", file.relPath, importPath)
	})
}
