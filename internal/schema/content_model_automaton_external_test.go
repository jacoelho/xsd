package schema_test

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

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

type automatonConstructionCase struct {
	name     string
	schema   string
	limitErr bool
	maxMiB   uint64
}

// automatonConstructionCases bound the bytes compilation allocates. The
// determinization bounds are about four times the compiled tables each schema
// reaches: 3.1 MiB of rows and edges at the state limit, and 16.6 MiB summed
// over the separately determinized models in choice. The fan-out bound is
// about 1.2 KiB per position over the 49,000 positions of g1 to g13.
func automatonConstructionCases() []automatonConstructionCase {
	return []automatonConstructionCase{
		{name: "nested-counted-sequences", schema: nestedCountedSequenceSchema(17), limitErr: true, maxMiB: 12},
		{name: "counted-sequences-in-choice", schema: countedSequenceInChoiceSchema(12), maxMiB: 64},
		{name: "group-fan-out", schema: groupFanOutSchema(17), limitErr: true, maxMiB: 56},
	}
}

func (tt automatonConstructionCase) compile() (*xsdSchema.Schema, error) {
	return xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("automaton.xsd", []byte(tt.schema))})
}

func (tt automatonConstructionCase) check(tb testing.TB, err error) {
	tb.Helper()
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
