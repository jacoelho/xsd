package identity

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestApplyElementCapturesRecordsValue(t *testing.T) {
	match := &Match{
		Fields: []FieldState{{Count: 1}},
	}

	ApplyElementCaptures(false, []FieldCapture{{Match: match, FieldIndex: 0}}, runtime.VKString, []byte("one"))

	field := match.Fields[0]
	if !field.HasValue {
		t.Fatalf("field.HasValue = false, want true")
	}
	if field.KeyKind != runtime.VKString {
		t.Fatalf("field.KeyKind = %v, want %v", field.KeyKind, runtime.VKString)
	}
	if got := string(field.KeyBytes); got != "one" {
		t.Fatalf("field.KeyBytes = %q, want %q", got, "one")
	}
}

func TestCloseFrameFinalizesMatchesAndClosesScopes(t *testing.T) {
	var state State[int]
	rt := &runtime.Schema{Elements: make([]runtime.Element, 2)}
	state.Scopes.Push(Scope{
		RootID: 7,
		Constraints: []ConstraintState{{
			Category: runtime.ICUnique,
			Matches:  make(map[uint64]*Match),
		}},
	})
	scope := &state.Scopes.Items()[0]
	match := &Match{
		Constraint: &scope.Constraints[0],
		ID:         7,
		Fields:     []FieldState{{Count: 1}},
	}
	scope.Constraints[0].Matches[match.ID] = match

	if err := CloseFrame(rt, nil, &state, 7, 1, false, []FieldCapture{{Match: match, FieldIndex: 0}}, []*Match{match}, runtime.VKString, []byte("one")); err != nil {
		t.Fatalf("CloseFrame(): %v", err)
	}

	if got := len(scope.Constraints[0].Rows); got != 1 {
		t.Fatalf("rows = %d, want 1", got)
	}
	if got := len(scope.Constraints[0].Matches); got != 0 {
		t.Fatalf("matches = %d, want 0", got)
	}
	if got := state.Scopes.Len(); got != 0 {
		t.Fatalf("scopes len = %d, want 0", got)
	}
	if state.HasCommitted() {
		t.Fatalf("unexpected committed violations")
	}
}

func TestCloseFrameRejectsUnknownElement(t *testing.T) {
	err := CloseFrame(&runtime.Schema{}, nil, &State[int]{}, 1, 1, false, nil, nil, runtime.VKString, nil)
	if err == nil {
		t.Fatalf("expected missing element error")
	}
}
