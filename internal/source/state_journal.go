package source

import "github.com/jacoelho/xsd/internal/parser"

type stateJournalOpKind uint8

const (
	stateJournalOpUnmarkMerged stateJournalOpKind = iota
	stateJournalOpRemovePendingDirective
	stateJournalOpDecPendingCount
)

type stateJournalOp struct {
	sourceKey     loadKey
	targetKey     loadKey
	kind          stateJournalOpKind
	directiveKind parser.DirectiveKind
}

type stateJournal struct {
	ops []stateJournalOp
}

func (j *stateJournal) append(other *stateJournal) {
	if j == nil || other == nil || len(other.ops) == 0 {
		return
	}
	j.ops = append(j.ops, other.ops...)
}

func (j *stateJournal) recordMarkMerged(kind parser.DirectiveKind, baseKey, targetKey loadKey) {
	j.ops = append(j.ops, stateJournalOp{
		kind:          stateJournalOpUnmarkMerged,
		directiveKind: kind,
		sourceKey:     baseKey,
		targetKey:     targetKey,
	})
}

func (j *stateJournal) recordAppendPendingDirective(kind parser.DirectiveKind, sourceKey, targetKey loadKey) {
	j.ops = append(j.ops, stateJournalOp{
		kind:          stateJournalOpRemovePendingDirective,
		directiveKind: kind,
		sourceKey:     sourceKey,
		targetKey:     targetKey,
	})
}

func (j *stateJournal) recordIncPendingCount(targetKey loadKey) {
	j.ops = append(j.ops, stateJournalOp{
		kind:      stateJournalOpDecPendingCount,
		sourceKey: targetKey,
	})
}

func (j *stateJournal) rollback(loader *SchemaLoader) {
	if loader == nil {
		return
	}
	for i := len(j.ops) - 1; i >= 0; i-- {
		op := j.ops[i]
		switch op.kind {
		case stateJournalOpUnmarkMerged:
			loader.imports.unmarkMerged(op.directiveKind, op.sourceKey, op.targetKey)
		case stateJournalOpRemovePendingDirective:
			if entry, ok := loader.state.entry(op.sourceKey); ok && entry != nil {
				entry.pendingDirectives = removePendingDirective(entry.pendingDirectives, op.directiveKind, op.targetKey)
			}
			loader.cleanupEntryIfUnused(op.sourceKey)
		case stateJournalOpDecPendingCount:
			if entry, ok := loader.state.entry(op.sourceKey); ok && entry != nil {
				_ = decPendingCount(entry, op.sourceKey)
			}
			loader.cleanupEntryIfUnused(op.sourceKey)
		}
	}
}

func rollbackSourcePending(loader *SchemaLoader, sourceKey loadKey) {
	if loader == nil {
		return
	}
	entry, ok := loader.state.entry(sourceKey)
	if !ok || entry == nil || len(entry.pendingDirectives) == 0 {
		return
	}
	for _, pending := range entry.pendingDirectives {
		if target, ok := loader.state.entry(pending.targetKey); ok && target != nil {
			_ = decPendingCount(target, pending.targetKey)
		}
		loader.cleanupEntryIfUnused(pending.targetKey)
	}
	clearPendingDirectives(entry)
	loader.cleanupEntryIfUnused(sourceKey)
}
