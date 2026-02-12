package preprocessor

import (
	"fmt"

	parser "github.com/jacoelho/xsd/internal/parser"
)

func (s *loadSession) processImport(schema *parser.Schema, imp parser.ImportInfo) error {
	if imp.SchemaLocation == "" {
		if s.loader.config.AllowMissingImportLocations {
			return nil
		}
		return fmt.Errorf("import missing schemaLocation")
	}
	importNS := imp.Namespace
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

	plan, err := s.loader.planImportMerge(imp.SchemaLocation, imp.Namespace, importedSchema, len(schema.GlobalDecls))
	if err != nil {
		s.resetTrackedEntry(importKey)
		return err
	}
	if _, err := s.loader.applyDirectiveMerge(schema, importedSchema, plan, "imported", imp.SchemaLocation); err != nil {
		return err
	}
	s.markDirectiveMerged(parser.DirectiveImport, s.key, importKey)
	return nil
}
