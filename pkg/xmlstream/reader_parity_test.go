package xmlstream

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/jacoelho/xsd/pkg/xmltext"
)

type streamTraceToken struct {
	kind       EventKind
	local      string
	text       string
	scopeDepth int
	attrs      []string
}

func TestNextNextRawNextResolvedParity(t *testing.T) {
	input := `<?xml version="1.0"?>
<!DOCTYPE root>
<?pi test?>
<!--lead-->
<root xmlns="urn:default" xmlns:p="urn:p" p:a="1" b="2">
  <p:child p:x="3">txt</p:child>
</root>`

	opts := []Option{xmltext.EmitComments(true), xmltext.EmitPI(true), xmltext.EmitDirectives(true)}
	traceEvent, err := collectEventTrace(input, opts...)
	if err != nil {
		t.Fatalf("collect event trace: %v", err)
	}
	traceRaw, err := collectRawTrace(input, opts...)
	if err != nil {
		t.Fatalf("collect raw trace: %v", err)
	}
	traceResolved, err := collectResolvedTrace(input, opts...)
	if err != nil {
		t.Fatalf("collect resolved trace: %v", err)
	}

	if len(traceEvent) != len(traceRaw) {
		t.Fatalf("event/raw lengths = %d/%d, want equal", len(traceEvent), len(traceRaw))
	}
	if len(traceEvent) != len(traceResolved) {
		t.Fatalf("event/resolved lengths = %d/%d, want equal", len(traceEvent), len(traceResolved))
	}

	for i := range traceEvent {
		if !equalStreamTraceToken(traceEvent[i], traceRaw[i]) {
			t.Fatalf("event/raw token %d mismatch: %#v != %#v", i, traceEvent[i], traceRaw[i])
		}
		if !equalStreamTraceToken(traceEvent[i], traceResolved[i]) {
			t.Fatalf("event/resolved token %d mismatch: %#v != %#v", i, traceEvent[i], traceResolved[i])
		}
	}
}

func equalStreamTraceToken(a, b streamTraceToken) bool {
	if a.kind != b.kind || a.local != b.local || a.text != b.text || a.scopeDepth != b.scopeDepth {
		return false
	}
	if len(a.attrs) != len(b.attrs) {
		return false
	}
	for i := range a.attrs {
		if a.attrs[i] != b.attrs[i] {
			return false
		}
	}
	return true
}

func collectEventTrace(input string, opts ...Option) ([]streamTraceToken, error) {
	r, err := NewReader(strings.NewReader(input), opts...)
	if err != nil {
		return nil, err
	}
	var out []streamTraceToken
	for {
		ev, err := r.Next()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, streamTraceToken{
			kind:       ev.Kind,
			local:      ev.Name.Local,
			text:       string(ev.Text),
			scopeDepth: ev.ScopeDepth,
			attrs:      eventAttrsTrace(ev.Attrs),
		})
	}
}

func collectRawTrace(input string, opts ...Option) ([]streamTraceToken, error) {
	r, err := NewReader(strings.NewReader(input), opts...)
	if err != nil {
		return nil, err
	}
	var out []streamTraceToken
	for {
		ev, err := r.NextRaw()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, streamTraceToken{
			kind:       ev.Kind,
			local:      string(ev.Name.Local),
			text:       string(ev.Text),
			scopeDepth: ev.ScopeDepth,
			attrs:      rawAttrsTrace(ev.Attrs),
		})
	}
}

func collectResolvedTrace(input string, opts ...Option) ([]streamTraceToken, error) {
	r, err := NewReader(strings.NewReader(input), opts...)
	if err != nil {
		return nil, err
	}
	var out []streamTraceToken
	for {
		ev, err := r.NextResolved()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		out = append(out, streamTraceToken{
			kind:       ev.Kind,
			local:      string(ev.Local),
			text:       string(ev.Text),
			scopeDepth: ev.ScopeDepth,
			attrs:      resolvedAttrsTrace(ev.Attrs),
		})
	}
}

func eventAttrsTrace(attrs []Attr) []string {
	out := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		out = append(out, attr.Name.Local+"="+string(attr.Value))
	}
	return out
}

func rawAttrsTrace(attrs []RawAttr) []string {
	out := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		out = append(out, string(attr.Name.Local)+"="+string(attr.Value))
	}
	return out
}

func resolvedAttrsTrace(attrs []ResolvedAttr) []string {
	out := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		out = append(out, string(attr.Local)+"="+string(attr.Value))
	}
	return out
}
