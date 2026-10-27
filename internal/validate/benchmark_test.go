package validate

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/value"
)

func BenchmarkRecordIdentityValueIDREFS(b *testing.B) {
	for _, refs := range []int{1, 10, 100, 1000} {
		b.Run(fmt.Sprintf("refs_%d", refs), func(b *testing.B) {
			program, err := value.NewBuilder(value.BuilderOptions{}).Seal()
			if err != nil {
				b.Fatal(err)
			}
			id, ok := value.BuiltinTypeID("IDREFS")
			if !ok {
				b.Fatal("BuiltinTypeID(IDREFS) failed")
			}
			validated, err := program.Validate(id, benchmarkIDREFS(refs), value.Resolver{}, value.NeedIdentity, defaultMaxInstanceValueWork, nil)
			if err != nil {
				b.Fatal(err)
			}
			recorder := NewIdentityRecorderForTest()
			recorder.PushPath("root")
			recorder.PushPath("refs")
			if path := recorder.PathString(); path != "/root/refs" {
				b.Fatalf("PathString() = %q, want /root/refs", path)
			}
			if activateErr := recorder.session.doc.identity.activateDocumentIdentity(recorder.session.doc.Depth()); activateErr != nil {
				b.Fatal(activateErr)
			}
			if recordErr := recorder.RecordIdentityValue(validated, 1, 1); recordErr != nil {
				b.Fatal(recordErr)
			}
			identity := &recorder.session.doc.identity
			if identity.entries != refs || len(identity.idrefs) != refs {
				b.Fatalf("recorded entries/refs = %d/%d, want %d/%d", identity.entries, len(identity.idrefs), refs, refs)
			}
			for i, ref := range identity.idrefs {
				if ref.Value != "id"+strconv.Itoa(i) || ref.Path.String() != "/root/refs" || ref.Line != 1 || ref.Col != 1 {
					b.Fatalf("reference %d = %q at %s:%d:%d", i, ref.Value, ref.Path.String(), ref.Line, ref.Col)
				}
			}
			recorder.ResetIdentity()
			b.ReportAllocs()
			for b.Loop() {
				recorder.ResetIdentity()
				if err := recorder.RecordIdentityValue(validated, 1, 1); err != nil {
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
			opts := Options{MaxInstanceDepth: depth}
			b.ReportAllocs()
			b.SetBytes(int64(len(xml)))
			for b.Loop() {
				if err := CheckXMLWellFormed(strings.NewReader(xml), opts); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkNestedIdentityTablePropagation(b *testing.B) {
	const constraint xsdSchema.IdentityConstraintID = 1
	for _, depth := range []int{16, 64, 256} {
		b.Run(fmt.Sprintf("depth_%d", depth), func(b *testing.B) {
			keys := make([]string, depth)
			for i := range keys {
				keys[i] = strconv.Itoa(i)
			}
			b.ReportAllocs()
			for b.Loop() {
				scopes := make([]identityScope, depth)
				for i := range scopes {
					scopeDepth := i + 1
					scopes[i].depth = scopeDepth
					scopes[i].tables = map[xsdSchema.IdentityConstraintID]map[string]identityTableEntry{
						constraint: {keys[i]: {node: uint64(scopeDepth), originDepth: scopeDepth}},
					}
				}
				var evaluation identityEvaluation
				evaluation.scopes = scopes
				for len(evaluation.scopes) > 1 {
					last := len(evaluation.scopes) - 1
					if _, err := evaluation.closeScopes(evaluation.scopes[last].depth, nil); err != nil {
						b.Fatal(err)
					}
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
