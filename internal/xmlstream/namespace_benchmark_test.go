package xmlstream

import (
	"encoding/xml"
	"fmt"
	"slices"
	"strconv"
	"testing"
)

func BenchmarkNamespaceAdmissionChurn(b *testing.B) {
	for _, depth := range []int{16, 64, 256} {
		for _, declarations := range []namespaceBenchmarkDeclarationMode{namespaceBenchmarkNoDeclarations, namespaceBenchmarkDeclarations} {
			for _, persistent := range []bool{false, true} {
				name := fmt.Sprintf("depth_%d/%s/persistent_%t", depth, declarations, persistent)
				b.Run(name, func(b *testing.B) {
					starts := namespaceBenchmarkStarts(depth, declarations)
					lexicalAttrs := namespaceBenchmarkLexicalAttrs(starts)
					frames := make([]frame, depth)
					contexts := make([]Context, depth)
					var namespaceStack stack
					var values cache
					b.ReportAllocs()
					for b.Loop() {
						restoreNamespaceBenchmarkLexicalAttrs(starts, lexicalAttrs)
						namespaceStack.Reset(depth * 2)
						for i := range starts {
							admitted, _, err := namespaceStack.StartStream(&starts[i], &values)
							if err != nil {
								b.Fatal(err)
							}
							frames[i] = admitted
							if persistent {
								contexts[i] = namespaceStack.Context()
							}
						}
						for _, admitted := range slices.Backward(frames) {
							if err := namespaceStack.End(admitted, LexicalName{Local: "e"}); err != nil {
								b.Fatal(err)
							}
						}
					}
				})
			}
		}
	}
}

func BenchmarkNamespaceDefaultResolveChurn(b *testing.B) {
	for _, depth := range []int{16, 64, 256} {
		for _, shadow := range []namespaceBenchmarkShadowMode{namespaceBenchmarkInherited, namespaceBenchmarkShadowed} {
			name := fmt.Sprintf("depth_%d/%s", depth, shadow)
			b.Run(name, func(b *testing.B) {
				starts := namespaceBenchmarkDefaultStarts(depth, shadow)
				lexicalAttrs := namespaceBenchmarkLexicalAttrs(starts)
				frames := make([]frame, depth)
				var namespaceStack stack
				var values cache
				b.ReportAllocs()
				for b.Loop() {
					restoreNamespaceBenchmarkLexicalAttrs(starts, lexicalAttrs)
					namespaceStack.Reset(depth * 2)
					for i := range starts {
						admitted, _, err := namespaceStack.StartStream(&starts[i], &values)
						if err != nil {
							b.Fatal(err)
						}
						frames[i] = admitted
					}
					for _, admitted := range slices.Backward(frames) {
						if err := namespaceStack.End(admitted, LexicalName{Local: "item"}); err != nil {
							b.Fatal(err)
						}
					}
				}
			})
		}
	}
}

type namespaceBenchmarkShadowMode uint8

const (
	namespaceBenchmarkInherited namespaceBenchmarkShadowMode = iota
	namespaceBenchmarkShadowed
)

func (m namespaceBenchmarkShadowMode) String() string {
	if m == namespaceBenchmarkShadowed {
		return "shadow_true"
	}
	return "shadow_false"
}

type namespaceBenchmarkDeclarationMode uint8

const (
	namespaceBenchmarkNoDeclarations namespaceBenchmarkDeclarationMode = iota
	namespaceBenchmarkDeclarations
)

func (m namespaceBenchmarkDeclarationMode) String() string {
	if m == namespaceBenchmarkDeclarations {
		return "churn_true"
	}
	return "churn_false"
}

func namespaceBenchmarkStarts(depth int, declarations namespaceBenchmarkDeclarationMode) []StartElement {
	starts := make([]StartElement, depth)
	for i := range starts {
		starts[i].Name.Local = "e"
		if declarations == namespaceBenchmarkDeclarations {
			starts[i].Attr = []Attr{{
				Name:  xml.Name{Space: "xmlns", Local: "p" + strconv.Itoa(i)},
				Value: "urn:" + strconv.Itoa(i),
			}}
		}
	}
	return starts
}

func namespaceBenchmarkDefaultStarts(depth int, shadow namespaceBenchmarkShadowMode) []StartElement {
	starts := make([]StartElement, depth)
	for i := range starts {
		starts[i].Name.Local = "item"
		if i == 0 || shadow == namespaceBenchmarkShadowed {
			starts[i].Attr = []Attr{{
				Name:  xml.Name{Local: "xmlns"},
				Value: fmt.Sprintf("urn:default:%d", i),
			}}
		}
	}
	return starts
}

// StartStream expands attribute names in place. Benchmarks reuse the same
// starts to avoid measuring setup allocations, so keep the lexical spellings
// in a flat snapshot and restore only names between iterations.
func namespaceBenchmarkLexicalAttrs(starts []StartElement) []xml.Name {
	var count int
	for i := range starts {
		count += len(starts[i].Attr)
	}
	names := make([]xml.Name, 0, count)
	for i := range starts {
		for j := range starts[i].Attr {
			names = append(names, starts[i].Attr[j].Name)
		}
	}
	return names
}

func restoreNamespaceBenchmarkLexicalAttrs(starts []StartElement, names []xml.Name) {
	index := 0
	for i := range starts {
		for j := range starts[i].Attr {
			starts[i].Attr[j].Name = names[index]
			index++
		}
	}
}

const scratchOppParentURI = "urn:parent"

type scratchOppEvent struct {
	lexical   []Attr
	attrs     []Attr
	wantError string
}

type scratchOppFixture struct {
	namespaces stack
	values     cache
	parent     frame
	saved      Context
	events     []scratchOppEvent
}

func BenchmarkNamespaceScratchLifecycle(b *testing.B) {
	cases := []struct {
		name    string
		widths  []int
		failure string
	}{
		{name: "warm16", widths: []int{16}},
		{name: "warm17", widths: []int{17}},
		{name: "warm64", widths: []int{64}},
		{name: "wide_empty_small", widths: []int{64, 0, 1, 17, 64}},
		{name: "duplicate_recovery", widths: []int{0, 1, 64}, failure: "duplicate"},
		{name: "unbound_recovery", widths: []int{0, 1, 64}, failure: "unbound"},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			fixture := newScratchOppFixture(b, tc.widths, tc.failure)
			fixture.run(b, true)
			b.ReportAllocs()
			for b.Loop() {
				fixture.run(b, false)
			}
			b.StopTimer()
			if err := fixture.namespaces.End(fixture.parent, LexicalName{Local: "root"}); err != nil {
				b.Fatal(err)
			}
			fixture.namespaces.Reset(64)
			assertScratchOppContext(b, fixture.saved)
		})
	}
}

func newScratchOppFixture(b *testing.B, widths []int, failure string) scratchOppFixture {
	b.Helper()
	fixture := scratchOppFixture{}
	root := StartElement{Name: xml.Name{Local: "root"}, Attr: []Attr{
		{Name: xml.Name{Space: "xmlns", Local: "p"}, Value: scratchOppParentURI},
		{Name: xml.Name{Space: "xmlns", Local: "q"}, Value: scratchOppParentURI},
	}}
	parent, name, err := fixture.namespaces.StartStream(&root, &fixture.values)
	if err != nil {
		b.Fatal(err)
	}
	if name != (xml.Name{Local: "root"}) {
		b.Fatalf("root name = %#v", name)
	}
	fixture.parent = parent
	fixture.saved = fixture.namespaces.Context()
	if failure != "" {
		attrs := []Attr{
			{Name: xml.Name{Space: "xmlns", Local: "p"}, Value: "urn:shadow"},
			{Name: xml.Name{Space: "xmlns", Local: "q"}, Value: "urn:shadow"},
		}
		attrs = append(attrs, scratchOppAttributes(16)...)
		event := scratchOppEvent{}
		if failure == "duplicate" {
			attrs = append(attrs, Attr{Name: xml.Name{Space: "q", Local: "a0"}, Value: "value"})
			event.wantError = "duplicate attribute {urn:shadow}a0"
		} else {
			attrs = append(attrs, Attr{Name: xml.Name{Space: "missing", Local: "bad"}, Value: "value"})
			event.wantError = "unbound namespace prefix missing"
		}
		event.lexical = attrs
		event.attrs = make([]Attr, len(attrs))
		fixture.events = append(fixture.events, event)
	}
	for _, width := range widths {
		attrs := scratchOppAttributes(width)
		fixture.events = append(fixture.events, scratchOppEvent{lexical: attrs, attrs: make([]Attr, len(attrs))})
	}
	return fixture
}

func scratchOppAttributes(width int) []Attr {
	attrs := make([]Attr, width)
	for i := range attrs {
		prefix := "p"
		if i%2 != 0 {
			prefix = "q"
		}
		attrs[i] = Attr{Name: xml.Name{Space: prefix, Local: "a" + strconv.Itoa(i)}, Value: "value"}
	}
	return attrs
}

//nolint:revive,thelper // One kernel serves untimed verification and timing; test-stack tracking stays outside measurement.
func (fixture *scratchOppFixture) run(b *testing.B, verify bool) {
	for i := range fixture.events {
		event := &fixture.events[i]
		copy(event.attrs, event.lexical)
		start := StartElement{Name: xml.Name{Local: "child"}, Attr: event.attrs}
		admitted, name, err := fixture.namespaces.StartStream(&start, &fixture.values)
		if event.wantError != "" {
			if err == nil {
				b.Fatal("expected namespace admission failure")
			}
			if verify {
				assertScratchOppFailure(b, event, start, err)
			}
		} else {
			if err != nil {
				b.Fatal(err)
			}
			if verify {
				assertScratchOppSuccess(b, event, start, name)
			}
			if endErr := fixture.namespaces.End(admitted, LexicalName{Local: "child"}); endErr != nil {
				b.Fatal(endErr)
			}
		}
		if verify {
			if matchErr := fixture.namespaces.MatchEnd(fixture.parent, LexicalName{Local: "root"}); matchErr != nil {
				b.Fatal(matchErr)
			}
			assertScratchOppContext(b, fixture.namespaces.Context())
			assertScratchOppContext(b, fixture.saved)
		}
	}
}

func assertScratchOppFailure(b *testing.B, event *scratchOppEvent, start StartElement, err error) {
	b.Helper()
	if err.Error() != event.wantError {
		b.Fatalf("failure = %q, want %q", err, event.wantError)
	}
	for i, attr := range start.Attr {
		if attr.Name != event.lexical[i].Name || attr.Value != event.lexical[i].Value {
			b.Fatalf("failed attribute %d changed: %#v", i, attr)
		}
	}
}

func assertScratchOppSuccess(b *testing.B, event *scratchOppEvent, start StartElement, name xml.Name) {
	b.Helper()
	if name != (xml.Name{Local: "child"}) || len(start.Attr) != len(event.lexical) {
		b.Fatalf("admitted start = %#v", start)
	}
	for i, attr := range start.Attr {
		want := xml.Name{Space: scratchOppParentURI, Local: "a" + strconv.Itoa(i)}
		if attr.Name != want || attr.Value != "value" {
			b.Fatalf("admitted attribute %d = %#v, want %#v with value", i, attr, want)
		}
	}
}

func assertScratchOppContext(b *testing.B, context Context) {
	b.Helper()
	for _, prefix := range []string{"p", "q"} {
		if uri, ok := context.Lookup(prefix); !ok || uri != scratchOppParentURI {
			b.Fatalf("context prefix %q = %q/%t", prefix, uri, ok)
		}
	}
}
