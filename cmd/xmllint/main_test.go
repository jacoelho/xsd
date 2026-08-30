package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/xsderrors"
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
		{name: "negative_max_identity_entries", args: []string{"--schema", "schema.xsd", "--max-identity-entries", "-1", "doc.xml"}, want: "--max-identity-entries cannot be negative"},
		{name: "negative_max_instance_bytes", args: []string{"--schema", "schema.xsd", "--max-instance-bytes", "-1", "doc.xml"}, want: "--max-instance-bytes cannot be negative"},
		{name: "rejects_noout", args: []string{"--noout", "--schema", "schema.xsd", "doc.xml"}, want: "flag provided but not defined: -noout"},
		{name: "rejects_huge", args: []string{"--huge", "--schema", "schema.xsd", "doc.xml"}, want: "flag provided but not defined: -huge"},
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

	var stdout, stderr bytes.Buffer
	if code := runWithOpen([]string{"--schema", schema, doc}, &stdout, &stderr, func(path string) (io.ReadCloser, error) {
		return os.Open(path) //nolint:gosec // Test opens files created under t.TempDir.
	}); code != 0 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if stdout.Len() != 0 || stderr.Len() != 0 {
		t.Fatalf("run() stdout/stderr = %q/%q, want silence", stdout.String(), stderr.String())
	}
}

func TestRunHelpWritesUsageToStdout(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	if code := runWithOpen([]string{"--help"}, &stdout, &stderr, nil); code != 0 {
		t.Fatalf("runWithOpen() code = %d", code)
	}
	if stdout.String() != usage {
		t.Fatalf("stdout = %q, want %q", stdout.String(), usage)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want silence", stderr.String())
	}
}

func TestRunReportsValidationFailure(t *testing.T) {
	dir := t.TempDir()
	schema := writeXMLLintTestFile(t, dir, "schema.xsd", xmllintTestSchema)
	doc := writeXMLLintTestFile(t, dir, "invalid.xml", `<root><v>x</v></root>`)

	var stderr bytes.Buffer
	if code := runWithOpen([]string{"--schema", schema, doc}, io.Discard, &stderr, func(path string) (io.ReadCloser, error) {
		return os.Open(path) //nolint:gosec // Test opens files created under t.TempDir.
	}); code != 1 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "validation.facet") {
		t.Fatalf("run() stderr = %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), doc+" fails to validate") {
		t.Fatalf("run() stderr = %q", stderr.String())
	}
}

func TestRunEnforcesMaxInstanceBytes(t *testing.T) {
	dir := t.TempDir()
	schema := writeXMLLintTestFile(t, dir, "schema.xsd", xmllintTestSchema)
	doc := writeXMLLintTestFile(t, dir, "valid.xml", `<root><v>7</v></root>`)

	var stderr bytes.Buffer
	args := []string{"--schema", schema, "--max-instance-bytes", "4", doc}
	if code := runWithOpen(args, io.Discard, &stderr, func(path string) (io.ReadCloser, error) {
		return os.Open(path) //nolint:gosec // Test opens files created under t.TempDir.
	}); code != 1 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), string(xsderrors.CodeValidationLimit)) {
		t.Fatalf("run() stderr = %q, want validation limit", stderr.String())
	}
}

func TestRunReportsArgumentFailure(t *testing.T) {
	var stderr bytes.Buffer
	if code := runWithOpen([]string{"--schema", "schema.xsd", "--max-errors", "-1", "doc.xml"}, io.Discard, &stderr, nil); code != 2 {
		t.Fatalf("run() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--max-errors cannot be negative") {
		t.Fatalf("run() stderr = %q", stderr.String())
	}
}

func TestRunReturnsArgumentFailureWhenStderrWriteFails(t *testing.T) {
	stderr := errWriter{err: errors.New("write failed")}
	if code := runWithOpen([]string{"--schema", "schema.xsd", "--max-errors", "-1", "doc.xml"}, io.Discard, stderr, nil); code != 1 {
		t.Fatalf("run() code = %d, want 1", code)
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
	if code := runWithOpen([]string{"--schema", schema, "valid.xml"}, io.Discard, &stderr, open); code != 1 {
		t.Fatalf("runWithOpen() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), docErr.Error()) {
		t.Fatalf("runWithOpen() stderr = %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "valid.xml fails to validate") {
		t.Fatalf("runWithOpen() stderr = %q", stderr.String())
	}
}

func TestRunClosesReaderReturnedWithOpenError(t *testing.T) {
	dir := t.TempDir()
	schema := writeXMLLintTestFile(t, dir, "schema.xsd", xmllintTestSchema)
	openErr := errors.New("open failed")
	reader := &countingReadCloser{Reader: strings.NewReader(`<root/>`)}

	var stderr bytes.Buffer
	code := runWithOpen([]string{"--schema", schema, "doc.xml"}, io.Discard, &stderr, func(string) (io.ReadCloser, error) {
		return reader, openErr //nolint:nilnil // The regression requires the permitted reader-plus-error result.
	})
	if code != 1 {
		t.Fatalf("runWithOpen() code = %d, stderr = %q", code, stderr.String())
	}
	if reader.closes != 1 {
		t.Fatalf("reader closes = %d, want 1", reader.closes)
	}
}

func TestRunRejectsTypedNilDocumentReader(t *testing.T) {
	dir := t.TempDir()
	schema := writeXMLLintTestFile(t, dir, "schema.xsd", xmllintTestSchema)
	var reader *typedNilReadCloser

	var stderr bytes.Buffer
	code := runWithOpen([]string{"--schema", schema, "doc.xml"}, io.Discard, &stderr, func(string) (io.ReadCloser, error) {
		return reader, nil
	})
	if code != 1 {
		t.Fatalf("runWithOpen() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "document opener returned nil reader") {
		t.Fatalf("runWithOpen() stderr = %q", stderr.String())
	}
}

func TestRunDoesNotCloseTypedNilReaderReturnedWithError(t *testing.T) {
	dir := t.TempDir()
	schema := writeXMLLintTestFile(t, dir, "schema.xsd", xmllintTestSchema)
	openErr := errors.New("open failed")
	var reader *typedNilReadCloser

	var stderr bytes.Buffer
	code := runWithOpen([]string{"--schema", schema, "doc.xml"}, io.Discard, &stderr, func(string) (io.ReadCloser, error) {
		return reader, openErr //nolint:nilnil // Typed-nil plus error is the regression contract.
	})
	if code != 1 || !strings.Contains(stderr.String(), openErr.Error()) {
		t.Fatalf("runWithOpen() code/stderr = %d/%q", code, stderr.String())
	}
}

func TestRunReportsValidationFailureBeforeCloseFailure(t *testing.T) {
	dir := t.TempDir()
	schema := writeXMLLintTestFile(t, dir, "schema.xsd", xmllintTestSchema)
	docErr := errors.New("close failed")
	open := func(path string) (io.ReadCloser, error) {
		if path != "invalid.xml" {
			return nil, os.ErrNotExist
		}
		return closeErrorReader{Reader: strings.NewReader(`<root><v>x</v></root>`), err: docErr}, nil
	}

	var stderr bytes.Buffer
	if code := runWithOpen([]string{"--schema", schema, "invalid.xml"}, io.Discard, &stderr, open); code != 1 {
		t.Fatalf("runWithOpen() code = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "validation.facet") {
		t.Fatalf("runWithOpen() stderr = %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), docErr.Error()) {
		t.Fatalf("runWithOpen() stderr = %q", stderr.String())
	}
}

type closeErrorReader struct {
	io.Reader

	err error
}

type countingReadCloser struct {
	io.Reader

	closes int
}

type typedNilReadCloser struct{}

func (*typedNilReadCloser) Read([]byte) (int, error) {
	panic("typed nil reader must not be read")
}

func (*typedNilReadCloser) Close() error {
	panic("typed nil reader must not be closed")
}

func (r *countingReadCloser) Close() error {
	r.closes++
	return nil
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
