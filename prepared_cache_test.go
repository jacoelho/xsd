package xsd

import (
	"sync"
	"testing"
	"testing/fstest"
)

func preparedSchemaForCacheTests(t *testing.T) *PreparedSchema {
	t.Helper()
	fsys := fstest.MapFS{
		"schema.xsd": &fstest.MapFile{Data: []byte(`<?xml version="1.0"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:string"/>
</xs:schema>`)},
	}
	prepared, err := PrepareWithOptions(fsys, "schema.xsd", NewLoadOptions())
	if err != nil {
		t.Fatalf("PrepareWithOptions() error = %v", err)
	}
	return prepared
}

func TestPreparedSchemaBuildCachesEquivalentRuntimeOptions(t *testing.T) {
	prepared := preparedSchemaForCacheTests(t)
	opts := NewRuntimeOptions().
		WithMaxDFAStates(256).
		WithMaxOccursLimit(2048).
		WithInstanceMaxDepth(64)

	first, err := prepared.BuildWithOptions(opts)
	if err != nil {
		t.Fatalf("first BuildWithOptions() error = %v", err)
	}
	second, err := prepared.BuildWithOptions(opts)
	if err != nil {
		t.Fatalf("second BuildWithOptions() error = %v", err)
	}

	if got := prepared.runtimeBuildCacheLen(); got != 1 {
		t.Fatalf("runtime cache size = %d, want 1", got)
	}
	if first.engine == nil || second.engine == nil {
		t.Fatal("expected non-nil engines")
	}
	if first.engine.rt != second.engine.rt {
		t.Fatal("expected equivalent runtime options to reuse compiled runtime")
	}
}

func TestPreparedSchemaBuildCacheConcurrentSafety(t *testing.T) {
	prepared := preparedSchemaForCacheTests(t)
	opts := NewRuntimeOptions().WithMaxDFAStates(128).WithMaxOccursLimit(4096)

	const workers = 24
	const iterations = 8
	errCh := make(chan error, workers*iterations)
	rtCh := make(chan *Schema, workers*iterations)
	var wg sync.WaitGroup
	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()
			for range iterations {
				sch, err := prepared.BuildWithOptions(opts)
				if err != nil {
					errCh <- err
					return
				}
				rtCh <- sch
			}
		}()
	}
	wg.Wait()
	close(errCh)
	close(rtCh)

	for err := range errCh {
		t.Fatalf("BuildWithOptions() concurrent error = %v", err)
	}

	var first *Schema
	for sch := range rtCh {
		if sch == nil || sch.engine == nil || sch.engine.rt == nil {
			t.Fatal("expected non-nil schema engine/runtime")
		}
		if first == nil {
			first = sch
			continue
		}
		if sch.engine.rt != first.engine.rt {
			t.Fatal("expected concurrent builds with equivalent options to reuse compiled runtime")
		}
	}
	if got := prepared.runtimeBuildCacheLen(); got != 1 {
		t.Fatalf("runtime cache size = %d, want 1", got)
	}
}

func TestPreparedSchemaBuildCacheKeyIgnoresInstanceOnlyLimits(t *testing.T) {
	prepared := preparedSchemaForCacheTests(t)
	compileOpts := NewRuntimeOptions().
		WithMaxDFAStates(512).
		WithMaxOccursLimit(4096)

	first, err := prepared.BuildWithOptions(compileOpts.WithInstanceMaxDepth(32))
	if err != nil {
		t.Fatalf("first BuildWithOptions() error = %v", err)
	}
	second, err := prepared.BuildWithOptions(compileOpts.WithInstanceMaxDepth(256))
	if err != nil {
		t.Fatalf("second BuildWithOptions() error = %v", err)
	}

	if first.engine == nil || first.engine.rt == nil || second.engine == nil || second.engine.rt == nil {
		t.Fatal("expected non-nil engines and runtime schemas")
	}
	if first.engine.rt != second.engine.rt {
		t.Fatal("expected instance-only option changes to reuse compiled runtime schema")
	}
	if got := prepared.runtimeBuildCacheLen(); got != 1 {
		t.Fatalf("runtime cache size = %d, want 1", got)
	}
}

func TestPreparedSchemaBuildCacheIsBounded(t *testing.T) {
	prepared := preparedSchemaForCacheTests(t)
	firstOpts := NewRuntimeOptions().WithMaxDFAStates(1).WithMaxOccursLimit(1)

	first, err := prepared.BuildWithOptions(firstOpts)
	if err != nil {
		t.Fatalf("first BuildWithOptions() error = %v", err)
	}
	if first == nil || first.engine == nil || first.engine.rt == nil {
		t.Fatal("expected non-nil first runtime schema")
	}

	for i := 2; i <= maxPreparedRuntimeBuildCacheEntries+2; i++ {
		opts := NewRuntimeOptions().
			WithMaxDFAStates(uint32(i)).
			WithMaxOccursLimit(uint32(i))
		if _, err := prepared.BuildWithOptions(opts); err != nil {
			t.Fatalf("BuildWithOptions(%d) error = %v", i, err)
		}
	}

	if got := prepared.runtimeBuildCacheLen(); got != maxPreparedRuntimeBuildCacheEntries {
		t.Fatalf("runtime cache size = %d, want %d", got, maxPreparedRuntimeBuildCacheEntries)
	}

	rebuilt, err := prepared.BuildWithOptions(firstOpts)
	if err != nil {
		t.Fatalf("rebuilt BuildWithOptions() error = %v", err)
	}
	if rebuilt == nil || rebuilt.engine == nil || rebuilt.engine.rt == nil {
		t.Fatal("expected non-nil rebuilt runtime schema")
	}
	if rebuilt.engine.rt == first.engine.rt {
		t.Fatal("expected oldest runtime cache entry to be evicted")
	}
	if got := prepared.runtimeBuildCacheLen(); got != maxPreparedRuntimeBuildCacheEntries {
		t.Fatalf("runtime cache size after reinsert = %d, want %d", got, maxPreparedRuntimeBuildCacheEntries)
	}
}

func TestPreparedSchemaBuildCacheSeparatesCompileOptions(t *testing.T) {
	prepared := preparedSchemaForCacheTests(t)
	optsA := NewRuntimeOptions().WithMaxDFAStates(64).WithMaxOccursLimit(1024)
	optsB := NewRuntimeOptions().WithMaxDFAStates(128).WithMaxOccursLimit(1024)

	firstA, err := prepared.BuildWithOptions(optsA)
	if err != nil {
		t.Fatalf("first BuildWithOptions(optsA) error = %v", err)
	}
	firstB, err := prepared.BuildWithOptions(optsB)
	if err != nil {
		t.Fatalf("first BuildWithOptions(optsB) error = %v", err)
	}
	secondA, err := prepared.BuildWithOptions(optsA)
	if err != nil {
		t.Fatalf("second BuildWithOptions(optsA) error = %v", err)
	}

	if got := prepared.runtimeBuildCacheLen(); got != 2 {
		t.Fatalf("runtime cache size = %d, want 2", got)
	}
	if firstA == nil || firstA.engine == nil || firstA.engine.rt == nil {
		t.Fatal("expected non-nil runtime for optsA")
	}
	if firstB == nil || firstB.engine == nil || firstB.engine.rt == nil {
		t.Fatal("expected non-nil runtime for optsB")
	}
	if secondA == nil || secondA.engine == nil || secondA.engine.rt == nil {
		t.Fatal("expected non-nil runtime for second optsA build")
	}
	if firstA.engine.rt == firstB.engine.rt {
		t.Fatal("different compile options should not share runtime cache entry")
	}
	if secondA.engine.rt != firstA.engine.rt {
		t.Fatal("equivalent compile options should reuse runtime cache entry")
	}
}
