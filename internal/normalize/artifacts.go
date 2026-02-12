package normalize

import (
	"iter"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complextypeplan"
	"github.com/jacoelho/xsd/internal/model"
	expparser "github.com/jacoelho/xsd/internal/parser"
)

// Artifacts stores normalized schema artifacts for runtime compilation.
type Artifacts struct {
	schema       *expparser.Schema
	registry     *analysis.Registry
	refs         *analysis.ResolvedReferences
	complexTypes *complextypeplan.Plan
}

// Schema returns the normalized schema graph.
func (a *Artifacts) Schema() *expparser.Schema {
	if a == nil {
		return nil
	}
	return a.schema
}

// Registry returns the normalized registry.
func (a *Artifacts) Registry() *analysis.Registry {
	if a == nil {
		return nil
	}
	return a.registry
}

// References returns resolved references.
func (a *Artifacts) References() *analysis.ResolvedReferences {
	if a == nil {
		return nil
	}
	return a.refs
}

// ComplexTypes returns the precomputed complex type plan.
func (a *Artifacts) ComplexTypes() *complextypeplan.Plan {
	if a == nil {
		return nil
	}
	return a.complexTypes
}

// GlobalElementOrderSeq yields deterministic global element order.
func (a *Artifacts) GlobalElementOrderSeq() iter.Seq[model.QName] {
	return func(yield func(model.QName) bool) {
		if a == nil || a.registry == nil {
			return
		}
		for _, entry := range a.registry.ElementOrder {
			if !entry.Global {
				continue
			}
			if !yield(entry.QName) {
				return
			}
		}
	}
}
