package xsd_test

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/jacoelho/xsd"
)

const sessionOwnershipSchema = `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="root">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="item" minOccurs="0" maxOccurs="unbounded">
          <xs:complexType mixed="true">
            <xs:attribute name="value" type="xs:string" use="required"/>
          </xs:complexType>
        </xs:element>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`

func sessionOwnershipDocument() string {
	var b strings.Builder
	b.WriteString("<root>")
	for i := range 256 {
		fmt.Fprintf(&b, `<item value="value-%03d">text-%03d</item>`, i, i)
	}
	b.WriteString("</root>")
	return b.String()
}

type sessionOwnershipWorker struct {
	engine  *xsd.Engine
	session *xsd.Session
	reader  *strings.Reader
	doc     string
	opts    xsd.ValidateOptions
}

func newSessionOwnershipWorker(b *testing.B, engine *xsd.Engine, doc string, explicit bool) sessionOwnershipWorker { //nolint:revive // The benchmark intentionally compares Engine-owned and explicit Session-owned state.
	b.Helper()
	worker := sessionOwnershipWorker{
		engine: engine,
		reader: strings.NewReader(doc),
		doc:    doc,
	}
	if explicit {
		session, err := engine.NewSession(worker.opts)
		if err != nil {
			b.Fatal(err)
		}
		worker.session = session
	}
	return worker
}

func (w *sessionOwnershipWorker) validate() error {
	if w.session != nil {
		return w.session.Validate(w.reader)
	}
	return w.engine.ValidateWithOptions(w.reader, w.opts)
}

func (w *sessionOwnershipWorker) reset() {
	w.reader.Reset(w.doc)
}

func compileSessionOwnershipEngine(b *testing.B) (*xsd.Engine, string) {
	b.Helper()
	engine, err := xsd.Compile(xsd.Bytes("session-ownership.xsd", []byte(sessionOwnershipSchema)))
	if err != nil {
		b.Fatal(err)
	}
	return engine, sessionOwnershipDocument()
}

func BenchmarkSessionOwnershipSerial(b *testing.B) {
	engine, doc := compileSessionOwnershipEngine(b)
	for _, tc := range []struct {
		name     string
		explicit bool
	}{
		{name: "engine-idle1"},
		{name: "session-worker1", explicit: true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			worker := newSessionOwnershipWorker(b, engine, doc, tc.explicit)
			if err := worker.validate(); err != nil {
				b.Fatal(err)
			}
			worker.reset()
			b.SetBytes(int64(len(doc)))
			b.ReportAllocs()
			for b.Loop() {
				if err := worker.validate(); err != nil {
					b.Fatal(err)
				}
				worker.reset()
			}
		})
	}
}

func BenchmarkSessionOwnershipSteady(b *testing.B) {
	engine, doc := compileSessionOwnershipEngine(b)
	for _, workers := range []int{1, 4, 16} {
		for _, tc := range []struct {
			name     string
			explicit bool
		}{
			{name: "engine"},
			{name: "sessions", explicit: true},
		} {
			b.Run(fmt.Sprintf("%s/workers-%d", tc.name, workers), func(b *testing.B) {
				states := make([]sessionOwnershipWorker, workers)
				for i := range states {
					states[i] = newSessionOwnershipWorker(b, engine, doc, tc.explicit)
					if err := states[i].validate(); err != nil {
						b.Fatal(err)
					}
					states[i].reset()
				}

				start := make(chan struct{})
				errs := make(chan error, workers)
				var wg sync.WaitGroup
				wg.Add(workers)
				for i := range states {
					startAt := b.N * i / workers
					endAt := b.N * (i + 1) / workers
					go func(i, startAt, endAt int) {
						defer wg.Done()
						<-start
						for range endAt - startAt {
							if err := states[i].validate(); err != nil {
								errs <- err
								return
							}
							states[i].reset()
						}
					}(i, startAt, endAt)
				}

				b.SetBytes(int64(len(doc)))
				b.ReportAllocs()
				b.ResetTimer()
				close(start)
				wg.Wait()
				b.StopTimer()
				close(errs)
				for err := range errs {
					b.Fatal(err)
				}
			})
		}
	}
}

// sessionOwnershipBarrier releases all workers after they have reached the
// same phase. A separate completion barrier keeps one worker from entering
// the next burst while another is still finishing the current one.
type sessionOwnershipBarrier struct {
	mu      sync.Mutex
	cond    *sync.Cond
	parties int
	arrived int
	phase   uint64
}

func newSessionOwnershipBarrier(parties int) *sessionOwnershipBarrier {
	b := &sessionOwnershipBarrier{parties: parties}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *sessionOwnershipBarrier) wait() {
	b.mu.Lock()
	phase := b.phase
	b.arrived++
	if b.arrived == b.parties {
		b.arrived = 0
		b.phase++
		b.cond.Broadcast()
		b.mu.Unlock()
		return
	}
	for phase == b.phase {
		b.cond.Wait()
	}
	b.mu.Unlock()
}

// sessionOwnershipReadGate blocks the first reader call until all validation
// sessions have checked out their state. This makes overlap observable even
// with GOMAXPROCS=1; a barrier before Validate would only measure scheduling.
type sessionOwnershipReadGate struct {
	source strings.Reader
	doc    string
	gate   *sessionOwnershipBarrier
	first  bool
}

func (r *sessionOwnershipReadGate) Read(p []byte) (int, error) {
	if r.first && r.gate != nil {
		r.first = false
		r.gate.wait()
	}
	return r.source.Read(p)
}

func (r *sessionOwnershipReadGate) reset() {
	r.source.Reset(r.doc)
	r.first = true
}

type sessionOwnershipOverlapWorker struct {
	engine  *xsd.Engine
	session *xsd.Session
	reader  sessionOwnershipReadGate
	opts    xsd.ValidateOptions
}

func newSessionOwnershipOverlapWorker(b *testing.B, engine *xsd.Engine, doc string, explicit bool) sessionOwnershipOverlapWorker { //nolint:revive // The benchmark intentionally compares Engine-owned and explicit Session-owned state.
	b.Helper()
	worker := sessionOwnershipOverlapWorker{
		engine: engine,
		reader: sessionOwnershipReadGate{source: *strings.NewReader(doc), doc: doc},
	}
	if explicit {
		session, err := engine.NewSession(worker.opts)
		if err != nil {
			b.Fatal(err)
		}
		worker.session = session
	}
	return worker
}

func (w *sessionOwnershipOverlapWorker) validate() error {
	if w.session != nil {
		return w.session.Validate(&w.reader)
	}
	return w.engine.ValidateWithOptions(&w.reader, w.opts)
}

func BenchmarkSessionOwnershipOverlap(b *testing.B) {
	engine, doc := compileSessionOwnershipEngine(b)
	for _, tc := range []struct {
		name     string
		explicit bool
	}{
		{name: "engine"},
		{name: "sessions", explicit: true},
	} {
		for _, workerCount := range []int{2, 4, 16} {
			b.Run(fmt.Sprintf("%s/workers-%d", tc.name, workerCount), func(b *testing.B) {
				states := make([]sessionOwnershipOverlapWorker, workerCount)
				for i := range states {
					states[i] = newSessionOwnershipOverlapWorker(b, engine, doc, tc.explicit)
					if err := states[i].validate(); err != nil {
						b.Fatal(err)
					}
				}
				start := newSessionOwnershipBarrier(len(states))
				for i := range states {
					states[i].reader.gate = start
					states[i].reader.reset()
				}
				done := newSessionOwnershipBarrier(len(states))
				errs := make([]error, len(states))
				var wg sync.WaitGroup
				wg.Add(len(states))
				ready := make(chan struct{})
				for i := range states {
					go func(i int) {
						defer wg.Done()
						<-ready
						for range b.N {
							err := states[i].validate()
							if errs[i] == nil && err != nil {
								errs[i] = err
							}
							done.wait()
							states[i].reader.reset()
						}
					}(i)
				}

				b.SetBytes(int64(len(states) * len(doc)))
				b.ReportAllocs()
				b.ResetTimer()
				close(ready)
				wg.Wait()
				b.StopTimer()
				for _, err := range errs {
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func warmSessionOwnershipOverlap(b *testing.B, engine *xsd.Engine, doc string, explicit bool, workerCount, rounds int) []sessionOwnershipOverlapWorker {
	b.Helper()
	states := make([]sessionOwnershipOverlapWorker, workerCount)
	for i := range states {
		states[i] = newSessionOwnershipOverlapWorker(b, engine, doc, explicit)
		if err := states[i].validate(); err != nil {
			b.Fatal(err)
		}
	}
	start := newSessionOwnershipBarrier(workerCount)
	for i := range states {
		states[i].reader.gate = start
		states[i].reader.reset()
	}
	done := newSessionOwnershipBarrier(workerCount)
	errs := make([]error, workerCount)
	var wg sync.WaitGroup
	wg.Add(workerCount)
	ready := make(chan struct{})
	for i := range states {
		go func(i int) {
			defer wg.Done()
			<-ready
			for range rounds {
				err := states[i].validate()
				if errs[i] == nil && err != nil {
					errs[i] = err
				}
				done.wait()
				states[i].reader.reset()
			}
		}(i)
	}
	close(ready)
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			b.Fatal(err)
		}
	}
	for i := range states {
		states[i].reader.gate = nil
	}
	return states
}

// Setup, warmup, and the explicit GC happen outside the timed loop. The live
// heap metric includes runtime, fixture, and goroutine noise, so compare each
// leaf in an isolated benchmark invocation; it is not a post-callback profile.
func BenchmarkSessionOwnershipRetained(b *testing.B) {
	engine, doc := compileSessionOwnershipEngine(b)
	b.Run("engine-1", func(b *testing.B) {
		baseline := sessionOwnershipHeapAlloc(engine, doc)
		workers := warmSessionOwnershipOverlap(b, engine, doc, false, 16, 8)
		runtime.GC()
		var stats runtime.MemStats
		runtime.KeepAlive(workers)
		runtime.ReadMemStats(&stats)
		liveHeapDelta := float64(stats.HeapAlloc) - float64(baseline)
		b.SetBytes(int64(len(doc)))
		b.ReportAllocs()
		b.ResetTimer()
		for b.Loop() {
			if err := workers[0].validate(); err != nil {
				b.Fatal(err)
			}
			workers[0].reader.reset()
		}
		b.StopTimer()
		b.ReportMetric(liveHeapDelta, "live_heap_delta_bytes")
		runtime.KeepAlive(workers)
	})
	for _, count := range []int{1, 4, 16} {
		b.Run(fmt.Sprintf("sessions-%d", count), func(b *testing.B) {
			baseline := sessionOwnershipHeapAlloc(engine, doc)
			workers := warmSessionOwnershipOverlap(b, engine, doc, true, count, 8)
			runtime.GC()
			var stats runtime.MemStats
			runtime.KeepAlive(workers)
			runtime.ReadMemStats(&stats)
			liveHeapDelta := float64(stats.HeapAlloc) - float64(baseline)
			b.SetBytes(int64(len(doc)))
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				worker := &workers[i%len(workers)]
				if err := worker.validate(); err != nil {
					b.Fatal(err)
				}
				worker.reader.reset()
			}
			b.StopTimer()
			b.ReportMetric(liveHeapDelta, "live_heap_delta_bytes")
			runtime.KeepAlive(workers)
		})
	}
}

func sessionOwnershipHeapAlloc(engine *xsd.Engine, doc string) uint64 {
	runtime.GC()
	runtime.KeepAlive(engine)
	runtime.KeepAlive(doc)
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.HeapAlloc
}
