package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/semantics"
)

// Prepare clones, normalizes, and validates a parsed schema.
func Prepare(parsed *parser.Schema) (*Prepared, error) {
	ctx, err := semantics.Prepare(parsed)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: %w", err)
	}
	return preparedFromContext(ctx), nil
}

// PrepareOwned normalizes and validates a parsed schema in place.
func PrepareOwned(parsed *parser.Schema) (*Prepared, error) {
	ctx, err := semantics.PrepareOwned(parsed)
	if err != nil {
		return nil, fmt.Errorf("prepare schema: %w", err)
	}
	return preparedFromContext(ctx), nil
}

func preparedFromContext(ctx *semantics.Context) *Prepared {
	if ctx == nil {
		return nil
	}
	complexTypes, err := ctx.ComplexTypes()
	if err != nil {
		panic(fmt.Sprintf("compiler: prepared semantics context missing complex types: %v", err))
	}
	return &Prepared{
		schema:       ctx.Schema(),
		registry:     ctx.Registry(),
		refs:         resolvedReferencesFromSemantics(ctx.References()),
		complexTypes: complexTypes,
	}
}

func resolvedReferencesFromSemantics(refs *semantics.ResolvedReferences) *ResolvedReferences {
	if refs == nil {
		return nil
	}
	return &ResolvedReferences{
		ElementRefs:   refs.ElementRefs,
		AttributeRefs: refs.AttributeRefs,
		GroupRefs:     refs.GroupRefs,
	}
}
