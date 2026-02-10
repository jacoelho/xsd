package parser

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/schemaxml"
)

func TestParseWithImportsOptionsWithPool_ZeroValuePool(t *testing.T) {
	t.Parallel()

	schemaXML := `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root" type="xs:string"/></xs:schema>`
	pool := &schemaxml.DocumentPool{}

	result, err := ParseWithImportsOptionsWithPool(strings.NewReader(schemaXML), pool)
	if err != nil {
		t.Fatalf("ParseWithImportsOptionsWithPool() error = %v", err)
	}
	if result == nil || result.Schema == nil {
		t.Fatal("ParseWithImportsOptionsWithPool() returned nil result/schema")
	}
	if len(result.Schema.ElementDecls) == 0 {
		t.Fatal("expected parsed top-level element declaration")
	}

	stats := pool.Stats()
	if stats.Acquires == 0 || stats.Releases == 0 {
		t.Fatalf("zero-value pool stats did not advance: %+v", stats)
	}
}
