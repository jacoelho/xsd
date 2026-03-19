package architecture_test

import "testing"

func TestLeafPackagesDoNotImportParentShell(t *testing.T) {
	imports := collectPackageImports(t)
	shells := []string{
		internalPkg("compiler"),
		internalPkg("preprocessor"),
		internalPkg("validator"),
	}

	for pkg, pkgImports := range imports {
		for _, shell := range shells {
			if pkg == shell || !hasPkgPrefix(pkg, shell) {
				continue
			}
			if _, ok := pkgImports[shell]; ok {
				t.Errorf("leaf package %s imports parent shell package %s", pkg, shell)
			}
		}
	}
}
