package xmlstream

import (
	"errors"
	"strings"
	"testing"
)

func TestReaderEndOwnershipErrorsRemainStateErrors(t *testing.T) {
	var reader Reader
	if err := reader.Reset(strings.NewReader(`<root><child/></root>`), Config{}); err != nil {
		t.Fatal(err)
	}

	_ = mustNextToken(t, &reader)
	rootFrame, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)
	childFrame, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	_ = mustNextToken(t, &reader)

	matchErr := reader.MatchEnd(rootFrame)
	if !errors.Is(matchErr, ErrInvalidFrame) {
		t.Fatalf("MatchEnd with non-top frame = %v, want %v", matchErr, ErrInvalidFrame)
	}
	boundary, ok := errors.AsType[*Error](matchErr)
	if !ok || boundary.Kind != ErrorState {
		t.Fatalf("MatchEnd error = %#v, want ErrorState", matchErr)
	}
	if err := reader.MatchEnd(childFrame); err != nil {
		t.Fatal(err)
	}
	commitErr := reader.CommitEnd(rootFrame)
	if !errors.Is(commitErr, ErrInvalidFrame) {
		t.Fatalf("CommitEnd with non-top frame = %v, want %v", commitErr, ErrInvalidFrame)
	}
	boundary, ok = errors.AsType[*Error](commitErr)
	if !ok || boundary.Kind != ErrorState {
		t.Fatalf("CommitEnd error = %#v, want ErrorState", commitErr)
	}
	if reader.Depth() != 2 {
		t.Fatalf("depth after rejected CommitEnd = %d, want 2", reader.Depth())
	}
	if err := reader.CommitEnd(childFrame); err != nil {
		t.Fatal(err)
	}
}

func TestReaderRejectsForeignHandlesWithoutConsumingTransitions(t *testing.T) {
	var reader, other Reader
	for _, current := range []*Reader{&reader, &other} {
		if err := current.Reset(strings.NewReader(`<r/>`), Config{}); err != nil {
			t.Fatal(err)
		}
		_ = mustNextToken(t, current)
	}
	own, _, err := reader.Start()
	if err != nil {
		t.Fatal(err)
	}
	foreign, _, err := other.Start()
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.AbortStart(foreign); !errors.Is(err, ErrInvalidFrame) {
		t.Fatalf("AbortStart with foreign handle = %v, want %v", err, ErrInvalidFrame)
	}
	_ = mustNextToken(t, &reader)
	if err := reader.MatchEnd(foreign); !errors.Is(err, ErrInvalidFrame) {
		t.Fatalf("MatchEnd with foreign handle = %v, want %v", err, ErrInvalidFrame)
	}
	if err := reader.MatchEnd(own); err != nil {
		t.Fatal(err)
	}
	if err := reader.CommitEnd(foreign); !errors.Is(err, ErrInvalidFrame) {
		t.Fatalf("CommitEnd with foreign handle = %v, want %v", err, ErrInvalidFrame)
	}
	if err := reader.CommitEnd(own); err != nil {
		t.Fatal(err)
	}
	if reader.Depth() != 0 || other.Depth() != 1 {
		t.Fatalf("depths after commit = %d, %d, want 0, 1", reader.Depth(), other.Depth())
	}
}
