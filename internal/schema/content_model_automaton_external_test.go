package schema_test

import (
	"errors"
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/xsderrors"
)

// nestedCountedSequenceSchema nests depth units of sequence(e, choice(f, g)),
// each with maxOccurs 2 and its own element names, after the choice. Each
// unit owns a counter, so the deterministic automaton grows exponentially
// with depth while the source automaton has three positions per unit.
func nestedCountedSequenceSchema(depth int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType>`)
	for i := depth; i >= 1; i-- {
		fmt.Fprintf(&b, `<xs:sequence maxOccurs="2"><xs:element name="e%d"/><xs:choice><xs:element name="f%d"/><xs:element name="g%d"/></xs:choice>`, i, i, i)
	}
	b.WriteString(strings.Repeat(`</xs:sequence>`, depth))
	b.WriteString(`</xs:complexType></xs:element></xs:schema>`)
	return b.String()
}

// countedSequenceInChoiceSchema is nestedCountedSequenceSchema with each inner
// unit in place of the outer unit's g, so every nested group is a model of
// its own and is determinized separately.
func countedSequenceInChoiceSchema(depth int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType>`)
	for i := depth; i >= 1; i-- {
		fmt.Fprintf(&b, `<xs:sequence maxOccurs="2"><xs:element name="e%d"/><xs:choice><xs:element name="f%d"/><xs:sequence>`, i, i)
	}
	b.WriteString(strings.Repeat(`</xs:sequence></xs:choice></xs:sequence>`, depth))
	b.WriteString(`</xs:complexType></xs:element></xs:schema>`)
	return b.String()
}

// groupFanOutSchema defines group g0 as sequence(e, choice(f, g)) and each
// g(i) as a sequence of two references to g(i-1), so g(i) expands to 3*2^i
// positions. The first group with more positions than the state limit stops
// compilation, after its source automaton is built.
func groupFanOutSchema(depth int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	b.WriteString(`<xs:group name="g0"><xs:sequence><xs:element name="e"/><xs:choice><xs:element name="f"/><xs:element name="g"/></xs:choice></xs:sequence></xs:group>`)
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&b, `<xs:group name="g%d"><xs:sequence><xs:group ref="g%d"/><xs:group ref="g%d"/></xs:sequence></xs:group>`, i, i-1, i-1)
	}
	fmt.Fprintf(&b, `<xs:element name="root"><xs:complexType><xs:group ref="g%d"/></xs:complexType></xs:element></xs:schema>`, depth)
	return b.String()
}

// wideGroupFanOutSchema defines group g0 as a choice of width elements and
// each g(i) as a sequence of two references to g(i-1), so every position of
// one g0 copy is followed by every first position of the next copy.
func wideGroupFanOutSchema(width, depth int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:group name="g0"><xs:choice>`)
	for i := range width {
		fmt.Fprintf(&b, `<xs:element name="e%d"/>`, i)
	}
	b.WriteString(`</xs:choice></xs:group>`)
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&b, `<xs:group name="g%d"><xs:sequence><xs:group ref="g%d"/><xs:group ref="g%d"/></xs:sequence></xs:group>`, i, i-1, i-1)
	}
	fmt.Fprintf(&b, `<xs:element name="root"><xs:complexType><xs:group ref="g%d"/></xs:complexType></xs:element></xs:schema>`, depth)
	return b.String()
}

// optionalSequenceSchema is a sequence of head and width optional elements,
// repeated up to maxOccurs times. Each optional position is followed by every
// later one, so each has its own follow set.
func optionalSequenceSchema(head string, width int, maxOccurs string) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence maxOccurs="%s">%s`, maxOccurs, head)
	for i := range width {
		fmt.Fprintf(&b, `<xs:element name="e%d" minOccurs="0"/>`, i)
	}
	b.WriteString(`</xs:sequence></xs:complexType></xs:element></xs:schema>`)
	return b.String()
}

func repeatedOptionalWildcardSequenceSchema(width int) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType><xs:sequence maxOccurs="unbounded"><xs:element name="z"/>`)
	for i := range width {
		fmt.Fprintf(&b, `<xs:any namespace="urn:n%d" processContents="skip" minOccurs="0"/>`, i)
	}
	b.WriteString(`</xs:sequence></xs:complexType></xs:element></xs:schema>`)
	return b.String()
}

type automatonConstructionCase struct {
	name         string
	schema       string
	limitErr     bool
	stepLimitErr bool
	// tableLimitErr expects MaxSchemaDependencySteps, which the compiled
	// tables of the model exhaust before its analysis does.
	tableLimitErr bool
	maxMiB        uint64
}

// automatonConstructionCases bound the bytes compilation allocates. The
// determinization bounds are about four times the compiled tables each schema
// reaches: 3.1 MiB of rows and edges at the state limit, and 16.6 MiB summed
// over the separately determinized models in choice. The fan-out bound is
// about 1.2 KiB per position over the 49,000 positions of g1 to g13. The other
// schemas exhaust the default MaxContentModelAnalysisSteps, except the direct
// optional sequence, whose edge references and row indexes exhaust the
// default MaxSchemaDependencySteps first. The bound of the
// repeated optional sequence, the costliest optional-element shape measured,
// is three bytes per step, and the others two; both are measured margins
// under the derived row-edge bound in docs/RESOURCE_LIMITS.md.
func automatonConstructionCases() []automatonConstructionCase {
	return []automatonConstructionCase{
		{name: "nested-counted-sequences", schema: nestedCountedSequenceSchema(17), limitErr: true, maxMiB: 12},
		{name: "counted-sequences-in-choice", schema: countedSequenceInChoiceSchema(12), maxMiB: 64},
		{name: "group-fan-out", schema: groupFanOutSchema(17), limitErr: true, maxMiB: 56},
		{name: "wide-group-fan-out-10", schema: wideGroupFanOutSchema(256, 10), stepLimitErr: true, maxMiB: 32},
		{name: "wide-group-fan-out-11", schema: wideGroupFanOutSchema(256, 11), stepLimitErr: true, maxMiB: 32},
		{name: "wide-group-fan-out-12", schema: wideGroupFanOutSchema(256, 12), stepLimitErr: true, maxMiB: 32},
		{name: "counted-optional-sequence", schema: optionalSequenceSchema("", 1060, "2"), stepLimitErr: true, maxMiB: 32},
		{name: "direct-optional-sequence", schema: optionalSequenceSchema("", 2060, "1"), tableLimitErr: true, maxMiB: 32},
		{name: "repeated-optional-sequence", schema: optionalSequenceSchema(`<xs:element name="z"/>`, 1720, "unbounded"), stepLimitErr: true, maxMiB: 48},
	}
}

func (tt automatonConstructionCase) compile() (*xsdSchema.Schema, error) {
	return xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("automaton.xsd", []byte(tt.schema))})
}

func (tt automatonConstructionCase) check(tb testing.TB, err error) {
	tb.Helper()
	if tt.tableLimitErr {
		x, ok := errors.AsType[*xsderrors.Error](err)
		if !ok || x.Code() != xsderrors.CodeSchemaLimit || !strings.HasPrefix(x.Message(), "compiler work exceeds MaxSchemaDependencySteps") {
			tb.Fatalf("Compile() error = %v, want the dependency step limit", err)
		}
		return
	}
	if tt.stepLimitErr {
		x, ok := errors.AsType[*xsderrors.Error](err)
		if !ok || x.Code() != xsderrors.CodeSchemaLimit || !strings.HasPrefix(x.Message(), "compiler work exceeds MaxContentModelAnalysisSteps") {
			tb.Fatalf("Compile() error = %v, want the content-model analysis step limit", err)
		}
		return
	}
	if !tt.limitErr {
		if err != nil {
			tb.Fatalf("Compile() error = %v", err)
		}
		return
	}
	x, ok := errors.AsType[*xsderrors.Error](err)
	if !ok || x.Code() != xsderrors.CodeSchemaLimit || x.Message() != "content model DFA state limit exceeded" {
		tb.Fatalf("Compile() error = %v, want the DFA state limit", err)
	}
}

func TestContentModelAutomatonConstructionAllocationIsBounded(t *testing.T) {
	for _, tt := range automatonConstructionCases() {
		t.Run(tt.name, func(t *testing.T) {
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			_, err := tt.compile()
			runtime.ReadMemStats(&after)
			tt.check(t, err)
			if got := after.TotalAlloc - before.TotalAlloc; got > tt.maxMiB<<20 {
				t.Fatalf("allocated %d MiB, want under %d MiB", got>>20, tt.maxMiB)
			}
		})
	}
}

func BenchmarkCompileContentModelAutomata(b *testing.B) {
	for _, tt := range automatonConstructionCases() {
		b.Run(tt.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_, err := tt.compile()
				tt.check(b, err)
			}
		})
	}
}

// The shape is the repeated optional wildcard sequence docs/RESOURCE_LIMITS.md
// lists, and the bound, 3 bytes per step, is a measured margin over its peak
// under the derived row-edge bound documented there. A 1 ms HeapInuse
// sampler sees a lower bound on the true peak, so the floor rejects a run in
// which it missed construction instead of passing it.
func TestContentModelAutomatonConstructionPeakHeapIsBounded(t *testing.T) {
	const bytesPerStep = 3
	const sampledFloor = 8 << 20
	limits, err := xsdSchema.NormalizeOptions(xsdSchema.Options{})
	if err != nil {
		t.Fatal(err)
	}
	bound := uint64(bytesPerStep * limits.MaxContentModelAnalysisSteps) // #nosec G115 -- the default step limit is positive and small.
	tt := automatonConstructionCase{schema: repeatedOptionalWildcardSequenceSchema(1900), stepLimitErr: true}
	defer debug.SetGCPercent(debug.SetGCPercent(100))
	runtime.GC()
	runtime.GC()
	baseline := heapInuse()
	sampler := startHeapSampler(time.Millisecond)
	_, err = tt.compile()
	peak := sampler.stop()
	tt.check(t, err)
	delta := peak - min(peak, baseline)
	t.Logf("sampled peak heap %d KiB above baseline, bound %d KiB", delta>>10, bound>>10)
	if delta < sampledFloor {
		t.Fatalf("sampled peak heap %d MiB above baseline, want at least %d MiB: the sampler missed construction", delta>>20, sampledFloor>>20)
	}
	if delta > bound {
		t.Fatalf("peak heap %d MiB above baseline, want at most %d MiB", delta>>20, bound>>20)
	}
}

type heapSampler struct {
	done chan struct{}
	peak chan uint64
}

func startHeapSampler(interval time.Duration) heapSampler {
	s := heapSampler{done: make(chan struct{}), peak: make(chan uint64)}
	started := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		peak := heapInuse()
		close(started)
		for {
			select {
			case <-ticker.C:
				peak = max(peak, heapInuse())
			case <-s.done:
				s.peak <- max(peak, heapInuse())
				return
			}
		}
	}()
	<-started
	return s
}

func (s heapSampler) stop() uint64 {
	close(s.done)
	return <-s.peak
}

func heapInuse() uint64 {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return stats.HeapInuse
}
