package compiler

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestTrackerMarksAndUnmarksMergedEdges(t *testing.T) {
	tracker := NewTracker[string]()

	if tracker.AlreadyMerged(parser.DirectiveInclude, "base", "target") {
		t.Fatal("unexpected merged edge before mark")
	}

	tracker.MarkMerged(parser.DirectiveInclude, "base", "target")
	if !tracker.AlreadyMerged(parser.DirectiveInclude, "base", "target") {
		t.Fatal("expected merged edge after mark")
	}

	tracker.UnmarkMerged(parser.DirectiveInclude, "base", "target")
	if tracker.AlreadyMerged(parser.DirectiveInclude, "base", "target") {
		t.Fatal("unexpected merged edge after unmark")
	}
}

func TestTrackerKeepsDirectiveKindsIsolated(t *testing.T) {
	tracker := NewTracker[string]()
	tracker.MarkMerged(parser.DirectiveInclude, "base", "target")

	if tracker.AlreadyMerged(parser.DirectiveImport, "base", "target") {
		t.Fatal("merge marker leaked across directive kinds")
	}
}
