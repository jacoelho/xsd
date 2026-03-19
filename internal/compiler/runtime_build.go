package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/compiler/lower"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
)

// PreparedArtifacts stores immutable runtime-build prerequisites.
type PreparedArtifacts struct {
	schema     *parser.Schema
	registry   *analysis.Registry
	refs       *analysis.ResolvedReferences
	validators *lower.CompiledValidators
}

// BuildArtifacts compiles resolved semantic artifacts into a runtime schema model.
func BuildArtifacts(
	sch *parser.Schema,
	reg *analysis.Registry,
	refs *analysis.ResolvedReferences,
	validators *lower.CompiledValidators,
	cfg BuildConfig,
) (*runtime.Schema, error) {
	prepared, err := PrepareBuildArtifacts(sch, reg, refs, validators)
	if err != nil {
		return nil, err
	}
	return prepared.Build(cfg)
}

// PrepareBuildArtifacts packages compiler-owned artifacts for repeated runtime builds.
func PrepareBuildArtifacts(
	sch *parser.Schema,
	reg *analysis.Registry,
	refs *analysis.ResolvedReferences,
	validators *lower.CompiledValidators,
) (*PreparedArtifacts, error) {
	if err := validateBuildInputs(sch, reg, refs); err != nil {
		return nil, err
	}
	if validators == nil {
		return nil, fmt.Errorf("runtime build: validators are nil")
	}
	return &PreparedArtifacts{
		schema:     sch,
		registry:   reg,
		refs:       refs,
		validators: validators,
	}, nil
}

// Schema returns the prepared schema graph.
func (p *PreparedArtifacts) Schema() *parser.Schema {
	if p == nil {
		return nil
	}
	return p.schema
}

// Registry returns deterministic component IDs for the prepared schema.
func (p *PreparedArtifacts) Registry() *analysis.Registry {
	if p == nil {
		return nil
	}
	return p.registry
}

// References returns the resolved reference index for the prepared schema.
func (p *PreparedArtifacts) References() *analysis.ResolvedReferences {
	if p == nil {
		return nil
	}
	return p.refs
}

// Validators returns the compiled validator bundle for the prepared schema.
func (p *PreparedArtifacts) Validators() *lower.CompiledValidators {
	if p == nil {
		return nil
	}
	return p.validators
}

// Build compiles prepared artifacts into a runtime schema model.
func (p *PreparedArtifacts) Build(cfg BuildConfig) (*runtime.Schema, error) {
	if p == nil {
		return nil, fmt.Errorf("runtime build: prepared artifacts are nil")
	}
	return lower.Build(p.schema, p.registry, p.refs, p.validators, lower.Config{
		MaxDFAStates:   cfg.MaxDFAStates,
		MaxOccursLimit: cfg.MaxOccursLimit,
	})
}

func prepareBuildArtifactsFromPlan(
	sch *parser.Schema,
	reg *analysis.Registry,
	refs *analysis.ResolvedReferences,
	complexTypes *lower.ComplexTypePlan,
) (*PreparedArtifacts, error) {
	if err := validateBuildInputs(sch, reg, refs); err != nil {
		return nil, err
	}
	validators, err := lower.CompileWithComplexTypePlan(sch, reg, complexTypes)
	if err != nil {
		return nil, fmt.Errorf("runtime build: compile validators: %w", err)
	}
	return PrepareBuildArtifacts(sch, reg, refs, validators)
}

func validateBuildInputs(sch *parser.Schema, reg *analysis.Registry, refs *analysis.ResolvedReferences) error {
	if sch == nil {
		return fmt.Errorf("runtime build: schema is nil")
	}
	if reg == nil {
		return fmt.Errorf("runtime build: registry is nil")
	}
	if refs == nil {
		return fmt.Errorf("runtime build: references are nil")
	}
	return nil
}
