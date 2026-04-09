package analysis_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompilerArtifactFilesOwnArtifactPrefix(t *testing.T) {
	root := repoRoot(t)
	compilerDir := filepath.Join(root, "internal", "compiler")
	semanticsDir := filepath.Join(root, "internal", "semantics")
	validatorBuildDir := filepath.Join(root, "internal", "validatorbuild")

	required := []string{
		"artifact_core.go",
		"artifact_build_orchestrate.go",
		"artifact_build_registry.go",
		"artifact_emit.go",
		"artifact_defaults_orchestrate.go",
		"artifact_resolver_core.go",
		"artifact_value_keys_flow.go",
	}
	for _, name := range required {
		path := filepath.Join(validatorBuildDir, name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("missing validatorbuild artifact file %s: %v", path, err)
		}
	}

	entries, err := os.ReadDir(semanticsDir)
	if err != nil {
		t.Fatalf("read semantics dir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "artifact_") {
			t.Fatalf("artifact file still present under semantics: %s", filepath.Join(semanticsDir, entry.Name()))
		}
	}

	legacyCompiler := []string{
		"default_fixed_set.go",
		"defaults_orchestrate.go",
		"defaults_value_types.go",
		"resolver_core.go",
		"resolver_properties.go",
		"value_keys_flow.go",
		"values.go",
		"comparable_value.go",
		"whitespace.go",
	}
	for _, name := range legacyCompiler {
		path := filepath.Join(compilerDir, name)
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("legacy compiler artifact helper file still present: %s", path)
		}
	}

	compilerEntries, err := os.ReadDir(compilerDir)
	if err != nil {
		t.Fatalf("read compiler dir: %v", err)
	}
	for _, entry := range compilerEntries {
		if strings.HasPrefix(entry.Name(), "artifact_") {
			t.Fatalf("artifact file still present under compiler: %s", filepath.Join(compilerDir, entry.Name()))
		}
	}
}
