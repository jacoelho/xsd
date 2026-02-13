package preprocessor

import "github.com/jacoelho/xsd/internal/parser"

type importTracker struct {
	merged map[parser.DirectiveKind]map[loadKey]map[loadKey]bool
}

func newImportTracker() importTracker {
	return importTracker{merged: make(map[parser.DirectiveKind]map[loadKey]map[loadKey]bool)}
}

func (t *importTracker) alreadyMerged(kind parser.DirectiveKind, baseKey, targetKey loadKey) bool {
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

func (t *importTracker) markMerged(kind parser.DirectiveKind, baseKey, targetKey loadKey) {
	if t.merged == nil {
		t.merged = make(map[parser.DirectiveKind]map[loadKey]map[loadKey]bool)
	}
	byKind := t.merged[kind]
	if byKind == nil {
		byKind = make(map[loadKey]map[loadKey]bool)
		t.merged[kind] = byKind
	}
	merged := byKind[baseKey]
	if merged == nil {
		merged = make(map[loadKey]bool)
		byKind[baseKey] = merged
	}
	merged[targetKey] = true
}

func (t *importTracker) unmarkMerged(kind parser.DirectiveKind, baseKey, targetKey loadKey) {
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
