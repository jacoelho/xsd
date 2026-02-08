package source

import "github.com/jacoelho/xsd/internal/parser"

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

	loadedSchema, err := s.loader.loadResolved(doc, systemID, targetKey)
	if err != nil {
		return directiveLoadResult{}, err
	}
	return directiveLoadResult{
		schema: loadedSchema,
		target: targetKey,
		status: directiveLoadStatusLoaded,
	}, nil
}

func (s *loadSession) resetTrackedEntry(key loadKey) {
	entry, ok := s.loader.state.entry(key)
	if !ok || entry == nil {
		return
	}
	s.loader.resetEntry(entry, key)
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
