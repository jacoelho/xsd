package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validatorgen"
)

// PreparedArtifacts stores immutable runtime-build prerequisites.
type PreparedArtifacts struct {
	schema     *parser.Schema
	registry   *analysis.Registry
	refs       *analysis.ResolvedReferences
	validators *validatorgen.CompiledValidators
}

// BuildArtifacts compiles resolved semantic artifacts into a runtime schema model.
func BuildArtifacts(
	sch *parser.Schema,
	reg *analysis.Registry,
	refs *analysis.ResolvedReferences,
	validators *validatorgen.CompiledValidators,
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
	validators *validatorgen.CompiledValidators,
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

// Build compiles prepared artifacts into a runtime schema model.
func (p *PreparedArtifacts) Build(cfg BuildConfig) (*runtime.Schema, error) {
	if p == nil {
		return nil, fmt.Errorf("runtime build: prepared artifacts are nil")
	}
	return buildArtifactsWithValidators(p.schema, p.registry, p.refs, p.validators, cfg)
}
