package compiler

import (
	"testing"

	"github.com/jacoelho/xsd/internal/schemaast"
)

func TestTrackingAppendDeduplicatesByKindAndTarget(t *testing.T) {
	t.Parallel()

	var tracking Tracking[string]
	first := tracking.Append(Directive[string]{Kind: schemaast.DirectiveImport, TargetKey: "a"})
	second := tracking.Append(Directive[string]{Kind: schemaast.DirectiveImport, TargetKey: "a"})
	third := tracking.Append(Directive[string]{Kind: schemaast.DirectiveInclude, TargetKey: "a"})

	if !first || second == true || !third {
		t.Fatalf("append results = (%v, %v, %v), want (true, false, true)", first, second, third)
	}
	if len(tracking.Directives) != 2 {
		t.Fatalf("len(Directives) = %d, want 2", len(tracking.Directives))
	}
}

func TestTrackingDecrementUnderflow(t *testing.T) {
	t.Parallel()

	var tracking Tracking[string]
	if err := tracking.Decrement("schema.xsd"); err == nil {
		t.Fatal("Decrement() error = nil, want underflow error")
	}
}

func TestTrackingRemoveAndReset(t *testing.T) {
	t.Parallel()

	tracking := Tracking[string]{
		Directives: []Directive[string]{
			{Kind: schemaast.DirectiveImport, TargetKey: "a"},
			{Kind: schemaast.DirectiveInclude, TargetKey: "b"},
		},
		Count: 2,
	}

	tracking.Remove(schemaast.DirectiveImport, "a")
	if len(tracking.Directives) != 1 || tracking.Directives[0].TargetKey != "b" {
		t.Fatalf("Directives after Remove() = %#v, want only target b", tracking.Directives)
	}

	tracking.Clear()
	if len(tracking.Directives) != 0 || tracking.Count != 2 {
		t.Fatalf("tracking after Clear() = %+v, want directives cleared and count preserved", tracking)
	}

	tracking.Reset()
	if len(tracking.Directives) != 0 || tracking.Count != 0 {
		t.Fatalf("tracking after Reset() = %+v, want zero value", tracking)
	}
}
