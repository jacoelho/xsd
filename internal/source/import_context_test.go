package source

import (
	"testing"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestMergeImportContextsKeepsDistinctKeys(t *testing.T) {
	loader := &SchemaLoader{}
	left := parser.NewSchema()
	right := parser.NewSchema()

	keyA := parser.ImportContextKey("a||b", "c||d")
	keyB := parser.ImportContextKey("a", "b||c||d")
	if keyA == keyB {
		t.Fatalf("ImportContextKey collision: %q", keyA)
	}

	left.ImportContexts = map[string]parser.ImportContext{
		keyA: {
			TargetNamespace: model.NamespaceURI("urn:left"),
		},
	}
	right.ImportContexts = map[string]parser.ImportContext{
		keyB: {
			TargetNamespace: model.NamespaceURI("urn:right"),
		},
	}

	if err := loader.mergeSchema(left, right, loadmerge.MergeInclude, loadmerge.KeepNamespace, len(left.GlobalDecls)); err != nil {
		t.Fatalf("mergeSchema error = %v", err)
	}
	if len(left.ImportContexts) != 2 {
		t.Fatalf("ImportContexts size = %d, want 2", len(left.ImportContexts))
	}
	if _, ok := left.ImportContexts[keyA]; !ok {
		t.Fatalf("missing ImportContexts entry for %q", keyA)
	}
	if _, ok := left.ImportContexts[keyB]; !ok {
		t.Fatalf("missing ImportContexts entry for %q", keyB)
	}
}
