package valruntime

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/facets"
	"github.com/jacoelho/xsd/internal/runtime"
)

// ExecuteCallbacks supplies the caller-owned runtime operations needed to run
// one value-validation request.
type ExecuteCallbacks[M any] struct {
	PrepareMetrics      func(Plan, M) (M, bool)
	Normalize           func(runtime.ValidatorMeta, []byte, Options, Plan) ([]byte, func())
	ValidateNoCanonical func(runtime.ValidatorMeta, []byte, M) ([]byte, error)
	ValidateCanonical   func(runtime.ValidatorMeta, []byte, []byte, Plan, M, bool) ([]byte, error)
}

// HasLengthFacet reports whether one validator facet program contains length checks.
func HasLengthFacet(meta runtime.ValidatorMeta, facetCode []runtime.FacetInstr) bool {
	if meta.Facets.Len == 0 {
		return false
	}
	ok, err := facets.RuntimeProgramHasOp(meta, facetCode, runtime.FLength, runtime.FMinLength, runtime.FMaxLength)
	return err == nil && ok
}

// Execute runs the generic value-validation control flow around one validator request.
func Execute[M any](
	meta runtime.ValidatorMeta,
	lexical []byte,
	opts Options,
	hasLengthFacet bool,
	state M,
	callbacks ExecuteCallbacks[M],
) ([]byte, error) {
	plan := Build(meta, opts, hasLengthFacet)

	metricsState := state
	metricsInternal := false
	if callbacks.PrepareMetrics != nil {
		metricsState, metricsInternal = callbacks.PrepareMetrics(plan, state)
	}

	normalized := lexical
	finishNormalize := func() {}
	if callbacks.Normalize != nil {
		normalized, finishNormalize = callbacks.Normalize(meta, lexical, opts, plan)
	}
	defer finishNormalize()

	if !plan.NeedCanonical {
		if callbacks.ValidateNoCanonical == nil {
			return nil, fmt.Errorf("missing no-canonical validator")
		}
		return callbacks.ValidateNoCanonical(meta, normalized, metricsState)
	}
	if callbacks.ValidateCanonical == nil {
		return nil, fmt.Errorf("missing canonical validator")
	}
	return callbacks.ValidateCanonical(meta, lexical, normalized, plan, metricsState, metricsInternal)
}
