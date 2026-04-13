package compiler

import (
	"errors"
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/parser"
)

type loadSession struct {
	doc      io.ReadCloser
	loader   *Loader
	key      loadKey
	systemID string
	journal  Journal[loadKey]
}

func newLoadSession(loader *Loader, systemID string, key loadKey, doc io.ReadCloser) *loadSession {
	return &loadSession{
		loader:   loader,
		systemID: systemID,
		key:      key,
		doc:      doc,
	}
}

func (s *loadSession) loadResolved() (*parser.Schema, error) {
	if s == nil || s.loader == nil {
		return nil, errors.New("load session missing")
	}
	if loadedSchema, ok := s.loader.state.loadedSchema(s.key); ok {
		if err := Close(s.doc, s.systemID); err != nil {
			return nil, err
		}
		return loadedSchema, nil
	}
	sch, err := checkCircularLoad[loadKey, *parser.Schema](&s.loader.state, s.key, s.systemID)
	if err != nil || sch != nil {
		return sch, errors.Join(err, Close(s.doc, s.systemID))
	}

	result, err := Parse(s.doc, s.systemID, s.loader.config.DocumentPool, s.loader.config.SchemaParseOptions...)
	if err != nil {
		return nil, err
	}
	return s.loader.loadParsedWithJournal(result, s.systemID, s.key, &s.journal)
}

func (s *loadSession) resolveIncludeTarget(info parser.IncludeInfo) (LoadResult[loadKey], error) {
	if s == nil || s.loader == nil {
		return LoadResult[loadKey]{}, errors.New("load session missing")
	}
	doc, systemID, err := s.loader.resolver.Resolve(ResolveRequest{
		BaseSystemID:   s.systemID,
		SchemaLocation: info.SchemaLocation,
		Kind:           ResolveInclude,
	})
	if err != nil {
		return LoadResult[loadKey]{}, err
	}
	targetKey := s.loader.loadKey(systemID, s.key.etn)
	if s.loader.imports.AlreadyMerged(parser.DirectiveInclude, s.key, targetKey) {
		if closeErr := Close(doc, systemID); closeErr != nil {
			return LoadResult[loadKey]{}, closeErr
		}
		return LoadResult[loadKey]{Target: targetKey, Status: StatusDeferred}, nil
	}
	if s.loader.state.IsLoading(targetKey) {
		if closeErr := Close(doc, systemID); closeErr != nil {
			return LoadResult[loadKey]{}, closeErr
		}
		_ = s.loader.deferDirective(targetKey, Directive[loadKey]{
			Kind:             parser.DirectiveInclude,
			TargetKey:        s.key,
			SchemaLocation:   info.SchemaLocation,
			IncludeDeclIndex: info.DeclIndex,
			IncludeIndex:     info.IncludeIndex,
		}, &s.journal)
		return LoadResult[loadKey]{Target: targetKey, Status: StatusDeferred}, nil
	}

	targetSession := newLoadSession(s.loader, systemID, targetKey, doc)
	targetSession.journal = s.journal
	schema, err := targetSession.loadResolved()
	s.journal = targetSession.journal
	if err != nil {
		return LoadResult[loadKey]{}, err
	}
	return LoadResult[loadKey]{
		Schema: schema,
		Target: targetKey,
		Status: StatusLoaded,
	}, nil
}

func (s *loadSession) resolveImportTarget(info parser.ImportInfo) (LoadResult[loadKey], error) {
	if s == nil || s.loader == nil {
		return LoadResult[loadKey]{}, errors.New("load session missing")
	}
	if info.SchemaLocation == "" {
		if s.loader.config.AllowMissingImportLocations {
			return LoadResult[loadKey]{Status: StatusSkippedMissing}, nil
		}
		return LoadResult[loadKey]{}, errors.New("import missing schemaLocation")
	}
	doc, systemID, err := s.loader.resolver.Resolve(ResolveRequest{
		BaseSystemID:   s.systemID,
		SchemaLocation: info.SchemaLocation,
		ImportNS:       []byte(info.Namespace),
		Kind:           ResolveImport,
	})
	if err != nil {
		if s.loader.config.AllowMissingImportLocations && isNotFound(err) {
			return LoadResult[loadKey]{Status: StatusSkippedMissing}, nil
		}
		return LoadResult[loadKey]{}, err
	}
	targetKey := s.loader.loadKey(systemID, info.Namespace)
	if s.loader.imports.AlreadyMerged(parser.DirectiveImport, s.key, targetKey) {
		if closeErr := Close(doc, systemID); closeErr != nil {
			return LoadResult[loadKey]{}, closeErr
		}
		return LoadResult[loadKey]{Target: targetKey, Status: StatusDeferred}, nil
	}
	if s.loader.state.IsLoading(targetKey) {
		if closeErr := Close(doc, systemID); closeErr != nil {
			return LoadResult[loadKey]{}, closeErr
		}
		_ = s.loader.deferDirective(targetKey, Directive[loadKey]{
			Kind:              parser.DirectiveImport,
			TargetKey:         s.key,
			SchemaLocation:    info.SchemaLocation,
			ExpectedNamespace: info.Namespace,
		}, &s.journal)
		return LoadResult[loadKey]{Target: targetKey, Status: StatusDeferred}, nil
	}

	targetSession := newLoadSession(s.loader, systemID, targetKey, doc)
	targetSession.journal = s.journal
	schema, err := targetSession.loadResolved()
	s.journal = targetSession.journal
	if err != nil {
		return LoadResult[loadKey]{}, err
	}
	return LoadResult[loadKey]{
		Schema: schema,
		Target: targetKey,
		Status: StatusLoaded,
	}, nil
}

// Status reports the outcome of loading one directive target.
type Status uint8

const (
	StatusLoaded Status = iota
	StatusDeferred
	StatusSkippedMissing
)

// LoadResult carries the schema and target key produced by one directive load.
type LoadResult[K comparable] struct {
	Schema *parser.Schema
	Target K
	Status Status
}

func (s *loadSession) processDirectives(schema *parser.Schema, directives []parser.Directive) error {
	for _, directive := range directives {
		switch directive.Kind {
		case parser.DirectiveInclude:
			if err := s.processInclude(schema, directive.Include); err != nil {
				return err
			}
		case parser.DirectiveImport:
			if err := s.processImport(schema, directive.Import); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected directive kind: %d", directive.Kind)
		}
	}
	return nil
}

func (s *loadSession) processInclude(schema *parser.Schema, include parser.IncludeInfo) error {
	result, err := s.resolveIncludeTarget(include)
	if err != nil {
		return fmt.Errorf("load included schema %s: %w", include.SchemaLocation, err)
	}
	switch result.Status {
	case StatusDeferred:
		return nil
	case StatusSkippedMissing:
		return fmt.Errorf("included schema %s not found", include.SchemaLocation)
	case StatusLoaded:
	default:
		return fmt.Errorf("unexpected include load status: %d", result.Status)
	}

	entry, ok := s.loader.state.entry(s.key)
	if !ok || entry == nil {
		return fmt.Errorf("include tracking missing for %s", s.key.systemID)
	}
	plan, err := PlanInclude(s.key.etn, entry.includeInserted, schema, include, include.SchemaLocation, result.Schema)
	if err != nil {
		if tracked, ok := s.loader.state.entry(result.Target); ok && tracked != nil {
			s.loader.resetEntry(tracked, result.Target)
		}
		return err
	}
	inserted, err := ApplyPlanned(schema, result.Schema, plan, "included", include.SchemaLocation)
	if err != nil {
		return err
	}
	if err := RecordIncludeInserted(entry.includeInserted, include.IncludeIndex, inserted); err != nil {
		return err
	}
	s.markDirectiveMerged(parser.DirectiveInclude, s.key, result.Target)
	return nil
}

func (s *loadSession) processImport(schema *parser.Schema, imp parser.ImportInfo) error {
	result, err := s.resolveImportTarget(imp)
	if err != nil {
		return fmt.Errorf("load imported schema %s: %w", imp.SchemaLocation, err)
	}
	switch result.Status {
	case StatusDeferred, StatusSkippedMissing:
		return nil
	case StatusLoaded:
	default:
		return fmt.Errorf("unexpected import load status: %d", result.Status)
	}

	plan, err := PlanImport(imp.SchemaLocation, imp.Namespace, result.Schema, len(schema.GlobalDecls))
	if err != nil {
		if entry, ok := s.loader.state.entry(result.Target); ok && entry != nil {
			s.loader.resetEntry(entry, result.Target)
		}
		return err
	}
	if _, err := ApplyPlanned(schema, result.Schema, plan, "imported", imp.SchemaLocation); err != nil {
		return err
	}
	s.markDirectiveMerged(parser.DirectiveImport, s.key, result.Target)
	return nil
}

func (s *loadSession) rollback() {
	if s == nil || s.loader == nil {
		return
	}
	s.journal.Rollback(s.loader.stateRollbackCallbacks())
}

func (s *loadSession) markDirectiveMerged(kind parser.DirectiveKind, baseKey, targetKey loadKey) {
	if s == nil || s.loader == nil {
		return
	}
	s.loader.imports.MarkMerged(kind, baseKey, targetKey)
	s.journal.RecordMarkMerged(kind, baseKey, targetKey)
}
