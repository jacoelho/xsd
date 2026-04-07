package preprocessor

import (
	"fmt"
	"io"

	"github.com/jacoelho/xsd/internal/parser"
)

func (s *loadSession) processInclude(schema *parser.Schema, include parser.IncludeInfo) error {
	return ProcessInclude(IncludeConfig[loadKey]{
		Info: include,
		Load: func(info parser.IncludeInfo) (LoadResult[loadKey], error) {
			return Load(LoadConfig[loadKey]{
				Resolve: func() (io.ReadCloser, string, error) {
					return s.loader.resolver.Resolve(ResolveRequest{
						BaseSystemID:   s.systemID,
						SchemaLocation: info.SchemaLocation,
						Kind:           ResolveInclude,
					})
				},
				Key: func(systemID string) loadKey {
					return s.loader.loadKey(systemID, s.key.etn)
				},
				AlreadyMerged: func(targetKey loadKey) bool {
					return s.loader.imports.AlreadyMerged(parser.DirectiveInclude, s.key, targetKey)
				},
				IsLoading: s.loader.state.IsLoading,
				OnLoading: func(targetKey loadKey) {
					_ = s.loader.deferDirective(targetKey, Directive[loadKey]{
						Kind:             parser.DirectiveInclude,
						TargetKey:        s.key,
						SchemaLocation:   info.SchemaLocation,
						IncludeDeclIndex: info.DeclIndex,
						IncludeIndex:     info.IncludeIndex,
					}, &s.journal)
				},
				Load: func(doc io.ReadCloser, systemID string, targetKey loadKey) (*parser.Schema, error) {
					return s.loader.loadResolvedWithJournal(doc, systemID, targetKey, &s.journal)
				},
				Close: Close,
			})
		},
		Merge: func(includedSchema *parser.Schema, includeKey loadKey) error {
			entry, ok := s.loader.state.entry(s.key)
			if !ok || entry == nil {
				return fmt.Errorf("include tracking missing for %s", s.key.systemID)
			}
			plan, err := PlanInclude(s.key.etn, entry.includeInserted, schema, include, include.SchemaLocation, includedSchema)
			if err != nil {
				if tracked, ok := s.loader.state.entry(includeKey); ok && tracked != nil {
					s.loader.resetEntry(tracked, includeKey)
				}
				return err
			}
			inserted, err := ApplyPlanned(schema, includedSchema, plan, "included", include.SchemaLocation)
			if err != nil {
				return err
			}
			if err := RecordIncludeInserted(entry.includeInserted, include.IncludeIndex, inserted); err != nil {
				return err
			}
			s.markDirectiveMerged(parser.DirectiveInclude, s.key, includeKey)
			return nil
		},
	})
}
