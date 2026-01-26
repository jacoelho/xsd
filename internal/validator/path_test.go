package validator

import (
	"testing"

	"github.com/jacoelho/xsd/internal/types"
)

func TestPathStackStringIncludesNamespace(t *testing.T) {
	var p pathStack
	p.push(types.QName{Namespace: "urn:a", Local: "root"})
	p.push(types.QName{Namespace: "urn:b", Local: "root"})

	if got := p.String(); got != "/{urn:a}root/{urn:b}root" {
		t.Fatalf("path = %q, want %q", got, "/{urn:a}root/{urn:b}root")
	}
}
