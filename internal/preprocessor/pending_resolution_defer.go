package preprocessor

import parser "github.com/jacoelho/xsd/internal/parser"

func (l *Loader) deferImport(sourceKey, targetKey loadKey, schemaLocation, expectedNamespace string, journal *stateJournal) bool {
	directive := pendingDirective{
		kind:              parser.DirectiveImport,
		targetKey:         targetKey,
		schemaLocation:    schemaLocation,
		expectedNamespace: expectedNamespace,
	}
	return l.deferDirective(sourceKey, directive, journal)
}

func (l *Loader) deferInclude(sourceKey, targetKey loadKey, include parser.IncludeInfo, journal *stateJournal) bool {
	directive := pendingDirective{
		kind:             parser.DirectiveInclude,
		targetKey:        targetKey,
		schemaLocation:   include.SchemaLocation,
		includeDeclIndex: include.DeclIndex,
		includeIndex:     include.IncludeIndex,
	}
	return l.deferDirective(sourceKey, directive, journal)
}

func (l *Loader) deferDirective(sourceKey loadKey, directive pendingDirective, journal *stateJournal) bool {
	sourceEntry := l.state.ensureEntry(sourceKey)
	if !appendPendingDirective(sourceEntry, directive) {
		return false
	}
	if journal != nil {
		journal.recordAppendPendingDirective(directive.kind, sourceKey, directive.targetKey)
	}

	targetEntry := l.state.ensureEntry(directive.targetKey)
	incPendingCount(targetEntry)
	if journal != nil {
		journal.recordIncPendingCount(directive.targetKey)
	}
	return true
}
