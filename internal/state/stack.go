package state

// StateStack is a reusable LIFO stack.
type StateStack[T any] struct {
	items []T
}

// NewStateStack creates a stack with an optional capacity hint.
func NewStateStack[T any](capacity int) StateStack[T] {
	if capacity <= 0 {
		return StateStack[T]{}
	}
	return StateStack[T]{items: make([]T, 0, capacity)}
}

// Push adds one value to the stack top.
func (s *StateStack[T]) Push(value T) {
	s.items = append(s.items, value)
}

// Pop removes and returns the top value.
func (s *StateStack[T]) Pop() (T, bool) {
	var zero T
	if s == nil || len(s.items) == 0 {
		return zero, false
	}
	last := len(s.items) - 1
	value := s.items[last]
	s.items = s.items[:last]
	return value, true
}

// Peek returns the top value without removing it.
func (s *StateStack[T]) Peek() (T, bool) {
	var zero T
	if s == nil || len(s.items) == 0 {
		return zero, false
	}
	return s.items[len(s.items)-1], true
}

// Len reports the current stack depth.
func (s *StateStack[T]) Len() int {
	if s == nil {
		return 0
	}
	return len(s.items)
}

// Cap reports the underlying slice capacity.
func (s *StateStack[T]) Cap() int {
	if s == nil {
		return 0
	}
	return cap(s.items)
}

// Items returns the stack backing slice in push order.
func (s *StateStack[T]) Items() []T {
	if s == nil {
		return nil
	}
	return s.items
}

// Reset clears the stack while retaining capacity.
func (s *StateStack[T]) Reset() {
	if s == nil {
		return
	}
	s.items = s.items[:0]
}

// Drop clears the stack and releases backing storage.
func (s *StateStack[T]) Drop() {
	if s == nil {
		return
	}
	s.items = nil
}
