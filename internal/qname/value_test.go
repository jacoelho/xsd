package qname

import (
	"testing"

	"github.com/jacoelho/xsd/internal/xmlnames"
)

func TestParseQNameValueWithPrefix(t *testing.T) {
	got, err := ParseQNameValue("p:item", map[string]string{"p": "urn:test"})
	if err != nil {
		t.Fatalf("ParseQNameValue() error = %v", err)
	}
	if got.Namespace != NamespaceURI("urn:test") || got.Local != "item" {
		t.Fatalf("ParseQNameValue() = {%s}%s, want {urn:test}item", got.Namespace, got.Local)
	}
}

func TestParseQNameValueWithDefaultNamespace(t *testing.T) {
	got, err := ParseQNameValue("item", map[string]string{"": "urn:default"})
	if err != nil {
		t.Fatalf("ParseQNameValue() error = %v", err)
	}
	if got.Namespace != NamespaceURI("urn:default") || got.Local != "item" {
		t.Fatalf("ParseQNameValue() = {%s}%s, want {urn:default}item", got.Namespace, got.Local)
	}
}

func TestParseQNameValueXMLPrefix(t *testing.T) {
	got, err := ParseQNameValue("xml:lang", nil)
	if err != nil {
		t.Fatalf("ParseQNameValue(xml:lang) error = %v", err)
	}
	if got.Namespace != NamespaceURI(xmlnames.XMLNamespace) || got.Local != "lang" {
		t.Fatalf("ParseQNameValue(xml:lang) = {%s}%s, want {%s}lang", got.Namespace, got.Local, xmlnames.XMLNamespace)
	}
}

func TestParseQNameValueXMLPrefixWrongBinding(t *testing.T) {
	if _, err := ParseQNameValue("xml:lang", map[string]string{"xml": "urn:wrong"}); err == nil {
		t.Fatal("ParseQNameValue(xml:lang) error = nil, want error")
	}
}

func TestParseQNameValueUnknownPrefix(t *testing.T) {
	if _, err := ParseQNameValue("p:item", map[string]string{}); err == nil {
		t.Fatal("ParseQNameValue() error = nil, want error")
	}
}
