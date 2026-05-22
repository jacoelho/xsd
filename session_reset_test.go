package xsd

import (
	"encoding/xml"
	"strconv"
	"testing"
)

func TestSessionResetDropsOversizedDocumentState(t *testing.T) {
	var s session
	s.errors = make([]error, 1, maxRetainedSliceCap+1)
	s.stack = make([]frame, 1, maxRetainedSliceCap+1)
	s.ns.frames = make([]int, 1, maxRetainedSliceCap+1)
	s.ns.bindings = make([]namespaceBinding, 1, maxRetainedSliceCap+1)
	s.text = make([]byte, 1, maxRetainedBufferCap+1)
	s.path = make([]string, 1, maxRetainedSliceCap+1)
	s.namePath = make([]runtimeName, 1, maxRetainedSliceCap+1)
	s.elementNames = make([]xml.Name, 1, maxRetainedSliceCap+1)
	s.allBits = make([]uint64, 1, maxRetainedSliceCap+1)
	s.idrefs = make([]identityRef, 1, maxRetainedSliceCap+1)
	s.idScopes = make([]identityScope, 1, maxRetainedSliceCap+1)
	s.idSelections = make([]identitySelection, 1, maxRetainedSliceCap+1)
	s.identityFieldValues = make([]identityFieldValue, 1, maxRetainedSliceCap+1)
	s.identityMatches = make([]identityFieldMatch, 1, maxRetainedSliceCap+1)
	s.ids = make(map[string]string, maxRetainedMapLen+1)
	s.schemaLocationNamespaces = make(map[string]bool, maxRetainedMapLen+1)
	for i := range maxRetainedMapLen + 1 {
		key := strconv.Itoa(i)
		s.ids[key] = key
		s.schemaLocationNamespaces[key] = true
	}

	s.reset()

	if cap(s.errors) != 0 ||
		cap(s.stack) != 0 ||
		cap(s.ns.frames) != 0 ||
		cap(s.ns.bindings) != 0 ||
		cap(s.text) != 0 ||
		cap(s.path) != 0 ||
		cap(s.namePath) != 0 ||
		cap(s.elementNames) != 0 ||
		cap(s.allBits) != 0 ||
		cap(s.idrefs) != 0 ||
		cap(s.idScopes) != 0 ||
		cap(s.idSelections) != 0 ||
		cap(s.identityFieldValues) != 0 ||
		cap(s.identityMatches) != 0 {
		t.Fatalf("reset retained oversized state")
	}
	if s.ids != nil {
		t.Fatalf("ids map retained after reset")
	}
	if s.schemaLocationNamespaces != nil {
		t.Fatalf("schema location namespace map retained after reset")
	}
}

func TestSessionResetClearsRetainedSliceCapacity(t *testing.T) {
	var s session
	s.path = append(make([]string, 0, maxRetainedSliceCap), "stale")
	s.path = s.path[:0]

	s.reset()

	if cap(s.path) == 0 {
		t.Fatal("path capacity was not retained")
	}
	if s.path[:cap(s.path)][0] != "" {
		t.Fatal("reset retained stale path string")
	}
}
