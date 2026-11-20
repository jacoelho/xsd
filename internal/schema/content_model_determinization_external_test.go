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

type nestedCountedSequence uint8

const (
	nestedAfterChoice nestedCountedSequence = iota
	nestedInChoice
)

// nestedCountedSequenceSchema nests depth units of sequence(e, choice(f, g)),
// each with maxOccurs 2. nestedAfterChoice appends the inner unit to the
// sequence; nestedInChoice puts it in place of g. Each unit owns a counter,
// so the deterministic automaton grows exponentially with depth.
func nestedCountedSequenceSchema(depth int, nesting nestedCountedSequence) string {
	var b strings.Builder
	b.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root"><xs:complexType>`)
	for i := depth; i >= 1; i-- {
		switch nesting {
		case nestedAfterChoice:
			fmt.Fprintf(&b, `<xs:sequence maxOccurs="2"><xs:element name="e%d"/><xs:choice><xs:element name="f%d"/><xs:element name="g%d"/></xs:choice>`, i, i, i)
		case nestedInChoice:
			fmt.Fprintf(&b, `<xs:sequence maxOccurs="2"><xs:element name="e%d"/><xs:choice><xs:element name="f%d"/><xs:sequence>`, i, i)
		}
	}
	for range depth {
		switch nesting {
		case nestedAfterChoice:
			b.WriteString(`</xs:sequence>`)
		case nestedInChoice:
			b.WriteString(`</xs:sequence></xs:choice></xs:sequence>`)
		}
	}
	b.WriteString(`</xs:complexType></xs:element></xs:schema>`)
	return b.String()
}

func compileNestedCountedSequence(depth int, nesting nestedCountedSequence) (*xsdSchema.Schema, error) {
	return xsdSchema.Compile(xsdSchema.Options{}, []source.Source{
		source.Bytes("nested.xsd", []byte(nestedCountedSequenceSchema(depth, nesting))),
	})
}

func TestContentModelDeterminizationAllocationIsBounded(t *testing.T) {
	for _, tt := range []struct {
		name     string
		depth    int
		nesting  nestedCountedSequence
		limitErr bool
		maxMiB   uint64
	}{
		// Each bound is about four times the compiled tables the schema
		// reaches: 3.1 MiB of rows and edges at the state limit, and
		// 16.6 MiB summed over the nested models near it.
		{name: "state limit", depth: 17, nesting: nestedAfterChoice, limitErr: true, maxMiB: 12},
		{name: "near state limit", depth: 12, nesting: nestedInChoice, maxMiB: 64},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			_, err := compileNestedCountedSequence(tt.depth, tt.nesting)
			runtime.ReadMemStats(&after)
			if tt.limitErr {
				expectStateLimit(t, err)
			} else if err != nil {
				t.Fatalf("Compile() error = %v", err)
			}
			if got := after.TotalAlloc - before.TotalAlloc; got > tt.maxMiB<<20 {
				t.Fatalf("allocated %d MiB, want under %d MiB", got>>20, tt.maxMiB)
			}
		})
	}
}

func expectStateLimit(t *testing.T, err error) {
	t.Helper()
	expectCode(t, err, xsderrors.CodeSchemaLimit)
	if x, _ := errors.AsType[*xsderrors.Error](err); x.Message() != "content model DFA state limit exceeded" {
		t.Fatalf("Compile() error = %v, want the DFA state limit", err)
	}
}

func BenchmarkCompileNestedCountedSequences(b *testing.B) {
	for _, tt := range []struct {
		name    string
		depth   int
		nesting nestedCountedSequence
	}{
		{name: "state-limit", depth: 17, nesting: nestedAfterChoice},
		{name: "near-state-limit", depth: 12, nesting: nestedInChoice},
	} {
		b.Run(tt.name, func(b *testing.B) {
			text := []byte(nestedCountedSequenceSchema(tt.depth, tt.nesting))
			b.ReportAllocs()
			for b.Loop() {
				_, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("nested.xsd", text)})
				if tt.name != "state-limit" && err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
