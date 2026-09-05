package tests_test

import (
	"encoding/json"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type listedPackage struct {
	ImportPath string   `json:"ImportPath"` //nolint:tagliatelle // go list -json uses exported field names.
	Imports    []string `json:"Imports"`    //nolint:tagliatelle // go list -json uses exported field names.
	Deps       []string `json:"Deps"`       //nolint:tagliatelle // go list -json uses exported field names.
}

func TestInternalCapabilityImportAllowlist(t *testing.T) {
	const module = "github.com/jacoelho/xsd"
	allowed := map[string]map[string]bool{
		module + "/internal/compile": allowProjectImports(
			"internal/lex", "internal/runtime", "internal/source", "internal/stream",
			"internal/uriref", "internal/vocab", "internal/xmlns", "xsderrors"),
		module + "/internal/format": allowProjectImports(
			"internal/lex", "internal/stream", "internal/vocab", "internal/xmlns", "xsderrors"),
		module + "/internal/lex": {},
		module + "/internal/runtime": allowProjectImports(
			"internal/lex", "internal/uriref", "internal/vocab", "xsderrors"),
		module + "/internal/source": allowProjectImports("internal/uriref", "xsderrors"),
		module + "/internal/stream": allowProjectImports("internal/lex", "internal/vocab"),
		module + "/internal/uriref": {},
		module + "/internal/validate": allowProjectImports(
			"internal/lex", "internal/runtime", "internal/stream", "internal/uriref",
			"internal/vocab", "internal/xmlns", "xsderrors"),
		module + "/internal/vocab": {},
		module + "/internal/xmlns": allowProjectImports("internal/stream", "internal/vocab"),
	}

	packages := listPackages(t, "./internal/...")
	for path, pkg := range packages {
		packageAllowed, ok := allowed[path]
		if !ok {
			t.Fatalf("internal package %s has no architecture classification", path)
		}
		for _, imported := range pkg.Imports {
			if strings.HasPrefix(imported, module+"/") && !packageAllowed[imported] {
				t.Fatalf("%s directly imports unapproved project package %s", path, imported)
			}
		}
	}
}

func TestLibraryPackagesAreContextFree(t *testing.T) {
	fset := token.NewFileSet()
	for _, path := range productionLibraryGoFiles(t) {
		parsed, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		if importsPath(parsed, "context") {
			t.Fatalf("library file %s imports context", path)
		}
	}
}

func TestSchemaSourceIOOwnership(t *testing.T) {
	const sourceImport = "github.com/jacoelho/xsd/internal/source"
	root := repoRoot(t)
	sourceDir := filepath.Join(root, "internal", "source") + string(filepath.Separator)
	compileDir := filepath.Join(root, "internal", "compile") + string(filepath.Separator)
	fset := token.NewFileSet()
	goImporter := importer.ForCompiler(fset, "source", nil)
	for _, path := range productionLibraryGoFiles(t) {
		parsed, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		importsSource := false
		for _, imp := range parsed.Imports {
			imported, ok := goImportPath(imp)
			if !ok {
				t.Fatalf("decode import path %s in %s", imp.Path.Value, path)
			}
			switch {
			case isSchemaFileIOImport(imported):
				if !strings.HasPrefix(path, sourceDir) {
					t.Fatalf("library file %s imports source I/O package %s outside internal/source", path, imported)
				}
			case isNetworkTransportImport(imported):
				t.Fatalf("library file %s imports network-capable package %s", path, imported)
			}
			importsSource = importsSource || imported == sourceImport
		}
		if !importsSource {
			continue
		}
		info := &types.Info{
			Defs:       make(map[*ast.Ident]types.Object),
			Uses:       make(map[*ast.Ident]types.Object),
			Selections: make(map[*ast.SelectorExpr]*types.Selection),
		}
		conf := types.Config{
			Importer: goImporter,
			Error:    func(error) {},
		}
		packagePath := "github.com/jacoelho/xsd"
		if dir, err := filepath.Rel(root, filepath.Dir(path)); err == nil && dir != "." {
			packagePath += "/" + filepath.ToSlash(dir)
		}
		_, _ = conf.Check(packagePath, fset, []*ast.File{parsed}, info) //nolint:errcheck // Expected cross-file-name errors do not prevent imported object resolution.
		for _, obj := range info.Uses {
			fn, ok := sourcePackageFunction(obj)
			if !ok || sourceFunctionAllowed(path, root, compileDir, fn.Name()) {
				continue
			}
			t.Fatalf("library file %s references internal/source.%s outside its owner", path, fn.Name())
		}
		for _, selection := range info.Selections {
			obj := selection.Obj()
			fn, ok := obj.(*types.Func)
			if !ok || fn.Pkg() == nil || fn.Pkg().Path() != sourceImport {
				continue
			}
			if !sourceMethodAllowed(path, root, compileDir, fn) {
				t.Fatalf("library file %s references unapproved internal/source method %s.%s", path, methodReceiverName(fn), fn.Name())
			}
		}
	}
}

func sourcePackageFunction(obj types.Object) (*types.Func, bool) {
	fn, ok := obj.(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "github.com/jacoelho/xsd/internal/source" {
		return nil, false
	}
	signature, ok := fn.Type().(*types.Signature)
	return fn, ok && signature.Recv() == nil
}

func sourceFunctionAllowed(path, root, compileDir, name string) bool {
	if filepath.Dir(path) == root {
		return name == "Bytes" || name == "File" || name == "Opener"
	}
	if !strings.HasPrefix(path, compileDir) {
		return false
	}
	switch name {
	case "IsReferenceResolutionError", "IsSchemaLimitError", "Key", "NewReferenceBase":
		return true
	default:
		return false
	}
}

func sourceMethodAllowed(path, root, compileDir string, fn *types.Func) bool {
	receiver := methodReceiverName(fn)
	if filepath.Dir(path) == root {
		return receiver == "Source" && fn.Name() == "WithResolver"
	}
	if !strings.HasPrefix(path, compileDir) {
		return false
	}
	switch receiver {
	case "Input":
		return fn.Name() == "Finish"
	case "ReferenceBase":
		return fn.Name() == "WithXMLBase"
	case "Resolution":
		return fn.Name() == "Source" || fn.Name() == "Target"
	case "Source":
		switch fn.Name() {
		case "OpenInput", "Name", "ResolveFrom", "SameResolutionContext":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func methodReceiverName(fn *types.Func) string {
	signature, ok := fn.Type().(*types.Signature)
	if !ok || signature.Recv() == nil {
		return ""
	}
	receiver := signature.Recv().Type()
	if pointer, pointerOK := receiver.(*types.Pointer); pointerOK {
		receiver = pointer.Elem()
	}
	named, namedOK := receiver.(*types.Named)
	if !namedOK {
		return ""
	}
	return named.Obj().Name()
}

func isSchemaFileIOImport(path string) bool {
	return path == "os" || strings.HasPrefix(path, "os/") ||
		path == "io/ioutil" || path == "syscall" ||
		path == "golang.org/x/sys" || strings.HasPrefix(path, "golang.org/x/sys/")
}

func isNetworkTransportImport(path string) bool {
	return path == "net" || path == "net/smtp" || path == "crypto/tls" ||
		path == "net/http" || strings.HasPrefix(path, "net/http/") ||
		path == "net/rpc" || strings.HasPrefix(path, "net/rpc/")
}

func productionLibraryGoFiles(t *testing.T) []string {
	t.Helper()
	root := repoRoot(t)
	files := productionRootFiles(t, root)
	for _, dir := range []string{"internal", "xsderrors"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			files = append(files, path)
			return nil
		})
		if err != nil {
			t.Fatalf("walk %s: %v", dir, err)
		}
	}
	return files
}

func allowProjectImports(paths ...string) map[string]bool {
	const module = "github.com/jacoelho/xsd/"
	allowed := make(map[string]bool, len(paths))
	for _, path := range paths {
		allowed[module+path] = true
	}
	return allowed
}

func TestInternalPhasePackageImportGraph(t *testing.T) {
	packages := listPackages(t, "./internal/...")

	for _, path := range []string{
		"github.com/jacoelho/xsd/internal/compile",
		"github.com/jacoelho/xsd/internal/validate",
	} {
		if _, ok := packages[path]; !ok {
			t.Fatalf("phase package %s is missing", path)
		}
	}

	assertNoDeps(t, packages["github.com/jacoelho/xsd/internal/compile"],
		"github.com/jacoelho/xsd",
		"github.com/jacoelho/xsd/internal/validate",
	)
	assertNoDeps(t, packages["github.com/jacoelho/xsd/internal/validate"],
		"github.com/jacoelho/xsd",
		"github.com/jacoelho/xsd/internal/compile",
	)
}

func TestValidationInputPackageImportGraph(t *testing.T) {
	packages := listPackages(t, "./internal/validate", "./internal/stream")
	assertImports(t, packages["github.com/jacoelho/xsd/internal/validate"],
		"github.com/jacoelho/xsd/internal/stream",
	)
	assertNoDeps(t, packages["github.com/jacoelho/xsd/internal/stream"],
		"github.com/jacoelho/xsd/internal/validate",
		"github.com/jacoelho/xsd",
	)
}

func TestFormatPackageImportGraph(t *testing.T) {
	packages := listPackages(t, "./internal/format")
	assertNoDeps(t, packages["github.com/jacoelho/xsd/internal/format"],
		"github.com/jacoelho/xsd",
		"github.com/jacoelho/xsd/internal/compile",
		"github.com/jacoelho/xsd/internal/validate",
	)
}

func TestPublicLibraryPackageSurface(t *testing.T) {
	packages := listPackages(t, "./...")
	allowed := map[string]bool{
		"github.com/jacoelho/xsd":           true,
		"github.com/jacoelho/xsd/xsderrors": true,
	}
	for path := range packages {
		switch {
		case allowed[path]:
			continue
		case strings.Contains(path, "/internal/"):
			continue
		case strings.Contains(path, "/cmd/"):
			continue
		case path == "github.com/jacoelho/xsd/tests":
			continue
		default:
			t.Fatalf("unexpected public library package %s", path)
		}
	}
}

func TestXMLNamespacePackageImportGraph(t *testing.T) {
	packages := listPackages(t, ".", "./internal/xmlns")
	assertNoDeps(t, packages["github.com/jacoelho/xsd/internal/xmlns"],
		"github.com/jacoelho/xsd",
		"github.com/jacoelho/xsd/internal/compile",
		"github.com/jacoelho/xsd/internal/validate",
	)
}

func TestRuntimeVocabularyPackageImportGraph(t *testing.T) {
	packages := listPackages(t, ".", "./internal/runtime")
	assertNoDeps(t, packages["github.com/jacoelho/xsd/internal/runtime"],
		"github.com/jacoelho/xsd",
		"github.com/jacoelho/xsd/internal/compile",
		"github.com/jacoelho/xsd/internal/validate",
	)
}

func TestSourcePackageImportGraph(t *testing.T) {
	packages := listPackages(t, ".", "./internal/source")
	assertImports(t, packages["github.com/jacoelho/xsd"],
		"github.com/jacoelho/xsd/internal/source",
	)
	assertNoDeps(t, packages["github.com/jacoelho/xsd/internal/source"],
		"github.com/jacoelho/xsd",
		"github.com/jacoelho/xsd/internal/compile",
		"github.com/jacoelho/xsd/internal/validate",
		"github.com/jacoelho/xsd/internal/vocab",
	)
}

func listPackages(t *testing.T, patterns ...string) map[string]listedPackage {
	t.Helper()
	args := append([]string{"list", "-json"}, patterns...)
	//nolint:gosec // test-controlled go list patterns only.
	cmd := exec.CommandContext(t.Context(), "go", args...)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	dec := json.NewDecoder(strings.NewReader(string(out)))
	packages := make(map[string]listedPackage)
	for dec.More() {
		var pkg listedPackage
		if err := dec.Decode(&pkg); err != nil {
			t.Fatalf("decode go list JSON: %v", err)
		}
		packages[pkg.ImportPath] = pkg
	}
	return packages
}

func assertImports(t *testing.T, pkg listedPackage, required ...string) {
	t.Helper()
	for _, want := range required {
		if !slices.Contains(pkg.Imports, want) {
			t.Fatalf("%s does not import required package %s", pkg.ImportPath, want)
		}
	}
}

func assertNoDeps(t *testing.T, pkg listedPackage, forbidden ...string) {
	t.Helper()
	for _, dep := range pkg.Deps {
		for _, bad := range forbidden {
			if forbiddenDependency(dep, bad) {
				t.Fatalf("%s depends on forbidden phase package %s via %s", pkg.ImportPath, bad, dep)
			}
		}
	}
}

func forbiddenDependency(dep, forbidden string) bool {
	if forbidden == "github.com/jacoelho/xsd" {
		return dep == forbidden
	}
	return dep == forbidden || strings.HasPrefix(dep, forbidden+"/")
}
