package parser

import (
	"strings"
	"testing"
)

func TestParseDerivationContentErrors(t *testing.T) {
	tests := []struct {
		name    string
		xml     string
		wantErr string
		parse   func(*testing.T, string) error
	}{
		{
			name: "simpleContent duplicate derivation",
			xml: `<?xml version="1.0"?>
<xs:simpleContent xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:restriction base="xs:string"/>
  <xs:extension base="xs:string"/>
</xs:simpleContent>`,
			wantErr: "simpleContent must have exactly one derivation child (restriction or extension)",
			parse: func(t *testing.T, xml string) error {
				doc := parseDoc(t, xml)
				_, err := parseSimpleContent(doc, doc.DocumentElement(), NewSchema())
				return err
			},
		},
		{
			name: "complexContent annotation after derivation",
			xml: `<?xml version="1.0"?>
<xs:complexContent xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:extension base="xs:anyType"/>
  <xs:annotation/>
</xs:complexContent>`,
			wantErr: "complexContent: annotation must appear before restriction or extension",
			parse: func(t *testing.T, xml string) error {
				doc := parseDoc(t, xml)
				_, err := parseComplexContent(doc, doc.DocumentElement(), NewSchema())
				return err
			},
		},
		{
			name: "complexContent missing derivation",
			xml: `<?xml version="1.0"?>
<xs:complexContent xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:annotation/>
</xs:complexContent>`,
			wantErr: "complexContent must have exactly one derivation child (restriction or extension)",
			parse: func(t *testing.T, xml string) error {
				doc := parseDoc(t, xml)
				_, err := parseComplexContent(doc, doc.DocumentElement(), NewSchema())
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.parse(t, tt.xml)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestParseAttributeReferenceConflictRules(t *testing.T) {
	tests := []struct {
		name    string
		xml     string
		wantErr string
	}{
		{
			name: "rejects type attribute on reference",
			xml: `<?xml version="1.0"?>
<xs:attribute xmlns:xs="http://www.w3.org/2001/XMLSchema"
              xmlns:tns="urn:test"
              ref="tns:base"
              type="xs:string"/>`,
			wantErr: "attribute reference cannot have 'type' attribute",
		},
		{
			name: "rejects form attribute on reference",
			xml: `<?xml version="1.0"?>
<xs:attribute xmlns:xs="http://www.w3.org/2001/XMLSchema"
              xmlns:tns="urn:test"
              ref="tns:base"
              form="qualified"/>`,
			wantErr: "attribute reference cannot have 'form' attribute",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := parseDoc(t, tt.xml)
			schema := NewSchema()
			schema.TargetNamespace = "urn:test"
			schema.NamespaceDecls["tns"] = "urn:test"

			_, err := parseAttribute(doc, doc.DocumentElement(), schema, true)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}
