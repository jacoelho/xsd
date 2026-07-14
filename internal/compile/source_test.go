package compile

import (
	"context"
	"slices"
	"testing"

	"github.com/jacoelho/xsd/internal/source"
)

func TestLoadedSchemaDocumentsSortBySourceName(t *testing.T) {
	limits, err := NormalizeOptions(Options{})
	if err != nil {
		t.Fatalf("NormalizeOptions() error = %v", err)
	}
	c, err := newCompiler(context.Background(), limits)
	if err != nil {
		t.Fatalf("newCompiler() error = %v", err)
	}
	schema := []byte(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"/>`)
	if err := c.LoadForTest([]source.Source{
		source.Bytes("file:///b.xsd", schema),
		source.Bytes("a.xsd", schema),
	}); err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if got, want := c.DocumentNamesForTest(), []string{"a.xsd", "file:///b.xsd"}; !slices.Equal(got, want) {
		t.Fatalf("doc names = %v, want %v", got, want)
	}
}
