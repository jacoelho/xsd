package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWithArgs(t *testing.T) {
	tempDir := t.TempDir()
	schemaPath := writeFile(t, tempDir, "schema.xsd", `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           elementFormDefault="qualified">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)
	validDocPath := writeFile(t, tempDir, "valid.xml", `<root xmlns="urn:test">ok</root>`)
	invalidDocPath := writeFile(t, tempDir, "invalid.xml", `<wrong/>`)
	longDocPath := writeFile(t, tempDir, "long.xml", `<root xmlns="urn:test">abcdefghijklmnopqrstuvwxyz</root>`)
	missingSchemaPath := filepath.Join(tempDir, "missing.xsd")

	tests := []struct {
		name        string
		programName string
		args        []string
		wantCode    int
		wantStdout  []string
		wantStderr  []string
	}{
		{
			name:        "missing schema prints usage with injected program name",
			programName: "custom-xmllint",
			args:        []string{validDocPath},
			wantCode:    2,
			wantStderr: []string{
				"error: --schema is required",
				"Usage: custom-xmllint --schema <schema.xsd> <document.xml>",
			},
		},
		{
			name:        "negative instance max token size rejected",
			programName: "xmllint",
			args:        []string{"--schema", schemaPath, "--instance-max-token-size", "-1", validDocPath},
			wantCode:    2,
			wantStderr:  []string{"error: --instance-max-token-size must be >= 0"},
		},
		{
			name:        "schema load failure",
			programName: "xmllint",
			args:        []string{"--schema", missingSchemaPath, validDocPath},
			wantCode:    1,
			wantStderr:  []string{"error loading schema:"},
		},
		{
			name:        "validation failure",
			programName: "xmllint",
			args:        []string{"--schema", schemaPath, invalidDocPath},
			wantCode:    1,
			wantStderr: []string{
				"[xsd-root-not-declared]",
				invalidDocPath + " fails to validate",
			},
		},
		{
			name:        "success",
			programName: "xmllint",
			args:        []string{"--schema", schemaPath, validDocPath},
			wantCode:    0,
			wantStdout:  []string{validDocPath + " validates"},
		},
		{
			name:        "instance max token size applied",
			programName: "xmllint",
			args:        []string{"--schema", schemaPath, "--instance-max-token-size", "8", longDocPath},
			wantCode:    1,
			wantStderr: []string{
				"[xml-parse-error]",
				longDocPath + " fails to validate",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			gotCode := runWithArgs(tt.programName, tt.args, &stdout, &stderr)
			if gotCode != tt.wantCode {
				t.Fatalf("runWithArgs() code = %d, want %d\nstdout:\n%s\nstderr:\n%s", gotCode, tt.wantCode, stdout.String(), stderr.String())
			}
			for _, want := range tt.wantStdout {
				if !strings.Contains(stdout.String(), want) {
					t.Fatalf("stdout = %q, want substring %q", stdout.String(), want)
				}
			}
			for _, want := range tt.wantStderr {
				if !strings.Contains(stderr.String(), want) {
					t.Fatalf("stderr = %q, want substring %q", stderr.String(), want)
				}
			}
		})
	}
}

func TestRunWithArgsSupportsSymlinkedSchemaRoot(t *testing.T) {
	tempDir := t.TempDir()
	schemaPath := writeFile(t, tempDir, "outside/main.xsd", `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:test"
           xmlns:tns="urn:test"
           xmlns:dep="urn:dep"
           elementFormDefault="qualified">
  <xs:import namespace="urn:dep" schemaLocation="deps/dep.xsd"/>
  <xs:element name="root" type="dep:codeType"/>
</xs:schema>`)
	writeFile(t, tempDir, "links/deps/dep.xsd", `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"
           targetNamespace="urn:dep"
           xmlns:tns="urn:dep">
  <xs:simpleType name="codeType">
    <xs:restriction base="xs:string"/>
  </xs:simpleType>
</xs:schema>`)
	linkPath := writeSymlink(t, tempDir, "links/current.xsd", schemaPath)
	validDocPath := writeFile(t, tempDir, "valid.xml", `<root xmlns="urn:test">ok</root>`)

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	gotCode := runWithArgs("xmllint", []string{"--schema", linkPath, validDocPath}, &stdout, &stderr)
	if gotCode != 0 {
		t.Fatalf("runWithArgs() code = %d, want 0\nstdout:\n%s\nstderr:\n%s", gotCode, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), validDocPath+" validates") {
		t.Fatalf("stdout = %q, want validation success", stdout.String())
	}
}

func writeFile(t *testing.T, root, name, content string) string {
	t.Helper()

	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}

func writeSymlink(t *testing.T, root, name, target string) string {
	t.Helper()

	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	linkTarget, err := filepath.Rel(filepath.Dir(path), target)
	if err != nil {
		t.Fatalf("filepath.Rel(%q, %q) error = %v", filepath.Dir(path), target, err)
	}
	if err := os.Symlink(linkTarget, path); err != nil {
		if errors.Is(err, fs.ErrPermission) || errors.Is(err, errors.ErrUnsupported) {
			t.Skipf("symlink creation unavailable in test environment: %v", err)
		}
		t.Fatalf("Symlink(%q, %q) error = %v", linkTarget, path, err)
	}
	return path
}
