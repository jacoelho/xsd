package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestPublicAPISurfaceAllowlist(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	allowlist := map[string]map[string]string{
		"pkg/xmlstream": {
			"Attr":              "type",
			"ElementID":         "type",
			"ErrUnboundPrefix":  "var",
			"Event":             "type",
			"EventCharData":     "const",
			"EventComment":      "const",
			"EventDirective":    "const",
			"EventEndElement":   "const",
			"EventKind":         "type",
			"EventPI":           "const",
			"EventStartElement": "const",
			"JoinOptions":       "func",
			"NameID":            "type",
			"NamespaceDecl":     "type",
			"NewReader":         "func",
			"Option":            "type",
			"QName":             "type",
			"RawAttr":           "type",
			"RawEvent":          "type",
			"RawName":           "type",
			"Reader":            "type",
			"ResolvedAttr":      "type",
			"ResolvedEvent":     "type",
			"Unmarshaler":       "type",
			"XMLNSNamespace":    "const",
			"XMLNamespace":      "const",
			"XSDNamespace":      "const",
			"XSINamespace":      "const",
		},
		"pkg/xmltext": {
			"Attr":                  "type",
			"CoalesceCharData":      "func",
			"Decoder":               "type",
			"EmitComments":          "func",
			"EmitDirectives":        "func",
			"EmitPI":                "func",
			"FastValidation":        "func",
			"JoinOptions":           "func",
			"Kind":                  "type",
			"KindCDATA":             "const",
			"KindCharData":          "const",
			"KindComment":           "const",
			"KindDirective":         "const",
			"KindEndElement":        "const",
			"KindNone":              "const",
			"KindPI":                "const",
			"KindStartElement":      "const",
			"MaxAttrs":              "func",
			"MaxDepth":              "func",
			"MaxQNameInternEntries": "func",
			"MaxTokenSize":          "func",
			"NewDecoder":            "func",
			"Options":               "type",
			"RawAttr":               "type",
			"RawToken":              "type",
			"RawTokenSpan":          "type",
			"ResolveEntities":       "func",
			"Strict":                "func",
			"SyntaxError":           "type",
			"Token":                 "type",
			"TokenSizes":            "type",
			"TrackLineColumn":       "func",
			"WithCharsetReader":     "func",
			"WithEntityMap":         "func",
		},
	}

	for pkgDir, expected := range allowlist {
		actual, err := exportedPackageSymbols(filepath.Join(root, pkgDir))
		if err != nil {
			t.Fatalf("collect exports for %s: %v", pkgDir, err)
		}
		assertAllowlistedExports(t, pkgDir, expected, actual)
	}
}

func exportedPackageSymbols(dir string) (map[string]string, error) {
	fset := token.NewFileSet()
	exports := make(map[string]string)
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		for _, decl := range file.Decls {
			switch typed := decl.(type) {
			case *ast.GenDecl:
				switch typed.Tok {
				case token.CONST:
					for _, spec := range typed.Specs {
						valueSpec, ok := spec.(*ast.ValueSpec)
						if !ok {
							continue
						}
						for _, name := range valueSpec.Names {
							if ast.IsExported(name.Name) {
								exports[name.Name] = "const"
							}
						}
					}
				case token.VAR:
					for _, spec := range typed.Specs {
						valueSpec, ok := spec.(*ast.ValueSpec)
						if !ok {
							continue
						}
						for _, name := range valueSpec.Names {
							if ast.IsExported(name.Name) {
								exports[name.Name] = "var"
							}
						}
					}
				case token.TYPE:
					for _, spec := range typed.Specs {
						typeSpec, ok := spec.(*ast.TypeSpec)
						if !ok {
							continue
						}
						if ast.IsExported(typeSpec.Name.Name) {
							exports[typeSpec.Name.Name] = "type"
						}
					}
				}
			case *ast.FuncDecl:
				if typed.Recv == nil && ast.IsExported(typed.Name.Name) {
					exports[typed.Name.Name] = "func"
				}
			}
		}
		return nil
	})
	return exports, err
}

func assertAllowlistedExports(t *testing.T, pkgDir string, expected, actual map[string]string) {
	t.Helper()
	for name, kind := range actual {
		wantKind, ok := expected[name]
		if !ok {
			t.Fatalf("public API allowlist violation in %s: unexpected export %s (%s)", pkgDir, name, kind)
		}
		if wantKind != kind {
			t.Fatalf("public API allowlist violation in %s: export %s kind=%s, want=%s", pkgDir, name, kind, wantKind)
		}
	}
	missing := make([]string, 0, len(expected))
	for name := range expected {
		if _, ok := actual[name]; !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	t.Fatalf("public API allowlist mismatch in %s: missing exports %v", pkgDir, missing)
}
