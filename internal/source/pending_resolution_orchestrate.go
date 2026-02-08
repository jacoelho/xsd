package source

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
)

func (l *SchemaLoader) resolvePendingImportsFor(sourceKey loadKey) error {
	sourceEntry, pendingDirectives, source, err := l.pendingResolutionInputs(sourceKey)
	if err != nil || len(pendingDirectives) == 0 {
		return err
	}

	staged, err := l.stagePendingTargets(pendingDirectives)
	if err != nil {
		return err
	}
	if err := l.applyPendingDirectives(pendingDirectives, source, staged); err != nil {
		return err
	}
	if err := l.commitStagedTargets(staged); err != nil {
		return err
	}
	l.markPendingMerged(sourceKey, pendingDirectives)
	sourceEntry.pendingDirectives = nil
	return l.resolvePendingTargets(pendingDirectives)
}

func (l *SchemaLoader) applyPendingDirectives(
	pendingDirectives []pendingDirective,
	source *parser.Schema,
	staged map[loadKey]*stagedPendingTarget,
) error {
	for _, directive := range pendingDirectives {
		target := staged[directive.targetKey]
		if target == nil {
			return fmt.Errorf("pending directive target not staged: %s", directive.targetKey.systemID)
		}
		if err := l.applyPendingDirective(directive, source, target); err != nil {
			return err
		}
	}
	return nil
}

func (l *SchemaLoader) applyPendingDirective(directive pendingDirective, source *parser.Schema, target *stagedPendingTarget) error {
	switch directive.kind {
	case parser.DirectiveInclude:
		return l.applyPendingInclude(directive, source, target)
	case parser.DirectiveImport:
		return l.applyPendingImport(directive, source, target)
	default:
		return fmt.Errorf("unknown pending directive kind: %d", directive.kind)
	}
}
