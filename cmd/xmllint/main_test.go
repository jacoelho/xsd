package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const xmllintTestSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="v" type="xs:int"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

func TestParseArgsRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "missing_schema", args: []string{"doc.xml"}, want: "--schema is required"},
		{name: "missing_doc", args: []string{"--schema", "schema.xsd"}, want: "one XML document path is required"},
		{name: "negative_max_errors", args: []string{"--schema", "schema.xsd", "--max-errors", "-1", "doc.xml"}, want: "--max-errors cannot be negative"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseArgs(test.args)
			if err == nil {
				t.Fatal("parseArgs() succeeded")
			}
			if err.Error() != test.want {
				t.Fatalf("parseArgs() error = %q, want %q", err, test.want)
			}
		})
	}
}

func TestRunValidatesDocument(t *testing.T) {
	dir := t.TempDir()
	schema := writeXMLLintTestFile(t, dir, "schema.xsd", xmllintTestSchema)
	doc := writeXMLLintTestFile(t, dir, "valid.xml", `<root><v>7</v></root>`)

	var stderr bytes.Buffer
	if code := run([]string{"--noout", "--schema", schema, doc}, &stderr); code != 0 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), doc+" validates") {
		t.Fatalf("run() stderr = %q", stderr.String())
	}
}

func TestRunReportsValidationFailure(t *testing.T) {
	dir := t.TempDir()
	schema := writeXMLLintTestFile(t, dir, "schema.xsd", xmllintTestSchema)
	doc := writeXMLLintTestFile(t, dir, "invalid.xml", `<root><v>x</v></root>`)

	var stderr bytes.Buffer
	if code := run([]string{"--schema", schema, doc}, &stderr); code != 1 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "validation.facet") {
		t.Fatalf("run() stderr = %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), doc+" fails to validate") {
		t.Fatalf("run() stderr = %q", stderr.String())
	}
}

func TestRunReportsArgumentFailure(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"--schema", "schema.xsd", "--max-errors", "-1", "doc.xml"}, &stderr); code != 2 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--max-errors cannot be negative") {
		t.Fatalf("run() stderr = %q", stderr.String())
	}
}

func TestRunReturnsArgumentFailureWhenStderrWriteFails(t *testing.T) {
	stderr := errWriter{err: errors.New("write failed")}
	if code := run([]string{"--schema", "schema.xsd", "--max-errors", "-1", "doc.xml"}, stderr); code != 2 {
		t.Fatalf("run() code = %d, want 2", code)
	}
}

func TestRunReportsDocumentCloseFailure(t *testing.T) {
	dir := t.TempDir()
	schema := writeXMLLintTestFile(t, dir, "schema.xsd", xmllintTestSchema)
	docErr := errors.New("close failed")
	open := func(path string) (io.ReadCloser, error) {
		if path != "valid.xml" {
			return nil, os.ErrNotExist
		}
		return closeErrorReader{Reader: strings.NewReader(`<root><v>7</v></root>`), err: docErr}, nil
	}

	var stderr bytes.Buffer
	if code := runWithOpen([]string{"--schema", schema, "valid.xml"}, &stderr, open); code != 1 {
		t.Fatalf("runWithOpen() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), docErr.Error()) {
		t.Fatalf("runWithOpen() stderr = %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "valid.xml fails to validate") {
		t.Fatalf("runWithOpen() stderr = %q", stderr.String())
	}
}

type closeErrorReader struct {
	io.Reader
	err error
}

func (r closeErrorReader) Close() error {
	return r.err
}

type errWriter struct {
	err error
}

func (w errWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func writeXMLLintTestFile(t *testing.T, dir, name, data string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}
