package source

import (
	"bytes"
	"errors"
	"io"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

type errCloseReader struct {
	*bytes.Reader
	closeErr error
}

func (r *errCloseReader) Close() error {
	return r.closeErr
}

type trackingResolver struct {
	docs      map[string][]byte
	failOnNth map[string]int
	closeErr  error
	calls     map[string]int
}

func (r *trackingResolver) Resolve(req ResolveRequest) (io.ReadCloser, string, error) {
	systemID := req.SchemaLocation
	data, ok := r.docs[systemID]
	if !ok {
		return nil, "", fsErrNotExist()
	}
	if r.calls == nil {
		r.calls = make(map[string]int)
	}
	r.calls[systemID]++
	var closeErr error
	if nth, ok := r.failOnNth[systemID]; ok && r.calls[systemID] == nth {
		closeErr = r.closeErr
	}
	return &errCloseReader{Reader: bytes.NewReader(data), closeErr: closeErr}, systemID, nil
}

func fsErrNotExist() error {
	return errors.New("schema not found")
}

func TestLoadReportsCloseErrorOnDuplicateInclude(t *testing.T) {
	root := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:include schemaLocation="inc.xsd"/>
  <xs:include schemaLocation="inc.xsd"/>
</xs:schema>`)
	inc := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)
	closeErr := errors.New("close failed")

	res := &trackingResolver{
		docs: map[string][]byte{
			"root.xsd": root,
			"inc.xsd":  inc,
		},
		failOnNth: map[string]int{
			"inc.xsd": 2,
		},
		closeErr: closeErr,
	}

	loader := NewLoader(Config{Resolver: res})
	_, err := loader.Load("root.xsd")
	if err == nil {
		t.Fatalf("expected close error")
	}
	if !errors.Is(err, closeErr) {
		t.Fatalf("expected close error, got %v", err)
	}
}

func TestLoadRejectsImportNamespaceMismatchWithoutNamespace(t *testing.T) {
	root := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:root">
  <xs:import schemaLocation="imp.xsd"/>
</xs:schema>`)
	imp := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:imp">
  <xs:element name="e" type="xs:string"/>
</xs:schema>`)

	res := &trackingResolver{
		docs: map[string][]byte{
			"root.xsd": root,
			"imp.xsd":  imp,
		},
	}

	loader := NewLoader(Config{Resolver: res})
	_, err := loader.Load("root.xsd")
	if err == nil {
		t.Fatalf("expected import namespace mismatch error")
	}
	want := "imported schema imp.xsd namespace mismatch: expected no namespace, got urn:imp"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestLoadDirectiveSchemaMissingImportIsSkippedNotDeferred(t *testing.T) {
	loader := NewLoader(Config{
		FS:                          fstest.MapFS{},
		AllowMissingImportLocations: true,
	})
	session := newLoadSession(
		loader,
		"root.xsd",
		loader.loadKey("root.xsd", types.NamespaceURI("urn:root")),
		nil,
	)

	result, err := session.loadDirectiveSchema(
		parser.DirectiveImport,
		ResolveRequest{
			BaseSystemID:   "root.xsd",
			SchemaLocation: "missing.xsd",
			ImportNS:       []byte("urn:other"),
			Kind:           ResolveImport,
		},
		func(systemID string) loadKey {
			return loader.loadKey(systemID, types.NamespaceURI("urn:other"))
		},
		true,
		nil,
	)
	if err != nil {
		t.Fatalf("loadDirectiveSchema missing import error = %v, want nil", err)
	}
	if result.status != directiveLoadStatusSkippedMissing {
		t.Fatalf("status = %v, want skipped-missing", result.status)
	}
}
