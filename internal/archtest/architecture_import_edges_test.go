package archtest_test

import (
	"strings"
	"testing"
)

func TestCorePhaseImportEdges(t *testing.T) {
	imports := collectPackageImports(t)

	assertNoExactImports(
		t,
		imports,
		internalPkg("archtest"),
		internalPkg("compiler"),
		internalPkg("validator"),
	)

	assertNoExactImports(
		t,
		imports,
		internalPkg("contentmodel"),
		internalPkg("parser"),
		internalPkg("schemaast"),
		internalPkg("archtest"),
		internalPkg("compiler"),
		internalPkg("validator"),
	)

	assertNoExactImports(
		t,
		imports,
		internalPkg("validator"),
		internalPkg("parser"),
		internalPkg("archtest"),
		internalPkg("compiler"),
	)

	assertPrefixPackagesDoNotImportExact(
		t,
		imports,
		internalPkg("validator"),
		internalPkg("parser"),
		internalPkg("archtest"),
		internalPkg("compiler"),
	)

	assertPrefixPackagesDoNotImportExact(
		t,
		imports,
		internalPkg("contentmodel"),
		internalPkg("parser"),
		internalPkg("archtest"),
		internalPkg("compiler"),
		internalPkg("validator"),
	)
}

func TestCompilePackagesUseSchemaASTBoundary(t *testing.T) {
	imports := collectPackageImports(t)
	for _, pkg := range []string{
		internalPkg("compiler"),
		internalPkg("schemair"),
		internalPkg("xsdpath"),
		internalPkg("runtimebuild/valuebuild"),
		internalPkg("contentmodel"),
		internalPkg("runtimebuild"),
	} {
		assertNoExactImports(t, imports, pkg, internalPkg("model"), internalPkg("parser"))
	}
}

func TestRetiredArchitecturePackagesStayGone(t *testing.T) {
	imports := collectPackageImports(t)
	for _, pkg := range []string{
		internalPkg("analysis"),
		internalPkg("schemair/" + "components"),
		internalPkg("schemaindex"),
		internalPkg("schemaeffective"),
		internalPkg("semantics"),
		internalPkg("schemair/" + "check"),
		internalPkg("schemair/" + "resolveindex"),
		internalPkg("schemair/" + "valuebuild"),
		internalPkg("schemair/" + "upamodel"),
		internalPkg("schemair/" + "xpath"),
	} {
		if _, ok := imports[pkg]; ok {
			t.Fatalf("retired package still present: %s", pkg)
		}
	}
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
