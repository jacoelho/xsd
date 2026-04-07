package preprocessor

import (
	"github.com/jacoelho/xsd/internal/parser"
)

func (l *Loader) loadParsedWithJournal(
	result *parser.ParseResult,
	systemID string,
	key loadKey,
	parentJournal *Journal[loadKey],
) (*parser.Schema, error) {
	sch, err := l.cachedOrCircularSchema(key, systemID)
	if err != nil || sch != nil {
		return sch, err
	}

	sch = result.Schema
	lifecycle := l.parsedEntryLifecycle(key, systemID, sch, result.Includes, result.Imports)
	return ApplyParsed(sch, ApplyCallbacks[schemaEntry]{
		Begin: lifecycle.Begin,
		Init:  lifecycle.Init,
		ApplyDirectives: func() error {
			return l.applyParsedDirectives(systemID, key, sch, result.Directives, parentJournal)
		},
		Commit: lifecycle.Commit,
		ResolvePending: func() error {
			return l.resolvePendingImportsFor(key)
		},
		RollbackPending: func() {
			rollbackSourcePending(l, key)
		},
		Rollback: lifecycle.Rollback,
	})
}

func (l *Loader) cachedOrCircularSchema(key loadKey, systemID string) (*parser.Schema, error) {
	if loadedSchema, ok := l.state.loadedSchema(key); ok {
		return loadedSchema, nil
	}
	return checkCircularLoad[loadKey, *parser.Schema](&l.state, key, systemID)
}

func (l *Loader) parsedEntryLifecycle(
	key loadKey,
	systemID string,
	sch *parser.Schema,
	includes []parser.IncludeInfo,
	imports []parser.ImportInfo,
) entryLifecycle[schemaEntry] {
	return entryLifecycle[schemaEntry]{
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

func (l *Loader) applyParsedDirectives(
	systemID string,
	key loadKey,
	sch *parser.Schema,
	directives []parser.Directive,
	parentJournal *Journal[loadKey],
) (err error) {
	session := newLoadSession(l, systemID, key, nil)
	defer func() {
		if err != nil {
			session.rollback()
		}
	}()
	if err := session.processDirectives(sch, directives); err != nil {
		return err
	}
	if parentJournal != nil {
		parentJournal.Append(&session.journal)
	}
	return nil
}
