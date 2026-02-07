package traversal

// VisitTracker tracks visited values during recursive traversal.
type VisitTracker[T comparable] struct {
	seen map[T]struct{}
}

// NewVisitTracker creates a tracker with initialized storage.
func NewVisitTracker[T comparable]() VisitTracker[T] {
	return VisitTracker[T]{seen: make(map[T]struct{})}
}

// Enter marks value as visited and reports whether it was new.
func (v *VisitTracker[T]) Enter(value T) bool {
	if v == nil {
		return false
	}
	if v.seen == nil {
		v.seen = make(map[T]struct{})
	}
	if _, ok := v.seen[value]; ok {
		return false
	}
	v.seen[value] = struct{}{}
	return true
}

// Seen reports whether value was already visited.
func (v *VisitTracker[T]) Seen(value T) bool {
	if v == nil || v.seen == nil {
		return false
	}
	_, ok := v.seen[value]
	return ok
}
