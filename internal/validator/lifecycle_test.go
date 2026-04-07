package validator

import (
	"fmt"
	"testing"
)

func TestStateCheckpointRollback(t *testing.T) {
	state := State[int]{
		Active:      true,
		NextNodeID:  2,
		Uncommitted: []error{fmt.Errorf("before")},
		Committed:   []Violation{{Message: "before"}},
	}
	state.Frames.Push(1)
	state.Scopes.Push(Scope{RootID: 1})

	snapshot := state.Checkpoint()

	state.Active = false
	state.NextNodeID = 7
	state.Frames.Push(2)
	state.Scopes.Push(Scope{RootID: 2})
	state.Uncommitted = append(state.Uncommitted, fmt.Errorf("after"))
	state.Committed = append(state.Committed, Violation{Message: "after"})

	state.Rollback(snapshot)

	if !state.Active {
		t.Fatalf("state.Active = false, want true")
	}
	if state.NextNodeID != 2 {
		t.Fatalf("state.NextNodeID = %d, want 2", state.NextNodeID)
	}
	if state.Frames.Len() != 1 {
		t.Fatalf("frames len = %d, want 1", state.Frames.Len())
	}
	if state.Scopes.Len() != 1 {
		t.Fatalf("scopes len = %d, want 1", state.Scopes.Len())
	}
	if len(state.Uncommitted) != 1 {
		t.Fatalf("uncommitted len = %d, want 1", len(state.Uncommitted))
	}
	if len(state.Committed) != 1 {
		t.Fatalf("committed len = %d, want 1", len(state.Committed))
	}
}

func TestStateCloseScopesCollectsViolations(t *testing.T) {
	state := State[int]{}
	state.Scopes.Push(Scope{
		RootID: 2,
		Constraints: []ConstraintState{{
			Violations: []Violation{{Message: "other"}},
		}},
	})
	state.Scopes.Push(Scope{
		RootID: 7,
		Constraints: []ConstraintState{{
			Violations: []Violation{{Message: "first"}},
		}},
	})
	state.Scopes.Push(Scope{
		RootID: 7,
		Constraints: []ConstraintState{{
			Violations: []Violation{{Message: "second"}},
		}},
	})

	state.CloseScopes(7)

	if state.Scopes.Len() != 1 {
		t.Fatalf("scopes len = %d, want 1", state.Scopes.Len())
	}
	if !state.HasCommitted() {
		t.Fatalf("expected committed violations")
	}
	got := state.DrainCommitted()
	if len(got) != 2 {
		t.Fatalf("committed len = %d, want 2", len(got))
	}
	if got[0].Message != "second" || got[1].Message != "first" {
		t.Fatalf("committed = %+v, want LIFO scope order", got)
	}
	if state.HasCommitted() {
		t.Fatalf("committed queue not drained")
	}
}

func TestStateDrainUncommittedClones(t *testing.T) {
	state := State[int]{Uncommitted: []error{fmt.Errorf("one")}}

	drained := state.DrainUncommitted()
	if len(drained) != 1 {
		t.Fatalf("drained len = %d, want 1", len(drained))
	}
	if len(state.Uncommitted) != 0 {
		t.Fatalf("state uncommitted len = %d, want 0", len(state.Uncommitted))
	}

	drained[0] = fmt.Errorf("changed")
	if len(state.Uncommitted) != 0 {
		t.Fatalf("state changed after drain mutation")
	}
}
