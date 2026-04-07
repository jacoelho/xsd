package compiler

import (
	"github.com/jacoelho/xsd/internal/parser"
)

func (l *Loader) stateRollbackCallbacks() RollbackCallbacks[loadKey] {
	return RollbackCallbacks[loadKey]{
		UnmarkMerged: func(kind parser.DirectiveKind, baseKey, targetKey loadKey) {
			l.imports.UnmarkMerged(kind, baseKey, targetKey)
		},
		RemovePendingDirective: func(kind parser.DirectiveKind, sourceKey, targetKey loadKey) {
			if entry, ok := l.state.entry(sourceKey); ok && entry != nil {
				entry.pending.Remove(kind, targetKey)
			}
		},
		DecPendingCount: func(targetKey loadKey) {
			if entry, ok := l.state.entry(targetKey); ok && entry != nil {
				_ = entry.pending.Decrement(targetKey.systemID)
			}
		},
		CleanupKey: l.cleanupEntryIfUnused,
	}
}

func rollbackSourcePending(loader *Loader, sourceKey loadKey) {
	if loader == nil {
		return
	}
	entry, ok := loader.state.entry(sourceKey)
	if !ok || entry == nil || len(entry.pending.Directives) == 0 {
		return
	}
	for _, directive := range entry.pending.Directives {
		if target, ok := loader.state.entry(directive.TargetKey); ok && target != nil {
			_ = target.pending.Decrement(directive.TargetKey.systemID)
		}
		loader.cleanupEntryIfUnused(directive.TargetKey)
	}
	entry.pending.Clear()
	loader.cleanupEntryIfUnused(sourceKey)
}
