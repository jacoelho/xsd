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
	"strings"
	"testing"
)

func TestStreamBorrowedAttributeFieldsStayBehindAccessors(t *testing.T) {
	root := repoRoot(t)
	fset := token.NewFileSet()
	packages := make(map[string]*streamBoundaryPackage)
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		slashFile := filepath.ToSlash(rel)
		if entry.IsDir() {
			switch slashFile {
			case ".git", ".codex", "docs/spec", "tests/corpus":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(file) != ".go" || strings.HasPrefix(slashFile, "internal/stream/") {
			return nil
		}
		match, err := build.Default.MatchFile(filepath.Dir(file), filepath.Base(file))
		if err != nil {
			return err
		}
		if !match {
			return nil
		}
		parsed, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			return err
		}
		if strings.HasSuffix(file, "_test.go") {
			checkStreamBoundaryFile(t, fset, nil, parsed)
			return nil
		}
		dir := filepath.ToSlash(filepath.Dir(slashFile))
		key := dir + "\x00" + parsed.Name.Name
		pkg := packages[key]
		if pkg == nil {
			pkg = &streamBoundaryPackage{dir: dir, name: parsed.Name.Name}
			packages[key] = pkg
		}
		pkg.files = append(pkg.files, parsed)
		return nil
	})
	if err != nil {
		t.Fatalf("walk Go files: %v", err)
	}
	for _, pkg := range packages {
		checkStreamBoundaryPackage(t, fset, pkg)
	}
}

type streamBoundaryPackage struct {
	dir   string
	name  string
	files []*ast.File
}

func checkStreamBoundaryPackage(t *testing.T, fset *token.FileSet, pkg *streamBoundaryPackage) {
	t.Helper()
	info := &types.Info{
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	conf := types.Config{
		Importer: importer.ForCompiler(fset, "source", nil),
	}
	path := "github.com/jacoelho/xsd"
	if pkg.dir != "." {
		path += "/" + pkg.dir
	}
	if strings.HasSuffix(pkg.name, "_test") {
		path += "_test"
	}
	if _, err := conf.Check(path, fset, pkg.files, info); err != nil {
		t.Fatalf("type-check %s: %v", path, err)
	}
	for _, file := range pkg.files {
		checkStreamBoundaryFile(t, fset, info, file)
	}
}

func checkStreamBoundaryFile(t *testing.T, fset *token.FileSet, info *types.Info, file *ast.File) {
	t.Helper()
	parents := astParentMap(file)
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if info != nil {
				checkStreamBoundarySelector(t, fset, info, parents, n)
			}
			if n.Sel.Name == "raw" {
				t.Fatalf("%s uses borrowed streamAttr field %s directly", fset.Position(n.Sel.Pos()), n.Sel.Name)
			}
		case *ast.KeyValueExpr:
			if key, ok := n.Key.(*ast.Ident); ok && key.Name == "raw" {
				t.Fatalf("%s constructs borrowed streamAttr field %s directly", fset.Position(key.Pos()), key.Name)
			}
		case *ast.ValueSpec:
			for _, valueType := range streamBorrowedTypeExprs(n.Type) {
				t.Fatalf("%s declares retained borrowed stream type %s", fset.Position(valueType.Pos()), exprString(valueType))
			}
		case *ast.Field:
			if isFuncParamOrResult(n, parents) {
				return true
			}
			for _, fieldType := range streamBorrowedTypeExprs(n.Type) {
				t.Fatalf("%s stores borrowed stream type %s in a field", fset.Position(fieldType.Pos()), exprString(fieldType))
			}
		case *ast.CompositeLit:
			if _, ok := parents[n].(*ast.ReturnStmt); ok {
				return true
			}
			for _, litType := range streamBorrowedTypeExprs(n.Type) {
				t.Fatalf("%s constructs retained borrowed stream type %s", fset.Position(litType.Pos()), exprString(litType))
			}
		}
		return true
	})
}

func astParentMap(root ast.Node) map[ast.Node]ast.Node {
	parents := make(map[ast.Node]ast.Node)
	var stack []ast.Node
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if len(stack) > 0 {
			parents[n] = stack[len(stack)-1]
		}
		stack = append(stack, n)
		return true
	})
	return parents
}

func checkStreamBoundarySelector(t *testing.T, fset *token.FileSet, info *types.Info, parents map[ast.Node]ast.Node, sel *ast.SelectorExpr) {
	t.Helper()
	selection := info.Selections[sel]
	if selection == nil || selection.Kind() != types.FieldVal {
		return
	}
	switch {
	case isStreamNamed(selection.Recv(), "Token"):
		switch sel.Sel.Name {
		case "Data", "Directive", "Start":
			if !allowedBorrowedTokenFieldUse(sel, parents) {
				t.Fatalf("%s uses borrowed token field %s outside an allowed immediate consumer", fset.Position(sel.Sel.Pos()), sel.Sel.Name)
			}
		}
	case isStreamNamed(selection.Recv(), "StartElement") && sel.Sel.Name == "Attr":
		if !allowedBorrowedStartAttrUse(sel, parents) {
			t.Fatalf("%s accesses borrowed token start attributes outside an allowed immediate consumer", fset.Position(sel.Sel.Pos()))
		}
	}
}

func isStreamNamed(typ types.Type, name string) bool {
	if ptr, ok := typ.(*types.Pointer); ok {
		typ = ptr.Elem()
	}
	named, ok := typ.(*types.Named)
	if !ok || named.Obj().Name() != name || named.Obj().Pkg() == nil {
		return false
	}
	return named.Obj().Pkg().Path() == "github.com/jacoelho/xsd/internal/stream"
}

func allowedBorrowedTokenFieldUse(sel *ast.SelectorExpr, parents map[ast.Node]ast.Node) bool {
	parent := parents[sel]
	if sel.Sel.Name == "Start" {
		if selector, ok := parent.(*ast.SelectorExpr); ok {
			return selector.Sel.Name == "XMLStartElement" || selector.Sel.Name == "Attr"
		}
		return parentCallName(parent) == "start"
	}
	switch parentCallName(parent) {
	case "chars", "isDOCTYPEDeclaration", "IsDOCTYPEDeclaration", "isXMLWhitespaceBytes", "IsXMLWhitespaceBytes", "ValidateDirective":
		return true
	default:
		return false
	}
}

func allowedBorrowedStartAttrUse(sel *ast.SelectorExpr, parents map[ast.Node]ast.Node) bool {
	parent := parents[sel]
	switch parent.(type) {
	case *ast.RangeStmt, *ast.IndexExpr:
		return true
	}
	switch parentCallName(parent) {
	case "len", "pushNamespaces", "translateStartElement", "xsiStartAttributeFlagsFor", "recordSchemaLocationHints",
		"schemaElementStart", "ElementStart", "validateAttributes", "HasXSITypeAttribute", "RootStart",
		"PushStream", "ValidateUniqueAttributes":
		return true
	default:
		return false
	}
}

func parentCallName(parent ast.Node) string {
	call, ok := parent.(*ast.CallExpr)
	if !ok {
		return ""
	}
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	default:
		return ""
	}
}

func isFuncParamOrResult(field *ast.Field, parents map[ast.Node]ast.Node) bool {
	list, ok := parents[field].(*ast.FieldList)
	if !ok {
		return false
	}
	_, ok = parents[list].(*ast.FuncType)
	return ok
}

func streamBorrowedTypeExprs(expr ast.Expr) []ast.Expr {
	if expr == nil {
		return nil
	}
	var out []ast.Expr
	ast.Inspect(expr, func(n ast.Node) bool {
		switch n := n.(type) {
		case nil:
			return true
		case *ast.SelectorExpr:
			if isStreamSelector(n, "Token") || isStreamSelector(n, "StartElement") || isStreamSelector(n, "Attr") {
				out = append(out, n)
			}
		}
		return true
	})
	return out
}

func isStreamSelector(sel *ast.SelectorExpr, name string) bool {
	if sel.Sel.Name != name {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "stream"
}

func exprString(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.SelectorExpr:
		if pkg, ok := x.X.(*ast.Ident); ok {
			return pkg.Name + "." + x.Sel.Name
		}
	case *ast.ArrayType:
		return "[]" + exprString(x.Elt)
	}
	return "stream borrowed type"
}
