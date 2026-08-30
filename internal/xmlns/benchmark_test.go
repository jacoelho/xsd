package xmlns

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
					frames := make([]Frame, depth)
					contexts := make([]Context, depth)
					var stack Stack
					b.ReportAllocs()
					for b.Loop() {
						stack.Reset(depth * 2)
						for i := range starts {
							frame, _, err := stack.StartXML(starts[i])
							if err != nil {
								b.Fatal(err)
							}
							frames[i] = frame
							if persistent {
								contexts[i] = stack.Context()
							}
						}
						for _, frame := range slices.Backward(frames) {
							if err := stack.End(frame, LexicalName{Local: "e"}); err != nil {
								b.Fatal(err)
							}
						}
					}
				})
			}
		}
	}
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

func namespaceBenchmarkStarts(depth int, declarations namespaceBenchmarkDeclarationMode) []xml.StartElement {
	starts := make([]xml.StartElement, depth)
	for i := range starts {
		starts[i].Name.Local = "e"
		if declarations == namespaceBenchmarkDeclarations {
			starts[i].Attr = []xml.Attr{{
				Name:  xml.Name{Space: "xmlns", Local: "p" + strconv.Itoa(i)},
				Value: "urn:" + strconv.Itoa(i),
			}}
		}
	}
	return starts
}
