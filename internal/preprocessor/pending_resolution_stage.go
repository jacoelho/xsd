package preprocessor

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/parser"
)

type stagedPendingTarget struct {
	schema *parser.Schema
	entry  *schemaEntry
}

func (l *Loader) pendingResolutionInputs(sourceKey loadKey) (*schemaEntry, []pendingDirective, *parser.Schema, error) {
	sourceEntry := l.state.ensureEntry(sourceKey)
	if sourceEntry.pendingCount > 0 {
		return sourceEntry, nil, nil, nil
	}
	pendingDirectives := sourceEntry.pendingDirectives
	if len(pendingDirectives) == 0 {
		return sourceEntry, nil, nil, nil
	}
	source := l.state.schemaForKey(sourceKey)
	if source == nil {
		return nil, nil, nil, fmt.Errorf("pending import source not found: %s", sourceKey.systemID)
	}
	return sourceEntry, pendingDirectives, source, nil
}

func (l *Loader) stagePendingTargets(pendingDirectives []pendingDirective) (map[loadKey]*stagedPendingTarget, error) {
	staged := make(map[loadKey]*stagedPendingTarget, len(pendingDirectives))
	for _, directive := range pendingDirectives {
		if _, ok := staged[directive.targetKey]; ok {
			continue
		}
		target, err := l.schemaForKeyStrict(directive.targetKey)
		if err != nil {
			return nil, err
		}
		entry, ok := l.state.entry(directive.targetKey)
		if !ok || entry == nil {
			return nil, fmt.Errorf("pending directive tracking missing for %s", directive.targetKey.systemID)
		}
		stagedEntry := &schemaEntry{}
		if len(entry.includeInserted) > 0 {
			stagedEntry.includeInserted = slices.Clone(entry.includeInserted)
		}
		staged[directive.targetKey] = &stagedPendingTarget{
			schema: loadmerge.CloneSchemaForMerge(target),
			entry:  stagedEntry,
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
