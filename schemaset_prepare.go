package xsd

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/preprocessor"
	internalset "github.com/jacoelho/xsd/internal/set"
	"github.com/jacoelho/xsd/internal/xmltree"
)

func (s *SchemaSet) prepareResolved(load resolvedLoadOptions) (*internalset.PreparedSchema, error) {
	if len(s.entries) == 1 {
		entry := s.entries[0]
		return internalset.Prepare(internalset.PrepareConfig{
			FS:                          entry.fsys,
			Location:                    entry.location,
			AllowMissingImportLocations: load.allowMissingImportLocations,
			SchemaParseOptions:          load.schemaLimits.options(),
		})
	}
	merged, err := s.loadAndMergeAll(load)
	if err != nil {
		return nil, err
	}
	return internalset.PrepareParsedOwned(merged)
}

func (s *SchemaSet) loadAndMergeAll(load resolvedLoadOptions) (*parser.Schema, error) {
	var merged *parser.Schema
	merger := loadmerge.DefaultMerger{}
	insertAt := 0

	for i, entry := range s.entries {
		loader := preprocessor.NewLoader(preprocessor.Config{
			FS:                          entry.fsys,
			AllowMissingImportLocations: load.allowMissingImportLocations,
			SchemaParseOptions:          load.schemaLimits.options(),
			DocumentPool:                xmltree.NewDocumentPool(),
		})
		parsed, err := loader.Load(entry.location)
		if err != nil {
			return nil, fmt.Errorf("load parsed schema %s: %w", entry.location, err)
		}
		if i == 0 {
			merged = parsed
			insertAt = len(merged.GlobalDecls)
			continue
		}

		kind := loadmerge.MergeImport
		if parsed.TargetNamespace == merged.TargetNamespace {
			kind = loadmerge.MergeInclude
		}
		if err := merger.Merge(merged, parsed, kind, loadmerge.KeepNamespace, insertAt); err != nil {
			return nil, fmt.Errorf("merge schema %s: %w", entry.location, err)
		}
		insertAt = len(merged.GlobalDecls)
	}
	if merged == nil {
		return nil, fmt.Errorf("no schema roots loaded")
	}
	return merged, nil
}
