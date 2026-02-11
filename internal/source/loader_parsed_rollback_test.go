package source

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestLoadParsedResolvePendingFailureRollsBackTargetPendingCounts(t *testing.T) {
	loader := &SchemaLoader{
		state:   newLoadState(),
		imports: newImportTracker(),
	}

	sourceKey := loadKey{systemID: "source.xsd", etn: model.NamespaceURI("urn:source")}
	targetKey := loadKey{systemID: "target.xsd", etn: model.NamespaceURI("urn:target")}

	sourceEntry := loader.state.ensureEntry(sourceKey)
	sourceEntry.pendingDirectives = []pendingDirective{
		{
			kind:              parser.DirectiveImport,
			targetKey:         targetKey,
			schemaLocation:    "target.xsd",
			expectedNamespace: "urn:expected",
		},
	}
	targetEntry := loader.state.ensureEntry(targetKey)
	targetEntry.schema = parser.NewSchema()
	targetEntry.pendingCount = 1

	result := &parser.ParseResult{
		Schema:     parser.NewSchema(),
		Directives: nil,
		Imports:    nil,
		Includes:   nil,
	}
	result.Schema.TargetNamespace = model.NamespaceURI("urn:actual")

	if _, err := loader.loadParsed(result, sourceKey.systemID, sourceKey); err == nil {
		t.Fatalf("expected pending import resolution error")
	}

	targetEntry, ok := loader.state.entry(targetKey)
	if !ok || targetEntry == nil {
		t.Fatalf("expected target entry to remain present")
	}
	if targetEntry.pendingCount != 0 {
		t.Fatalf("target pendingCount = %d, want 0", targetEntry.pendingCount)
	}
}
