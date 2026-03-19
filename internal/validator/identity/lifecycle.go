package identity

import (
	"slices"

	"github.com/jacoelho/xsd/internal/stack"
)

// State stores identity-constraint runtime state for one validation session.
type State[F any] struct {
	Frames      stack.Stack[F]
	Scopes      stack.Stack[Scope]
	Uncommitted []error
	Committed   []Violation
	NextNodeID  uint64
	Active      bool
}

// Snapshot captures enough state to roll identity processing back after a failed start event.
type Snapshot struct {
	nextNodeID  uint64
	framesLen   int
	scopesLen   int
	uncommitted int
	committed   int
	active      bool
}

// Reset clears all state while retaining backing storage.
func (s *State[F]) Reset() {
	if s == nil {
		return
	}
	s.Active = false
	s.NextNodeID = 0
	s.Frames.Reset()
	s.Scopes.Reset()
	s.Uncommitted = s.Uncommitted[:0]
	s.Committed = s.Committed[:0]
}

// Checkpoint returns a rollback point for the current state.
func (s *State[F]) Checkpoint() Snapshot {
	if s == nil {
		return Snapshot{}
	}
	return Snapshot{
		nextNodeID:  s.NextNodeID,
		framesLen:   s.Frames.Len(),
		scopesLen:   s.Scopes.Len(),
		uncommitted: len(s.Uncommitted),
		committed:   len(s.Committed),
		active:      s.Active,
	}
}

// Rollback restores the state to the provided snapshot.
func (s *State[F]) Rollback(snapshot Snapshot) {
	if s == nil {
		return
	}
	for s.Frames.Len() > snapshot.framesLen {
		s.Frames.Pop()
	}
	for s.Scopes.Len() > snapshot.scopesLen {
		s.Scopes.Pop()
	}
	if snapshot.uncommitted <= len(s.Uncommitted) {
		s.Uncommitted = s.Uncommitted[:snapshot.uncommitted]
	}
	if snapshot.committed <= len(s.Committed) {
		s.Committed = s.Committed[:snapshot.committed]
	}
	s.NextNodeID = snapshot.nextNodeID
	s.Active = snapshot.active
}

// CloseScopes resolves and removes all scopes rooted at frameID.
func (s *State[F]) CloseScopes(frameID uint64) {
	if s == nil {
		return
	}
	for {
		scope, ok := s.Scopes.Peek()
		if !ok || scope.RootID != frameID {
			return
		}
		s.appendScopeViolations(&scope)
		s.Scopes.Pop()
	}
}

func (s *State[F]) appendScopeViolations(scope *Scope) {
	if s == nil || scope == nil {
		return
	}
	for i := range scope.Constraints {
		s.Committed = appendViolations(s.Committed, scope.Constraints[i].Violations)
	}
	s.Committed = appendViolations(s.Committed, ResolveScope(scope))
}

func appendViolations(dst, issues []Violation) []Violation {
	if len(issues) == 0 {
		return dst
	}
	return append(dst, issues...)
}

// DrainCommitted returns finalized violations and clears the committed queue.
func (s *State[F]) DrainCommitted() []Violation {
	if s == nil || len(s.Committed) == 0 {
		return nil
	}
	out := s.Committed
	s.Committed = s.Committed[:0]
	return out
}

// DrainUncommitted returns immediate processing failures and clears the queue.
func (s *State[F]) DrainUncommitted() []error {
	if s == nil || len(s.Uncommitted) == 0 {
		return nil
	}
	out := slices.Clone(s.Uncommitted)
	s.Uncommitted = s.Uncommitted[:0]
	return out
}

// HasCommitted reports whether finalized violations are pending.
func (s *State[F]) HasCommitted() bool {
	return s != nil && len(s.Committed) > 0
}
