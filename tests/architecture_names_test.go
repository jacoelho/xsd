package tests

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// architectureNonGoNames are the code-shaped names ARCHITECTURE.md uses for
// things the module does not declare.
var architectureNonGoNames = []string{
	// Rejected Alternatives names an API or option that does not exist.
	"Builder.ValidateUncharged",
	"CheckWellFormed",
	"MaxIdentityFieldSlots",
	"MaxSchemaLocationNamespaceBytes",
	"MaxSchemaLocationNamespaces",
	// Standard library identifiers.
	"fmt.Sprintf",
	"io.ErrShortWrite",
	"unsafe.Sizeof",
	// XSD attribute names.
	"maxOccurs",
	"minOccurs",
	// W3C corpus case names, a file and a JavaScript class.
	"addB194",
	"time_maxInclusive007_119",
	"Makefile",
	"ValidationWorkerClient",
}

var architectureNamePattern = regexp.MustCompile("`([A-Za-z_][A-Za-z0-9_]*(?:\\.[A-Za-z_][A-Za-z0-9_]*)*)`")

func TestArchitectureNamesOnlyDeclaredGoIdentifiers(t *testing.T) {
	root := ".."
	doc, err := os.ReadFile(filepath.Join(root, "ARCHITECTURE.md"))
	if err != nil {
		t.Fatal(err)
	}
	declared := moduleDeclarations(t, root)
	nonGo := make(map[string]bool, len(architectureNonGoNames))
	for _, name := range architectureNonGoNames {
		nonGo[name] = true
	}

	named := architectureCodeNames(string(doc))
	for _, name := range staleArchitectureNames(named, declared, nonGo) {
		t.Errorf("ARCHITECTURE.md names `%s`, which no Go file in the module declares", name)
	}
	for _, name := range architectureNonGoNames {
		if declared[lastName(name)] {
			t.Errorf("architectureNonGoNames lists %q, which is a Go declaration", name)
		}
		if !slices.Contains(named, name) {
			t.Errorf("architectureNonGoNames lists %q, which ARCHITECTURE.md no longer names", name)
		}
	}
}

func TestStaleArchitectureNamesReportsUndeclaredIdentifiers(t *testing.T) {
	doc := "`declaredOwner` charges `removedOwner`; `pkg.MissingRead` and `pkg.DeclaredRead` " +
		"are reads; `minOccurs` is XSD; `go.mod`, `XSD` and `ID` are not identifiers."
	declared := map[string]bool{"declaredOwner": true, "DeclaredRead": true}
	nonGo := map[string]bool{"minOccurs": true}

	got := staleArchitectureNames(architectureCodeNames(doc), declared, nonGo)
	want := []string{"pkg.MissingRead", "removedOwner"}
	if !slices.Equal(got, want) {
		t.Fatalf("stale names = %q, want %q", got, want)
	}
}

// architectureCodeNames returns the distinct code spans of doc shaped like a
// Go identifier or selector whose last element mixes case, which is how Go
// names are written and acronyms, file names and spec terms are not.
func architectureCodeNames(doc string) []string {
	var names []string
	for _, match := range architectureNamePattern.FindAllStringSubmatch(doc, -1) {
		name := match[1]
		last := lastName(name)
		if strings.ToUpper(last) == last || strings.ToLower(last) == last {
			continue
		}
		if !slices.Contains(names, name) {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

func lastName(name string) string {
	return name[strings.LastIndexByte(name, '.')+1:]
}

func staleArchitectureNames(named []string, declared, nonGo map[string]bool) []string {
	var stale []string
	for _, name := range named {
		if !declared[lastName(name)] && !nonGo[name] {
			stale = append(stale, name)
		}
	}
	return stale
}

// moduleDeclarations returns every name a Go file of the module declares at
// package level: functions, methods, types, constants, variables, and the
// fields and methods of declared struct and interface types, in production and
// test files of every build constraint. Parameters and locals are not
// declarations a document names.
func moduleDeclarations(t *testing.T, root string) map[string]bool {
	t.Helper()
	declared := make(map[string]bool)
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); path != root && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "bin") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				declared[decl.Name.Name] = true
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						declared[spec.Name.Name] = true
						addMemberNames(declared, spec.Type)
					case *ast.ValueSpec:
						for _, name := range spec.Names {
							declared[name.Name] = true
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return declared
}

// addMemberNames adds the fields and methods of the struct and interface types
// in expr, including nested ones, but not the parameters of function types.
func addMemberNames(declared map[string]bool, expr ast.Expr) {
	ast.Inspect(expr, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncType:
			return false
		case *ast.Field:
			for _, name := range node.Names {
				declared[name.Name] = true
			}
		}
		return true
	})
}
