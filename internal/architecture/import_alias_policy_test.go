package architecture_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestImportAliasPolicy(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if name == "vendor" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		node, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		for _, imp := range node.Imports {
			if imp.Name == nil {
				continue
			}
			if imp.Name.Name == "xsderrors" {
				continue
			}
			pos := fset.Position(imp.Name.Pos())
			t.Errorf(
				"%s:%s:%s disallowed import alias %q for %s (only xsderrors allowed)",
				rel,
				strconv.Itoa(pos.Line),
				strconv.Itoa(pos.Column),
				imp.Name.Name,
				imp.Path.Value,
			)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk go files for import alias policy: %v", err)
	}
}
