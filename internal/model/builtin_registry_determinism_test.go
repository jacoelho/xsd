package model

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

type builtinSnapshot struct {
	builtin   *BuiltinType
	primitive Type
	facets    *FundamentalFacets
}

func TestDefaultBuiltinRegistryConcurrentAccessIsDeterministic(t *testing.T) {
	names := make([]TypeName, 0, len(defaultBuiltinRegistry.ordered))
	snapshots := make(map[TypeName]builtinSnapshot, len(defaultBuiltinRegistry.ordered))
	for _, builtin := range defaultBuiltinRegistry.ordered {
		if builtin == nil {
			continue
		}
		name := TypeName(builtin.name)
		names = append(names, name)
		snapshots[name] = builtinSnapshot{
			builtin:   builtin,
			primitive: builtin.PrimitiveType(),
			facets:    builtin.FundamentalFacets(),
		}
	}
	if len(names) == 0 {
		t.Fatal("default builtin registry is empty")
	}

	const (
		workers    = 24
		iterations = 300
		timeout    = 5 * time.Second
	)

	errCh := make(chan error, 1)
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			<-start
			for range iterations {
				for _, name := range names {
					snapshot := snapshots[name]

					got := GetBuiltin(name)
					if got != snapshot.builtin {
						select {
						case errCh <- fmt.Errorf("GetBuiltin(%s) pointer mismatch", name):
						default:
						}
						return
					}
					if gotNS := GetBuiltinNS(XSDNamespace, string(name)); gotNS != snapshot.builtin {
						select {
						case errCh <- fmt.Errorf("GetBuiltinNS(%s) pointer mismatch", name):
						default:
						}
						return
					}
					if gotNS := GetBuiltinNS("urn:other", string(name)); gotNS != nil {
						select {
						case errCh <- fmt.Errorf("GetBuiltinNS non-XSD namespace returned builtin for %s", name):
						default:
						}
						return
					}
					if gotPrimitive := got.PrimitiveType(); gotPrimitive != snapshot.primitive {
						select {
						case errCh <- fmt.Errorf("PrimitiveType(%s) pointer mismatch", name):
						default:
						}
						return
					}
					if gotFacets := got.FundamentalFacets(); gotFacets != snapshot.facets {
						select {
						case errCh <- fmt.Errorf("FundamentalFacets(%s) pointer mismatch", name):
						default:
						}
						return
					}
				}
			}
		}()
	}
	close(start)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errCh:
		t.Fatal(err)
	case <-done:
	case <-time.After(timeout):
		t.Fatalf("concurrent builtin registry determinism test timed out after %s", timeout)
	}
}
