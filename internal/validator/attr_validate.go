package validator

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
)

// AttrValidationResult holds the validated and applied attributes for one type-level validation pass.
type AttrValidationResult struct {
	Attrs   []Start
	Applied []Applied
}

// TypeCallbacks supplies the root-owned behavior needed to validate attributes
// for one already-resolved runtime type.
type TypeCallbacks struct {
	PrepareValidated func(store bool, size int) []Start
	PreparePresent   func(size int) []bool
	ValidateSimple   func(input []Start, classes []Class, store bool, validated []Start) ([]Start, error)
	ValidateComplex  func(ct *runtime.ComplexType, present []bool, input []Start, classes []Class, store bool, validated []Start) ([]Start, bool, error)
	ApplyDefaults    func(uses []runtime.AttrUse, present []bool, storeAttrs, seenID bool) ([]Applied, error)
}

// ValidateType validates attributes for one resolved runtime type.
func ValidateType(
	rt *runtime.Schema,
	typ runtime.Type,
	classified Classification,
	input []Start,
	store bool,
	callbacks TypeCallbacks,
) (AttrValidationResult, error) {
	if classified.DuplicateErr != nil {
		return AttrValidationResult{}, classified.DuplicateErr
	}

	validated := callbacks.PrepareValidated(store, len(input))
	if typ.Kind != runtime.TypeComplex {
		simpleValidated, err := callbacks.ValidateSimple(input, classified.Classes, store, validated)
		if err != nil {
			return AttrValidationResult{}, err
		}
		return AttrValidationResult{Attrs: simpleValidated}, nil
	}

	if int(typ.Complex.ID) >= len(rt.ComplexTypes) {
		return AttrValidationResult{}, fmt.Errorf("complex type %d not found", typ.Complex.ID)
	}
	ct := &rt.ComplexTypes[typ.Complex.ID]
	uses := Uses(rt.AttrIndex.Uses, ct.Attrs)
	present := callbacks.PreparePresent(len(uses))

	validated, seenID, err := callbacks.ValidateComplex(ct, present, input, classified.Classes, store, validated)
	if err != nil {
		return AttrValidationResult{}, err
	}
	applied, err := callbacks.ApplyDefaults(uses, present, store, seenID)
	if err != nil {
		return AttrValidationResult{}, err
	}

	return AttrValidationResult{
		Attrs:   validated,
		Applied: applied,
	}, nil
}
