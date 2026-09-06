package tests_test

import (
	"go/ast"
	"go/build"
	"go/importer"
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
			switch slash {
			case ".git", ".codex", "docs/spec", "tests/corpus", "internal/xmlstream":
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
	info := &types.Info{
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Types:      make(map[ast.Expr]types.TypeAndValue),
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	path := "github.com/jacoelho/xsd"
	if pkg.dir != "." {
		path += "/" + pkg.dir
	}
	if _, err := conf.Check(path, fset, pkg.files, info); err != nil {
		t.Fatalf("type-check %s: %v", path, err)
	}
	for _, file := range pkg.files {
		checkXMLStreamFile(t, fset, info, file)
	}
}

func checkXMLStreamFile(t *testing.T, fset *token.FileSet, info *types.Info, file *ast.File) {
	t.Helper()
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.StructType:
			for _, field := range node.Fields.List {
				if typ := info.TypeOf(field.Type); borrowedXMLStreamType(typ) {
					t.Fatalf("%s stores borrowed XML stream type %s in a struct field", fset.Position(field.Pos()), borrowedTypeLabel(typ))
				}
			}
		case *ast.TypeSpec:
			if typ := info.TypeOf(node.Type); borrowedXMLStreamType(typ) {
				t.Fatalf("%s aliases borrowed XML stream type %s", fset.Position(node.Pos()), borrowedTypeLabel(typ))
			}
		case *ast.FuncType:
			checkXMLStreamResults(t, fset, info, node)
		case *ast.GenDecl:
			if node.Tok == token.VAR {
				checkXMLStreamPackageVars(t, fset, info, node)
			}
		case *ast.ReturnStmt:
			for _, result := range node.Results {
				if value, ok := borrowedXMLStreamValue(info, result); ok {
					t.Fatalf("%s returns borrowed XML stream value %s", fset.Position(result.Pos()), value)
				}
			}
		case *ast.GoStmt, *ast.DeferStmt:
			call := delayedCall(node)
			for _, argument := range call.Args {
				if value, ok := borrowedXMLStreamValue(info, argument); ok {
					t.Fatalf("%s sends borrowed XML stream value %s across a delayed call", fset.Position(argument.Pos()), value)
				}
			}
		case *ast.AssignStmt:
			checkXMLStreamAssignment(t, fset, info, node)
		}
		return true
	})
}

func checkXMLStreamResults(t *testing.T, fset *token.FileSet, info *types.Info, fn *ast.FuncType) {
	t.Helper()
	if fn.Results == nil {
		return
	}
	for _, field := range fn.Results.List {
		if typ := info.TypeOf(field.Type); borrowedXMLStreamType(typ) {
			t.Fatalf("%s returns borrowed XML stream type %s", fset.Position(field.Pos()), borrowedTypeLabel(typ))
		}
	}
}

func checkXMLStreamPackageVars(t *testing.T, fset *token.FileSet, info *types.Info, decl *ast.GenDecl) {
	t.Helper()
	for _, spec := range decl.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for _, name := range valueSpec.Names {
			if object := info.Defs[name]; object != nil {
				if scope := object.Parent(); scope == nil || scope.Parent() != nil {
					continue
				}
				if typ := object.Type(); borrowedXMLStreamType(typ) {
					t.Fatalf("%s retains borrowed XML stream variable %s (%s)", fset.Position(name.Pos()), name.Name, borrowedTypeLabel(typ))
				}
			}
		}
	}
}

func checkXMLStreamAssignment(t *testing.T, fset *token.FileSet, info *types.Info, assignment *ast.AssignStmt) {
	t.Helper()
	for i, destination := range assignment.Lhs {
		if i >= len(assignment.Rhs) {
			break
		}
		value, ok := borrowedXMLStreamValue(info, assignment.Rhs[i])
		if !ok {
			continue
		}
		if _, local := destination.(*ast.Ident); local {
			continue
		}
		if _, field := destination.(*ast.SelectorExpr); field {
			t.Fatalf("%s stores borrowed XML stream value %s in a field", fset.Position(destination.Pos()), value)
		}
		if _, index := destination.(*ast.IndexExpr); index {
			t.Fatalf("%s stores borrowed XML stream value %s in indexed state", fset.Position(destination.Pos()), value)
		}
		t.Fatalf("%s stores borrowed XML stream value %s", fset.Position(destination.Pos()), value)
	}
}

func delayedCall(node ast.Node) *ast.CallExpr {
	switch node := node.(type) {
	case *ast.GoStmt:
		return node.Call
	case *ast.DeferStmt:
		return node.Call
	default:
		return nil
	}
}

func borrowedXMLStreamValue(info *types.Info, expression ast.Expr) (string, bool) {
	if expression == nil {
		return "", false
	}
	if typ := info.TypeOf(expression); borrowedXMLStreamType(typ) {
		return borrowedTypeLabel(typ), true
	}
	if call, ok := expression.(*ast.CallExpr); ok && info.Types[call.Fun].IsType() {
		for _, argument := range call.Args {
			if value, ok := borrowedXMLStreamValue(info, argument); ok && typeCanHideBorrowed(info.TypeOf(expression)) {
				return value, true
			}
		}
	}
	return "", false
}

func borrowedXMLStreamType(typ types.Type) bool {
	_, ok := borrowedXMLStreamTypeLabel(typ)
	return ok
}

func borrowedTypeLabel(typ types.Type) string {
	label, _ := borrowedXMLStreamTypeLabel(typ)
	return label
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

func typeCanHideBorrowed(typ types.Type) bool {
	if typ == nil {
		return false
	}
	typ = types.Unalias(typ)
	switch typ := typ.(type) {
	case *types.Interface, *types.Signature:
		return true
	case *types.Named:
		return typeCanHideBorrowed(typ.Underlying())
	case *types.Pointer:
		return typeCanHideBorrowed(typ.Elem())
	case *types.Array:
		return typeCanHideBorrowed(typ.Elem())
	case *types.Slice:
		return typeCanHideBorrowed(typ.Elem())
	case *types.Map:
		return typeCanHideBorrowed(typ.Key()) || typeCanHideBorrowed(typ.Elem())
	case *types.Chan:
		return typeCanHideBorrowed(typ.Elem())
	case *types.Struct:
		for field := range typ.Fields() {
			if typeCanHideBorrowed(field.Type()) {
				return true
			}
		}
	}
	return false
}
