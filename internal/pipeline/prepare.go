package pipeline

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/parser"
	semantic "github.com/jacoelho/xsd/internal/semantic"
	semanticcheck "github.com/jacoelho/xsd/internal/semanticcheck"
	semanticresolve "github.com/jacoelho/xsd/internal/semanticresolve"
)

// PreparedSchema stores semantic artifacts needed for runtime compilation.
type PreparedSchema struct {
	Ancestors *semantic.AncestorIndex
	Refs      *semantic.ResolvedReferences
	Registry  *semantic.Registry
	Schema    *parser.Schema
}

// Prepare validates and resolves a parsed schema for runtime compilation.
func Prepare(sch *parser.Schema) (*PreparedSchema, error) {
	if sch == nil {
		return nil, fmt.Errorf("prepare schema: schema is nil")
	}
	if sch.Phase < parser.PhaseResolved {
		if err := runSemanticPipeline(sch); err != nil {
			return nil, err
		}
	} else if err := semantic.RequireResolved(sch); err != nil {
		return nil, fmt.Errorf("prepare schema: %w", err)
	}

	reg, err := semantic.AssignIDs(sch)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: assign IDs: %w", err)
	}
	refs, err := semantic.ResolveReferences(sch, reg)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: resolve references: %w", err)
	}
	if err := semantic.DetectCycles(sch); err != nil {
		return nil, fmt.Errorf("prepare schema: detect cycles: %w", err)
	}
	if !sch.UPAValidated {
		if err := semantic.ValidateUPA(sch, reg); err != nil {
			return nil, fmt.Errorf("prepare schema: validate UPA: %w", err)
		}
		sch.UPAValidated = true
	}
	ancestors, err := semantic.BuildAncestors(sch, reg)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: build ancestors: %w", err)
	}
	return &PreparedSchema{
		Ancestors: ancestors,
		Refs:      refs,
		Registry:  reg,
		Schema:    sch,
	}, nil
}

func runSemanticPipeline(sch *parser.Schema) error {
	if sch.Phase <= parser.PhaseParsed {
		structureErrors := semanticcheck.ValidateStructure(sch)
		if len(structureErrors) > 0 {
			return formatSchemaErrors(structureErrors)
		}
		if err := semantic.MarkSemantic(sch); err != nil {
			return fmt.Errorf("prepare schema: %w", err)
		}
	}

	if err := semanticresolve.ResolveTypeReferences(sch); err != nil {
		return fmt.Errorf("prepare schema: resolve type references: %w", err)
	}
	refErrors := semanticresolve.ValidateReferences(sch)
	if len(refErrors) > 0 {
		return formatSchemaErrors(refErrors)
	}

	parser.UpdatePlaceholderState(sch)
	if err := semantic.MarkResolved(sch); err != nil {
		return fmt.Errorf("prepare schema: %w", err)
	}
	return nil
}

func formatSchemaErrors(validationErrors []error) error {
	if len(validationErrors) == 0 {
		return nil
	}
	errs := validationErrors
	if len(validationErrors) > 1 {
		errs = make([]error, len(validationErrors))
		copy(errs, validationErrors)
		slices.SortStableFunc(errs, func(a, b error) int {
			return strings.Compare(a.Error(), b.Error())
		})
	}
	var errMsg strings.Builder
	errMsg.WriteString("schema validation failed:")
	for _, err := range errs {
		errMsg.WriteString("\n  - ")
		errMsg.WriteString(err.Error())
	}
	return errors.New(errMsg.String())
}
