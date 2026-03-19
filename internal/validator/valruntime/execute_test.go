package valruntime

import (
	"testing"

	"github.com/jacoelho/xsd/internal/runtime"
)

func TestHasLengthFacet(t *testing.T) {
	t.Parallel()

	meta := runtime.ValidatorMeta{
		Kind:   runtime.VString,
		Facets: runtime.FacetProgramRef{Off: 1, Len: 1},
	}
	code := []runtime.FacetInstr{
		{},
		{Op: runtime.FLength},
	}
	if !HasLengthFacet(meta, code) {
		t.Fatal("HasLengthFacet() = false, want true")
	}
}

func TestExecuteNoCanonical(t *testing.T) {
	t.Parallel()

	var order []string
	got, err := Execute(
		runtime.ValidatorMeta{Kind: runtime.VString},
		[]byte("lexical"),
		Options{},
		false,
		nil,
		ExecuteCallbacks[*State]{
			PrepareMetrics: func(plan Plan, state *State) (*State, bool) {
				order = append(order, "prepare")
				return state, false
			},
			Normalize: func(meta runtime.ValidatorMeta, lexical []byte, opts Options, plan Plan) ([]byte, func()) {
				order = append(order, "normalize")
				return []byte("normalized"), func() {
					order = append(order, "finish")
				}
			},
			ValidateNoCanonical: func(meta runtime.ValidatorMeta, normalized []byte, state *State) ([]byte, error) {
				order = append(order, "nocanon")
				if string(normalized) != "normalized" {
					t.Fatalf("normalized = %q, want normalized", normalized)
				}
				return []byte("ok"), nil
			},
			ValidateCanonical: func(runtime.ValidatorMeta, []byte, []byte, Plan, *State, bool) ([]byte, error) {
				t.Fatal("ValidateCanonical should not be called")
				return nil, nil
			},
		},
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if string(got) != "ok" {
		t.Fatalf("Execute() = %q, want ok", got)
	}
	want := []string{"prepare", "normalize", "nocanon", "finish"}
	if len(order) != len(want) {
		t.Fatalf("order = %#v, want %#v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("order = %#v, want %#v", order, want)
		}
	}
}

func TestExecuteCanonical(t *testing.T) {
	t.Parallel()

	var gotMetricsInternal bool
	inputState := &State{}
	got, err := Execute(
		runtime.ValidatorMeta{Kind: runtime.VQName},
		[]byte("lexical"),
		Options{RequireCanonical: true},
		false,
		inputState,
		ExecuteCallbacks[*State]{
			PrepareMetrics: func(plan Plan, state *State) (*State, bool) {
				if !plan.NeedCanonical {
					t.Fatal("plan.NeedCanonical = false, want true")
				}
				return state, true
			},
			Normalize: func(meta runtime.ValidatorMeta, lexical []byte, opts Options, plan Plan) ([]byte, func()) {
				return []byte("normalized"), func() {}
			},
			ValidateNoCanonical: func(runtime.ValidatorMeta, []byte, *State) ([]byte, error) {
				t.Fatal("ValidateNoCanonical should not be called")
				return nil, nil
			},
			ValidateCanonical: func(meta runtime.ValidatorMeta, lexical, normalized []byte, plan Plan, state *State, metricsInternal bool) ([]byte, error) {
				if string(lexical) != "lexical" || string(normalized) != "normalized" {
					t.Fatalf("got lexical=%q normalized=%q", lexical, normalized)
				}
				if state != inputState {
					t.Fatalf("state = %p, want %p", state, inputState)
				}
				gotMetricsInternal = metricsInternal
				return []byte("canonical"), nil
			},
		},
	)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if string(got) != "canonical" {
		t.Fatalf("Execute() = %q, want canonical", got)
	}
	if !gotMetricsInternal {
		t.Fatal("metricsInternal = false, want true")
	}
}
