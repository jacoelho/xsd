package compiler

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/schemaast"
)

// LoadDocuments loads schema documents without constructing the legacy schema graph.
func (l *Loader) LoadDocuments(location string) (*schemaast.DocumentSet, error) {
	if err := l.beginLocationLoad(); err != nil {
		return nil, err
	}
	if l == nil || l.resolver == nil {
		return nil, fmt.Errorf("no resolver configured")
	}
	state := documentLoadState{
		loader: l,
		seen:   make(map[loadKey]bool),
		active: make(map[loadKey]bool),
	}
	if err := state.load("", location, ResolveInclude, schemaast.NamespaceEmpty); err != nil {
		return nil, err
	}
	return &schemaast.DocumentSet{Documents: state.documents}, nil
}

type documentLoadState struct {
	loader    *Loader
	seen      map[loadKey]bool
	active    map[loadKey]bool
	documents []schemaast.SchemaDocument
}

func (s *documentLoadState) load(baseSystemID, location string, kind ResolveKind, expectedNS schemaast.NamespaceURI) (err error) {
	req := ResolveRequest{
		BaseSystemID:   baseSystemID,
		SchemaLocation: location,
		Kind:           kind,
	}
	if kind == ResolveImport {
		req.ImportNS = []byte(expectedNS)
	}
	doc, systemID, err := s.loader.resolver.Resolve(req)
	if err != nil {
		if kind == ResolveImport && s.loader.config.AllowMissingImportLocations && isNotFound(err) {
			return nil
		}
		return err
	}
	if doc == nil {
		return fmt.Errorf("resolve %s: document is nil", systemID)
	}
	defer func() {
		if closeErr := doc.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close %s: %w", systemID, closeErr)
		}
	}()

	result, err := schemaast.ParseDocumentWithImportsOptionsWithPool(doc, s.loader.config.DocumentPool, s.loader.config.SchemaParseOptions...)
	if err != nil {
		return err
	}
	if result.Document == nil {
		return fmt.Errorf("parse %s: document is nil", systemID)
	}
	parsed := *result.Document
	parsed.Location = systemID
	if baseSystemID != "" {
		if err := validateDocumentNamespace(kind, expectedNS, &parsed); err != nil {
			return err
		}
	}
	if err := validateDocumentImports(&parsed); err != nil {
		return err
	}

	key := s.loader.loadKey(systemID, parsed.TargetNamespace)
	if s.seen[key] {
		return nil
	}
	if s.active[key] {
		return nil
	}
	s.active[key] = true
	defer delete(s.active, key)

	for _, directive := range parsed.Directives {
		if directive.Kind != schemaast.DirectiveInclude {
			continue
		}
		target := directive.Include.SchemaLocation
		if target == "" {
			return fmt.Errorf("include missing schemaLocation")
		}
		if err := s.load(systemID, target, ResolveInclude, parsed.TargetNamespace); err != nil {
			return fmt.Errorf("load included schema %s: %w", target, err)
		}
	}
	s.documents = append(s.documents, parsed)
	for _, directive := range parsed.Directives {
		switch directive.Kind {
		case schemaast.DirectiveInclude:
			continue
		case schemaast.DirectiveImport:
			target := directive.Import.SchemaLocation
			if target == "" {
				if s.loader.config.AllowMissingImportLocations {
					continue
				}
				return errors.New("import missing schemaLocation")
			}
			if err := s.load(systemID, target, ResolveImport, directive.Import.Namespace); err != nil {
				return fmt.Errorf("load imported schema %s: %w", target, err)
			}
		default:
			return fmt.Errorf("unexpected directive kind: %d", directive.Kind)
		}
	}
	s.seen[key] = true
	return nil
}

func validateDocumentImports(doc *schemaast.SchemaDocument) error {
	if doc == nil {
		return nil
	}
	if doc.TargetNamespace == schemaast.NamespaceEmpty {
		for _, imp := range doc.Imports {
			if imp.Namespace == schemaast.NamespaceEmpty {
				return fmt.Errorf("schema without targetNamespace cannot use import without namespace attribute (namespace attribute is required)")
			}
		}
	}
	for _, imp := range doc.Imports {
		if imp.Namespace == schemaast.NamespaceEmpty {
			continue
		}
		if doc.TargetNamespace != schemaast.NamespaceEmpty && imp.Namespace == doc.TargetNamespace {
			return fmt.Errorf("import namespace %s must be different from target namespace", imp.Namespace)
		}
	}
	return nil
}

func validateDocumentNamespace(kind ResolveKind, expectedNS schemaast.NamespaceURI, doc *schemaast.SchemaDocument) error {
	if doc == nil {
		return fmt.Errorf("schema document is nil")
	}
	switch kind {
	case ResolveInclude:
		if expectedNS != schemaast.NamespaceEmpty && doc.TargetNamespace == schemaast.NamespaceEmpty {
			schemaast.RemapChameleonDocument(doc, expectedNS)
			return nil
		}
		if doc.TargetNamespace != expectedNS {
			return fmt.Errorf("included schema namespace %q does not match %q", doc.TargetNamespace, expectedNS)
		}
	case ResolveImport:
		if doc.TargetNamespace != expectedNS {
			return fmt.Errorf("imported schema namespace %q does not match %q", doc.TargetNamespace, expectedNS)
		}
	}
	return nil
}
