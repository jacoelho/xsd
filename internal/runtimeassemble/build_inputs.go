package runtimeassemble

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/ids"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/validatorgen"
)

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

func buildArtifactsWithValidators(
	sch *parser.Schema,
	reg *analysis.Registry,
	refs *analysis.ResolvedReferences,
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
		schema:       sch,
		registry:     reg,
		refs:         refs,
		validators:   validators,
		limits:       contentmodel.Limits{MaxDFAStates: cfg.MaxDFAStates},
		builder:      runtime.NewBuilder(),
		typeIDs:      make(map[ids.TypeID]runtime.TypeID),
		elemIDs:      make(map[ids.ElemID]runtime.ElemID),
		attrIDs:      make(map[ids.AttrID]runtime.AttrID),
		builtinIDs:   make(map[types.TypeName]runtime.TypeID),
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
