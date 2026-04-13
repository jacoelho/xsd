package compiler

import (
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestLoadResolvedReturnsCachedSchema(t *testing.T) {
	t.Parallel()

	doc := &testReadCloser{Reader: strings.NewReader("")}
	want := parser.NewSchema()
	loader := NewLoader(LoaderConfig{})
	key := loadKey{systemID: "schema.xsd"}
	entry := loader.state.ensureEntry(key)
	entry.state = schemaStateLoaded
	entry.schema = want

	got, err := newLoadSession(loader, "schema.xsd", key, doc).loadResolved()
	if err != nil {
		t.Fatalf("loadResolved() error = %v", err)
	}
	if got != want {
		t.Fatalf("loadResolved() schema = %p, want %p", got, want)
	}
	if !doc.closed {
		t.Fatal("loadResolved() did not close cached document")
	}
}

func TestLoadResolvedReturnsLoadingSchemaAndCloseError(t *testing.T) {
	t.Parallel()

	doc := &testReadCloser{
		Reader:   strings.NewReader(""),
		closeErr: errors.New("close failed"),
	}
	wantSchema := parser.NewSchema()
	loader := NewLoader(LoaderConfig{})
	key := loadKey{systemID: "schema.xsd"}
	entry := loader.state.ensureEntry(key)
	entry.state = schemaStateLoading
	entry.schema = wantSchema

	got, err := newLoadSession(loader, "schema.xsd", key, doc).loadResolved()
	if got != wantSchema {
		t.Fatalf("loadResolved() schema = %p, want %p", got, wantSchema)
	}
	if !errors.Is(err, doc.closeErr) {
		t.Fatalf("loadResolved() error = %v, want close error", err)
	}
}

func TestLoadResolvedParsesAndApplies(t *testing.T) {
	t.Parallel()

	doc := &testReadCloser{Reader: strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)}
	loader := NewLoader(LoaderConfig{})
	key := loadKey{systemID: "schema.xsd"}

	got, err := newLoadSession(loader, "schema.xsd", key, doc).loadResolved()
	if err != nil {
		t.Fatalf("loadResolved() error = %v", err)
	}
	if got == nil {
		t.Fatal("loadResolved() schema = nil")
	}
	if got.ElementDecls[model.QName{Local: "root"}] == nil {
		t.Fatal("loadResolved() missing parsed root element")
	}
	if !doc.closed {
		t.Fatal("loadResolved() did not close parsed document")
	}
}
