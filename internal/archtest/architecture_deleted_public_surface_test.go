package archtest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetiredPublicPackagesAreGone(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{"errors", "pkg"} {
		path := filepath.Join(root, rel)
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("retired public package path still exists: %s", rel)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat retired public package path %s: %v", rel, err)
		}
	}
}

func TestRetiredPublicRootExportsAreGone(t *testing.T) {
	got := collectRootExports(t)
	retired := []string{
		"type BuildOption",
		"type BuildOptionValue",
		"type CompileOption",
		"type PreparedSchema",
		"type SourceOption",
		"type SourceOptionValue",
		"type SourceSet",
		"type ValidateOption",
		"type ValidateOptionValue",
		"func AllowMissingImportLocations",
		"func Compile",
		"func InstanceMaxAttrs",
		"func InstanceMaxDepth",
		"func InstanceMaxQNameInternEntries",
		"func InstanceMaxTokenSize",
		"func MaxDFAStates",
		"func MaxOccursLimit",
		"func NewSourceSet",
		"func SchemaMaxAttrs",
		"func SchemaMaxDepth",
		"func SchemaMaxQNameInternEntries",
		"func SchemaMaxTokenSize",
		"method PreparedSchema.Build",
		"method SourceSet.AddFS",
		"method SourceSet.Build",
		"method SourceSet.Prepare",
		"method SourceSet.WithOptions",
	}
	for _, item := range retired {
		if _, ok := got[item]; ok {
			t.Fatalf("retired public export still present: %s", item)
		}
	}
}

func TestRetiredPublicImportPathsAreGone(t *testing.T) {
	root := repoRoot(t)
	retired := []string{
		importLiteral("errors"),
		importLiteral("pkg/xmlstream"),
		importLiteral("pkg/xmltext"),
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		for _, term := range retired {
			if strings.Contains(text, term) {
				rel, relErr := filepath.Rel(root, path)
				if relErr != nil {
					rel = path
				}
				t.Fatalf("%s imports retired public path %s", rel, term)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repo: %v", err)
	}
}

func importLiteral(path string) string {
	return `"` + modulePath + "/" + path + `"`
}
