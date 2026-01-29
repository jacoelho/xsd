package loader

import (
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func TestResolvePendingIncludesDrainsPending(t *testing.T) {
	loader := &SchemaLoader{
		state:   newLoadState(),
		imports: newImportTracker(),
	}

	sourceKey := loadKey{systemID: "included.xsd"}
	targetKey := loadKey{systemID: "root.xsd"}

	source := parser.NewSchema()
	target := parser.NewSchema()

	sourceEntry := loader.state.ensureEntry(sourceKey)
	sourceEntry.schema = source
	sourceEntry.pendingDirectives = []pendingDirective{{
		kind:           parser.DirectiveInclude,
		targetKey:      targetKey,
		schemaLocation: "included.xsd",
	}}

	targetEntry := loader.state.ensureEntry(targetKey)
	targetEntry.schema = target
	targetEntry.pendingCount = 1

	if err := loader.resolvePendingImportsFor(sourceKey); err != nil {
		t.Fatalf("resolve pending includes: %v", err)
	}
	if targetEntry.pendingCount != 0 {
		t.Fatalf("pendingCount = %d, want 0", targetEntry.pendingCount)
	}
	if len(sourceEntry.pendingDirectives) != 0 {
		t.Fatalf("pendingDirectives = %d, want 0", len(sourceEntry.pendingDirectives))
	}
}

func TestResolvePendingImportsRejectsNamespaceMismatch(t *testing.T) {
	loader := &SchemaLoader{
		state:   newLoadState(),
		imports: newImportTracker(),
	}

	sourceKey := loadKey{systemID: "imported.xsd", etn: types.NamespaceURI("urn:actual")}
	targetKey := loadKey{systemID: "root.xsd"}

	source := parser.NewSchema()
	source.TargetNamespace = types.NamespaceURI("urn:actual")
	target := parser.NewSchema()

	sourceEntry := loader.state.ensureEntry(sourceKey)
	sourceEntry.schema = source
	sourceEntry.pendingDirectives = []pendingDirective{{
		kind:              parser.DirectiveImport,
		targetKey:         targetKey,
		schemaLocation:    "imported.xsd",
		expectedNamespace: "urn:expected",
	}}

	targetEntry := loader.state.ensureEntry(targetKey)
	targetEntry.schema = target
	targetEntry.pendingCount = 1

	err := loader.resolvePendingImportsFor(sourceKey)
	if err == nil {
		t.Fatalf("expected namespace mismatch error")
	}
	if !strings.Contains(err.Error(), "namespace mismatch") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDecrementPendingAndResolveUnderflow(t *testing.T) {
	loader := &SchemaLoader{
		state:   newLoadState(),
		imports: newImportTracker(),
	}
	key := loadKey{systemID: "root.xsd"}
	loader.state.ensureEntry(key)

	if err := loader.decrementPendingAndResolve(key); err == nil {
		t.Fatalf("expected pendingCount underflow error")
	}
}
