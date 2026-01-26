package types

import (
	"sync"
	"testing"
)

func TestBuiltinParseValueConcurrent(t *testing.T) {
	bt := GetBuiltin(TypeNameString)
	if bt == nil {
		t.Fatal("GetBuiltin(string) returned nil")
	}

	const workers = 32
	errCh := make(chan error, workers)
	var wg sync.WaitGroup

	for range workers {
		wg.Go(func() {
			if _, err := bt.ParseValue("hello"); err != nil {
				errCh <- err
			}
		})
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("ParseValue error: %v", err)
	}
}
