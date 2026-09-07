package validate

import (
	"strconv"
	"strings"
	"testing"

	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/source"
)

func TestIdentityDispatchResetBoundsRetainedState(t *testing.T) {
	t.Run("active slice at bound reset", func(t *testing.T) {
		var evaluation identityEvaluation
		evaluation.dispatch.activeByConstraint = map[xsdSchema.IdentityConstraintID][]identityActiveScope{
			1: make([]identityActiveScope, 1, maxRetainedSliceCap),
		}

		evaluation.resetIdentityDispatch(maxRetainedMapLen, maxRetainedSliceCap)

		active, retained := evaluation.dispatch.activeByConstraint[1]
		if !retained || len(active) != 0 || cap(active) != maxRetainedSliceCap {
			t.Fatalf("reset active slice: retained=%t len=%d cap=%d, want retained with len=0 cap=%d", retained, len(active), cap(active), maxRetainedSliceCap)
		}
	})

	tests := []struct {
		name           string
		shape          string
		size           int
		cleanup        string
		retainMap      bool
		retainActive   bool
		retainSelector bool
	}{
		{
			name:           "nested below bound reset",
			shape:          "nested-scopes",
			size:           maxRetainedSliceCap / 16,
			cleanup:        "reset",
			retainMap:      true,
			retainActive:   true,
			retainSelector: true,
		},
		{
			name:           "nested above bound reset",
			shape:          "nested-scopes",
			size:           maxRetainedSliceCap + 1,
			cleanup:        "reset",
			retainMap:      true,
			retainSelector: true,
		},
		{
			name:           "nested above bound discard",
			shape:          "nested-scopes",
			size:           maxRetainedSliceCap + 1,
			cleanup:        "discard",
			retainMap:      true,
			retainSelector: true,
		},
		{
			name:           "wide below bound reset",
			shape:          "wide-selectors",
			size:           maxRetainedMapLen / 16,
			cleanup:        "reset",
			retainMap:      true,
			retainActive:   true,
			retainSelector: true,
		},
		{
			name:    "wide above bounds reset",
			shape:   "wide-selectors",
			size:    maxRetainedMapLen + 1,
			cleanup: "reset",
		},
		{
			name:    "wide above bounds discard",
			shape:   "wide-selectors",
			size:    maxRetainedMapLen + 1,
			cleanup: "discard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rt := compileRuntimeForTest(t, identityDispatchRetentionSchema(tt.shape, tt.size))
			session, err := newSessionForTest(rt, Options{
				MaxInstanceDepth:   tt.size + 2,
				MaxIdentityScopes:  tt.size + 2,
				MaxIdentityEntries: tt.size + 2,
			})
			if err != nil {
				t.Fatal(err)
			}
			validateUnclosedIdentityDocument(t, session, identityDispatchRetentionDocument(tt.shape, tt.size))

			before := identityDispatchRetentionStateForTest(&session.session.doc.identity)
			if before.mapLen == 0 || before.activeCap == 0 {
				t.Fatalf("live dispatch state = %+v, want populated state", before)
			}
			if !tt.retainMap && before.mapLen <= maxRetainedMapLen {
				t.Fatalf("live dispatch map length = %d, want above %d", before.mapLen, maxRetainedMapLen)
			}

			switch tt.cleanup {
			case "reset":
				session.session.reset()
			case "discard":
				session.session.doc.identity.discard()
			default:
				t.Fatalf("unknown cleanup %q", tt.cleanup)
			}
			after := identityDispatchRetentionStateForTest(&session.session.doc.identity)
			if after.mapNil == tt.retainMap {
				t.Fatalf("%s dispatch map nil = %t, want retained=%t (state %+v)", tt.cleanup, after.mapNil, tt.retainMap, after)
			}
			if tt.retainMap && after.mapLen != before.mapLen {
				t.Fatalf("%s dispatch map length = %d, want unchanged %d (state %+v)", tt.cleanup, after.mapLen, before.mapLen, after)
			}
			if tt.retainActive {
				if after.activeCap != before.activeCap || after.activeCap > maxRetainedSliceCap {
					t.Fatalf("%s active-scope capacity = %d, want unchanged %d at or below %d (state %+v)", tt.cleanup, after.activeCap, before.activeCap, maxRetainedSliceCap, after)
				}
			} else if after.activeCap != 0 {
				t.Fatalf("%s active-scope capacity = %d, want dropped (state %+v)", tt.cleanup, after.activeCap, after)
			}
			if tt.retainSelector {
				if after.selectorCap != before.selectorCap || after.selectorCap > maxRetainedSliceCap {
					t.Fatalf("%s selector capacity = %d, want unchanged %d at or below %d (state %+v)", tt.cleanup, after.selectorCap, before.selectorCap, maxRetainedSliceCap, after)
				}
			} else if after.selectorCap != 0 {
				t.Fatalf("%s selector capacity = %d, want dropped (state %+v)", tt.cleanup, after.selectorCap, after)
			}

			session.session.reset()
			if err := session.Validate(strings.NewReader(`<root id="ok"/>`)); err != nil {
				t.Fatalf("Validate() after %s: %v", tt.cleanup, err)
			}
		})
	}
}

func BenchmarkIdentityDispatchResetIncomplete(b *testing.B) {
	tests := []struct {
		name  string
		shape string
		size  int
	}{
		{name: "nested/256", shape: "nested-scopes", size: 256},
		{name: "nested/4096", shape: "nested-scopes", size: 4096},
		{name: "nested/8192", shape: "nested-scopes", size: 8192},
		{name: "wide/256", shape: "wide-selectors", size: 256},
		{name: "wide/4096", shape: "wide-selectors", size: 4096},
		{name: "wide/8192", shape: "wide-selectors", size: 8192},
	}
	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			rt := compileIdentityDispatchRuntimeForBenchmark(b, identityDispatchRetentionSchema(tt.shape, tt.size))
			session, err := newSessionForTest(rt, Options{
				MaxInstanceDepth:   tt.size + 2,
				MaxIdentityScopes:  tt.size + 2,
				MaxIdentityEntries: tt.size + 2,
			})
			if err != nil {
				b.Fatal(err)
			}
			document := identityDispatchRetentionDocument(tt.shape, tt.size)
			// Exclude the first-call allocation from the recurring reset measurement.
			if err := session.Validate(strings.NewReader(document)); err == nil {
				b.Fatal("incomplete identity document unexpectedly validated")
			}
			b.SetBytes(int64(len(document)))
			b.ReportAllocs()
			for b.Loop() {
				if err := session.Validate(strings.NewReader(document)); err == nil {
					b.Fatal("incomplete identity document unexpectedly validated")
				}
			}
			b.ReportMetric(1, "errors/op")
		})
	}
}

func compileIdentityDispatchRuntimeForBenchmark(tb testing.TB, schema string) *xsdSchema.Schema {
	tb.Helper()
	rt, err := xsdSchema.Compile(xsdSchema.Options{}, []source.Source{source.Bytes("identity-dispatch-benchmark.xsd", []byte(schema))})
	if err != nil {
		tb.Fatal(err)
	}
	return rt
}

type identityDispatchRetentionState struct {
	mapNil      bool
	mapLen      int
	activeCap   int
	selectorCap int
}

func identityDispatchRetentionStateForTest(e *identityEvaluation) identityDispatchRetentionState {
	state := identityDispatchRetentionState{
		mapNil:      e.dispatch.activeByConstraint == nil,
		mapLen:      len(e.dispatch.activeByConstraint),
		selectorCap: cap(e.dispatch.selectorHits),
	}
	for _, active := range e.dispatch.activeByConstraint {
		if cap(active) > state.activeCap {
			state.activeCap = cap(active)
		}
	}
	return state
}

func validateUnclosedIdentityDocument(t *testing.T, session *Session, document string) {
	t.Helper()
	if err := session.session.resetParser(strings.NewReader(document)); err != nil {
		t.Fatal(err)
	}
	if err := session.session.validateTokens(); err == nil {
		t.Fatal("unclosed identity document unexpectedly validated")
	}
	session.session.reader.Detach()
}

func identityDispatchRetentionSchema(shape string, size int) string {
	var schema strings.Builder
	schema.WriteString(`<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema"><xs:element name="root">`)
	if shape == "nested-scopes" {
		schema.WriteString(`<xs:complexType><xs:sequence><xs:element ref="root" minOccurs="0"/></xs:sequence><xs:attribute name="id" type="xs:string"/></xs:complexType>`)
	} else {
		schema.WriteString(`<xs:complexType><xs:attribute name="id" type="xs:string"/></xs:complexType>`)
	}
	constraintCount := size
	if shape == "nested-scopes" {
		constraintCount = 1
	}
	for i := range constraintCount {
		schema.WriteString(`<xs:key name="k`)
		schema.WriteString(strconv.Itoa(i))
		schema.WriteString(`"><xs:selector xpath="."/><xs:field xpath="@id"/></xs:key>`)
	}
	schema.WriteString(`</xs:element></xs:schema>`)
	return schema.String()
}

func identityDispatchRetentionDocument(shape string, size int) string {
	if shape != "nested-scopes" {
		return `<root id="x">`
	}
	var document strings.Builder
	document.Grow(len(`<root id="x">`) * size)
	for range size {
		document.WriteString(`<root id="x">`)
	}
	return document.String()
}
