package xmlstream

import (
	"errors"
	"io"
	"strings"
	"testing"
)

func TestNextResolvedNameIDsStableAcrossRepeatedNames(t *testing.T) {
	input := `<root xmlns="urn:test"><child a="1"/><child a="2"/></root>`

	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	rootStart, err := r.NextResolved()
	if err != nil {
		t.Fatalf("root start: %v", err)
	}
	if rootStart.Kind != EventStartElement {
		t.Fatalf("root kind = %v, want %v", rootStart.Kind, EventStartElement)
	}

	firstStart, err := r.NextResolved()
	if err != nil {
		t.Fatalf("first child start: %v", err)
	}
	firstEnd, err := r.NextResolved()
	if err != nil {
		t.Fatalf("first child end: %v", err)
	}
	secondStart, err := r.NextResolved()
	if err != nil {
		t.Fatalf("second child start: %v", err)
	}
	secondEnd, err := r.NextResolved()
	if err != nil {
		t.Fatalf("second child end: %v", err)
	}

	if _, err := r.NextResolved(); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("root end: %v", err)
	}

	if firstStart.NameID == 0 || secondStart.NameID == 0 {
		t.Fatalf("child NameIDs = %d/%d, want non-zero", firstStart.NameID, secondStart.NameID)
	}
	if firstStart.NameID != secondStart.NameID {
		t.Fatalf("child NameIDs = %d/%d, want stable IDs", firstStart.NameID, secondStart.NameID)
	}
	if firstStart.NameID != firstEnd.NameID || secondStart.NameID != secondEnd.NameID {
		t.Fatalf("start/end NameIDs mismatch: first=%d/%d second=%d/%d", firstStart.NameID, firstEnd.NameID, secondStart.NameID, secondEnd.NameID)
	}
	if len(firstStart.Attrs) != 1 || len(secondStart.Attrs) != 1 {
		t.Fatalf("attr counts = %d/%d, want 1/1", len(firstStart.Attrs), len(secondStart.Attrs))
	}
	if firstStart.Attrs[0].NameID == 0 || secondStart.Attrs[0].NameID == 0 {
		t.Fatalf("attr NameIDs = %d/%d, want non-zero", firstStart.Attrs[0].NameID, secondStart.Attrs[0].NameID)
	}
	if firstStart.Attrs[0].NameID != secondStart.Attrs[0].NameID {
		t.Fatalf("attr NameIDs = %d/%d, want stable IDs", firstStart.Attrs[0].NameID, secondStart.Attrs[0].NameID)
	}
}
