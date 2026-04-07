package compiler

import "github.com/jacoelho/xsd/internal/parser"

// Tracker records which directive edges have already been merged.
type Tracker[K comparable] struct {
	merged map[parser.DirectiveKind]map[K]map[K]bool
}

// NewTracker returns an initialized directive merge tracker.
func NewTracker[K comparable]() Tracker[K] {
	return Tracker[K]{merged: make(map[parser.DirectiveKind]map[K]map[K]bool)}
}

// AlreadyMerged reports whether one base -> target directive edge was merged.
func (t *Tracker[K]) AlreadyMerged(kind parser.DirectiveKind, baseKey, targetKey K) bool {
	if t == nil || len(t.merged) == 0 {
		return false
	}
	byKind := t.merged[kind]
	if byKind == nil {
		return false
	}
	merged := byKind[baseKey]
	return merged[targetKey]
}

// MarkMerged records one merged base -> target directive edge.
func (t *Tracker[K]) MarkMerged(kind parser.DirectiveKind, baseKey, targetKey K) {
	if t.merged == nil {
		t.merged = make(map[parser.DirectiveKind]map[K]map[K]bool)
	}
	byKind := t.merged[kind]
	if byKind == nil {
		byKind = make(map[K]map[K]bool)
		t.merged[kind] = byKind
	}
	merged := byKind[baseKey]
	if merged == nil {
		merged = make(map[K]bool)
		byKind[baseKey] = merged
	}
	merged[targetKey] = true
}

// UnmarkMerged removes one merged base -> target directive edge.
func (t *Tracker[K]) UnmarkMerged(kind parser.DirectiveKind, baseKey, targetKey K) {
	if t == nil || len(t.merged) == 0 {
		return
	}
	byKind := t.merged[kind]
	if byKind == nil {
		return
	}
	merged := byKind[baseKey]
	if merged == nil {
		return
	}
	delete(merged, targetKey)
	if len(merged) == 0 {
		delete(byKind, baseKey)
	}
	if len(byKind) == 0 {
		delete(t.merged, kind)
	}
}
