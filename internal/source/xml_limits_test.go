package source

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jacoelho/xsd/pkg/xmlstream"
	"github.com/jacoelho/xsd/pkg/xmltext"
)

func TestSchemaParseMaxDepth(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <a><b><c><d><e><f/></e></d></c></b></a>
</xs:schema>`

	fs := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}
	loader := NewLoader(Config{
		FS:                 fs,
		SchemaParseOptions: []xmlstream.Option{xmltext.MaxDepth(4)},
	})
	if _, err := loader.Load("schema.xsd"); err == nil || !strings.Contains(err.Error(), "MaxDepth") {
		t.Fatalf("expected MaxDepth error, got %v", err)
	}
}

func TestSchemaParseMaxAttrs(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" a="1" b="2" c="3" d="4">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`

	fs := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}
	loader := NewLoader(Config{
		FS:                 fs,
		SchemaParseOptions: []xmlstream.Option{xmltext.MaxAttrs(2)},
	})
	if _, err := loader.Load("schema.xsd"); err == nil || !strings.Contains(err.Error(), "MaxAttrs") {
		t.Fatalf("expected MaxAttrs error, got %v", err)
	}
}

func TestSchemaParseMaxTokenSize(t *testing.T) {
	schemaXML := `<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <a>abcdefghijklmnopqrstuvwxyz</a>
</xs:schema>`

	fs := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(schemaXML)},
	}
	loader := NewLoader(Config{
		FS:                 fs,
		SchemaParseOptions: []xmlstream.Option{xmltext.MaxTokenSize(8)},
	})
	if _, err := loader.Load("schema.xsd"); err == nil || !strings.Contains(err.Error(), "MaxTokenSize") {
		t.Fatalf("expected MaxTokenSize error, got %v", err)
	}
}
