package runtimeassemble

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complextypeplan"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validatorgen"
)

// BuildConfig configures runtime schema compilation.
type BuildConfig struct {
	MaxDFAStates   uint32
	MaxOccursLimit uint32
}

// PreparedArtifacts stores immutable runtime-build prerequisites.
type PreparedArtifacts struct {
	schema     *parser.Schema
	registry   *analysis.Registry
	refs       *analysis.ResolvedReferences
	validators *validatorgen.CompiledValidators
}

// BuildComplexTypePlan precomputes shared complex-type artifacts during prepare.
func BuildComplexTypePlan(sch *parser.Schema, reg *analysis.Registry) (*complextypeplan.Plan, error) {
	return validatorgen.BuildComplexTypePlan(sch, reg)
}

// BuildArtifacts compiles resolved semantic artifacts into a runtime schema model.
func BuildArtifacts(sch *parser.Schema, reg *analysis.Registry, refs *analysis.ResolvedReferences, cfg BuildConfig) (*runtime.Schema, error) {
	prepared, err := PrepareBuildArtifacts(sch, reg, refs)
	if err != nil {
		return nil, err
	}
	return prepared.Build(cfg)
}

// PrepareBuildArtifacts compiles validator artifacts once for repeated runtime builds.
func PrepareBuildArtifacts(sch *parser.Schema, reg *analysis.Registry, refs *analysis.ResolvedReferences) (*PreparedArtifacts, error) {
	return PrepareBuildArtifactsWithComplexTypePlan(sch, reg, refs, nil)
}

// PrepareBuildArtifactsWithComplexTypePlan compiles validator artifacts once for repeated runtime builds.
func PrepareBuildArtifactsWithComplexTypePlan(
	sch *parser.Schema,
	reg *analysis.Registry,
	refs *analysis.ResolvedReferences,
	complexTypes *complextypeplan.Plan,
) (*PreparedArtifacts, error) {
	if err := validateBuildInputs(sch, reg, refs); err != nil {
		return nil, err
	}
	validators, err := validatorgen.CompileWithComplexTypePlan(sch, reg, complexTypes)
	if err != nil {
		return nil, fmt.Errorf("runtime build: compile validators: %w", err)
	}
	return &PreparedArtifacts{
		schema:     sch,
		registry:   reg,
		refs:       refs,
		validators: validators,
	}, nil
}

// Build compiles prepared artifacts into a runtime schema model.
func (p *PreparedArtifacts) Build(cfg BuildConfig) (*runtime.Schema, error) {
	if p == nil {
		return nil, fmt.Errorf("runtime build: prepared artifacts are nil")
	}
	return buildArtifactsWithValidators(p.schema, p.registry, p.refs, p.validators, cfg)
}
