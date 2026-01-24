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
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := enum.ValidateLexical("a", base); err != nil {
				t.Errorf("ValidateLexical error: %v", err)
			}
		}()
	}
	wg.Wait()
}
