package archtest_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConcernClustersStayGrouped(t *testing.T) {
	root := repoRoot(t)

	checks := []struct {
		dir       string
		prefix    string
		maxFiles  int
		wantFiles int
	}{
		{dir: filepath.Join(root, "internal", "schemair"), prefix: "semantics_particles", maxFiles: 3},
	}

	for _, check := range checks {
		files, err := prefixedGoFiles(check.dir, check.prefix)
		if err != nil {
			t.Fatalf("scan %s: %v", check.dir, err)
		}
		if check.wantFiles > 0 && len(files) != check.wantFiles {
			t.Errorf("%s prefix %q should stay grouped in %d file(s), found %d: %v", check.dir, check.prefix, check.wantFiles, len(files), files)
		}
		if check.maxFiles > 0 && len(files) > check.maxFiles {
			t.Errorf("%s prefix %q should stay grouped in <= %d file(s), found %d: %v", check.dir, check.prefix, check.maxFiles, len(files), files)
		}
	}

	retired := []string{
		filepath.Join(root, "internal", "validator", "runtime_text_value.go"),
		filepath.Join(root, "internal", "validator", "runtime_value_execution.go"),
		filepath.Join(root, "internal", "validator", "value_atomic_canonicalize.go"),
		filepath.Join(root, "internal", "validator", "value_atomic_validate.go"),
		filepath.Join(root, "internal", "validator", "value_binary_canonicalize.go"),
		filepath.Join(root, "internal", "validator", "value_id_track.go"),
		filepath.Join(root, "internal", "validator", "value_id_track_core.go"),
		filepath.Join(root, "internal", "validator", "value_key_derivation.go"),
		filepath.Join(root, "internal", "validator", "value_list.go"),
		filepath.Join(root, "internal", "validator", "value_qname.go"),
		filepath.Join(root, "internal", "validator", "value_storage.go"),
		filepath.Join(root, "internal", "validator", "value_temporal_canonicalize.go"),
		filepath.Join(root, "internal", "validator", "value_union_core.go"),
	}
	for _, path := range retired {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("retired shard returned: %s", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", path, err)
		}
	}
}

func prefixedGoFiles(dir, prefix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, name)
	}
	return files, nil
}
