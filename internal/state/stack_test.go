package state

import "testing"

func TestStateStack(t *testing.T) {
	stack := NewStateStack[int](2)
	if got := stack.Len(); got != 0 {
		t.Fatalf("Len() = %d, want 0", got)
	}
	if got := stack.Cap(); got < 2 {
		t.Fatalf("Cap() = %d, want >= 2", got)
	}
	if items := stack.Items(); len(items) != 0 {
		t.Fatalf("Items() len = %d, want 0", len(items))
	}
	if _, ok := stack.Peek(); ok {
		t.Fatalf("Peek() ok = true, want false")
	}
	if _, ok := stack.Pop(); ok {
		t.Fatalf("Pop() ok = true, want false")
	}

	stack.Push(10)
	stack.Push(20)
	if got := stack.Len(); got != 2 {
		t.Fatalf("Len() = %d, want 2", got)
	}
	if items := stack.Items(); len(items) != 2 || items[0] != 10 || items[1] != 20 {
		t.Fatalf("Items() = %v, want [10 20]", items)
	}
	if got, ok := stack.Peek(); !ok || got != 20 {
		t.Fatalf("Peek() = (%d,%v), want (20,true)", got, ok)
	}

	if got, ok := stack.Pop(); !ok || got != 20 {
		t.Fatalf("Pop() = (%d,%v), want (20,true)", got, ok)
	}
	if got, ok := stack.Pop(); !ok || got != 10 {
		t.Fatalf("Pop() = (%d,%v), want (10,true)", got, ok)
	}
	if got := stack.Len(); got != 0 {
		t.Fatalf("Len() = %d, want 0", got)
	}

	stack.Push(30)
	stack.Reset()
	if got := stack.Len(); got != 0 {
		t.Fatalf("Len() = %d, want 0 after Reset()", got)
	}
	stack.Drop()
	if got := stack.Cap(); got != 0 {
		t.Fatalf("Cap() = %d, want 0 after Drop()", got)
	}
}
