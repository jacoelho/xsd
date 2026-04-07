package compiler

import "github.com/jacoelho/xsd/internal/parser"

type journalOpKind uint8

const (
	journalOpUnmarkMerged journalOpKind = iota
	journalOpRemovePendingDirective
	journalOpDecPendingCount
)

type journalOp[K comparable] struct {
	sourceKey     K
	targetKey     K
	kind          journalOpKind
	directiveKind parser.DirectiveKind
}

// Journal records rollback operations for directive processing.
type Journal[K comparable] struct {
	ops []journalOp[K]
}

// RollbackCallbacks describes root-owned rollback side effects.
type RollbackCallbacks[K comparable] struct {
	UnmarkMerged           func(parser.DirectiveKind, K, K)
	RemovePendingDirective func(parser.DirectiveKind, K, K)
	DecPendingCount        func(K)
	CleanupKey             func(K)
}

// Append appends one journal's operations onto another.
func (j *Journal[K]) Append(other *Journal[K]) {
	if j == nil || other == nil || len(other.ops) == 0 {
		return
	}
	j.ops = append(j.ops, other.ops...)
}

// RecordMarkMerged records the inverse of one merge marker update.
func (j *Journal[K]) RecordMarkMerged(kind parser.DirectiveKind, baseKey, targetKey K) {
	j.ops = append(j.ops, journalOp[K]{
		kind:          journalOpUnmarkMerged,
		directiveKind: kind,
		sourceKey:     baseKey,
		targetKey:     targetKey,
	})
}

// RecordAppendPendingDirective records the inverse of one pending directive append.
func (j *Journal[K]) RecordAppendPendingDirective(kind parser.DirectiveKind, sourceKey, targetKey K) {
	j.ops = append(j.ops, journalOp[K]{
		kind:          journalOpRemovePendingDirective,
		directiveKind: kind,
		sourceKey:     sourceKey,
		targetKey:     targetKey,
	})
}

// RecordIncPendingCount records the inverse of one pending-count increment.
func (j *Journal[K]) RecordIncPendingCount(targetKey K) {
	j.ops = append(j.ops, journalOp[K]{
		kind:      journalOpDecPendingCount,
		sourceKey: targetKey,
	})
}

// Rollback replays the journal in reverse using root-owned callbacks.
func (j *Journal[K]) Rollback(cb RollbackCallbacks[K]) {
	if j == nil || len(j.ops) == 0 {
		return
	}
	for i := len(j.ops) - 1; i >= 0; i-- {
		op := j.ops[i]
		switch op.kind {
		case journalOpUnmarkMerged:
			if cb.UnmarkMerged != nil {
				cb.UnmarkMerged(op.directiveKind, op.sourceKey, op.targetKey)
			}
		case journalOpRemovePendingDirective:
			if cb.RemovePendingDirective != nil {
				cb.RemovePendingDirective(op.directiveKind, op.sourceKey, op.targetKey)
			}
			if cb.CleanupKey != nil {
				cb.CleanupKey(op.sourceKey)
			}
		case journalOpDecPendingCount:
			if cb.DecPendingCount != nil {
				cb.DecPendingCount(op.sourceKey)
			}
			if cb.CleanupKey != nil {
				cb.CleanupKey(op.sourceKey)
			}
		}
	}
}
