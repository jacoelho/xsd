package tests_test

import (
	"errors"
	"fmt"
	"go/ast"
	"go/build"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// testOnlyExportAllowlist lists exported identifiers of internal packages that
// only tests use on purpose, with the reason.
var testOnlyExportAllowlist = map[string]string{
	"github.com/jacoelho/xsd/internal/corpus.Inventory.ShardPaths": "the corpus inventory's consumer is the tests harness, which is test code",
	"github.com/jacoelho/xsd/internal/corpus.Verdict.Expectation":  "the corpus inventory's consumer is the tests harness, which is test code",
}

// TestInternalExportsHaveProductionUses fails when an exported identifier of an
// internal package is used only by tests: such an export lets tests reach past
// the module's real surface. A use in the host build or the js/wasm build
// counts as production. Struct fields and methods of unexported types are not
// checked, because other packages cannot name them and an interface can reach
// such a method with no static use.
func TestInternalExportsHaveProductionUses(t *testing.T) {
	t.Parallel()
	root := repoRoot(t)
	uses := newExportUses()
	fset := token.NewFileSet()
	imp := importer.ForCompiler(fset, "source", nil)
	wasm := build.Default
	wasm.GOOS, wasm.GOARCH = "js", "wasm"
	for _, dir := range moduleGoPackageDirs(t, root) {
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			t.Fatal(err)
		}
		importPath := path.Join("github.com/jacoelho/xsd", filepath.ToSlash(rel))
		host, err := build.Default.ImportDir(dir, 0)
		if err != nil {
			t.Fatalf("%s: %v", dir, err)
		}
		uses.checkStrict(t, fset, imp, packageVariant{importPath: importPath, dir: dir, files: host.GoFiles})
		underTest := uses.checkStrict(t, fset, imp, packageVariant{importPath: importPath, dir: dir, files: slices.Concat(host.GoFiles, host.TestGoFiles)})
		// An external test package sees the helpers of the package's own test
		// files (export_test.go), so it imports that variant. Packages it
		// imports still see the plain variant, so their types do not match
		// and errors are ignored; a use left unresolved can only hide a
		// test-only export, never report one, because production uses come
		// from the strict checks.
		xtestImp := testVariantImporter{base: imp, path: importPath, pkg: underTest}
		uses.checkLenient(t, fset, xtestImp, packageVariant{importPath: importPath + "_test", dir: dir, files: host.XTestGoFiles})
		if js, err := wasm.ImportDir(dir, 0); err == nil {
			// The source importer resolves imports for the host build, so a
			// js/wasm-only import such as syscall/js does not type-check; the
			// uses it leaves resolvable still count.
			uses.checkLenient(t, fset, imp, packageVariant{importPath: importPath, dir: dir, files: js.GoFiles})
		}
	}
	for _, line := range uses.report(testOnlyExportAllowlist) {
		t.Error(line)
	}
}

func TestInternalExportUseClassification(t *testing.T) {
	t.Parallel()
	const prod = `package p

func Used() int { return 1 }
func TestOnly() int { return 2 }
func Unused() int { return 3 }

type T struct{ Field int }

func (T) TestOnlyMethod() int { return 4 }

type hidden struct{}

func (hidden) Method() int { return 5 }

var _ = Used()
`
	const test = `package p

var _ = TestOnly() + T{}.TestOnlyMethod() + T{}.Field + hidden{}.Method()
`
	fset := token.NewFileSet()
	var files []*ast.File
	for name, src := range map[string]string{"p.go": prod, "p_test.go": test} {
		file, err := parser.ParseFile(fset, name, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	uses := newExportUses()
	info := newExportUseInfo()
	if _, err := (&types.Config{}).Check("m/internal/p", fset, files, info); err != nil {
		t.Fatal(err)
	}
	uses.record(fset, info, "m/internal/p")
	got := uses.report(map[string]string{"m/internal/p.Gone": "reason"})
	want := []string{
		"p.go:9:10: m/internal/p.T.TestOnlyMethod is exported but used only by tests in m/internal/p",
		"p.go:4:6: m/internal/p.TestOnly is exported but used only by tests in m/internal/p",
		"allowlist entry m/internal/p.Gone matches no test-only export; remove it",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("report =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

type exportUses struct {
	decls     map[string]string
	prod      map[string]bool
	testUsers map[string]map[string]bool
}

func newExportUses() *exportUses {
	return &exportUses{decls: map[string]string{}, prod: map[string]bool{}, testUsers: map[string]map[string]bool{}}
}

// packageVariant is one set of a directory's files type-checked as one package.
type packageVariant struct {
	importPath string
	dir        string
	files      []string
}

// checkStrict type-checks v, failing the test on a type error, and records its
// declarations and uses.
func (u *exportUses) checkStrict(t *testing.T, fset *token.FileSet, imp types.Importer, v packageVariant) *types.Package {
	t.Helper()
	if len(v.files) == 0 {
		return nil
	}
	info := newExportUseInfo()
	pkg, err := (&types.Config{Importer: imp}).Check(v.importPath, fset, parseVariant(t, fset, v), info)
	if err != nil {
		t.Fatalf("type-check %s: %v", v.importPath, err)
	}
	u.record(fset, info, strings.TrimSuffix(v.importPath, "_test"))
	return pkg
}

// checkLenient type-checks v, logging rather than failing on type errors, and
// records the declarations and uses that still resolve.
func (u *exportUses) checkLenient(t *testing.T, fset *token.FileSet, imp types.Importer, v packageVariant) {
	t.Helper()
	if len(v.files) == 0 {
		return
	}
	info := newExportUseInfo()
	conf := types.Config{Importer: imp, Error: func(error) {}}
	if _, err := conf.Check(v.importPath, fset, parseVariant(t, fset, v), info); err != nil {
		t.Logf("type-check %s leniently, first ignored error: %v", v.importPath, err)
	}
	u.record(fset, info, strings.TrimSuffix(v.importPath, "_test"))
}

func parseVariant(t *testing.T, fset *token.FileSet, v packageVariant) []*ast.File {
	t.Helper()
	files := make([]*ast.File, 0, len(v.files))
	for _, name := range v.files {
		file, err := parser.ParseFile(fset, filepath.Join(v.dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	return files
}

func newExportUseInfo() *types.Info {
	return &types.Info{Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}}
}

// testVariantImporter imports path as pkg, the package compiled with its own
// test files, and every other import path through base.
type testVariantImporter struct {
	base types.Importer
	path string
	pkg  *types.Package
}

func (i testVariantImporter) Import(importPath string) (*types.Package, error) {
	if importPath == i.path && i.pkg != nil {
		return i.pkg, nil
	}
	return i.base.Import(importPath)
}

func (u *exportUses) record(fset *token.FileSet, info *types.Info, user string) {
	for _, obj := range info.Defs {
		key, ok := checkedExportKey(obj)
		if !ok {
			continue
		}
		if pos := fset.Position(obj.Pos()); !strings.HasSuffix(pos.Filename, "_test.go") {
			u.decls[key] = pos.String()
		}
	}
	for id, obj := range info.Uses {
		key, ok := checkedExportKey(obj)
		if !ok {
			continue
		}
		if !strings.HasSuffix(fset.Position(id.Pos()).Filename, "_test.go") {
			u.prod[key] = true
			continue
		}
		if u.testUsers[key] == nil {
			u.testUsers[key] = map[string]bool{}
		}
		u.testUsers[key][user] = true
	}
}

// report returns one line per test-only export the allowlist does not name and
// one per allowlist entry that names none, so the allowlist cannot go stale.
func (u *exportUses) report(allow map[string]string) []string {
	var keys []string
	for key := range u.decls {
		if !u.prod[key] && len(u.testUsers[key]) > 0 {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	var lines []string
	for _, key := range keys {
		if _, ok := allow[key]; ok {
			continue
		}
		users := make([]string, 0, len(u.testUsers[key]))
		for user := range u.testUsers[key] {
			users = append(users, user)
		}
		slices.Sort(users)
		lines = append(lines, fmt.Sprintf("%s: %s is exported but used only by tests in %s", u.decls[key], key, strings.Join(users, ", ")))
	}
	var stale []string
	for key := range allow {
		if _, ok := slices.BinarySearch(keys, key); !ok {
			stale = append(stale, key)
		}
	}
	slices.Sort(stale)
	for _, key := range stale {
		lines = append(lines, fmt.Sprintf("allowlist entry %s matches no test-only export; remove it", key))
	}
	return lines
}

// checkedExportKey identifies an exported package-level identifier, or an
// exported method of an exported type, declared in an internal package.
func checkedExportKey(obj types.Object) (string, bool) {
	if obj == nil || obj.Pkg() == nil || !obj.Exported() || !strings.Contains(obj.Pkg().Path()+"/", "/internal/") {
		return "", false
	}
	pkg := obj.Pkg().Path()
	if f, ok := obj.(*types.Func); ok {
		recv := f.Signature().Recv()
		if recv == nil {
			return pkg + "." + f.Name(), true
		}
		t := recv.Type()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		named, ok := types.Unalias(t).(*types.Named)
		if !ok || !named.Obj().Exported() {
			return "", false
		}
		return pkg + "." + named.Obj().Name() + "." + f.Name(), true
	}
	if obj.Parent() != obj.Pkg().Scope() {
		return "", false
	}
	switch obj.(type) {
	case *types.TypeName, *types.Const, *types.Var:
		return pkg + "." + obj.Name(), true
	}
	return "", false
}

// moduleGoPackageDirs returns every directory of the module holding a Go
// package, skipping hidden directories, testdata and node_modules.
func moduleGoPackageDirs(t *testing.T, root string) []string {
	t.Helper()
	var dirs []string
	err := filepath.WalkDir(root, func(p string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			return nil
		}
		if p != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "testdata" || entry.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if _, err := build.ImportDir(p, 0); err != nil {
			if _, ok := errors.AsType[*build.NoGoError](err); ok {
				return nil
			}
			return err
		}
		dirs = append(dirs, p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return dirs
}
