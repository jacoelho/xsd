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

	rootToken := mustNextToken(t, &reader)
	rootFrame, _, err := reader.Start(&rootToken.Start)
	if err != nil {
		t.Fatal(err)
	}
	childToken := mustNextToken(t, &reader)
	childFrame, _, err := reader.Start(&childToken.Start)
	if err != nil {
		t.Fatal(err)
	}
	childEnd := mustNextToken(t, &reader)

	matchErr := reader.MatchEnd(rootFrame, childEnd.End)
	if !errors.Is(matchErr, ErrInvalidFrame) {
		t.Fatalf("MatchEnd with non-top frame = %v, want %v", matchErr, ErrInvalidFrame)
	}
	boundary, ok := errors.AsType[*Error](matchErr)
	if !ok || boundary.Kind != ErrorState {
		t.Fatalf("MatchEnd error = %#v, want ErrorState", matchErr)
	}
	if err := reader.MatchEnd(childFrame, childEnd.End); err != nil {
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
