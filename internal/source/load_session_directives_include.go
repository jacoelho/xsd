package source

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/parser"
)

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
		s.resetTrackedEntry(includeKey)
		return fmt.Errorf("included schema %s has different target namespace: %s != %s",
			include.SchemaLocation, includedSchema.TargetNamespace, includingNS)
	}
	needsNamespaceRemap := !includingNS.IsEmpty() && includedSchema.TargetNamespace.IsEmpty()
	remapMode := loadmerge.KeepNamespace
	if needsNamespaceRemap {
		remapMode = loadmerge.RemapNamespace
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
	if err := s.loader.mergeSchema(schema, includedSchema, loadmerge.MergeInclude, remapMode, insertAt); err != nil {
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
