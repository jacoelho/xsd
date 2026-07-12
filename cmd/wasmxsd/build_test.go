//go:build !(js && wasm)

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWASMTargetBuilds(t *testing.T) {
	wasm := filepath.Join(t.TempDir(), "xsd.wasm")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-ldflags=-s -w", "-o", wasm, ".") //nolint:gosec // Test intentionally invokes fixed go build command.
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build js/wasm failed: %v\n%s", err, out)
	}
}

func TestHostTargetBuilds(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "wasmxsd")
	cmd := exec.CommandContext(t.Context(), "go", "build", "-o", bin, ".") //nolint:gosec // Test intentionally invokes fixed go build command.
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build host failed: %v\n%s", err, out)
	}
}
