package tests_test

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const xmlStreamImport = "github.com/jacoelho/xsd/internal/xmlstream"

// XML tokens and their byte fields borrow storage owned by one Reader. The
// consumers may pass them through synchronous calls, but no consumer may
// retain the token, a token-shaped alias, or a borrowed field in state that
// survives the current call.
func TestXMLStreamBorrowedValuesStayEphemeral(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	packages := collectXMLStreamPackages(t, root, fset)
	for _, pkg := range packages {
		checkXMLStreamPackage(t, fset, pkg)
	}
}

type xmlStreamPackage struct {
	dir           string
	files         []*ast.File
	usesXMLStream bool
}

func collectXMLStreamPackages(t *testing.T, root string, fset *token.FileSet) []*xmlStreamPackage {
	t.Helper()
	byPackage := make(map[string]*xmlStreamPackage)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		slash := filepath.ToSlash(rel)
		if entry.IsDir() {
			name := entry.Name()
			// Match the go command's package patterns, which ignore these
			// directories; they hold tool state and copies, not packages.
			if slash != "." && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata") {
				return filepath.SkipDir
			}
			switch slash {
			case "docs/spec", "tests/corpus", "internal/xmlstream":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		match, matchErr := build.Default.MatchFile(filepath.Dir(path), filepath.Base(path))
		if matchErr != nil {
			return matchErr
		}
		if !match {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		dir := filepath.ToSlash(filepath.Dir(slash))
		key := dir + "\x00" + file.Name.Name
		pkg := byPackage[key]
		if pkg == nil {
			pkg = &xmlStreamPackage{dir: dir}
			byPackage[key] = pkg
		}
		pkg.files = append(pkg.files, file)
		if fileImportsXMLStream(file) {
			pkg.usesXMLStream = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk XML stream consumers: %v", err)
	}
	result := make([]*xmlStreamPackage, 0, len(byPackage))
	for _, pkg := range byPackage {
		if pkg.usesXMLStream {
			result = append(result, pkg)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].dir < result[j].dir })
	return result
}

func fileImportsXMLStream(file *ast.File) bool {
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err == nil && path == xmlStreamImport {
			return true
		}
	}
	return false
}

func checkXMLStreamPackage(t *testing.T, fset *token.FileSet, pkg *xmlStreamPackage) {
	t.Helper()
	path := "github.com/jacoelho/xsd"
	if pkg.dir != "." {
		path += "/" + pkg.dir
	}
	violations, err := analyzeXMLStreamPackage(fset, path, pkg.files)
	if err != nil {
		t.Fatalf("type-check %s: %v", path, err)
	}
	for _, violation := range violations {
		t.Errorf("%s: %s retains borrowed XML stream data", fset.Position(violation.pos), violation.kind)
	}
}

func borrowedXMLStreamType(typ types.Type) bool {
	_, ok := borrowedXMLStreamTypeLabel(typ)
	return ok
}

func borrowedXMLStreamTypeLabel(typ types.Type) (string, bool) {
	seen := make(map[types.Type]bool)
	var visit func(types.Type) (string, bool)
	visit = func(typ types.Type) (string, bool) {
		if typ == nil {
			return "", false
		}
		typ = types.Unalias(typ)
		if seen[typ] {
			return "", false
		}
		seen[typ] = true
		switch typ := typ.(type) {
		case *types.Named:
			if obj := typ.Obj(); obj.Pkg() != nil && obj.Pkg().Path() == xmlStreamImport {
				switch obj.Name() {
				case "Token", "StartElement", "Attr":
					return obj.Name(), true
				}
				// Reader owns token storage; its private fields are not borrowed
				// values retained by a consumer.
				return "", false
			}
			return visit(typ.Underlying())
		case *types.Pointer:
			return visit(typ.Elem())
		case *types.Array:
			return visit(typ.Elem())
		case *types.Slice:
			return visit(typ.Elem())
		case *types.Map:
			if label, ok := visit(typ.Key()); ok {
				return label, true
			}
			return visit(typ.Elem())
		case *types.Chan:
			return visit(typ.Elem())
		case *types.Struct:
			for field := range typ.Fields() {
				if label, ok := visit(field.Type()); ok {
					return label, true
				}
			}
		case *types.Tuple:
			for variable := range typ.Variables() {
				if label, ok := visit(variable.Type()); ok {
					return label, true
				}
			}
		}
		return "", false
	}
	return visit(typ)
}
