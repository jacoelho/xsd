package preprocessor

import "github.com/jacoelho/xsd/internal/parser"

func (l *Loader) applyPendingInclude(directive pendingDirective, source *parser.Schema, target *stagedPendingTarget) error {
	includingNS := directive.targetKey.etn
	includeInfo := parser.IncludeInfo{
		SchemaLocation: directive.schemaLocation,
		DeclIndex:      directive.includeDeclIndex,
		IncludeIndex:   directive.includeIndex,
	}
	plan, err := l.planIncludeMerge(includingNS, target.entry, target.schema, includeInfo, directive.schemaLocation, source)
	if err != nil {
		return err
	}
	inserted, err := l.applyDirectiveMerge(target.schema, source, plan, "included", directive.schemaLocation)
	if err != nil {
		return err
	}
	return recordIncludeInserted(target.entry, directive.includeIndex, inserted)
}

func (l *Loader) applyPendingImport(directive pendingDirective, source *parser.Schema, target *stagedPendingTarget) error {
	plan, err := l.planImportMerge(directive.schemaLocation, directive.expectedNamespace, source, len(target.schema.GlobalDecls))
	if err != nil {
		return err
	}
	if _, err := l.applyDirectiveMerge(target.schema, source, plan, "imported", directive.schemaLocation); err != nil {
		return err
	}
	return nil
}

func (l *Loader) commitStagedTargets(staged map[loadKey]*stagedPendingTarget) error {
	for key, stagedTarget := range staged {
		target, err := l.schemaForKeyStrict(key)
		if err != nil {
			return err
		}
		*target = *stagedTarget.schema
		if entry, ok := l.state.entry(key); ok && entry != nil {
			entry.includeInserted = stagedTarget.entry.includeInserted
		}
	}
	return nil
}

func (l *Loader) markPendingMerged(sourceKey loadKey, pendingDirectives []pendingDirective) {
	for _, directive := range pendingDirectives {
		l.imports.markMerged(directive.kind, directive.targetKey, sourceKey)
	}
}
