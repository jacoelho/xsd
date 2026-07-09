package validate

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func BenchmarkRecordIdentityValueIDREFS(b *testing.B) {
	for _, refs := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("refs_%d", refs), func(b *testing.B) {
			value := runtime.SimpleValue{IDRefs: benchmarkIDREFS(refs)}
			recorder := NewIdentityRecorderForTest()
			recorder.PushPath("root")
			recorder.PushPath("refs")
			if path := recorder.PathString(); path != "/root/refs" {
				b.Fatalf("PathString() = %q, want /root/refs", path)
			}
			b.ReportAllocs()
			for b.Loop() {
				recorder.ResetIdentity()
				if err := recorder.RecordIdentityValue(value, 1, 1); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkCheckXMLWellFormedNested(b *testing.B) {
	for _, depth := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("depth_%d", depth), func(b *testing.B) {
			xml := strings.Repeat("<a>", depth) + strings.Repeat("</a>", depth)
			b.ReportAllocs()
			b.SetBytes(int64(len(xml)))
			for b.Loop() {
				if err := CheckXMLWellFormed(strings.NewReader(xml), Options{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func benchmarkIDREFS(refs int) string {
	var b strings.Builder
	for i := range refs {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString("id")
		b.WriteString(strconv.Itoa(i))
	}
	return b.String()
}
