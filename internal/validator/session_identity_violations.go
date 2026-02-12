package validator

import "slices"

func (s *identityState) closeScopes(frameID uint64) {
	for {
		scope, ok := s.scopes.Peek()
		if !ok || scope.rootID != frameID {
			return
		}
		s.appendScopeViolations(&scope)
		s.scopes.Pop()
	}
}

func (s *identityState) appendScopeViolations(scope *rtIdentityScope) {
	if s == nil || scope == nil {
		return
	}
	for i := range scope.constraints {
		if len(scope.constraints[i].violations) > 0 {
			s.committedViolations = append(s.committedViolations, scope.constraints[i].violations...)
		}
	}
	if errs := resolveScopeErrors(scope); len(errs) > 0 {
		s.committedViolations = append(s.committedViolations, errs...)
	}
}

func (s *identityState) drainCommitted() []error {
	if s == nil || len(s.committedViolations) == 0 {
		return nil
	}
	out := s.committedViolations
	s.committedViolations = s.committedViolations[:0]
	return out
}

func (s *identityState) drainUncommitted() []error {
	if s == nil || len(s.uncommittedViolations) == 0 {
		return nil
	}
	out := slices.Clone(s.uncommittedViolations)
	s.uncommittedViolations = s.uncommittedViolations[:0]
	return out
}

func (s *identityState) hasCommitted() bool {
	return s != nil && len(s.committedViolations) > 0
}
