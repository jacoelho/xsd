package architecture_test

import "testing"

func TestBuiltinsDoesNotImportModelDirectly(t *testing.T) {
	imports := collectPackageImports(t)
	builtinsPkg := internalPkg("builtins")
	modelPkg := internalPkg("model")

	builtinsImports, ok := imports[builtinsPkg]
	if !ok {
		t.Fatalf("package %s not found in import graph", builtinsPkg)
	}

	for imported := range builtinsImports {
		if hasPkgPrefix(imported, modelPkg) {
			t.Fatalf("%s must not import %s directly", builtinsPkg, imported)
		}
	}
}
