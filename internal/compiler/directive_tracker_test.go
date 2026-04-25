package compiler

import (
	"testing"

	"github.com/jacoelho/xsd/internal/schemaast"
)

func TestTrackerMarksAndUnmarksMergedEdges(t *testing.T) {
	tracker := NewTracker[string]()

	if tracker.AlreadyMerged(schemaast.DirectiveInclude, "base", "target") {
		t.Fatal("unexpected merged edge before mark")
	}

	tracker.MarkMerged(schemaast.DirectiveInclude, "base", "target")
	if !tracker.AlreadyMerged(schemaast.DirectiveInclude, "base", "target") {
		t.Fatal("expected merged edge after mark")
	}

	tracker.UnmarkMerged(schemaast.DirectiveInclude, "base", "target")
	if tracker.AlreadyMerged(schemaast.DirectiveInclude, "base", "target") {
		t.Fatal("unexpected merged edge after unmark")
	}
}

func TestTrackerKeepsDirectiveKindsIsolated(t *testing.T) {
	tracker := NewTracker[string]()
	tracker.MarkMerged(schemaast.DirectiveInclude, "base", "target")

	if tracker.AlreadyMerged(schemaast.DirectiveImport, "base", "target") {
		t.Fatal("merge marker leaked across directive kinds")
	}
}
