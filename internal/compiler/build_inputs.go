package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/semantics"
)

// Config configures runtime-schema lowering.
type Config struct {
	MaxDFAStates   uint32
	MaxOccursLimit uint32
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

// Build lowers prepared schema artifacts into an immutable runtime schema.
func Build(
	sch *parser.Schema,
	reg *analysis.Registry,
	refs *analysis.ResolvedReferences,
	validators *semantics.CompiledValidators,
	cfg Config,
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
		schema:       sch,
		registry:     reg,
		refs:         refs,
		validators:   validators,
		limits:       semantics.Limits{MaxDFAStates: cfg.MaxDFAStates},
		builder:      runtime.NewBuilder(),
		typeIDs:      make(map[analysis.TypeID]runtime.TypeID),
		elemIDs:      make(map[analysis.ElemID]runtime.ElemID),
		attrIDs:      make(map[analysis.AttrID]runtime.AttrID),
		builtinIDs:   make(map[model.TypeName]runtime.TypeID),
		complexIDs:   make(map[runtime.TypeID]uint32),
		maxOccurs:    maxOccursLimit,
		complexTypes: validators.ComplexTypes,
	}
	rt, err := builder.build()
	if err != nil {
		return nil, err
	}
	return rt, nil
}
