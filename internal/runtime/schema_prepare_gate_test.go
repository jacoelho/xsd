package runtime

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSchemaPrepareGateConcurrentCallersSharePreparation(t *testing.T) {
	const callers = 8

	rt := &Schema{}
	gate := newSchemaPrepareGate()
	start := make(chan struct{})
	started := make(chan struct{})
	release := make(chan struct{})

	var calls atomic.Int32
	prepareErr := make(chan error, 1)
	prepareErr <- nil
	prepare := func(*Schema) error {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return <-prepareErr
	}

	errs := make(chan error, callers)
	for range callers {
		go func() {
			<-start
			errs <- gate.prepare(rt, prepare)
		}()
	}
	close(start)

	waitForTestSignal(t, started, "prepare to start")

	if got := calls.Load(); got != 1 {
		t.Fatalf("prepare calls while running = %d, want 1", got)
	}

	close(release)
	for range callers {
		if err := waitForTestError(t, errs, "prepare caller"); err != nil {
			t.Fatalf("prepare caller error = %v", err)
		}
	}
	if got := atomic.LoadUint32(&rt.prepareState); got != schemaPrepareReady {
		t.Fatalf("prepare state = %d, want ready", got)
	}

	if err := gate.prepare(rt, prepare); err != nil {
		t.Fatalf("ready prepare error = %v", err)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("prepare calls after ready = %d, want 1", got)
	}
}

func TestSchemaPrepareGateRetriesAfterError(t *testing.T) {
	rt := &Schema{}
	gate := newSchemaPrepareGate()
	errPrepare := errors.New("prepare failed")
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondCalling := make(chan struct{})

	var calls atomic.Int32
	prepare := func(*Schema) error {
		switch calls.Add(1) {
		case 1:
			close(firstStarted)
			<-releaseFirst
			return errPrepare
		case 2:
			return nil
		default:
			return errors.New("unexpected prepare call")
		}
	}

	firstErr := make(chan error, 1)
	go func() {
		firstErr <- gate.prepare(rt, prepare)
	}()
	waitForTestSignal(t, firstStarted, "first prepare to start")

	secondErr := make(chan error, 1)
	go func() {
		close(secondCalling)
		secondErr <- gate.prepare(rt, prepare)
	}()
	waitForTestSignal(t, secondCalling, "second caller to enter prepare gate")

	close(releaseFirst)

	if err := waitForTestError(t, firstErr, "first prepare"); !errors.Is(err, errPrepare) {
		t.Fatalf("first prepare error = %v, want %v", err, errPrepare)
	}
	if err := waitForTestError(t, secondErr, "second prepare"); err != nil {
		t.Fatalf("second prepare error = %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("prepare calls = %d, want 2", got)
	}
	if got := atomic.LoadUint32(&rt.prepareState); got != schemaPrepareReady {
		t.Fatalf("prepare state = %d, want ready", got)
	}
}

func TestSchemaPrepareGateRetriesAfterPanic(t *testing.T) {
	rt := &Schema{}
	gate := newSchemaPrepareGate()
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondCalling := make(chan struct{})
	panicValue := "prepare panic"

	var calls atomic.Int32
	prepare := func(*Schema) error {
		switch calls.Add(1) {
		case 1:
			close(firstStarted)
			<-releaseFirst
			panic(panicValue)
		case 2:
			return nil
		default:
			return errors.New("unexpected prepare call")
		}
	}

	firstPanic := make(chan any, 1)
	go func() {
		defer func() {
			firstPanic <- recover()
		}()
		if err := gate.prepare(rt, prepare); err != nil {
			panic(err)
		}
	}()
	waitForTestSignal(t, firstStarted, "first prepare to start")

	secondErr := make(chan error, 1)
	go func() {
		close(secondCalling)
		secondErr <- gate.prepare(rt, prepare)
	}()
	waitForTestSignal(t, secondCalling, "second caller to enter prepare gate")

	close(releaseFirst)

	if got := waitForTestPanic(t, firstPanic, "first prepare"); got != panicValue {
		t.Fatalf("first prepare panic = %v, want %v", got, panicValue)
	}
	if err := waitForTestError(t, secondErr, "second prepare"); err != nil {
		t.Fatalf("second prepare error = %v", err)
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("prepare calls = %d, want 2", got)
	}
	if got := atomic.LoadUint32(&rt.prepareState); got != schemaPrepareReady {
		t.Fatalf("prepare state = %d, want ready", got)
	}
}

func TestPrepareValidationHotPathsReadyDoesNotEnterGate(t *testing.T) {
	rt := &Schema{}
	atomic.StoreUint32(&rt.prepareState, schemaPrepareReady)

	schemaPrepareGate.mu.Lock()
	defer schemaPrepareGate.mu.Unlock()

	done := make(chan error, 1)
	go func() {
		done <- rt.PrepareValidationHotPaths()
	}()

	if err := waitForTestError(t, done, "ready prepare fast path"); err != nil {
		t.Fatalf("PrepareValidationHotPaths() error = %v", err)
	}
}

func TestSchemaPrepareGateIndependentSchemasDoNotReleaseTogether(t *testing.T) {
	gate := newSchemaPrepareGate()
	rtA := &Schema{}
	rtB := &Schema{}
	startedA := make(chan struct{})
	startedB := make(chan struct{})
	releaseA := make(chan struct{})
	releaseB := make(chan struct{})
	waiterACalling := make(chan struct{})
	waiterBCalling := make(chan struct{})
	doneA := make(chan error, 2)
	doneB := make(chan error, 2)

	var onceA sync.Once
	var onceB sync.Once
	prepare := func(rt *Schema) error {
		switch rt {
		case rtA:
			onceA.Do(func() { close(startedA) })
			<-releaseA
		case rtB:
			onceB.Do(func() { close(startedB) })
			<-releaseB
		default:
			return errors.New("unexpected schema")
		}
		return nil
	}

	go func() {
		doneA <- gate.prepare(rtA, prepare)
	}()
	go func() {
		doneB <- gate.prepare(rtB, prepare)
	}()
	waitForTestSignal(t, startedA, "schema A prepare to start")
	waitForTestSignal(t, startedB, "schema B prepare to start")
	go func() {
		close(waiterACalling)
		doneA <- gate.prepare(rtA, prepare)
	}()
	go func() {
		close(waiterBCalling)
		doneB <- gate.prepare(rtB, prepare)
	}()
	waitForTestSignal(t, waiterACalling, "schema A waiter to enter prepare gate")
	waitForTestSignal(t, waiterBCalling, "schema B waiter to enter prepare gate")

	close(releaseA)
	for range 2 {
		if err := waitForTestError(t, doneA, "schema A prepare"); err != nil {
			t.Fatalf("schema A prepare error = %v", err)
		}
	}
	select {
	case err := <-doneB:
		t.Fatalf("schema B prepare returned before release: %v", err)
	default:
	}

	close(releaseB)
	for range 2 {
		if err := waitForTestError(t, doneB, "schema B prepare"); err != nil {
			t.Fatalf("schema B prepare error = %v", err)
		}
	}
}

func waitForTestSignal(t *testing.T, ch <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

func waitForTestError(t *testing.T, ch <-chan error, name string) error {
	t.Helper()
	select {
	case err := <-ch:
		return err
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
		return nil
	}
}

func waitForTestPanic(t *testing.T, ch <-chan any, name string) any {
	t.Helper()
	select {
	case recovered := <-ch:
		if recovered == nil {
			t.Fatalf("%s did not panic", name)
		}
		return recovered
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
		return nil
	}
}
