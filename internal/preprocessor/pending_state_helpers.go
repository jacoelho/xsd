package preprocessor

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
)

func appendPendingDirective(entry *schemaEntry, directive pendingDirective) bool {
	if entry == nil {
		return false
	}
	for _, existing := range entry.pendingDirectives {
		if existing.kind == directive.kind && existing.targetKey == directive.targetKey {
			return false
		}
	}
	entry.pendingDirectives = append(entry.pendingDirectives, directive)
	return true
}

func clearPendingDirectives(entry *schemaEntry) {
	if entry == nil {
		return
	}
	entry.pendingDirectives = nil
}

func resetPendingTracking(entry *schemaEntry) {
	if entry == nil {
		return
	}
	clearPendingDirectives(entry)
	entry.pendingCount = 0
}

func incPendingCount(entry *schemaEntry) {
	if entry == nil {
		return
	}
	entry.pendingCount++
}

func decPendingCount(entry *schemaEntry, key loadKey) error {
	if entry == nil {
		return fmt.Errorf("pending directive tracking missing for %s", key.systemID)
	}
	if entry.pendingCount == 0 {
		return fmt.Errorf("pending directive count underflow for %s", key.systemID)
	}
	entry.pendingCount--
	return nil
}

func removePendingDirective(directives []pendingDirective, kind parser.DirectiveKind, targetKey loadKey) []pendingDirective {
	for i, entry := range directives {
		if entry.kind == kind && entry.targetKey == targetKey {
			return append(directives[:i], directives[i+1:]...)
		}
	}
	return directives
}
