package architecture_test

import (
	"go/ast"
	"go/parser"
	"slices"
	"testing"
)

func TestModelConcernFunctionsNotExported(t *testing.T) {
	t.Parallel()

	forbidden := []string{
		"NormalizeValue",
		"ParseValueForType",
		"ApplyFacets",
		"ValidateValueAgainstFacets",
		"TypedValueForFacet",
		"IsLengthFacet",
		"ValuesEqual",
		"FormatEnumerationValues",
		"ParseDurationToTimeDuration",
		"NewMinInclusive",
		"NewMaxInclusive",
		"NewMinExclusive",
		"NewMaxExclusive",
		"IsQNameOrNotationType",
	}

	forEachParsedRepoProductionGoFile(t, parser.ParseComments, func(file repoGoFile, parsed *ast.File) {
		if !withinScope(file.relPath, "internal/model") {
			return
		}

		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name == nil {
				continue
			}
			if !fn.Name.IsExported() {
				continue
			}
			if !slices.Contains(forbidden, fn.Name.Name) {
				continue
			}
			// allow methods with the same names; only package-level helpers are forbidden.
			if fn.Recv != nil {
				continue
			}
			t.Fatalf("model concern export violation: %s declares exported %s", file.relPath, fn.Name.Name)
		}
	})
}
