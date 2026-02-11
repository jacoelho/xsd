package source

import (
	"fmt"

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

	entry, ok := s.loader.state.entry(s.key)
	if !ok || entry == nil {
		return fmt.Errorf("include tracking missing for %s", s.key.systemID)
	}
	plan, err := s.loader.planIncludeMerge(includingNS, entry, schema, include, include.SchemaLocation, includedSchema)
	if err != nil {
		s.resetTrackedEntry(includeKey)
		return err
	}
	inserted, err := s.loader.applyDirectiveMerge(schema, includedSchema, plan, "included", include.SchemaLocation)
	if err != nil {
		return err
	}
	if err := recordIncludeInserted(entry, include.IncludeIndex, inserted); err != nil {
		return err
	}
	s.markDirectiveMerged(parser.DirectiveInclude, s.key, includeKey)
	return nil
}
