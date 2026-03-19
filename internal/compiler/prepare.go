package compiler

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/compiler/lower"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semanticcheck"
	"github.com/jacoelho/xsd/internal/semanticresolve"
)

type prepareResult struct {
	schema       *parser.Schema
	registry     *analysis.Registry
	refs         *analysis.ResolvedReferences
	complexTypes *lower.ComplexTypePlan
}

// Prepare clones, normalizes, and validates a parsed schema.
func Prepare(parsed *parser.Schema) (*Prepared, error) {
	result, err := prepareParsedSchema(parsed, true)
	if err != nil {
		return nil, err
	}
	return preparedFromResult(result), nil
}

// PrepareOwned normalizes and validates a parsed schema in place.
func PrepareOwned(parsed *parser.Schema) (*Prepared, error) {
	result, err := prepareParsedSchema(parsed, false)
	if err != nil {
		return nil, err
	}
	return preparedFromResult(result), nil
}

// ResolveAndValidateOwned resolves semantic references and validates the schema in place.
func ResolveAndValidateOwned(sch *parser.Schema) error {
	if sch == nil {
		return fmt.Errorf("schema is nil")
	}
	if err := semanticresolve.ResolveGroupReferences(sch); err != nil {
		return fmt.Errorf("resolve group references: %w", err)
	}
	structureErrs := semanticcheck.ValidateStructure(sch)
	if len(structureErrs) > 0 {
		return formatValidationErrors(structureErrs)
	}
	if err := semanticresolve.NewResolver(sch).Resolve(); err != nil {
		return fmt.Errorf("resolve type references: %w", err)
	}
	refErrs := semanticresolve.ValidateReferences(sch)
	if len(refErrs) > 0 {
		return formatValidationErrors(refErrs)
	}
	deferredRangeErrs := semanticcheck.ValidateDeferredRangeFacetValues(sch)
	if len(deferredRangeErrs) > 0 {
		return formatValidationErrors(deferredRangeErrs)
	}
	if parser.HasPlaceholders(sch) {
		return fmt.Errorf("schema has unresolved placeholders")
	}
	return nil
}

// ValidateUPA validates Unique Particle Attribution for all prepared complex types.
func ValidateUPA(schema *parser.Schema, registry *analysis.Registry) error {
	if schema == nil {
		return fmt.Errorf("schema is nil")
	}
	if registry == nil {
		return fmt.Errorf("registry is nil")
	}
	if err := analysis.RequireResolved(schema); err != nil {
		return err
	}

	for _, entry := range registry.TypeOrder {
		ct, ok := entry.Type.(*model.ComplexType)
		if !ok {
			continue
		}
		if err := semanticcheck.ValidateUPA(schema, ct.Content(), schema.TargetNamespace); err != nil {
			return fmt.Errorf("%s: %w", complexTypeLabel(ct), err)
		}
	}
	return nil
}

func prepareParsedSchema(parsed *parser.Schema, clone bool) (*prepareResult, error) {
	if parsed == nil {
		return nil, fmt.Errorf("prepare schema: schema is nil")
	}
	if clone {
		parsed = parser.CloneSchema(parsed)
	}
	if err := ResolveAndValidateOwned(parsed); err != nil {
		return nil, fmt.Errorf("prepare schema: %w", err)
	}
	registry, err := analysis.AssignIDs(parsed)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: assign IDs: %w", err)
	}
	err = analysis.DetectCycles(parsed)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: detect cycles: %w", err)
	}
	err = ValidateUPA(parsed, registry)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: validate UPA: %w", err)
	}
	refs, err := analysis.ResolveReferences(parsed, registry)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: resolve references: %w", err)
	}
	complexTypes, err := lower.BuildComplexTypePlan(parsed, registry)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: complex type plan: %w", err)
	}
	return &prepareResult{
		schema:       parsed,
		registry:     registry,
		refs:         refs,
		complexTypes: complexTypes,
	}, nil
}

func preparedFromResult(result *prepareResult) *Prepared {
	if result == nil {
		return nil
	}
	return &Prepared{
		schema:       result.schema,
		registry:     result.registry,
		refs:         result.refs,
		complexTypes: result.complexTypes,
	}
}

func complexTypeLabel(ct *model.ComplexType) string {
	if ct == nil {
		return "complexType"
	}
	if ct.QName.IsZero() {
		return "anonymous complexType"
	}
	return fmt.Sprintf("complexType %s", ct.QName)
}

func formatValidationErrors(validationErrs []error) error {
	if len(validationErrs) == 0 {
		return nil
	}

	errs := validationErrs
	if len(validationErrs) > 1 {
		errs = slices.Clone(validationErrs)
		slices.SortStableFunc(errs, func(a, b error) int {
			return strings.Compare(a.Error(), b.Error())
		})
	}

	var msg strings.Builder
	msg.WriteString("schema validation failed:")
	for _, err := range errs {
		msg.WriteString("\n  - ")
		msg.WriteString(err.Error())
	}
	return errors.New(msg.String())
}
