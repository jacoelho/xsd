package architecture_test

import (
	"testing"
)

func TestCorePhaseImportEdges(t *testing.T) {
	imports := collectPackageImports(t)

	rules := map[string][]string{
		internalPkg("preprocessor"): {
			internalPkg("semanticresolve"),
			internalPkg("semanticcheck"),
			internalPkg("analysis"),
			internalPkg("compiler"),
			internalPkg("validatorgen"),
			internalPkg("validator"),
		},
		internalPkg("parser"): {
			internalPkg("semanticresolve"),
			internalPkg("semanticcheck"),
			internalPkg("analysis"),
			internalPkg("compiler"),
			internalPkg("validatorgen"),
			internalPkg("validator"),
		},
		internalPkg("semanticresolve"): {
			internalPkg("preprocessor"),
			internalPkg("compiler"),
			internalPkg("validatorgen"),
			internalPkg("validator"),
		},
		internalPkg("semanticcheck"): {
			internalPkg("preprocessor"),
			internalPkg("compiler"),
			internalPkg("validatorgen"),
			internalPkg("validator"),
		},
		internalPkg("analysis"): {
			internalPkg("preprocessor"),
			internalPkg("compiler"),
			internalPkg("validatorgen"),
			internalPkg("validator"),
		},
		internalPkg("validator"): {
			internalPkg("preprocessor"),
			internalPkg("parser"),
			internalPkg("semanticresolve"),
			internalPkg("semanticcheck"),
			internalPkg("analysis"),
			internalPkg("compiler"),
			internalPkg("validatorgen"),
		},
	}

	for pkg, forbidden := range rules {
		pkgImports, ok := imports[pkg]
		if !ok {
			t.Fatalf("package %s not found in import graph", pkg)
		}
		for imp := range pkgImports {
			for _, bad := range forbidden {
				if hasPkgPrefix(imp, bad) {
					t.Errorf("%s must not import %s", pkg, imp)
				}
			}
		}
	}
}
