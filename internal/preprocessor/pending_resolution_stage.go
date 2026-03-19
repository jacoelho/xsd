package preprocessor

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/parser"
)

type stagedPendingTarget struct {
	schema          *parser.Schema
	includeInserted []int
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
