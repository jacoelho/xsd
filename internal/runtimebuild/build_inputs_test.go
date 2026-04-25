package runtimebuild

import "testing"

func TestBuildRejectsMissingSchemaIR(t *testing.T) {
	_, err := Build(Input{})
	if err == nil || err.Error() != "runtime build: schema ir is nil" {
		t.Fatalf("Build() error = %v, want %q", err, "runtime build: schema ir is nil")
	}
}
