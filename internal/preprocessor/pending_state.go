package preprocessor

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
)

// Directive records one deferred include or import against a target key.
type Directive[K comparable] struct {
	TargetKey         K
	SchemaLocation    string
	ExpectedNamespace string
	IncludeDeclIndex  int
	IncludeIndex      int
	Kind              parser.DirectiveKind
}

// Tracking stores deferred directives plus the number of unresolved inbound
// dependencies blocking a schema from being merged.
type Tracking[K comparable] struct {
	Directives []Directive[K]
	Count      int
}

// Append records one deferred directive when it is not already present.
func (t *Tracking[K]) Append(directive Directive[K]) bool {
	if t == nil {
		return false
	}
	for _, existing := range t.Directives {
		if existing.Kind == directive.Kind && existing.TargetKey == directive.TargetKey {
			return false
		}
	}
	t.Directives = append(t.Directives, directive)
	return true
}

// Clear drops all deferred directives while preserving the unresolved counter.
func (t *Tracking[K]) Clear() {
	if t == nil {
		return
	}
	t.Directives = nil
}

// Reset clears all deferred directives and unresolved counters.
func (t *Tracking[K]) Reset() {
	if t == nil {
		return
	}
	t.Directives = nil
	t.Count = 0
}

// Remove deletes one matching deferred directive when present.
func (t *Tracking[K]) Remove(kind parser.DirectiveKind, targetKey K) {
	if t == nil {
		return
	}
	for i, entry := range t.Directives {
		if entry.Kind == kind && entry.TargetKey == targetKey {
			t.Directives = append(t.Directives[:i], t.Directives[i+1:]...)
			return
		}
	}
}

// Increment records one unresolved inbound dependency.
func (t *Tracking[K]) Increment() {
	if t == nil {
		return
	}
	t.Count++
}

// Decrement resolves one unresolved inbound dependency.
func (t *Tracking[K]) Decrement(label string) error {
	if t == nil {
		return fmt.Errorf("pending directive tracking missing for %s", label)
	}
	if t.Count == 0 {
		return fmt.Errorf("pending directive count underflow for %s", label)
	}
	t.Count--
	return nil
}
