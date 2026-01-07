package contentmodel

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// AllGroupElementInfo provides element information for all group validation.
// Implemented by grammar.AllGroupElement to avoid import cycles.
// Note: Elements with maxOccurs=0 are filtered out during compilation per XSD spec.
type AllGroupElementInfo interface {
	ElementQName() types.QName
	IsOptional() bool
	AllowsSubstitution() bool
}

// AllGroupValidator validates all group content models.
// Uses simple array-based validation instead of DFA.
// This correctly handles:
// - Missing required elements
// - Duplicate elements
// - Any order
// - Optional vs required elements
type AllGroupValidator struct {
	elements    []AllGroupElementInfo
	numRequired int
	mixed       bool
}

// NewAllGroupValidator creates a validator for an all group.
func NewAllGroupValidator(elements []AllGroupElementInfo, mixed bool) *AllGroupValidator {
	numRequired := 0
	for _, e := range elements {
		if !e.IsOptional() {
			numRequired++
		}
	}
	return &AllGroupValidator{
		elements:    elements,
		numRequired: numRequired,
		mixed:       mixed,
	}
}

// Validate checks that children satisfy the all group content model.
// Returns nil if valid, or a ValidationError describing the violation.
func (v *AllGroupValidator) Validate(children []xml.Element, matcher SymbolMatcher) error {
	// if all group is empty and there are no children, it's valid
	if len(v.elements) == 0 {
		if len(children) == 0 {
			return nil
		}
		// no elements allowed but got some
		return &ValidationError{
			Index:   0,
			Message: fmt.Sprintf("element %q not allowed", children[0].LocalName()),
			SubCode: ErrorCodeNotExpectedHere,
		}
	}

	elementSeen := make([]bool, len(v.elements))
	numRequiredSeen := 0

	for i, child := range children {
		childQName := types.QName{
			Namespace: types.NamespaceURI(child.NamespaceURI()),
			Local:     child.LocalName(),
		}

		found := false
		for j, elem := range v.elements {
			elemQName := elem.ElementQName()
			if elemQName.Equal(childQName) {
				// check for duplicate (each element can appear at most once in an all group)
				if elementSeen[j] {
					return &ValidationError{
						Index:   i,
						Message: fmt.Sprintf("element %q appears more than once in all group", child.LocalName()),
						SubCode: ErrorCodeNotExpectedHere,
					}
				}
				elementSeen[j] = true
				if !elem.IsOptional() {
					numRequiredSeen++
				}
				found = true
				break
			}

			if matcher != nil && elem.AllowsSubstitution() && matcher.IsSubstitutable(childQName, elemQName) {
				if elementSeen[j] {
					return &ValidationError{
						Index:   i,
						Message: fmt.Sprintf("element %q (substituting for %q) appears more than once in all group", child.LocalName(), elemQName.Local),
						SubCode: ErrorCodeNotExpectedHere,
					}
				}
				elementSeen[j] = true
				if !elem.IsOptional() {
					numRequiredSeen++
				}
				found = true
				break
			}
		}

		// element not found in the all group
		if !found {
			return &ValidationError{
				Index:   i,
				Message: fmt.Sprintf("element %q not allowed in all group", child.LocalName()),
				SubCode: ErrorCodeNotExpectedHere,
			}
		}
	}

	if numRequiredSeen != v.numRequired {
		for j, elem := range v.elements {
			if !elem.IsOptional() && !elementSeen[j] {
				return &ValidationError{
					Index:   len(children),
					Message: fmt.Sprintf("required element %q missing from all group", elem.ElementQName().Local),
					SubCode: ErrorCodeMissing,
				}
			}
		}
		// fallback message
		return &ValidationError{
			Index:   len(children),
			Message: "required elements missing from all group",
			SubCode: ErrorCodeMissing,
		}
	}

	return nil
}