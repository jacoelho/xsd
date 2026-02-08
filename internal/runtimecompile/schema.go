package runtimecompile

import (
	"fmt"

	models "github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	schema "github.com/jacoelho/xsd/internal/semantic"
	"github.com/jacoelho/xsd/internal/types"
)

// BuildConfig configures runtime schema compilation.
type BuildConfig struct {
	Limits         models.Limits
	MaxOccursLimit uint32
}

// BuildArtifacts compiles resolved semantic artifacts into a runtime schema model.
func BuildArtifacts(sch *parser.Schema, reg *schema.Registry, refs *schema.ResolvedReferences, cfg BuildConfig) (*runtime.Schema, error) {
	if sch == nil {
		return nil, fmt.Errorf("runtime build: schema is nil")
	}
	if reg == nil {
		return nil, fmt.Errorf("runtime build: registry is nil")
	}
	if refs == nil {
		return nil, fmt.Errorf("runtime build: references are nil")
	}

	validators, err := compileValidators(sch, reg)
	if err != nil {
		return nil, fmt.Errorf("runtime build: compile validators: %w", err)
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
		limits:     cfg.Limits,
		builder:    runtime.NewBuilder(),
		typeIDs:    make(map[schema.TypeID]runtime.TypeID),
		elemIDs:    make(map[schema.ElemID]runtime.ElemID),
		attrIDs:    make(map[schema.AttrID]runtime.AttrID),
		builtinIDs: make(map[types.TypeName]runtime.TypeID),
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
	validators      *compiledValidators
	registry        *schema.Registry
	typeIDs         map[schema.TypeID]runtime.TypeID
	builder         *runtime.Builder
	schema          *parser.Schema
	complexIDs      map[runtime.TypeID]uint32
	builtinIDs      map[types.TypeName]runtime.TypeID
	refs            *schema.ResolvedReferences
	anyElementRules map[*types.AnyElement]runtime.WildcardID
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
