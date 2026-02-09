package architecture_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestRuntimeCompilePackageRemoved(t *testing.T) {
	t.Parallel()

	root := repoRoot(t)
	dir := filepath.Join(root, "internal", "runtimecompile")
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return
	}
	t.Fatalf("runtimecompile package still exists: %s", dir)
}
