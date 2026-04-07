package compiler

import (
	"github.com/jacoelho/xsd/internal/parser"
)

func (l *Loader) applyPendingInclude(directive Directive[loadKey], source *parser.Schema, target *stagedPendingTarget) error {
	includingNS := directive.TargetKey.etn
	includeInfo := parser.IncludeInfo{
		SchemaLocation: directive.SchemaLocation,
		DeclIndex:      directive.IncludeDeclIndex,
		IncludeIndex:   directive.IncludeIndex,
	}
	plan, err := PlanInclude(includingNS, target.includeInserted, target.schema, includeInfo, directive.SchemaLocation, source)
	if err != nil {
		return err
	}
	inserted, err := ApplyPlanned(target.schema, source, plan, "included", directive.SchemaLocation)
	if err != nil {
		return err
	}
	return RecordIncludeInserted(target.includeInserted, directive.IncludeIndex, inserted)
}

func (l *Loader) applyPendingImport(directive Directive[loadKey], source *parser.Schema, target *stagedPendingTarget) error {
	plan, err := PlanImport(directive.SchemaLocation, directive.ExpectedNamespace, source, len(target.schema.GlobalDecls))
	if err != nil {
		return err
	}
	if _, err := ApplyPlanned(target.schema, source, plan, "imported", directive.SchemaLocation); err != nil {
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
			entry.includeInserted = stagedTarget.includeInserted
		}
	}
	return nil
}

func (l *Loader) markPendingMerged(sourceKey loadKey, pendingDirectives []Directive[loadKey]) {
	for _, directive := range pendingDirectives {
		l.imports.MarkMerged(directive.Kind, directive.TargetKey, sourceKey)
	}
}
