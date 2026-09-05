package compile

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/stream"
)

func TestSchemaParseStateAccumulatesOneTextRepresentation(t *testing.T) {
	t.Parallel()

	node := &rawNode{}
	state := schemaParseState{stack: []schemaParseFrame{{node: node}}}
	for _, chunk := range [][]byte{[]byte("first"), []byte(" "), []byte("second")} {
		if err := state.chars(chunk, stream.CharacterDataText, 1, 1); err != nil {
			t.Fatalf("chars() error = %v", err)
		}
	}
	if got := node.Text.String(); got != "first second" {
		t.Fatalf("raw text = %q, want %q", got, "first second")
	}
}

func TestRawNodeResolveQNameReturnsXMLName(t *testing.T) {
	t.Parallel()

	const schema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" xmlns="urn:default" xmlns:p="urn:prefixed"/>`
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := parseSchemaDocument("schema.xsd", "schema.xsd", []byte(schema), limits)
	if err != nil {
		t.Fatalf("parseSchemaDocument() error = %v", err)
	}
	tests := []struct {
		name    string
		lexical string
		want    xml.Name
		wantErr string
	}{
		{name: "default namespace", lexical: "item", want: xml.Name{Space: "urn:default", Local: "item"}},
		{name: "prefixed namespace", lexical: "p:item", want: xml.Name{Space: "urn:prefixed", Local: "item"}},
		{name: "unbound prefix", lexical: "missing:item", wantErr: "unbound QName prefix missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := doc.root.resolveQName(tt.lexical)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveQName(%q) error = %v, want %q", tt.lexical, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("resolveQName(%q) error = %v", tt.lexical, err)
			}
			if got != tt.want {
				t.Fatalf("resolveQName(%q) = %+v, want %+v", tt.lexical, got, tt.want)
			}
		})
	}
}
