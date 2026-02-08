package source

import (
	"errors"
	"testing"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/parser"
)

type mergeCall struct {
	kind     loadmerge.Kind
	remap    loadmerge.NamespaceRemapMode
	insertAt int
}

type stubMerger struct {
	err   error
	calls []mergeCall
}

func (m *stubMerger) Merge(target, source *parser.Schema, kind loadmerge.Kind, remap loadmerge.NamespaceRemapMode, insertAt int) error {
	m.calls = append(m.calls, mergeCall{kind: kind, remap: remap, insertAt: insertAt})
	return m.err
}

func TestSchemaLoaderMergeSchemaUsesConfiguredMerger(t *testing.T) {
	expectedErr := errors.New("merge failed")
	merger := &stubMerger{err: expectedErr}
	loader := NewLoader(Config{Merger: merger})

	err := loader.mergeSchema(parser.NewSchema(), parser.NewSchema(), loadmerge.MergeInclude, loadmerge.KeepNamespace, 3)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("mergeSchema error = %v, want %v", err, expectedErr)
	}
	if len(merger.calls) != 1 {
		t.Fatalf("expected 1 merge call, got %d", len(merger.calls))
	}
	call := merger.calls[0]
	if call.kind != loadmerge.MergeInclude || call.remap != loadmerge.KeepNamespace || call.insertAt != 3 {
		t.Fatalf("merge call = %+v, want include/keep/3", call)
	}
}
