package loader

import (
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/fstest"
)

type errOnSecondClose struct {
	io.Reader
	closed bool
}

func (r *errOnSecondClose) Close() error {
	if r.closed {
		return fmt.Errorf("closed twice")
	}
	r.closed = true
	return nil
}

func TestLoadResolvedClosesReaderOnceOnParseError(t *testing.T) {
	loader := NewLoader(Config{FS: fstest.MapFS{}})
	reader := &errOnSecondClose{Reader: strings.NewReader("<not-schema>")}

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
