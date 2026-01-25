package types

import (
	"sync"
	"testing"
)

func TestEnumerationValidateLexicalConcurrent(t *testing.T) {
	enum := &Enumeration{Values: []string{"a", "b"}}
	base := GetBuiltin(TypeNameString)
	if base == nil {
		t.Fatal("expected builtin string type")
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := enum.ValidateLexical("a", base); err != nil {
				t.Errorf("ValidateLexical error: %v", err)
			}
		})
	}
	wg.Wait()
}
