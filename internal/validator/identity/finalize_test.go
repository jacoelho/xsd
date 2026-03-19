package identity

import (
	"strings"
	"testing"

	xsderrors "github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/runtime"
)

func TestFieldStateAddNodeMarksMultiple(t *testing.T) {
	var state FieldState
	first := FieldNodeKey{Kind: FieldNodeElement, ElemID: 1}
	second := FieldNodeKey{Kind: FieldNodeElement, ElemID: 2}

	if !state.AddNode(first) {
		t.Fatalf("expected first node to be added")
	}
	if state.Count != 1 || state.Multiple {
		t.Fatalf("state after first add = %+v", state)
	}
	if state.AddNode(first) {
		t.Fatalf("expected duplicate node add to be ignored")
	}
	if !state.AddNode(second) {
		t.Fatalf("expected second node to be added")
	}
	if !state.Multiple || state.Count != 2 {
		t.Fatalf("state after second add = %+v", state)
	}
}

func TestFinalizeMatchesCollectsRowsAndViolations(t *testing.T) {
	t.Run("row", func(t *testing.T) {
		constraint := &ConstraintState{
			Category: runtime.ICUnique,
			Matches:  make(map[uint64]*Match),
		}
		match := &Match{
			Constraint: constraint,
			ID:         7,
			Fields: []FieldState{{
				Count:    1,
				KeyKind:  runtime.VKString,
				KeyBytes: []byte("one"),
				HasValue: true,
			}},
		}
		constraint.Matches[match.ID] = match

		FinalizeMatches(nil, []*Match{match})

		if len(constraint.Rows) != 1 {
			t.Fatalf("rows = %d, want 1", len(constraint.Rows))
		}
		if got := string(constraint.Rows[0].Values[0].Bytes); got != "one" {
			t.Fatalf("row value = %q, want %q", got, "one")
		}
		if len(constraint.Violations) != 0 {
			t.Fatalf("violations = %d, want 0", len(constraint.Violations))
		}
		if len(constraint.Matches) != 0 {
			t.Fatalf("matches not drained")
		}
	})

	t.Run("missing key field", func(t *testing.T) {
		constraint := &ConstraintState{
			Category: runtime.ICKey,
			Matches:  make(map[uint64]*Match),
		}
		match := &Match{
			Constraint: constraint,
			ID:         9,
			Fields:     []FieldState{{}},
		}
		constraint.Matches[match.ID] = match

		FinalizeMatches(nil, []*Match{match})

		if len(constraint.Violations) != 1 {
			t.Fatalf("violations = %d, want 1", len(constraint.Violations))
		}
		if constraint.Violations[0].Code != xsderrors.ErrIdentityAbsent {
			t.Fatalf("code = %v, want %v", constraint.Violations[0].Code, xsderrors.ErrIdentityAbsent)
		}
		if len(constraint.Matches) != 0 {
			t.Fatalf("matches not drained")
		}
	})
}

func TestResolveScopeIncludesConstraintName(t *testing.T) {
	scope := &Scope{
		Constraints: []ConstraintState{{
			ID:       1,
			Name:     "{urn:test}u1",
			Category: runtime.ICUnique,
			Rows: []Row{
				{
					Values: []runtime.ValueKey{{Kind: runtime.VKString, Bytes: []byte("dup"), Hash: runtime.HashKey(runtime.VKString, []byte("dup"))}},
					Hash:   1,
				},
				{
					Values: []runtime.ValueKey{{Kind: runtime.VKString, Bytes: []byte("dup"), Hash: runtime.HashKey(runtime.VKString, []byte("dup"))}},
					Hash:   1,
				},
			},
		}},
	}

	violations := ResolveScope(scope)
	if len(violations) != 1 {
		t.Fatalf("violations = %d, want 1", len(violations))
	}
	if violations[0].Code != xsderrors.ErrIdentityDuplicate {
		t.Fatalf("code = %v, want %v", violations[0].Code, xsderrors.ErrIdentityDuplicate)
	}
	if !strings.Contains(violations[0].Message, "identity constraint {urn:test}u1") {
		t.Fatalf("message = %q, want constraint name", violations[0].Message)
	}
}
