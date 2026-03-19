package preprocessor

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/xmltree"
)

type testReadCloser struct {
	io.Reader
	closeErr error
	closed   bool
}

func (r *testReadCloser) Close() error {
	r.closed = true
	return r.closeErr
}

func TestParseNilReader(t *testing.T) {
	t.Parallel()

	if _, err := Parse(nil, "schema.xsd", nil); err == nil {
		t.Fatal("Parse(nil) error = nil, want error")
	}
}

func TestParseClosesDocument(t *testing.T) {
	t.Parallel()

	doc := &testReadCloser{
		Reader: strings.NewReader(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`),
	}
	if _, err := Parse(doc, "schema.xsd", xmltree.NewDocumentPool()); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !doc.closed {
		t.Fatal("Parse() did not close document")
	}
}

func TestParseReturnsCloseErrorAfterSuccessfulParse(t *testing.T) {
	t.Parallel()

	doc := &testReadCloser{
		Reader:   strings.NewReader(`<?xml version="1.0"?><xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`),
		closeErr: errors.New("boom"),
	}
	if _, err := Parse(doc, "schema.xsd", nil); err == nil || !strings.Contains(err.Error(), "close schema.xsd: boom") {
		t.Fatalf("Parse() error = %v, want close error", err)
	}
}

func TestInitOriginsSetsMissingOriginsAndPreservesExisting(t *testing.T) {
	t.Parallel()

	sch := parser.NewSchema()
	elementName := model.QName{Namespace: "urn:test", Local: "root"}
	typeName := model.QName{Namespace: "urn:test", Local: "namedType"}
	groupName := model.QName{Namespace: "urn:test", Local: "group"}

	sch.ElementDecls[elementName] = nil
	sch.TypeDefs[typeName] = nil
	sch.Groups[groupName] = nil
	sch.ElementOrigins[elementName] = "existing"

	InitOrigins(sch, "schema.xsd")

	if got, want := sch.Location, parser.ImportContextKey("", "schema.xsd"); got != want {
		t.Fatalf("Location = %q, want %q", got, want)
	}
	if got := sch.ElementOrigins[elementName]; got != "existing" {
		t.Fatalf("ElementOrigins[%v] = %q, want preserved value", elementName, got)
	}
	if got := sch.TypeOrigins[typeName]; got != sch.Location {
		t.Fatalf("TypeOrigins[%v] = %q, want %q", typeName, got, sch.Location)
	}
	if got := sch.GroupOrigins[groupName]; got != sch.Location {
		t.Fatalf("GroupOrigins[%v] = %q, want %q", groupName, got, sch.Location)
	}
}

func TestRegisterImportsTracksByLocationAndNamespace(t *testing.T) {
	t.Parallel()

	sch := parser.NewSchema()
	sch.TargetNamespace = "urn:test"
	InitOrigins(sch, "schema.xsd")

	imports := []parser.ImportInfo{
		{Namespace: "urn:dep1"},
		{Namespace: "urn:dep2"},
	}
	RegisterImports(sch, imports)

	if !sch.ImportedNamespaces["urn:test"]["urn:dep1"] || !sch.ImportedNamespaces["urn:test"]["urn:dep2"] {
		t.Fatalf("ImportedNamespaces = %#v, want both imports registered", sch.ImportedNamespaces)
	}
	ctx := sch.ImportContexts[sch.Location]
	if ctx.TargetNamespace != sch.TargetNamespace {
		t.Fatalf("ImportContexts[%q].TargetNamespace = %q, want %q", sch.Location, ctx.TargetNamespace, sch.TargetNamespace)
	}
	if !ctx.Imports["urn:dep1"] || !ctx.Imports["urn:dep2"] {
		t.Fatalf("ImportContexts[%q].Imports = %#v, want both imports registered", sch.Location, ctx.Imports)
	}
}

func TestValidateImports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ns      model.NamespaceURI
		imports []parser.ImportInfo
		wantErr string
	}{
		{
			name:    "no target namespace requires import namespace",
			imports: []parser.ImportInfo{{Namespace: ""}},
			wantErr: "schema without targetNamespace cannot use import without namespace attribute",
		},
		{
			name:    "import namespace must differ from target namespace",
			ns:      "urn:test",
			imports: []parser.ImportInfo{{Namespace: "urn:test"}},
			wantErr: "import namespace urn:test must be different from target namespace",
		},
		{
			name:    "valid import",
			ns:      "urn:test",
			imports: []parser.ImportInfo{{Namespace: "urn:dep"}},
		},
	}

	for _, tc := range tests {

		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sch := parser.NewSchema()
			sch.TargetNamespace = tc.ns
			err := ValidateImports(sch, tc.imports)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("ValidateImports() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("ValidateImports() error = %v, want %q", err, tc.wantErr)
			}
		})
	}
}
