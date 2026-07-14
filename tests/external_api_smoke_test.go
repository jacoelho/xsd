package tests_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestExternalModuleUsesPublicSchemaAPI(t *testing.T) {
	dir := t.TempDir()
	writeExternalSmokeFile(t, filepath.Join(dir, "go.mod"), `module external-api-smoke

go 1.26.2

require github.com/jacoelho/xsd v0.0.0

replace github.com/jacoelho/xsd => `+strconv.Quote(repoRoot(t))+`
`)
	writeExternalSmokeFile(t, filepath.Join(dir, "api_test.go"), `package external_api_smoke

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
	"github.com/jacoelho/xsd/xsderrors"
)

func TestExternalAPI(t *testing.T) {
	var _ xsd.Resolver = xsd.ResolverFunc(func(context.Context, string, string) (xsd.SchemaSource, error) {
		return xsd.SchemaSource{}, xsderrors.ErrSchemaNotFound
	})
	engine, err := xsd.Compile(context.Background(), xsd.Bytes("schema.xsd", []byte(`+"`"+`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:int"/></xs:schema>`+"`"+`)))
	if err != nil {
		t.Fatal(err)
	}
	err = engine.Validate(context.Background(), strings.NewReader(`+"`"+`<root>x</root>`+"`"+`))
	var xerr *xsderrors.Error
	if !errors.As(err, &xerr) {
		t.Fatalf("error type = %T", err)
	}
}
`)
	cmd := exec.CommandContext(t.Context(), "go", "test", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("external go test failed: %v\n%s", err, out)
	}
}

func writeExternalSmokeFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.ToSlash(dir)
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}
