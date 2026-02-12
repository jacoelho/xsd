package preprocessor

import parser "github.com/jacoelho/xsd/internal/parser"

func (s *loadSession) markDirectiveMerged(kind parser.DirectiveKind, baseKey, targetKey loadKey) {
	if s == nil || s.loader == nil {
		return
	}
	s.loader.imports.markMerged(kind, baseKey, targetKey)
	s.journal.recordMarkMerged(kind, baseKey, targetKey)
}
