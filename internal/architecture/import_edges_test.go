package architecture_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
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
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
			},
		},
		{
			scopePath: "internal/parser",
			banned: []string{
				"github.com/jacoelho/xsd/internal/runtimecompile",
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
				"github.com/jacoelho/xsd/internal/runtimecompile",
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
			scopePath: "internal/runtimecompile",
			banned: []string{
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/loadmerge",
			banned: []string{
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/runtimecompile",
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
				"github.com/jacoelho/xsd/internal/runtimecompile",
				"github.com/jacoelho/xsd/internal/semantic",
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/schemaflow",
			banned: []string{
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/runtimecompile",
				"github.com/jacoelho/xsd/internal/source",
				"github.com/jacoelho/xsd/internal/validator",
			},
		},
		{
			scopePath: "internal/state",
			banned: []string{
				"github.com/jacoelho/xsd/internal/parser",
				"github.com/jacoelho/xsd/internal/pipeline",
				"github.com/jacoelho/xsd/internal/runtimecompile",
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
				"github.com/jacoelho/xsd/internal/runtimecompile",
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
				"github.com/jacoelho/xsd/internal/runtimecompile",
				"github.com/jacoelho/xsd/internal/semantic",
				"github.com/jacoelho/xsd/internal/semanticcheck",
				"github.com/jacoelho/xsd/internal/semanticresolve",
				"github.com/jacoelho/xsd/internal/source",
			},
		},
		{
			scopePath: "xsd.go",
			banned: []string{
				"github.com/jacoelho/xsd/internal/runtimecompile",
			},
		},
	}

	root := repoRoot(t)
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk go files: %v", err)
	}
	slices.Sort(files)

	fset := token.NewFileSet()
	for _, absPath := range files {
		parsed, err := parser.ParseFile(fset, absPath, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse imports %s: %v", absPath, err)
		}
		path, err := filepath.Rel(root, absPath)
		if err != nil {
			t.Fatalf("rel path %s: %v", absPath, err)
		}
		for _, rule := range rules {
			if !withinScope(path, rule.scopePath) {
				continue
			}
			for _, imp := range parsed.Imports {
				importPath := strings.Trim(imp.Path.Value, "\"")
				if slices.Contains(rule.banned, importPath) {
					t.Fatalf("import edge violation: %s imports %s", path, importPath)
				}
			}
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve working directory: %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("resolve repo root %s: %v", root, err)
	}
	return root
}

func withinScope(path, scope string) bool {
	if strings.HasSuffix(scope, ".go") {
		return filepath.Clean(path) == filepath.Clean(scope)
	}
	cleanScope := filepath.Clean(scope) + string(filepath.Separator)
	return strings.HasPrefix(filepath.Clean(path), cleanScope)
}
