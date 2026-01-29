package loader

import (
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

type loadSession struct {
	doc      io.ReadCloser
	loader   *SchemaLoader
	key      loadKey
	systemID string
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
	return parseSchemaDocument(s.doc, s.systemID)
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
	req := ResolveRequest{
		BaseSystemID:   s.systemID,
		SchemaLocation: include.SchemaLocation,
		Kind:           ResolveInclude,
	}
	doc, systemID, err := s.loader.resolve(req)
	if err != nil {
		return err
	}
	includeKey := s.loader.loadKey(systemID, includingNS)
	if s.loader.alreadyMergedInclude(s.key, includeKey) {
		if err := closeSchemaDoc(doc, systemID); err != nil {
			return err
		}
		return nil
	}
	if s.loader.state.isLoading(includeKey) {
		if err := closeSchemaDoc(doc, systemID); err != nil {
			return err
		}
		s.loader.deferInclude(includeKey, s.key, include.SchemaLocation)
		return nil
	}
	includedSchema, err := s.loader.loadResolved(doc, systemID, includeKey, skipSchemaValidation)
	if err != nil {
		return fmt.Errorf("load included schema %s: %w", include.SchemaLocation, err)
	}
	if !s.loader.isIncludeNamespaceCompatible(includingNS, includedSchema.TargetNamespace) {
		return fmt.Errorf("included schema %s has different target namespace: %s != %s",
			include.SchemaLocation, includedSchema.TargetNamespace, includingNS)
	}
	needsNamespaceRemap := !includingNS.IsEmpty() && includedSchema.TargetNamespace.IsEmpty()
	remapMode := keepNamespace
	if needsNamespaceRemap {
		remapMode = remapNamespace
	}
	if err := s.loader.mergeSchema(schema, includedSchema, mergeInclude, remapMode); err != nil {
		return fmt.Errorf("merge included schema %s: %w", include.SchemaLocation, err)
	}
	s.loader.markMergedInclude(s.key, includeKey)
	return nil
}

func (s *loadSession) processImport(schema *parser.Schema, imp parser.ImportInfo) error {
	if imp.SchemaLocation == "" && s.loader.resolver == nil {
		if s.loader.config.AllowMissingImportLocations {
			return nil
		}
		return fmt.Errorf("import missing schemaLocation")
	}
	req := ResolveRequest{
		BaseSystemID:   s.systemID,
		SchemaLocation: imp.SchemaLocation,
		ImportNS:       []byte(imp.Namespace),
		Kind:           ResolveImport,
	}
	doc, systemID, err := s.loader.resolve(req)
	if err != nil {
		if s.loader.config.AllowMissingImportLocations && isNotFound(err) {
			return nil
		}
		return err
	}
	importNS := types.NamespaceURI(imp.Namespace)
	importKey := s.loader.loadKey(systemID, importNS)
	if s.loader.alreadyMergedImport(s.key, importKey) {
		if err := closeSchemaDoc(doc, systemID); err != nil {
			return err
		}
		return nil
	}
	if s.loader.state.isLoading(importKey) {
		if err := closeSchemaDoc(doc, systemID); err != nil {
			return err
		}
		s.loader.deferImport(importKey, s.key, imp.SchemaLocation, imp.Namespace)
		return nil
	}
	importedSchema, err := s.loader.loadResolved(doc, systemID, importKey, skipSchemaValidation)
	if err != nil {
		return fmt.Errorf("load imported schema %s: %w", imp.SchemaLocation, err)
	}
	if imp.Namespace == "" {
		if !importedSchema.TargetNamespace.IsEmpty() {
			return fmt.Errorf("imported schema %s namespace mismatch: expected no namespace, got %s",
				imp.SchemaLocation, importedSchema.TargetNamespace)
		}
	} else if importedSchema.TargetNamespace != importNS {
		return fmt.Errorf("imported schema %s namespace mismatch: expected %s, got %s",
			imp.SchemaLocation, imp.Namespace, importedSchema.TargetNamespace)
	}
	if imp.Namespace == "" && !importedSchema.TargetNamespace.IsEmpty() {
		return fmt.Errorf("imported schema %s must have no target namespace when import namespace is omitted",
			imp.SchemaLocation)
	}
	if err := s.loader.mergeSchema(schema, importedSchema, mergeImport, keepNamespace); err != nil {
		return fmt.Errorf("merge imported schema %s: %w", imp.SchemaLocation, err)
	}
	s.loader.markMergedImport(s.key, importKey)
	return nil
}

func parseSchemaDocument(doc io.ReadCloser, systemID string) (result *parser.ParseResult, err error) {
	if doc == nil {
		return nil, fmt.Errorf("nil schema reader")
	}
	defer func() {
		if closeErr := closeSchemaDoc(doc, systemID); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	result, err = parser.ParseWithImports(doc)
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
