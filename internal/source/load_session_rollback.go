package source

import "github.com/jacoelho/xsd/internal/parser"

func (s *loadSession) rollback() {
	if s == nil || s.loader == nil {
		return
	}
	s.rollbackMerges()
	s.rollbackPending()
	s.rollbackKeyPending()
}

func (s *loadSession) rollbackMerges() {
	for i := len(s.merged.includes) - 1; i >= 0; i-- {
		rec := s.merged.includes[i]
		s.loader.imports.unmarkMerged(parser.DirectiveInclude, rec.base, rec.target)
	}
	for i := len(s.merged.imports) - 1; i >= 0; i-- {
		rec := s.merged.imports[i]
		s.loader.imports.unmarkMerged(parser.DirectiveImport, rec.base, rec.target)
	}
}

func (s *loadSession) rollbackPending() {
	for i := len(s.pending) - 1; i >= 0; i-- {
		change := s.pending[i]
		if entry, ok := s.loader.state.entry(change.sourceKey); ok && entry != nil {
			entry.pendingDirectives = removePendingDirective(entry.pendingDirectives, change.kind, change.targetKey)
		}
		if entry, ok := s.loader.state.entry(change.targetKey); ok && entry != nil {
			if entry.pendingCount > 0 {
				entry.pendingCount--
			}
		}
		s.loader.cleanupEntryIfUnused(change.sourceKey)
		s.loader.cleanupEntryIfUnused(change.targetKey)
	}
}

func (s *loadSession) rollbackKeyPending() {
	entry, ok := s.loader.state.entry(s.key)
	if !ok || entry == nil || len(entry.pendingDirectives) == 0 {
		return
	}
	for _, pending := range entry.pendingDirectives {
		if target, ok := s.loader.state.entry(pending.targetKey); ok && target != nil {
			if target.pendingCount > 0 {
				target.pendingCount--
			}
		}
		s.loader.cleanupEntryIfUnused(pending.targetKey)
	}
	entry.pendingDirectives = nil
	s.loader.cleanupEntryIfUnused(s.key)
}

func removePendingDirective(directives []pendingDirective, kind parser.DirectiveKind, targetKey loadKey) []pendingDirective {
	for i, entry := range directives {
		if entry.kind == kind && entry.targetKey == targetKey {
			return append(directives[:i], directives[i+1:]...)
		}
	}
	return directives
}
