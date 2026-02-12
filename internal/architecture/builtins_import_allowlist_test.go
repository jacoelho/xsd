package architecture_test

import "testing"

func TestBuiltinsImportAllowlist(t *testing.T) {
	imports := collectPackageImports(t)
	builtinsPkg := internalPkg("builtins")
	builtinsImports, ok := imports[builtinsPkg]
	if !ok {
		t.Fatalf("package %s not found in import graph", builtinsPkg)
	}

	allowed := map[string]struct{}{
		internalPkg("types"):              {},
		modulePath + "/internal/xmlnames": {},
	}
	for imported := range builtinsImports {
		if _, ok := allowed[imported]; !ok {
			t.Fatalf("%s has disallowed import: %s", builtinsPkg, imported)
		}
	}
}
