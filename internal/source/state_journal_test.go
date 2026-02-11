package source

import (
	"testing"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func TestStateJournalRollbackRevertsDeferredDirective(t *testing.T) {
	loader := &SchemaLoader{
		state:   newLoadState(),
		imports: newImportTracker(),
	}

	sourceKey := loadKey{systemID: "source.xsd", etn: model.NamespaceURI("urn:source")}
	targetKey := loadKey{systemID: "target.xsd", etn: model.NamespaceURI("urn:target")}
	include := parser.IncludeInfo{
		SchemaLocation: "source.xsd",
		DeclIndex:      0,
		IncludeIndex:   0,
	}

	var journal stateJournal
	if ok := loader.deferInclude(sourceKey, targetKey, include, &journal); !ok {
		t.Fatalf("expected deferred include to be recorded")
	}

	sourceEntry, ok := loader.state.entry(sourceKey)
	if !ok || sourceEntry == nil || len(sourceEntry.pendingDirectives) != 1 {
		t.Fatalf("expected one pending directive on source entry")
	}
	targetEntry, ok := loader.state.entry(targetKey)
	if !ok || targetEntry == nil || targetEntry.pendingCount != 1 {
		t.Fatalf("expected pending count on target entry")
	}

	journal.rollback(loader)

	if _, ok := loader.state.entry(sourceKey); ok {
		t.Fatalf("expected source entry to be cleaned after rollback")
	}
	if _, ok := loader.state.entry(targetKey); ok {
		t.Fatalf("expected target entry to be cleaned after rollback")
	}
}

func TestLoadSessionRollbackOnlyRevertsJournalOwnedPending(t *testing.T) {
	loader := &SchemaLoader{
		state:   newLoadState(),
		imports: newImportTracker(),
	}

	sourceKey := loadKey{systemID: "root.xsd", etn: model.NamespaceURI("urn:root")}
	existingTargetKey := loadKey{systemID: "existing.xsd", etn: model.NamespaceURI("urn:root")}
	sessionTargetKey := loadKey{systemID: "session.xsd", etn: model.NamespaceURI("urn:root")}

	sourceEntry := loader.state.ensureEntry(sourceKey)
	sourceEntry.pendingDirectives = []pendingDirective{
		{
			kind:      parser.DirectiveInclude,
			targetKey: existingTargetKey,
		},
	}
	existingTarget := loader.state.ensureEntry(existingTargetKey)
	existingTarget.pendingCount = 1

	session := newLoadSession(loader, sourceKey.systemID, sourceKey, nil)
	session.deferInclude(sourceKey, sessionTargetKey, parser.IncludeInfo{
		SchemaLocation: "session.xsd",
		DeclIndex:      0,
		IncludeIndex:   0,
	})
	session.rollback()

	if len(sourceEntry.pendingDirectives) != 1 {
		t.Fatalf("pendingDirectives = %d, want 1", len(sourceEntry.pendingDirectives))
	}
	if sourceEntry.pendingDirectives[0].targetKey != existingTargetKey {
		t.Fatalf("unexpected remaining pending directive target %s", sourceEntry.pendingDirectives[0].targetKey.systemID)
	}
	if existingTarget.pendingCount != 1 {
		t.Fatalf("existing pendingCount = %d, want 1", existingTarget.pendingCount)
	}
	if _, ok := loader.state.entry(sessionTargetKey); ok {
		t.Fatalf("session-owned target entry should be cleaned")
	}
}

func TestStateJournalAppendPropagatesNestedPendingRollback(t *testing.T) {
	loader := &SchemaLoader{
		state:   newLoadState(),
		imports: newImportTracker(),
	}

	sourceKey := loadKey{systemID: "root.xsd", etn: model.NamespaceURI("urn:root")}
	targetKey := loadKey{systemID: "nested.xsd", etn: model.NamespaceURI("urn:root")}
	include := parser.IncludeInfo{
		SchemaLocation: "nested.xsd",
		DeclIndex:      0,
		IncludeIndex:   0,
	}

	parent := newLoadSession(loader, sourceKey.systemID, sourceKey, nil)
	child := newLoadSession(loader, targetKey.systemID, targetKey, nil)
	child.deferInclude(sourceKey, targetKey, include)
	parent.journal.append(&child.journal)
	parent.rollback()

	if sourceEntry, ok := loader.state.entry(sourceKey); ok && sourceEntry != nil {
		if len(sourceEntry.pendingDirectives) != 0 {
			t.Fatalf("source pendingDirectives = %d, want 0", len(sourceEntry.pendingDirectives))
		}
		if sourceEntry.pendingCount != 0 {
			t.Fatalf("source pendingCount = %d, want 0", sourceEntry.pendingCount)
		}
	}
	if targetEntry, ok := loader.state.entry(targetKey); ok && targetEntry != nil {
		if targetEntry.pendingCount != 0 {
			t.Fatalf("target pendingCount = %d, want 0", targetEntry.pendingCount)
		}
	}
}

func TestStateJournalRollbackMutationStepFailureInjection(t *testing.T) {
	sourceKey := loadKey{systemID: "root.xsd", etn: model.NamespaceURI("urn:root")}
	targetKey := loadKey{systemID: "target.xsd", etn: model.NamespaceURI("urn:root")}

	type mutationStep struct {
		name  string
		apply func(*testing.T, *SchemaLoader, *stateJournal)
	}
	steps := []mutationStep{
		{
			name: "append pending directive",
			apply: func(t *testing.T, loader *SchemaLoader, journal *stateJournal) {
				t.Helper()
				sourceEntry := loader.state.ensureEntry(sourceKey)
				if !appendPendingDirective(sourceEntry, pendingDirective{
					kind:      parser.DirectiveInclude,
					targetKey: targetKey,
				}) {
					t.Fatalf("appendPendingDirective should record directive")
				}
				journal.recordAppendPendingDirective(parser.DirectiveInclude, sourceKey, targetKey)
			},
		},
		{
			name: "increment pending target count",
			apply: func(t *testing.T, loader *SchemaLoader, journal *stateJournal) {
				t.Helper()
				targetEntry := loader.state.ensureEntry(targetKey)
				incPendingCount(targetEntry)
				journal.recordIncPendingCount(targetKey)
			},
		},
		{
			name: "mark merged edge",
			apply: func(_ *testing.T, loader *SchemaLoader, journal *stateJournal) {
				loader.imports.markMerged(parser.DirectiveInclude, sourceKey, targetKey)
				journal.recordMarkMerged(parser.DirectiveInclude, sourceKey, targetKey)
			},
		},
	}

	for failAfter := range steps {
		loader := &SchemaLoader{
			state:   newLoadState(),
			imports: newImportTracker(),
		}
		var journal stateJournal
		for i := 0; i <= failAfter; i++ {
			steps[i].apply(t, loader, &journal)
		}

		journal.rollback(loader)

		if loader.imports.alreadyMerged(parser.DirectiveInclude, sourceKey, targetKey) {
			t.Fatalf("step %q: merged edge should be rolled back", steps[failAfter].name)
		}
		if _, ok := loader.state.entry(sourceKey); ok {
			t.Fatalf("step %q: source entry should be cleaned", steps[failAfter].name)
		}
		if _, ok := loader.state.entry(targetKey); ok {
			t.Fatalf("step %q: target entry should be cleaned", steps[failAfter].name)
		}
	}
}
