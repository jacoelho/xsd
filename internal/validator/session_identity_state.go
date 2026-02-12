package validator

func (s *identityState) reset() {
	if s == nil {
		return
	}
	s.active = false
	s.nextNodeID = 0
	s.frames.Reset()
	s.scopes.Reset()
	s.uncommittedViolations = s.uncommittedViolations[:0]
	s.committedViolations = s.committedViolations[:0]
}

func (s *identityState) checkpoint() identitySnapshot {
	if s == nil {
		return identitySnapshot{}
	}
	return identitySnapshot{
		nextNodeID:  s.nextNodeID,
		framesLen:   s.frames.Len(),
		scopesLen:   s.scopes.Len(),
		uncommitted: len(s.uncommittedViolations),
		committed:   len(s.committedViolations),
		active:      s.active,
	}
}

func (s *identityState) rollback(snapshot identitySnapshot) {
	if s == nil {
		return
	}
	for s.frames.Len() > snapshot.framesLen {
		s.frames.Pop()
	}
	for s.scopes.Len() > snapshot.scopesLen {
		s.scopes.Pop()
	}
	if snapshot.uncommitted <= len(s.uncommittedViolations) {
		s.uncommittedViolations = s.uncommittedViolations[:snapshot.uncommitted]
	}
	if snapshot.committed <= len(s.committedViolations) {
		s.committedViolations = s.committedViolations[:snapshot.committed]
	}
	s.nextNodeID = snapshot.nextNodeID
	s.active = snapshot.active
}
