package source

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/parser"
)

func (l *SchemaLoader) applyPendingInclude(directive pendingDirective, source *parser.Schema, target *stagedPendingTarget) error {
	includingNS := directive.targetKey.etn
	if !l.isIncludeNamespaceCompatible(includingNS, source.TargetNamespace) {
		return fmt.Errorf("included schema %s has different target namespace: %s != %s",
			directive.schemaLocation, source.TargetNamespace, includingNS)
	}
	remapMode := loadmerge.KeepNamespace
	if includingNS != "" && source.TargetNamespace == "" {
		remapMode = loadmerge.RemapNamespace
	}
	includeInfo := parser.IncludeInfo{
		SchemaLocation: directive.schemaLocation,
		DeclIndex:      directive.includeDeclIndex,
		IncludeIndex:   directive.includeIndex,
	}
	insertAt, err := includeInsertIndex(target.entry, includeInfo, len(target.schema.GlobalDecls))
	if err != nil {
		return err
	}
	beforeLen := len(target.schema.GlobalDecls)
	if err := l.mergeSchema(target.schema, source, loadmerge.MergeInclude, remapMode, insertAt); err != nil {
		return fmt.Errorf("merge included schema %s: %w", directive.schemaLocation, err)
	}
	inserted := len(target.schema.GlobalDecls) - beforeLen
	return recordIncludeInserted(target.entry, directive.includeIndex, inserted)
}

func (l *SchemaLoader) applyPendingImport(directive pendingDirective, source *parser.Schema, target *stagedPendingTarget) error {
	if directive.expectedNamespace != "" && source.TargetNamespace != directive.expectedNamespace {
		return fmt.Errorf("imported schema %s namespace mismatch: expected %s, got %s",
			directive.schemaLocation, directive.expectedNamespace, source.TargetNamespace)
	}
	if directive.expectedNamespace == "" && source.TargetNamespace != "" {
		return fmt.Errorf("imported schema %s namespace mismatch: expected no namespace, got %s",
			directive.schemaLocation, source.TargetNamespace)
	}
	if err := l.mergeSchema(target.schema, source, loadmerge.MergeImport, loadmerge.KeepNamespace, len(target.schema.GlobalDecls)); err != nil {
		return fmt.Errorf("merge imported schema %s: %w", directive.schemaLocation, err)
	}
	return nil
}

func (l *SchemaLoader) commitStagedTargets(staged map[loadKey]*stagedPendingTarget) error {
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

func (l *SchemaLoader) markPendingMerged(sourceKey loadKey, pendingDirectives []pendingDirective) {
	for _, directive := range pendingDirectives {
		switch directive.kind {
		case parser.DirectiveInclude:
			l.imports.markMerged(parser.DirectiveInclude, directive.targetKey, sourceKey)
		case parser.DirectiveImport:
			l.imports.markMerged(parser.DirectiveImport, directive.targetKey, sourceKey)
		}
	}
}
