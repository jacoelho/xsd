package main

import (
	"bytes"
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
				"[VALIDATE_ROOT_NOT_DECLARED]",
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

func writeFile(t *testing.T, root, name, content string) string {
	t.Helper()

	path := filepath.Join(root, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}
