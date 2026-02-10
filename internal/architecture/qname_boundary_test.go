package architecture_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestQNameStructDefinedOnlyInCanonicalPackage(t *testing.T) {
	t.Parallel()

	const canonicalPath = "internal/qname/qname.go"
	count := 0

	forEachParsedRepoProductionGoFile(t, parser.ParseComments, func(file repoGoFile, parsed *ast.File) {
		for _, decl := range parsed.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name.Name != "QName" {
					continue
				}
				if _, ok := typeSpec.Type.(*ast.StructType); !ok {
					continue
				}
				count++
				if file.relPath != canonicalPath {
					t.Fatalf("qname boundary violation: QName struct declared in %s", file.relPath)
				}
			}
		}
	})

	if count != 1 {
		t.Fatalf("QName struct declarations = %d, want 1 in %s", count, canonicalPath)
	}
}
