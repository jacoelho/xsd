package compiler

import "github.com/jacoelho/xsd/internal/parser"

func (s *loadSession) markDirectiveMerged(kind parser.DirectiveKind, baseKey, targetKey loadKey) {
	if s == nil || s.loader == nil {
		return
	}
	s.loader.imports.MarkMerged(kind, baseKey, targetKey)
	s.journal.RecordMarkMerged(kind, baseKey, targetKey)
}
