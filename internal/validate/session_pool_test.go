package validate

import (
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jacoelho/xsd/xsderrors"
)

func TestSessionPoolReusesOneSessionAndUpdatesLimits(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`)
	p := NewSessionPool(rt)
	if err := p.Validate(strings.NewReader(`<root/>`), Options{}); err != nil {
		t.Fatalf("first validation: %v", err)
	}
	want := p.idle.Load()
	if want == nil {
		t.Fatal("pool did not retain a completed session")
	}

	if err := p.Validate(strings.NewReader(`<root/>`), Options{MaxInstanceBytes: 1}); err == nil {
		t.Fatal("byte-limited validation succeeded")
	}
	if got := p.idle.Load(); got != want {
		t.Fatalf("pool replaced reusable session: got %p, want %p", got, want)
	}
	if err := p.Validate(strings.NewReader(`<root/>`), Options{}); err != nil {
		t.Fatalf("validation after limit update: %v", err)
	}
}

func TestSessionPoolPreservesErrorAndResetsQNameState(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:one" xmlns:t="urn:one">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`)
	p := NewSessionPool(rt)
	bad := p.Validate(strings.NewReader(`<t:root xmlns:t="urn:wrong"/>`), Options{})
	if bad == nil {
		t.Fatal("invalid namespace validation succeeded")
	}
	expectXSDCode(t, bad, xsderrors.CodeValidationRoot)
	if err := p.Validate(strings.NewReader(`<t:root xmlns:t="urn:one"/>`), Options{}); err != nil {
		t.Fatalf("validation after namespace error: %v", err)
	}
}

func TestSessionPoolDropsPanickingSession(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`)
	p := NewSessionPool(rt)
	if err := p.Validate(strings.NewReader(`<root/>`), Options{}); err != nil {
		t.Fatalf("warm validation: %v", err)
	}
	if p.idle.Load() == nil {
		t.Fatal("warm validation did not populate idle session")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("panic reader did not panic")
			}
		}()
		if err := p.Validate(panicReader{}, Options{}); err != nil {
			t.Errorf("panic reader returned error: %v", err)
		}
	}()
	if got := p.idle.Load(); got != nil {
		t.Fatalf("pool retained panicking session %p", got)
	}
	if err := p.Validate(strings.NewReader(`<root/>`), Options{}); err != nil {
		t.Fatalf("validation after panic: %v", err)
	}
}

func TestSessionPoolConcurrentMissesDoNotBlock(t *testing.T) {
	rt := compileRuntimeForTest(t, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root" type="xs:anyType"/>
</xs:schema>`)
	p := NewSessionPool(rt)
	first := &blockingReader{
		started: make(chan struct{}),
		release: make(chan struct{}),
		data:    []byte(`<root/>`),
	}
	t.Cleanup(first.Release)
	firstDone := make(chan error, 1)
	go func() { firstDone <- p.Validate(first, Options{}) }()
	select {
	case <-first.started:
	case <-time.After(2 * time.Second):
		t.Fatal("first validation did not reach blocked reader")
	}

	secondDone := make(chan error, 1)
	go func() { secondDone <- p.Validate(strings.NewReader(`<root/>`), Options{}) }()
	var secondErr error
	select {
	case secondErr = <-secondDone:
	case <-time.After(2 * time.Second):
		secondErr = errors.New("second validation did not complete while first session was blocked")
	}
	first.Release()
	var firstErr error
	select {
	case firstErr = <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("blocked validation did not finish after release")
	}
	if err := firstErr; err != nil {
		t.Fatalf("blocked validation: %v", err)
	}
	if secondErr != nil {
		t.Fatal(secondErr)
	}
	if got := p.idle.Load(); got == nil {
		t.Fatal("pool retained no session after concurrent validation")
	}
}

type panicReader struct{}

func (panicReader) Read([]byte) (int, error) {
	panic("session pool test panic")
}

type blockingReader struct {
	started     chan struct{}
	release     chan struct{}
	once        sync.Once
	releaseOnce sync.Once
	data        []byte
}

func (r *blockingReader) Release() {
	r.releaseOnce.Do(func() { close(r.release) })
}

func (r *blockingReader) Read(dst []byte) (int, error) {
	r.once.Do(func() { close(r.started) })
	<-r.release
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := copy(dst, r.data)
	r.data = r.data[n:]
	return n, nil
}
