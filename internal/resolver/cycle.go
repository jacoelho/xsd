package resolver

import "fmt"

// CycleDetector manages visited/resolving state for graph traversal.
// Prevents infinite recursion by detecting cycles during resolution.
type CycleDetector[K comparable] struct {
	visited   map[K]bool
	resolving map[K]bool
}

// NewCycleDetector creates a new cycle detector.
func NewCycleDetector[K comparable]() *CycleDetector[K] {
	return &CycleDetector[K]{
		visited:   make(map[K]bool),
		resolving: make(map[K]bool),
	}
}

// Enter marks key as resolving and returns an error if already resolving (cycle detected).
func (c *CycleDetector[K]) Enter(key K) error {
	if c.resolving[key] {
		return fmt.Errorf("circular reference detected: %v", key)
	}
	c.resolving[key] = true
	return nil
}

// Leave marks key as visited and removes it from resolving state.
func (c *CycleDetector[K]) Leave(key K) {
	delete(c.resolving, key)
	c.visited[key] = true
}

// IsVisited returns true if key was already processed.
func (c *CycleDetector[K]) IsVisited(key K) bool {
	return c.visited[key]
}

// IsResolving returns true if key is currently being resolved.
// Use this for read-only cycle checks without modifying state.
func (c *CycleDetector[K]) IsResolving(key K) bool {
	return c.resolving[key]
}

// WithScope combines Enter/Leave with deferred cleanup.
// Executes fn within a scope that automatically handles cycle detection.
func (c *CycleDetector[K]) WithScope(key K, fn func() error) error {
	if err := c.Enter(key); err != nil {
		return err
	}
	defer c.Leave(key)
	return fn()
}
