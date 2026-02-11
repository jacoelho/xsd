package resolveguard

import (
	"errors"
	"testing"
)

func TestPointerResolve(t *testing.T) {
	guard := NewPointer[*int]()
	value := new(int)

	calls := 0
	if err := guard.Resolve(value, nil, func() error {
		calls++
		return nil
	}); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if err := guard.Resolve(value, nil, func() error {
		calls++
		return nil
	}); err != nil {
		t.Fatalf("Resolve() second call error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("Resolve() calls = %d, want 1", calls)
	}
}

func TestPointerResolveReentrantError(t *testing.T) {
	guard := NewPointer[*int]()
	value := new(int)
	wantErr := errors.New("reentrant")

	err := guard.Resolve(value, nil, func() error {
		return guard.Resolve(value, func() error { return wantErr }, func() error { return nil })
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Resolve() error = %v, want %v", err, wantErr)
	}

	if err := guard.Resolve(value, nil, func() error { return nil }); err != nil {
		t.Fatalf("Resolve() after error = %v", err)
	}
}

type fakeScope struct {
	visited map[string]bool
}

func (f *fakeScope) IsVisited(key string) bool {
	return f.visited[key]
}

func (f *fakeScope) WithScope(key string, fn func() error) error {
	f.visited[key] = true
	return fn()
}

func TestResolveNamed(t *testing.T) {
	scope := &fakeScope{visited: make(map[string]bool)}
	calls := 0
	if err := ResolveNamed[string](scope, "a", func() error {
		calls++
		return nil
	}); err != nil {
		t.Fatalf("ResolveNamed() error = %v", err)
	}
	if err := ResolveNamed[string](scope, "a", func() error {
		calls++
		return nil
	}); err != nil {
		t.Fatalf("ResolveNamed() second call error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("ResolveNamed() calls = %d, want 1", calls)
	}
}
