package compiler

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/parser"
)

type stagedPendingTarget struct {
	schema          *parser.Schema
	includeInserted []int
}

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

func (l *Loader) deferDirective(sourceKey loadKey, directive Directive[loadKey], journal *Journal[loadKey]) bool {
	sourceEntry := l.state.ensureEntry(sourceKey)
	if !sourceEntry.pending.Append(directive) {
		return false
	}
	if journal != nil {
		journal.RecordAppendPendingDirective(directive.Kind, sourceKey, directive.TargetKey)
	}

	targetEntry := l.state.ensureEntry(directive.TargetKey)
	targetEntry.pending.Increment()
	if journal != nil {
		journal.RecordIncPendingCount(directive.TargetKey)
	}
	return true
}

func (l *Loader) resolvePendingImportsFor(sourceKey loadKey) error {
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
	sourceEntry.pending.Clear()
	return l.resolvePendingTargets(pendingDirectives)
}

func (l *Loader) resolvePendingTargets(pendingDirectives []Directive[loadKey]) error {
	for _, directive := range pendingDirectives {
		targetEntry := l.state.ensureEntry(directive.TargetKey)
		if err := targetEntry.pending.Decrement(directive.TargetKey.systemID); err != nil {
			return err
		}
		if targetEntry.pending.Count == 0 {
			if err := l.resolvePendingImportsFor(directive.TargetKey); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *Loader) pendingResolutionInputs(sourceKey loadKey) (*schemaEntry, []Directive[loadKey], *parser.Schema, error) {
	sourceEntry := l.state.ensureEntry(sourceKey)
	if sourceEntry.pending.Count > 0 {
		return sourceEntry, nil, nil, nil
	}
	pendingDirectives := sourceEntry.pending.Directives
	if len(pendingDirectives) == 0 {
		return sourceEntry, nil, nil, nil
	}
	source := l.state.schemaForKey(sourceKey)
	if source == nil {
		return nil, nil, nil, fmt.Errorf("pending import source not found: %s", sourceKey.systemID)
	}
	return sourceEntry, pendingDirectives, source, nil
}

func (l *Loader) applyPendingDirectives(
	pendingDirectives []Directive[loadKey],
	source *parser.Schema,
	staged map[loadKey]*stagedPendingTarget,
) error {
	for _, directive := range pendingDirectives {
		target := staged[directive.TargetKey]
		if target == nil {
			return fmt.Errorf("pending directive target not staged: %s", directive.TargetKey.systemID)
		}
		switch directive.Kind {
		case parser.DirectiveInclude:
			if err := l.applyPendingInclude(directive, source, target); err != nil {
				return err
			}
		case parser.DirectiveImport:
			if err := l.applyPendingImport(directive, source, target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown pending directive kind: %d", directive.Kind)
		}
	}
	return nil
}

func (l *Loader) stagePendingTargets(pendingDirectives []Directive[loadKey]) (map[loadKey]*stagedPendingTarget, error) {
	staged := make(map[loadKey]*stagedPendingTarget, len(pendingDirectives))
	for _, directive := range pendingDirectives {
		if _, ok := staged[directive.TargetKey]; ok {
			continue
		}
		target, err := l.schemaForKeyStrict(directive.TargetKey)
		if err != nil {
			return nil, err
		}
		entry, ok := l.state.entry(directive.TargetKey)
		if !ok || entry == nil {
			return nil, fmt.Errorf("pending directive tracking missing for %s", directive.TargetKey.systemID)
		}
		staged[directive.TargetKey] = &stagedPendingTarget{
			schema:          parser.CloneSchemaForMerge(target),
			includeInserted: slices.Clone(entry.includeInserted),
		}
	}
	return staged, nil
}

func (l *Loader) schemaForKeyStrict(key loadKey) (*parser.Schema, error) {
	target := l.state.schemaForKey(key)
	if target == nil {
		return nil, fmt.Errorf("pending directive target not found: %s", key.systemID)
	}
	return target, nil
}
