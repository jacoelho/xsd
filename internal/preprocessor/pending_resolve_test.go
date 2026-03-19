package preprocessor

import (
	"testing"

	"github.com/jacoelho/xsd/internal/parser"
)

func TestResolve(t *testing.T) {
	t.Parallel()

	var order []string
	tracking := &Tracking[string]{
		Directives: []Directive[string]{
			{Kind: parser.DirectiveImport, TargetKey: "target"},
		},
		Count: 3,
	}

	err := ResolvePending("source", PendingResolveCallbacks[string, string, []string]{
		Inputs: func(sourceKey string) (*Tracking[string], []Directive[string], string, error) {
			order = append(order, "inputs:"+sourceKey)
			return tracking, tracking.Directives, "schema", nil
		},
		Stage: func(directives []Directive[string]) ([]string, error) {
			order = append(order, "stage")
			return []string{"staged"}, nil
		},
		Apply: func(directives []Directive[string], source string, staged []string) error {
			order = append(order, "apply:"+source+":"+staged[0])
			return nil
		},
		Commit: func(staged []string) error {
			order = append(order, "commit:"+staged[0])
			return nil
		},
		MarkMerged: func(sourceKey string, directives []Directive[string]) {
			order = append(order, "merged:"+sourceKey)
		},
		ResolveTargets: func(directives []Directive[string]) error {
			order = append(order, "resolve-targets")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(tracking.Directives) != 0 || tracking.Count != 3 {
		t.Fatalf("tracking after Resolve() = %+v, want directives cleared and count preserved", tracking)
	}
	want := []string{
		"inputs:source",
		"stage",
		"apply:schema:staged",
		"commit:staged",
		"merged:source",
		"resolve-targets",
	}
	if len(order) != len(want) {
		t.Fatalf("order = %#v, want %#v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %#v, want %#v", order, want)
		}
	}
}

func TestResolveTargets(t *testing.T) {
	t.Parallel()

	tracking := map[string]*Tracking[string]{
		"a": {Count: 1},
		"b": {Count: 2},
	}
	var resolved []string

	err := ResolvePendingTargets([]Directive[string]{
		{TargetKey: "a"},
		{TargetKey: "b"},
	}, PendingTargetCallbacks[string]{
		Tracking: func(key string) (*Tracking[string], error) {
			return tracking[key], nil
		},
		Resolve: func(key string) error {
			resolved = append(resolved, key)
			return nil
		},
		Label: func(key string) string {
			return key
		},
	})
	if err != nil {
		t.Fatalf("ResolveTargets() error = %v", err)
	}
	if tracking["a"].Count != 0 || tracking["b"].Count != 1 {
		t.Fatalf("counts = (%d, %d), want (0, 1)", tracking["a"].Count, tracking["b"].Count)
	}
	if len(resolved) != 1 || resolved[0] != "a" {
		t.Fatalf("resolved = %#v, want [\"a\"]", resolved)
	}
}

func TestApply(t *testing.T) {
	t.Parallel()

	staged := map[string]string{
		"a": "include-target",
		"b": "import-target",
	}
	var calls []string

	err := ApplyPending([]Directive[string]{
		{Kind: parser.DirectiveInclude, TargetKey: "a"},
		{Kind: parser.DirectiveImport, TargetKey: "b"},
	}, "source", staged, PendingApplyCallbacks[string, string, map[string]string, string]{
		Target: func(staged map[string]string, key string) (string, error) {
			return staged[key], nil
		},
		Include: func(directive Directive[string], source, target string) error {
			calls = append(calls, "include:"+source+":"+target)
			return nil
		},
		Import: func(directive Directive[string], source, target string) error {
			calls = append(calls, "import:"+source+":"+target)
			return nil
		},
	})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	want := []string{"include:source:include-target", "import:source:import-target"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %#v, want %#v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls = %#v, want %#v", calls, want)
		}
	}
}
