package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunCommandSurface(t *testing.T) {
	t.Parallel()

	t.Run("prepare command exists", func(t *testing.T) {
		t.Parallel()

		var out, errOut bytes.Buffer
		code := run([]string{"prepare", "-h"}, &out, &errOut)
		if code != 2 {
			t.Fatalf("run(prepare -h) code = %d, want 2", code)
		}
		if !strings.Contains(errOut.String(), "Usage of prepare:") {
			t.Fatalf("missing prepare usage in stderr: %q", errOut.String())
		}
	})

	for _, cmd := range []string{"validate", "all"} {
		cmd := cmd
		t.Run(cmd+" command rejected", func(t *testing.T) {
			t.Parallel()

			var out, errOut bytes.Buffer
			code := run([]string{cmd}, &out, &errOut)
			if code != 2 {
				t.Fatalf("run(%s) code = %d, want 2", cmd, code)
			}
			if !strings.Contains(errOut.String(), "Commands:") {
				t.Fatalf("expected usage output for %s, got: %q", cmd, errOut.String())
			}
		})
	}
}
