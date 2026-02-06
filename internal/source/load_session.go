package source

import (
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/pkg/xmlstream"
)

type loadSession struct {
	doc      io.ReadCloser
	loader   *SchemaLoader
	key      loadKey
	systemID string
	pending  []pendingChange
	merged   mergedChanges
}

type directiveLoadStatus uint8

const (
	directiveLoadStatusLoaded directiveLoadStatus = iota
	directiveLoadStatusDeferred
	directiveLoadStatusSkippedMissing
)

type directiveLoadResult struct {
	schema *parser.Schema
	target loadKey
	status directiveLoadStatus
}

type pendingChange struct {
	sourceKey loadKey
	targetKey loadKey
	kind      parser.DirectiveKind
}

type mergedChanges struct {
	includes []mergeRecord
	imports  []mergeRecord
}

type mergeRecord struct {
	base   loadKey
	target loadKey
}

func newLoadSession(loader *SchemaLoader, systemID string, key loadKey, doc io.ReadCloser) *loadSession {
	return &loadSession{
		loader:   loader,
		systemID: systemID,
		key:      key,
		doc:      doc,
	}
}

func (s *loadSession) handleCircularLoad() (*parser.Schema, error) {
	if !s.loader.state.isLoading(s.key) {
		return nil, nil
	}
	if schema, ok := s.loader.state.loadedSchema(s.key); ok {
		return schema, nil
	}
	inProgress, ok := s.loader.state.loadingSchema(s.key)
	if !ok || inProgress == nil {
		return nil, fmt.Errorf("circular dependency detected: %s", s.systemID)
	}
	return inProgress, nil
}

func (s *loadSession) parseSchema() (result *parser.ParseResult, err error) {
	return parseSchemaDocument(s.doc, s.systemID, s.loader.config.SchemaParseOptions...)
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
	includingNS := s.key.etn
	result, err := s.loadDirectiveSchema(
		parser.DirectiveInclude,
		ResolveRequest{
			BaseSystemID:   s.systemID,
			SchemaLocation: include.SchemaLocation,
			Kind:           ResolveInclude,
		},
		func(systemID string) loadKey {
			return s.loader.loadKey(systemID, includingNS)
		},
		false,
		func(targetKey loadKey) {
			s.deferInclude(targetKey, s.key, include)
		},
	)
	if err != nil {
		return fmt.Errorf("load included schema %s: %w", include.SchemaLocation, err)
	}
	switch result.status {
	case directiveLoadStatusDeferred:
		return nil
	case directiveLoadStatusSkippedMissing:
		return fmt.Errorf("included schema %s not found", include.SchemaLocation)
	}
	includedSchema := result.schema
	includeKey := result.target

	if !s.loader.isIncludeNamespaceCompatible(includingNS, includedSchema.TargetNamespace) {
		if entry, ok := s.loader.state.entry(includeKey); ok && entry != nil {
			s.loader.resetEntry(entry, includeKey)
		}
		return fmt.Errorf("included schema %s has different target namespace: %s != %s",
			include.SchemaLocation, includedSchema.TargetNamespace, includingNS)
	}
	needsNamespaceRemap := !includingNS.IsEmpty() && includedSchema.TargetNamespace.IsEmpty()
	remapMode := keepNamespace
	if needsNamespaceRemap {
		remapMode = remapNamespace
	}
	entry, ok := s.loader.state.entry(s.key)
	if !ok || entry == nil {
		return fmt.Errorf("include tracking missing for %s", s.key.systemID)
	}
	insertAt, err := includeInsertIndex(entry, include, len(schema.GlobalDecls))
	if err != nil {
		return err
	}
	beforeLen := len(schema.GlobalDecls)
	if err := s.loader.mergeSchema(schema, includedSchema, mergeInclude, remapMode, insertAt); err != nil {
		return fmt.Errorf("merge included schema %s: %w", include.SchemaLocation, err)
	}
	inserted := len(schema.GlobalDecls) - beforeLen
	if err := recordIncludeInserted(entry, include.IncludeIndex, inserted); err != nil {
		return err
	}
	s.loader.imports.markMerged(parser.DirectiveInclude, s.key, includeKey)
	s.merged.includes = append(s.merged.includes, mergeRecord{base: s.key, target: includeKey})
	return nil
}

func (s *loadSession) processImport(schema *parser.Schema, imp parser.ImportInfo) error {
	if imp.SchemaLocation == "" {
		if s.loader.config.AllowMissingImportLocations {
			return nil
		}
		return fmt.Errorf("import missing schemaLocation")
	}
	importNS := types.NamespaceURI(imp.Namespace)
	result, err := s.loadDirectiveSchema(
		parser.DirectiveImport,
		ResolveRequest{
			BaseSystemID:   s.systemID,
			SchemaLocation: imp.SchemaLocation,
			ImportNS:       []byte(imp.Namespace),
			Kind:           ResolveImport,
		},
		func(systemID string) loadKey {
			return s.loader.loadKey(systemID, importNS)
		},
		s.loader.config.AllowMissingImportLocations,
		func(targetKey loadKey) {
			s.deferImport(targetKey, s.key, imp.SchemaLocation, imp.Namespace)
		},
	)
	if err != nil {
		return fmt.Errorf("load imported schema %s: %w", imp.SchemaLocation, err)
	}
	switch result.status {
	case directiveLoadStatusDeferred, directiveLoadStatusSkippedMissing:
		return nil
	}
	importedSchema := result.schema
	importKey := result.target

	if imp.Namespace == "" {
		if !importedSchema.TargetNamespace.IsEmpty() {
			if entry, ok := s.loader.state.entry(importKey); ok && entry != nil {
				s.loader.resetEntry(entry, importKey)
			}
			return fmt.Errorf("imported schema %s namespace mismatch: expected no namespace, got %s",
				imp.SchemaLocation, importedSchema.TargetNamespace)
		}
	} else if importedSchema.TargetNamespace != importNS {
		if entry, ok := s.loader.state.entry(importKey); ok && entry != nil {
			s.loader.resetEntry(entry, importKey)
		}
		return fmt.Errorf("imported schema %s namespace mismatch: expected %s, got %s",
			imp.SchemaLocation, imp.Namespace, importedSchema.TargetNamespace)
	}
	if err := s.loader.mergeSchema(schema, importedSchema, mergeImport, keepNamespace, len(schema.GlobalDecls)); err != nil {
		return fmt.Errorf("merge imported schema %s: %w", imp.SchemaLocation, err)
	}
	s.loader.imports.markMerged(parser.DirectiveImport, s.key, importKey)
	s.merged.imports = append(s.merged.imports, mergeRecord{base: s.key, target: importKey})
	return nil
}

func (s *loadSession) loadDirectiveSchema(
	kind parser.DirectiveKind,
	req ResolveRequest,
	keyForSystemID func(systemID string) loadKey,
	allowNotFound bool,
	onLoading func(targetKey loadKey),
) (directiveLoadResult, error) {
	doc, systemID, err := s.loader.resolve(req)
	if err != nil {
		if allowNotFound && isNotFound(err) {
			return directiveLoadResult{status: directiveLoadStatusSkippedMissing}, nil
		}
		return directiveLoadResult{}, err
	}

	targetKey := keyForSystemID(systemID)
	if s.loader.imports.alreadyMerged(kind, s.key, targetKey) {
		if closeErr := closeSchemaDoc(doc, systemID); closeErr != nil {
			return directiveLoadResult{}, closeErr
		}
		return directiveLoadResult{
			target: targetKey,
			status: directiveLoadStatusDeferred,
		}, nil
	}
	if s.loader.state.isLoading(targetKey) {
		if closeErr := closeSchemaDoc(doc, systemID); closeErr != nil {
			return directiveLoadResult{}, closeErr
		}
		if onLoading != nil {
			onLoading(targetKey)
		}
		return directiveLoadResult{
			target: targetKey,
			status: directiveLoadStatusDeferred,
		}, nil
	}

	loadedSchema, err := s.loader.loadResolved(doc, systemID, targetKey, skipSchemaValidation)
	if err != nil {
		return directiveLoadResult{}, err
	}
	return directiveLoadResult{
		schema: loadedSchema,
		target: targetKey,
		status: directiveLoadStatusLoaded,
	}, nil
}

func (s *loadSession) deferImport(sourceKey, targetKey loadKey, schemaLocation, expectedNamespace string) {
	if s.loader.deferImport(sourceKey, targetKey, schemaLocation, expectedNamespace) {
		s.pending = append(s.pending, pendingChange{
			sourceKey: sourceKey,
			targetKey: targetKey,
			kind:      parser.DirectiveImport,
		})
	}
}

func (s *loadSession) deferInclude(sourceKey, targetKey loadKey, include parser.IncludeInfo) {
	if s.loader.deferInclude(sourceKey, targetKey, include) {
		s.pending = append(s.pending, pendingChange{
			sourceKey: sourceKey,
			targetKey: targetKey,
			kind:      parser.DirectiveInclude,
		})
	}
}

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

func parseSchemaDocument(doc io.ReadCloser, systemID string, opts ...xmlstream.Option) (result *parser.ParseResult, err error) {
	if doc == nil {
		return nil, fmt.Errorf("nil schema reader")
	}
	defer func() {
		if closeErr := closeSchemaDoc(doc, systemID); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	result, err = parser.ParseWithImportsOptions(doc, opts...)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", systemID, err)
	}

	return result, nil
}

func closeSchemaDoc(doc io.Closer, systemID string) error {
	if err := doc.Close(); err != nil {
		return fmt.Errorf("close %s: %w", systemID, err)
	}
	return nil
}
