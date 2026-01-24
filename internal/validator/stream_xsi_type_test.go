package validator

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

func TestResolveXsiTypeNilDeclaredType(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`

	schema, err := parser.Parse(strings.NewReader(schemaXML))
	if err != nil {
		t.Fatalf("Parse schema: %v", err)
	}

	v := New(mustCompile(t, schema))
	run := v.newStreamRun()

	docXML := `<root xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`
	dec, err := xmlstream.NewStringReader(strings.NewReader(docXML))
	if err != nil {
		t.Fatalf("NewStringReader: %v", err)
	}
	ev, err := dec.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}

	run.dec = dec
	xsiType, err := run.resolveXsiType(ev.ScopeDepth, "xs:string", nil, 0)
	if err != nil {
		t.Fatalf("resolveXsiType: %v", err)
	}
	if xsiType == nil {
		t.Fatal("expected xsiType, got nil")
	}
	if xsiType.QName.Local != string(types.TypeNameString) {
		t.Fatalf("xsiType local = %q, want %q", xsiType.QName.Local, string(types.TypeNameString))
	}
}
