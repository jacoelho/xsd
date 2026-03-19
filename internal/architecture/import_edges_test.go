package architecture_test

import (
	"strings"
	"testing"
)

func TestCorePhaseImportEdges(t *testing.T) {
	imports := collectPackageImports(t)

	forbiddenPhaseRoots := []string{
		internalPkg("semanticresolve"),
		internalPkg("semanticcheck"),
		internalPkg("analysis"),
		internalPkg("compiler"),
		internalPkg("validator"),
	}

	assertNoExactImports(t, imports, internalPkg("preprocessor"), forbiddenPhaseRoots...)
	assertNoExactImports(t, imports, internalPkg("preprocessor/merge"), forbiddenPhaseRoots...)
	assertNoExactImports(t, imports, internalPkg("preprocessor/resolve"), append(forbiddenPhaseRoots, internalPkg("parser"))...)

	assertNoExactImports(
		t,
		imports,
		internalPkg("parser"),
		internalPkg("semanticresolve"),
		internalPkg("semanticcheck"),
		internalPkg("analysis"),
		internalPkg("compiler"),
		internalPkg("validator"),
	)

	assertNoExactImports(
		t,
		imports,
		internalPkg("semanticresolve"),
		internalPkg("preprocessor"),
		internalPkg("compiler"),
		internalPkg("validator"),
	)
	assertNoExactImports(
		t,
		imports,
		internalPkg("semanticcheck"),
		internalPkg("preprocessor"),
		internalPkg("compiler"),
		internalPkg("validator"),
	)
	assertNoExactImports(
		t,
		imports,
		internalPkg("analysis"),
		internalPkg("preprocessor"),
		internalPkg("compiler"),
		internalPkg("validator"),
	)

	assertNoExactImports(
		t,
		imports,
		internalPkg("compiler/lower"),
		internalPkg("preprocessor"),
		internalPkg("semanticresolve"),
		internalPkg("semanticcheck"),
		internalPkg("validator"),
	)

	assertNoExactImports(
		t,
		imports,
		internalPkg("validator"),
		internalPkg("preprocessor"),
		internalPkg("parser"),
		internalPkg("semanticresolve"),
		internalPkg("semanticcheck"),
		internalPkg("analysis"),
		internalPkg("compiler"),
	)

	assertPrefixPackagesDoNotImportExact(
		t,
		imports,
		internalPkg("validator"),
		internalPkg("preprocessor"),
		internalPkg("parser"),
		internalPkg("semanticresolve"),
		internalPkg("semanticcheck"),
		internalPkg("analysis"),
		internalPkg("compiler"),
	)
}

func assertNoExactImports(t *testing.T, imports map[string]map[string]struct{}, pkg string, forbidden ...string) {
	t.Helper()

	pkgImports, ok := imports[pkg]
	if !ok {
		t.Fatalf("package %s not found in import graph", pkg)
	}
	for _, imp := range forbidden {
		if _, ok := pkgImports[imp]; ok {
			t.Errorf("%s must not import %s", pkg, imp)
		}
	}
}

func assertPrefixPackagesDoNotImportExact(t *testing.T, imports map[string]map[string]struct{}, prefix string, forbidden ...string) {
	t.Helper()

	prefix += "/"
	for pkg, pkgImports := range imports {
		if !strings.HasPrefix(pkg, prefix) {
			continue
		}
		for _, imp := range forbidden {
			if _, ok := pkgImports[imp]; ok {
				t.Errorf("%s must not import %s", pkg, imp)
			}
		}
	}
}
