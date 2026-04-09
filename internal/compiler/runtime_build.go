package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complexplan"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validatorbuild"
)

// PreparedArtifacts stores immutable runtime-build prerequisites.
type PreparedArtifacts struct {
	schema     *parser.Schema
	registry   *analysis.Registry
	refs       *analysis.ResolvedReferences
	validators *validatorbuild.ValidatorArtifacts
}

// PrepareBuildArtifacts packages compiler-owned artifacts for repeated runtime builds.
func PrepareBuildArtifacts(
	sch *parser.Schema,
	reg *analysis.Registry,
	refs *analysis.ResolvedReferences,
	validators *validatorbuild.ValidatorArtifacts,
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
func (p *PreparedArtifacts) Validators() *validatorbuild.ValidatorArtifacts {
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
	return Build(p.schema, p.registry, p.refs, p.validators, Config(cfg))
}

func prepareBuildArtifactsFromComplexTypes(
	sch *parser.Schema,
	reg *analysis.Registry,
	refs *analysis.ResolvedReferences,
	complexTypes *complexplan.ComplexTypes,
) (*PreparedArtifacts, error) {
	if err := validateBuildInputs(sch, reg, refs); err != nil {
		return nil, err
	}
	if complexTypes == nil {
		return nil, fmt.Errorf("runtime build: complex types are nil")
	}
	validators, err := validatorbuild.Compile(sch, reg, complexTypes)
	if err != nil {
		return nil, err
	}
	return PrepareBuildArtifacts(sch, reg, refs, validators)
}
