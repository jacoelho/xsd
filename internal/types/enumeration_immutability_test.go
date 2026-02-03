package types

import (
	"sync"
	"testing"
)

func TestEnumerationValuesDefensiveCopy(t *testing.T) {
	values := []string{"a", "b"}
	enum := NewEnumeration(values)
	values[0] = "changed"

	got := enum.Values()
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("Values() = %v, want [a b]", got)
	}

	got[0] = "mutated"
	after := enum.Values()
	if after[0] != "a" {
		t.Fatalf("Values() mutation should not affect enumeration, got %v", after)
	}
}

func TestEnumerationValueContextsDefensiveCopy(t *testing.T) {
	enum := NewEnumeration([]string{"p:val"})
	contexts := []map[string]string{{"p": "urn:test"}}
	enum.SetValueContexts(contexts)

	contexts[0]["p"] = "mutated"
	got := enum.ValueContexts()
	if got[0]["p"] != "urn:test" {
		t.Fatalf("ValueContexts() = %v, want original context", got)
	}

	got[0]["p"] = "changed"
	after := enum.ValueContexts()
	if after[0]["p"] != "urn:test" {
		t.Fatalf("ValueContexts() mutation should not affect enumeration, got %v", after)
	}
}

func TestEnumerationValueContextsIsolationUnderConcurrency(t *testing.T) {
	enum := NewEnumeration([]string{"p:val"})
	contexts := []map[string]string{{"p": "urn:test"}}
	enum.SetValueContexts(contexts)

	base := GetBuiltin(TypeNameQName)
	if base == nil {
		t.Fatal("expected builtin QName type")
	}

	var wg sync.WaitGroup
	errCh := make(chan error, 1)
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			if err := enum.ValidateLexicalQName("p:val", base, map[string]string{"p": "urn:test"}); err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			contexts[0]["p"] = "urn:test"
			contexts[0]["p"] = "urn:other"
		}
	}()
	wg.Wait()

	select {
	case err := <-errCh:
		t.Fatalf("ValidateLexicalQName error: %v", err)
	default:
	}
}
