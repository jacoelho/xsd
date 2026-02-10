package loadguard

import "fmt"

// EntryLifecycle tracks init/finalize/reset callbacks for a load-graph entry.
type EntryLifecycle[E any] struct {
	Enter    func() (*E, func())
	Init     func(entry *E) error
	Finalize func(entry *E)
	Reset    func(entry *E)
}

// Begin enters loading state and returns an entry with its cleanup callback.
func (l EntryLifecycle[E]) Begin() (*E, func(), error) {
	if l.Enter == nil {
		return nil, nil, fmt.Errorf("entry lifecycle begin callback is nil")
	}
	entry, cleanup := l.Enter()
	if cleanup == nil {
		cleanup = func() {}
	}
	return entry, cleanup, nil
}

// Commit marks the entry as fully loaded.
func (l EntryLifecycle[E]) Commit(entry *E) {
	if l.Finalize == nil {
		return
	}
	l.Finalize(entry)
}

// Rollback resets entry state after a failed load.
func (l EntryLifecycle[E]) Rollback(entry *E) {
	if l.Reset == nil {
		return
	}
	l.Reset(entry)
}
