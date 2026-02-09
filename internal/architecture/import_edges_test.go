package architecture_test

import (
	"go/ast"
	"go/parser"
	"slices"
	"strings"
	"testing"
)

type edgeRule struct {
	scopePath string
	banned    []string
}

func TestImportEdges(t *testing.T) {
	t.Parallel()

	rules := []edgeRule{
		{
			scopePath: "internal/pipeline",
			banned: []string{
				"github.com/jacoelho/xsd/internal/contentmodel",
				"github.com/jacoelho/xsd/internal/validatorcompile",
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
			},
		},
		{
			scopePath: "internal/parser",
			banned: []string{
				"github.com/jacoelho/xsd/internal/runtimebuild",
				"github.com/jacoelho/xsd/internal/validatorcompile",
				"github.com/jacoelho/xsd/internal/semantic",
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/source",
			banned: []string{
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/runtimebuild",
				"github.com/jacoelho/xsd/internal/validatorcompile",
				"github.com/jacoelho/xsd/internal/schemaflow",
				"github.com/jacoelho/xsd/internal/semantic",
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/facets",
			banned: []string{
				"github.com/jacoelho/xsd/internal/semantic",
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/fieldresolve",
			banned: []string{
				"github.com/jacoelho/xsd/internal/semantic",
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/runtimebuild",
			banned: []string{
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/validatorcompile",
			banned: []string{
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/runtimebuild",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/runtime",
			banned: []string{
				"github.com/jacoelho/xsd/pkg/xmlstream",
			},
		},
		{
			scopePath: "internal/loadmerge",
			banned: []string{
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/runtimebuild",
				"github.com/jacoelho/xsd/internal/validatorcompile",
				"github.com/jacoelho/xsd/internal/semantic",
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/loadgraph",
			banned: []string{
				"github.com/jacoelho/xsd/internal/parser",
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/runtimebuild",
				"github.com/jacoelho/xsd/internal/validatorcompile",
				"github.com/jacoelho/xsd/internal/semantic",
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/valueparse",
			banned: []string{
				"github.com/jacoelho/xsd/internal/parser",
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/runtimebuild",
				"github.com/jacoelho/xsd/internal/validatorcompile",
				"github.com/jacoelho/xsd/internal/semantic",
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/types",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/valuekey",
			banned: []string{
				"github.com/jacoelho/xsd/internal/parser",
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/runtimebuild",
				"github.com/jacoelho/xsd/internal/validatorcompile",
				"github.com/jacoelho/xsd/internal/semantic",
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/types",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/durationlex",
			banned: []string{
				"github.com/jacoelho/xsd/internal/parser",
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/runtimebuild",
				"github.com/jacoelho/xsd/internal/validatorcompile",
				"github.com/jacoelho/xsd/internal/semantic",
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/types",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/schemaflow",
			banned: []string{
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/runtimebuild",
				"github.com/jacoelho/xsd/internal/validatorcompile",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/state",
			banned: []string{
				"github.com/jacoelho/xsd/internal/parser",
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/runtimebuild",
				"github.com/jacoelho/xsd/internal/validatorcompile",
				"github.com/jacoelho/xsd/internal/semantic",
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/semantic",
			banned: []string{
				"github.com/jacoelho/xsd/internal/runtimebuild",
				"github.com/jacoelho/xsd/internal/validatorcompile",
				"github.com/jacoelho/xsd/internal/semanticresolve",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/semanticresolve",
			banned: []string{
				"github.com/jacoelho/xsd/internal/semanticcheck",
			},
		},
		{
			scopePath: "internal/validator",
			banned: []string{
				"github.com/jacoelho/xsd/internal/parser",
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/runtimebuild",
				"github.com/jacoelho/xsd/internal/validatorcompile",
				"github.com/jacoelho/xsd/internal/semantic",
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/types",
				"github.com/jacoelho/xsd/internal/xsdxml",
			},
		},
		{
			scopePath: "xsd.go",
			banned: []string{
				"github.com/jacoelho/xsd/internal/contentmodel",
				"github.com/jacoelho/xsd/internal/runtimebuild",
				"github.com/jacoelho/xsd/internal/validatorcompile",
			},
		},
	}

	forEachParsedRepoProductionGoFile(t, parser.ImportsOnly, func(file repoGoFile, parsed *ast.File) {
		for _, rule := range rules {
			if !withinScope(file.relPath, rule.scopePath) {
				continue
			}
			for _, imp := range parsed.Imports {
				importPath := strings.Trim(imp.Path.Value, "\"")
				if slices.Contains(rule.banned, importPath) {
					t.Fatalf("import edge violation: %s imports %s", file.relPath, importPath)
				}
			}
		}
	})
}
