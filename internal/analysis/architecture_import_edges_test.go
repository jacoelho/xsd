package analysis_test

import (
	"strings"
	"testing"
)

func TestCorePhaseImportEdges(t *testing.T) {
	imports := collectPackageImports(t)

	assertNoExactImports(
		t,
		imports,
		internalPkg("parser"),
		internalPkg("analysis"),
		internalPkg("compiler"),
		internalPkg("validator"),
	)

	assertNoExactImports(
		t,
		imports,
		internalPkg("analysis"),
		internalPkg("compiler"),
		internalPkg("validator"),
	)

	assertNoExactImports(
		t,
		imports,
		internalPkg("semantics"),
		internalPkg("compiler"),
		internalPkg("validator"),
	)

	assertNoExactImports(
		t,
		imports,
		internalPkg("validator"),
		internalPkg("parser"),
		internalPkg("analysis"),
		internalPkg("compiler"),
	)

	assertPrefixPackagesDoNotImportExact(
		t,
		imports,
		internalPkg("validator"),
		internalPkg("parser"),
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
