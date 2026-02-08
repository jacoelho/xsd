package source

import (
	"github.com/jacoelho/xsd/internal/loadgraph"
	"github.com/jacoelho/xsd/internal/parser"
)

func (l *SchemaLoader) loadParsed(result *parser.ParseResult, systemID string, key loadKey) (*parser.Schema, error) {
	sch, err := l.cachedOrCircularSchema(key, systemID)
	if err != nil || sch != nil {
		return sch, err
	}

	sch = result.Schema
	lifecycle := l.parsedEntryLifecycle(key, systemID, sch, result.Includes, result.Imports)

	entry, cleanup, err := lifecycle.Begin()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	if err := lifecycle.Initialize(entry); err != nil {
		return nil, err
	}
	if err := l.applyParsedDirectives(systemID, key, sch, result.Directives); err != nil {
		return nil, err
	}

	lifecycle.Commit(entry)
	if err := l.resolvePendingImportsFor(key); err != nil {
		lifecycle.Rollback(entry)
		return nil, err
	}

	return sch, nil
}

func (l *SchemaLoader) cachedOrCircularSchema(key loadKey, systemID string) (*parser.Schema, error) {
	if loadedSchema, ok := l.state.loadedSchema(key); ok {
		return loadedSchema, nil
	}
	return loadgraph.CheckCircular[loadKey, *parser.Schema](loadingSchemaState{state: &l.state}, key, systemID)
}

func (l *SchemaLoader) parsedEntryLifecycle(
	key loadKey,
	systemID string,
	sch *parser.Schema,
	includes []parser.IncludeInfo,
	imports []parser.ImportInfo,
) loadgraph.EntryLifecycle[schemaEntry] {
	return loadgraph.EntryLifecycle[schemaEntry]{
		Enter: func() (*schemaEntry, func()) {
			return l.enterLoading(key)
		},
		Init: func(entry *schemaEntry) error {
			return l.initLoadEntry(entry, sch, systemID, includes, imports)
		},
		Finalize: func(entry *schemaEntry) {
			l.finalizeLoad(entry, sch)
		},
		Reset: func(entry *schemaEntry) {
			l.resetEntry(entry, key)
		},
	}
}

func (l *SchemaLoader) applyParsedDirectives(systemID string, key loadKey, sch *parser.Schema, directives []parser.Directive) (err error) {
	session := newLoadSession(l, systemID, key, nil)
	defer func() {
		if err != nil {
			session.rollback()
		}
	}()
	if err := session.processDirectives(sch, directives); err != nil {
		return err
	}
	return nil
}
