package resolveguard

type pointerState uint8

const (
	pointerEnter pointerState = iota
	pointerResolving
	pointerResolved
)

// Pointer tracks reentrancy state for pointer-like keys.
type Pointer[K comparable] struct {
	resolving map[K]struct{}
	resolved  map[K]struct{}
}

// NewPointer creates a new pointer-scope guard.
func NewPointer[K comparable]() *Pointer[K] {
	return &Pointer[K]{
		resolving: make(map[K]struct{}),
		resolved:  make(map[K]struct{}),
	}
}

func (g *Pointer[K]) state(key K) pointerState {
	if _, ok := g.resolved[key]; ok {
		return pointerResolved
	}
	if _, ok := g.resolving[key]; ok {
		return pointerResolving
	}
	return pointerEnter
}

// Resolve runs fn inside guarded pointer scope.
func (g *Pointer[K]) Resolve(key K, onResolving, fn func() error) error {
	switch g.state(key) {
	case pointerResolved:
		return nil
	case pointerResolving:
		if onResolving != nil {
			return onResolving()
		}
		return nil
	}
	g.resolving[key] = struct{}{}
	if err := fn(); err != nil {
		delete(g.resolving, key)
		return err
	}
	delete(g.resolving, key)
	g.resolved[key] = struct{}{}
	return nil
}

// NamedScope provides visited/scoped traversal for named graph keys.
type NamedScope[K comparable] interface {
	IsVisited(K) bool
	WithScope(K, func() error) error
}

// ResolveNamed runs fn in named scope when key has not been visited yet.
func ResolveNamed[K comparable](scope NamedScope[K], key K, fn func() error) error {
	if scope.IsVisited(key) {
		return nil
	}
	return scope.WithScope(key, fn)
}
