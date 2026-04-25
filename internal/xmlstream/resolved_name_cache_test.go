package xmlstream

import (
	"errors"
	"fmt"
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

func TestNextResolvedEndNameIDsStableAfterNext(t *testing.T) {
	t.Parallel()
	testMixedModeEndNameIDStability(t, func(t *testing.T, r *Reader) {
		t.Helper()
		ev, err := r.Next()
		if err != nil {
			t.Fatalf("Next start: %v", err)
		}
		if ev.Kind != EventStartElement || ev.Name.Local != "root" {
			t.Fatalf("Next start = %v %s, want root start", ev.Kind, ev.Name.String())
		}
	})
}

func TestNextResolvedEndNameIDsStableAfterNextRaw(t *testing.T) {
	t.Parallel()
	testMixedModeEndNameIDStability(t, func(t *testing.T, r *Reader) {
		t.Helper()
		ev, err := r.NextRaw()
		if err != nil {
			t.Fatalf("NextRaw start: %v", err)
		}
		if ev.Kind != EventStartElement || string(ev.Name.Full) != "root" {
			t.Fatalf("NextRaw start = %v %q, want root start", ev.Kind, ev.Name.Full)
		}
	})
}

func testMixedModeEndNameIDStability(t *testing.T, consumeStart func(*testing.T, *Reader)) {
	t.Helper()

	input := `<wrapper xmlns="urn:test"><root></root><root></root></wrapper>`
	r, err := NewReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	wrapperStart, err := r.NextResolved()
	if err != nil {
		t.Fatalf("wrapper start: %v", err)
	}
	if wrapperStart.Kind != EventStartElement || string(wrapperStart.Local) != "wrapper" {
		t.Fatalf("wrapper start = %v %q, want wrapper start", wrapperStart.Kind, wrapperStart.Local)
	}

	consumeStart(t, r)

	firstEnd, err := r.NextResolved()
	if err != nil {
		t.Fatalf("first root end: %v", err)
	}
	secondStart, err := r.NextResolved()
	if err != nil {
		t.Fatalf("second root start: %v", err)
	}
	secondEnd, err := r.NextResolved()
	if err != nil {
		t.Fatalf("second root end: %v", err)
	}

	if firstEnd.Kind != EventEndElement || string(firstEnd.Local) != "root" {
		t.Fatalf("first root end = %v %q, want root end", firstEnd.Kind, firstEnd.Local)
	}
	if secondStart.Kind != EventStartElement || string(secondStart.Local) != "root" {
		t.Fatalf("second root start = %v %q, want root start", secondStart.Kind, secondStart.Local)
	}
	if secondEnd.Kind != EventEndElement || string(secondEnd.Local) != "root" {
		t.Fatalf("second root end = %v %q, want root end", secondEnd.Kind, secondEnd.Local)
	}
	if firstEnd.NameID == 0 {
		t.Fatalf("first root end NameID = 0, want non-zero")
	}
	if firstEnd.NameID != secondStart.NameID || secondStart.NameID != secondEnd.NameID {
		t.Fatalf("root NameIDs = %d/%d/%d, want stable IDs", firstEnd.NameID, secondStart.NameID, secondEnd.NameID)
	}
}

func TestResolvedNameCacheInternBytesStableAfterBufferReuse(t *testing.T) {
	t.Parallel()

	cache := newResolvedNameCache()
	buf := []byte("repeat")

	first := cache.internBytes("urn:test", buf)
	for i := range qnameCacheRecentSize + 2 {
		cache.intern("urn:test", fmt.Sprintf("n%d", i))
	}

	copy(buf, "mutate")

	second := cache.internBytes("urn:test", []byte("repeat"))
	if first.id != second.id {
		t.Fatalf("NameIDs = %d/%d, want stable IDs", first.id, second.id)
	}
}

func TestNextResolvedNameIDsStableAfterRecentEviction(t *testing.T) {
	t.Parallel()

	var input strings.Builder
	input.WriteString(`<root xmlns="urn:test">`)
	input.WriteString(`<repeat repeat-attr="1"/>`)
	for i := range qnameCacheRecentSize + 2 {
		fmt.Fprintf(&input, `<n%d a%d="%d"/>`, i, i, i)
	}
	input.WriteString(`<repeat repeat-attr="2"/>`)
	input.WriteString(`</root>`)

	r, err := NewReader(strings.NewReader(input.String()))
	if err != nil {
		t.Fatalf("NewReader error = %v", err)
	}

	var startIDs []NameID
	var endIDs []NameID
	var attrIDs []NameID
	for {
		ev, err := r.NextResolved()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("NextResolved error = %v", err)
		}
		if string(ev.Local) != "repeat" {
			continue
		}

		switch ev.Kind {
		case EventStartElement:
			startIDs = append(startIDs, ev.NameID)
			if len(ev.Attrs) != 1 {
				t.Fatalf("repeat attr count = %d, want 1", len(ev.Attrs))
			}
			attrIDs = append(attrIDs, ev.Attrs[0].NameID)
		case EventEndElement:
			endIDs = append(endIDs, ev.NameID)
		}
	}

	if len(startIDs) != 2 || len(endIDs) != 2 || len(attrIDs) != 2 {
		t.Fatalf("repeat events = starts:%d ends:%d attrs:%d, want 2/2/2", len(startIDs), len(endIDs), len(attrIDs))
	}
	if startIDs[0] == 0 || endIDs[0] == 0 || attrIDs[0] == 0 {
		t.Fatalf("first repeat NameIDs = start:%d end:%d attr:%d, want non-zero", startIDs[0], endIDs[0], attrIDs[0])
	}
	if startIDs[0] != startIDs[1] || startIDs[0] != endIDs[0] || startIDs[1] != endIDs[1] {
		t.Fatalf("repeat element NameIDs = starts:%v ends:%v, want stable IDs", startIDs, endIDs)
	}
	if attrIDs[0] != attrIDs[1] {
		t.Fatalf("repeat attr NameIDs = %v, want stable IDs", attrIDs)
	}
}
