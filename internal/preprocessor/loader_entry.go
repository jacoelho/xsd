package preprocessor

import "github.com/jacoelho/xsd/internal/parser"

func (l *Loader) enterLoading(key loadKey) (*schemaEntry, func()) {
	entry := l.state.ensureEntry(key)
	entry.state = schemaStateLoading
	entry.schema = nil
	cleanup := func() {
		if entry.state != schemaStateLoading {
			return
		}
		entry.state = schemaStateUnknown
		entry.schema = nil
		l.cleanupEntryIfUnused(key)
	}
	return entry, cleanup
}

func (l *Loader) initLoadEntry(entry *schemaEntry, sch *parser.Schema, systemID string, includes []parser.IncludeInfo, imports []parser.ImportInfo) error {
	initSchemaOrigins(sch, systemID)
	entry.schema = sch
	if len(includes) > 0 {
		entry.includeInserted = make([]int, len(includes))
	} else {
		entry.includeInserted = nil
	}
	registerImports(sch, imports)
	if validateErr := validateImportConstraints(sch, imports); validateErr != nil {
		return validateErr
	}
	return nil
}

func (l *Loader) finalizeLoad(entry *schemaEntry, sch *parser.Schema) {
	entry.schema = sch
	entry.state = schemaStateLoaded
}

func (l *Loader) resetEntry(entry *schemaEntry, key loadKey) {
	entry.schema = nil
	entry.state = schemaStateUnknown
	resetPendingTracking(entry)
	l.cleanupEntryIfUnused(key)
}
