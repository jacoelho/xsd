package runtimeassemble

import (
	"fmt"

	models "github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	schema "github.com/jacoelho/xsd/internal/schemaanalysis"
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
	registry   *schema.Registry
	refs       *schema.ResolvedReferences
	validators *validatorgen.CompiledValidators
}

// BuildArtifacts compiles resolved semantic artifacts into a runtime schema model.
func BuildArtifacts(sch *parser.Schema, reg *schema.Registry, refs *schema.ResolvedReferences, cfg BuildConfig) (*runtime.Schema, error) {
	prepared, err := PrepareBuildArtifacts(sch, reg, refs)
	if err != nil {
		return nil, err
	}
	return prepared.Build(cfg)
}

// PrepareBuildArtifacts compiles validator artifacts once for repeated runtime builds.
func PrepareBuildArtifacts(sch *parser.Schema, reg *schema.Registry, refs *schema.ResolvedReferences) (*PreparedArtifacts, error) {
	if err := validateBuildInputs(sch, reg, refs); err != nil {
		return nil, err
	}
	validators, err := validatorgen.Compile(sch, reg)
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

func validateBuildInputs(sch *parser.Schema, reg *schema.Registry, refs *schema.ResolvedReferences) error {
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

func buildArtifactsWithValidators(
	sch *parser.Schema,
	reg *schema.Registry,
	refs *schema.ResolvedReferences,
	validators *validatorgen.CompiledValidators,
	cfg BuildConfig,
) (*runtime.Schema, error) {
	if err := validateBuildInputs(sch, reg, refs); err != nil {
		return nil, err
	}
	if validators == nil {
		return nil, fmt.Errorf("runtime build: validators are nil")
	}
	maxOccursLimit := cfg.MaxOccursLimit
	if maxOccursLimit == 0 {
		maxOccursLimit = defaultMaxOccursLimit
	}

	builder := &schemaBuilder{
		schema:     sch,
		registry:   reg,
		refs:       refs,
		validators: validators,
		limits:     models.Limits{MaxDFAStates: cfg.MaxDFAStates},
		builder:    runtime.NewBuilder(),
		typeIDs:    make(map[schema.TypeID]runtime.TypeID),
		elemIDs:    make(map[schema.ElemID]runtime.ElemID),
		attrIDs:    make(map[schema.AttrID]runtime.AttrID),
		builtinIDs: make(map[model.TypeName]runtime.TypeID),
		complexIDs: make(map[runtime.TypeID]uint32),
		maxOccurs:  maxOccursLimit,
	}
	rt, err := builder.build()
	if err != nil {
		return nil, err
	}
	return rt, nil
}

type schemaBuilder struct {
	err             error
	attrIDs         map[schema.AttrID]runtime.AttrID
	elemIDs         map[schema.ElemID]runtime.ElemID
	validators      *validatorgen.CompiledValidators
	registry        *schema.Registry
	typeIDs         map[schema.TypeID]runtime.TypeID
	builder         *runtime.Builder
	schema          *parser.Schema
	complexIDs      map[runtime.TypeID]uint32
	builtinIDs      map[model.TypeName]runtime.TypeID
	refs            *schema.ResolvedReferences
	anyElementRules map[*model.AnyElement]runtime.WildcardID
	rt              *runtime.Schema
	paths           []runtime.PathProgram
	wildcards       []runtime.WildcardRule
	wildcardNS      []runtime.NamespaceID
	notations       []runtime.SymbolID
	maxOccurs       uint32
	anyTypeComplex  uint32
	limits          models.Limits
}

const defaultMaxOccursLimit = 1_000_000

func (b *schemaBuilder) build() (*runtime.Schema, error) {
	if err := b.initSymbols(); err != nil {
		return nil, err
	}
	if b.err != nil {
		return nil, b.err
	}
	rt, err := b.builder.Build()
	if err != nil {
		return nil, err
	}
	b.rt = rt
	b.rt.RootPolicy = runtime.RootStrict
	b.rt.Validators = b.validators.Validators
	b.rt.Facets = b.validators.Facets
	b.rt.Patterns = b.validators.Patterns
	b.rt.Enums = b.validators.Enums
	b.rt.Values = b.validators.Values
	b.rt.Notations = b.notations
	b.wildcards = make([]runtime.WildcardRule, 1)

	b.initIDs()
	if err := b.buildTypes(); err != nil {
		return nil, err
	}
	if err := b.buildAncestors(); err != nil {
		return nil, err
	}
	if err := b.buildAttributes(); err != nil {
		return nil, err
	}
	if err := b.buildElements(); err != nil {
		return nil, err
	}
	if err := b.buildModels(); err != nil {
		return nil, err
	}
	if err := b.buildIdentityConstraints(); err != nil {
		return nil, err
	}

	b.rt.Wildcards = b.wildcards
	b.rt.WildcardNS = b.wildcardNS
	b.rt.Paths = b.paths

	b.rt.BuildHash = computeBuildHash(b.rt)

	return b.rt, nil
}
