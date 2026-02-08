package source

import "github.com/jacoelho/xsd/internal/parser"

func (l *SchemaLoader) deferImport(sourceKey, targetKey loadKey, schemaLocation, expectedNamespace string) bool {
	sourceEntry := l.state.ensureEntry(sourceKey)
	for _, pending := range sourceEntry.pendingDirectives {
		if pending.kind == parser.DirectiveImport && pending.targetKey == targetKey {
			return false
		}
	}
	sourceEntry.pendingDirectives = append(sourceEntry.pendingDirectives, pendingDirective{
		kind:              parser.DirectiveImport,
		targetKey:         targetKey,
		schemaLocation:    schemaLocation,
		expectedNamespace: expectedNamespace,
	})
	targetEntry := l.state.ensureEntry(targetKey)
	targetEntry.pendingCount++
	return true
}

func (l *SchemaLoader) deferInclude(sourceKey, targetKey loadKey, include parser.IncludeInfo) bool {
	sourceEntry := l.state.ensureEntry(sourceKey)
	for _, pending := range sourceEntry.pendingDirectives {
		if pending.kind == parser.DirectiveInclude && pending.targetKey == targetKey {
			return false
		}
	}
	sourceEntry.pendingDirectives = append(sourceEntry.pendingDirectives, pendingDirective{
		kind:             parser.DirectiveInclude,
		targetKey:        targetKey,
		schemaLocation:   include.SchemaLocation,
		includeDeclIndex: include.DeclIndex,
		includeIndex:     include.IncludeIndex,
	})
	targetEntry := l.state.ensureEntry(targetKey)
	targetEntry.pendingCount++
	return true
}
