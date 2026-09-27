package xsd_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jacoelho/xsd"
)

// BenchmarkCompileIndependentContentModels measures the cost of running the
// content-model checks over many unrelated global groups. The larger fixtures
// require explicit admission limits so the benchmark stays focused on the
// compiler path rather than the default token/work ceilings.
func BenchmarkCompileIndependentContentModels(b *testing.B) {
	for _, count := range []int{5_000, 20_000, 50_000, 100_000} {
		b.Run(fmt.Sprintf("models_%d", count), func(b *testing.B) {
			schema := independentContentModelsSchema(count)
			b.SetBytes(int64(len(schema)))
			b.ReportAllocs()
			opts := xsd.CompileOptions{
				MaxSchemaTokenBytes:          8 << 20,
				MaxContentModelAnalysisSteps: 2_000_000,
			}
			for b.Loop() {
				if _, err := xsd.CompileWithOptions(opts, xsd.Bytes("independent-models.xsd", schema)); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func independentContentModelsSchema(count int) []byte {
	var schema strings.Builder
	schema.Grow(count * 48)
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">`)
	for i := range count {
		fmt.Fprintf(&schema, `<xs:group name="g%d"><xs:sequence/></xs:group>`, i)
	}
	schema.WriteString(`</xs:schema>`)
	return []byte(schema.String())
}
