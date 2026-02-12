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
			internalPkg("prep"),
			internalPkg("normalize"),
			internalPkg("compiler"),
			internalPkg("runtimeassemble"),
			internalPkg("validatorgen"),
			internalPkg("validationengine"),
			internalPkg("validator"),
		},
		internalPkg("parser"): {
			internalPkg("semanticresolve"),
			internalPkg("semanticcheck"),
			internalPkg("analysis"),
			internalPkg("prep"),
			internalPkg("normalize"),
			internalPkg("compiler"),
			internalPkg("runtimeassemble"),
			internalPkg("validatorgen"),
			internalPkg("validationengine"),
			internalPkg("validator"),
			internalPkg("set"),
		},
		internalPkg("semanticresolve"): {
			internalPkg("preprocessor"),
			internalPkg("normalize"),
			internalPkg("compiler"),
			internalPkg("set"),
			internalPkg("runtimeassemble"),
			internalPkg("validatorgen"),
			internalPkg("validationengine"),
			internalPkg("validator"),
		},
		internalPkg("semanticcheck"): {
			internalPkg("preprocessor"),
			internalPkg("normalize"),
			internalPkg("compiler"),
			internalPkg("set"),
			internalPkg("runtimeassemble"),
			internalPkg("validatorgen"),
			internalPkg("validationengine"),
			internalPkg("validator"),
		},
		internalPkg("analysis"): {
			internalPkg("preprocessor"),
			internalPkg("normalize"),
			internalPkg("compiler"),
			internalPkg("set"),
			internalPkg("runtimeassemble"),
			internalPkg("validatorgen"),
			internalPkg("validationengine"),
			internalPkg("validator"),
		},
		internalPkg("runtimeassemble"): {
			internalPkg("preprocessor"),
			internalPkg("semanticresolve"),
			internalPkg("semanticcheck"),
			internalPkg("normalize"),
			internalPkg("compiler"),
			internalPkg("set"),
			internalPkg("validationengine"),
			internalPkg("validator"),
		},
		internalPkg("validationengine"): {
			internalPkg("preprocessor"),
			internalPkg("parser"),
			internalPkg("semanticresolve"),
			internalPkg("semanticcheck"),
			internalPkg("analysis"),
			internalPkg("normalize"),
			internalPkg("compiler"),
			internalPkg("runtimeassemble"),
			internalPkg("validatorgen"),
			internalPkg("set"),
		},
		internalPkg("validator"): {
			internalPkg("preprocessor"),
			internalPkg("parser"),
			internalPkg("semanticresolve"),
			internalPkg("semanticcheck"),
			internalPkg("analysis"),
			internalPkg("normalize"),
			internalPkg("compiler"),
			internalPkg("runtimeassemble"),
			internalPkg("validatorgen"),
			internalPkg("set"),
		},
		internalPkg("set"): {
			internalPkg("semanticresolve"),
			internalPkg("semanticcheck"),
			internalPkg("analysis"),
			internalPkg("runtimeassemble"),
			internalPkg("validatorgen"),
			internalPkg("validationengine"),
			internalPkg("validator"),
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
