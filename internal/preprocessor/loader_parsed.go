package preprocessor

import (
	"github.com/jacoelho/xsd/internal/loadguard"
	"github.com/jacoelho/xsd/internal/parser"
)

func (l *Loader) loadParsed(result *parser.ParseResult, systemID string, key loadKey) (*parser.Schema, error) {
	return l.loadParsedWithJournal(result, systemID, key, nil)
}

func (l *Loader) loadParsedWithJournal(
	result *parser.ParseResult,
	systemID string,
	key loadKey,
	parentJournal *stateJournal,
) (*parser.Schema, error) {
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

	if lifecycle.Init != nil {
		if err := lifecycle.Init(entry); err != nil {
			return nil, err
		}
	}
	if err := l.applyParsedDirectives(systemID, key, sch, result.Directives, parentJournal); err != nil {
		return nil, err
	}

	lifecycle.Commit(entry)
	if err := l.resolvePendingImportsFor(key); err != nil {
		rollbackSourcePending(l, key)
		lifecycle.Rollback(entry)
		return nil, err
	}

	return sch, nil
}

func (l *Loader) cachedOrCircularSchema(key loadKey, systemID string) (*parser.Schema, error) {
	if loadedSchema, ok := l.state.loadedSchema(key); ok {
		return loadedSchema, nil
	}
	return loadguard.CheckCircular[loadKey, *parser.Schema](&l.state, key, systemID)
}

func (l *Loader) parsedEntryLifecycle(
	key loadKey,
	systemID string,
	sch *parser.Schema,
	includes []parser.IncludeInfo,
	imports []parser.ImportInfo,
) loadguard.EntryLifecycle[schemaEntry] {
	return loadguard.EntryLifecycle[schemaEntry]{
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
	parentJournal *stateJournal,
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
		parentJournal.append(&session.journal)
	}
	return nil
}
