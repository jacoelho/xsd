package semantics

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/parser"
)

// Prepare clones, resolves, validates, and indexes a parsed schema.
func Prepare(schema *parser.Schema) (*Context, error) {
	return prepare(schema, true)
}

// PrepareOwned resolves, validates, and indexes a parsed schema in place.
func PrepareOwned(schema *parser.Schema) (*Context, error) {
	return prepare(schema, false)
}

func prepare(schema *parser.Schema, clone bool) (*Context, error) {
	if schema == nil {
		return nil, fmt.Errorf("schema is nil")
	}
	if clone {
		schema = parser.CloneSchema(schema)
	}
	if err := resolveAndValidateOwned(schema); err != nil {
		return nil, err
	}
	registry, err := analysis.AssignIDs(schema)
	if err != nil {
		return nil, fmt.Errorf("assign IDs: %w", err)
	}
	if err := DetectCycles(schema); err != nil {
		return nil, fmt.Errorf("detect cycles: %w", err)
	}
	refs, err := ResolveReferences(schema, registry)
	if err != nil {
		return nil, fmt.Errorf("resolve references: %w", err)
	}
	ctx, err := Build(schema, registry, refs)
	if err != nil {
		return nil, err
	}
	return ctx, nil
}

func resolveAndValidateOwned(schema *parser.Schema) error {
	validationErrs, err := ResolveAndValidateSchema(schema)
	if err != nil {
		return err
	}
	if len(validationErrs) > 0 {
		return formatValidationErrors(validationErrs)
	}
	return nil
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
