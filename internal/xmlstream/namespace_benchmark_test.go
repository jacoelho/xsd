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
					frames := make([]frame, depth)
					contexts := make([]Context, depth)
					var namespaceStack stack
					var values cache
					b.ReportAllocs()
					for b.Loop() {
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
				frames := make([]frame, depth)
				var namespaceStack stack
				var values cache
				b.ReportAllocs()
				for b.Loop() {
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
