package architecture_test

import (
	"go/ast"
	"go/parser"
	"testing"
)

func TestBuiltinListImportBoundary(t *testing.T) {
	t.Parallel()

	const importPath = "github.com/jacoelho/xsd/internal/builtinlist"
	allowedScopes := []string{
		"internal/builtinlist",
		"internal/builtins",
		"internal/model",
	}

	forEachParsedRepoProductionGoFile(t, parser.ParseComments, func(file repoGoFile, parsed *ast.File) {
		if withinAnyScope(file.relPath, allowedScopes) {
			return
		}
		if len(importAliasesForPath(parsed, importPath, "builtinlist")) == 0 {
			return
		}
		t.Fatalf("builtinlist boundary violation: %s imports %s", file.relPath, importPath)
	})
}
