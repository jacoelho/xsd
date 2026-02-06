package loader

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadResolvedClosesReaderOnceOnParseError(t *testing.T) {
	loader := NewLoader(Config{FS: fstest.MapFS{}})
	reader := &testReadCloser{
		reader:            strings.NewReader("<not-schema>"),
		failOnSecondClose: true,
	}

	_, err := loader.LoadResolved(reader, "schema.xsd")
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if strings.Contains(err.Error(), "closed twice") {
		t.Fatalf("reader closed twice")
	}
	if !reader.closed {
		t.Fatalf("expected reader to be closed")
	}
}

func TestLoadResolvedClosesReaderOnFailStopEarlyReturn(t *testing.T) {
	loader := NewLoader(Config{FS: fstest.MapFS{}})

	if _, err := loader.LoadResolved(io.NopCloser(strings.NewReader("<not-schema>")), "schema.xsd"); err == nil {
		t.Fatalf("expected first load to fail and trip fail-stop")
	}

	reader := &testReadCloser{
		reader:            strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`),
		failOnSecondClose: true,
	}
	if _, err := loader.LoadResolved(reader, "schema.xsd"); !errors.Is(err, errLoaderFailed) {
		t.Fatalf("expected fail-stop error, got %v", err)
	}
	if !reader.closed {
		t.Fatalf("expected reader to be closed on fail-stop early return")
	}
}

func TestLoadResolvedClosesReaderOnMissingSystemID(t *testing.T) {
	loader := NewLoader(Config{})
	reader := &testReadCloser{
		reader:            strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`),
		failOnSecondClose: true,
	}

	_, err := loader.LoadResolved(reader, "")
	if err == nil || !strings.Contains(err.Error(), "missing systemID") {
		t.Fatalf("expected missing systemID error, got %v", err)
	}
	if !reader.closed {
		t.Fatalf("expected reader to be closed on missing systemID")
	}
}

func TestLoadResolvedFailStopEarlyReturnJoinsCloseError(t *testing.T) {
	loader := NewLoader(Config{FS: fstest.MapFS{}})

	if _, err := loader.LoadResolved(io.NopCloser(strings.NewReader("<not-schema>")), "schema.xsd"); err == nil {
		t.Fatalf("expected first load to fail and trip fail-stop")
	}

	closeErr := errors.New("close failed")
	reader := &testReadCloser{
		reader:   strings.NewReader(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`),
		closeErr: closeErr,
	}
	_, err := loader.LoadResolved(reader, "schema.xsd")
	if !errors.Is(err, errLoaderFailed) {
		t.Fatalf("expected fail-stop error, got %v", err)
	}
	if !errors.Is(err, closeErr) {
		t.Fatalf("expected close error to be joined, got %v", err)
	}
	if !reader.closed {
		t.Fatalf("expected reader close attempt")
	}
}
