package source

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

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
			s.resetTrackedEntry(importKey)
			return fmt.Errorf("imported schema %s namespace mismatch: expected no namespace, got %s",
				imp.SchemaLocation, importedSchema.TargetNamespace)
		}
	} else if importedSchema.TargetNamespace != importNS {
		s.resetTrackedEntry(importKey)
		return fmt.Errorf("imported schema %s namespace mismatch: expected %s, got %s",
			imp.SchemaLocation, imp.Namespace, importedSchema.TargetNamespace)
	}
	if err := s.loader.mergeSchema(schema, importedSchema, loadmerge.MergeImport, loadmerge.KeepNamespace, len(schema.GlobalDecls)); err != nil {
		return fmt.Errorf("merge imported schema %s: %w", imp.SchemaLocation, err)
	}
	s.loader.imports.markMerged(parser.DirectiveImport, s.key, importKey)
	s.merged.imports = append(s.merged.imports, mergeRecord{base: s.key, target: importKey})
	return nil
}
