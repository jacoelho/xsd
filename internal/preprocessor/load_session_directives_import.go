package preprocessor

import (
	"io"

	"github.com/jacoelho/xsd/internal/parser"
)

func (s *loadSession) processImport(schema *parser.Schema, imp parser.ImportInfo) error {
	return ProcessImport(ImportConfig[loadKey]{
		Info:                 imp,
		AllowMissingLocation: s.loader.config.AllowMissingImportLocations,
		Load: func(info parser.ImportInfo) (LoadResult[loadKey], error) {
			return Load(LoadConfig[loadKey]{
				AllowMissing: s.loader.config.AllowMissingImportLocations,
				Resolve: func() (io.ReadCloser, string, error) {
					return s.loader.resolver.Resolve(ResolveRequest{
						BaseSystemID:   s.systemID,
						SchemaLocation: info.SchemaLocation,
						ImportNS:       []byte(info.Namespace),
						Kind:           ResolveImport,
					})
				},
				IsNotFound: isNotFound,
				Key: func(systemID string) loadKey {
					return s.loader.loadKey(systemID, info.Namespace)
				},
				AlreadyMerged: func(targetKey loadKey) bool {
					return s.loader.imports.AlreadyMerged(parser.DirectiveImport, s.key, targetKey)
				},
				IsLoading: s.loader.state.IsLoading,
				OnLoading: func(targetKey loadKey) {
					_ = s.loader.deferDirective(targetKey, Directive[loadKey]{
						Kind:              parser.DirectiveImport,
						TargetKey:         s.key,
						SchemaLocation:    info.SchemaLocation,
						ExpectedNamespace: info.Namespace,
					}, &s.journal)
				},
				Load: func(doc io.ReadCloser, systemID string, targetKey loadKey) (*parser.Schema, error) {
					return s.loader.loadResolvedWithJournal(doc, systemID, targetKey, &s.journal)
				},
				Close: Close,
			})
		},
		Merge: func(importedSchema *parser.Schema, importKey loadKey) error {
			plan, err := PlanImport(imp.SchemaLocation, imp.Namespace, importedSchema, len(schema.GlobalDecls))
			if err != nil {
				if entry, ok := s.loader.state.entry(importKey); ok && entry != nil {
					s.loader.resetEntry(entry, importKey)
				}
				return err
			}
			if _, err := ApplyPlanned(schema, importedSchema, plan, "imported", imp.SchemaLocation); err != nil {
				return err
			}
			s.markDirectiveMerged(parser.DirectiveImport, s.key, importKey)
			return nil
		},
	})
}
