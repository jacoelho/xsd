package grammar

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// AllGroupElementInfo provides element information for all group schemacheck.
// Implemented by grammar.AllGroupElement to avoid import cycles.
// Note: Elements with maxOccurs=0 are filtered out during compilation per XSD spec.
type AllGroupElementInfo interface {
	ElementQName() types.QName
	ElementDecl() *CompiledElement
	IsOptional() bool
	AllowsSubstitution() bool
}

// AllGroupValidator validates all-group content models with array-based checks.
// It enforces required elements, uniqueness, and order-insensitivity.
type AllGroupValidator struct {
	minOccurs   types.Occurs
	elements    []AllGroupElementInfo
	numRequired int
	mixed       bool
}

type allGroupMatchKind int

const (
	allGroupNoMatch allGroupMatchKind = iota
	allGroupExactMatch
	allGroupSubstitutionMatch
)

func matchAllGroupElement(child types.QName, elements []AllGroupElementInfo, matcher SymbolMatcher) (int, allGroupMatchKind) {
	for i, elem := range elements {
		elemQName := elem.ElementQName()
		if elemQName.Equal(child) {
			return i, allGroupExactMatch
		}
		if matcher != nil && elem.AllowsSubstitution() && matcher.IsSubstitutable(child, elemQName) {
			return i, allGroupSubstitutionMatch
		}
	}
	return -1, allGroupNoMatch
}

// NewAllGroupValidator creates a validator for an all group.
func NewAllGroupValidator(elements []AllGroupElementInfo, mixed bool, minOccurs types.Occurs) *AllGroupValidator {
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
		minOccurs:   minOccurs,
	}
}

// Validate checks that children satisfy the all group content model.
// Returns nil if valid, or a ValidationError describing the violation.
func (v *AllGroupValidator) Validate(doc *xsdxml.Document, children []xsdxml.NodeID, matcher SymbolMatcher) error {
	if len(children) == 0 && v.minOccurs.IsZero() {
		return nil
	}
	if len(v.elements) == 0 {
		if len(children) == 0 {
			return nil
		}
		return &ValidationError{
			Index:   0,
			Message: fmt.Sprintf("element %q not allowed", doc.LocalName(children[0])),
			SubCode: ErrorCodeNotExpectedHere,
		}
	}

	elementSeen := make([]bool, len(v.elements))
	numRequiredSeen := 0

	for i, child := range children {
		childQName := types.QName{
			Namespace: types.NamespaceURI(doc.NamespaceURI(child)),
			Local:     doc.LocalName(child),
		}
		childLocal := doc.LocalName(child)

		idx, kind := matchAllGroupElement(childQName, v.elements, matcher)
		if kind == allGroupNoMatch {
			return &ValidationError{
				Index:   i,
				Message: fmt.Sprintf("element %q not allowed in all group", childLocal),
				SubCode: ErrorCodeNotExpectedHere,
			}
		}

		if elementSeen[idx] {
			elemQName := v.elements[idx].ElementQName()
			if kind == allGroupSubstitutionMatch {
				return &ValidationError{
					Index:   i,
					Message: fmt.Sprintf("element %q (substituting for %q) appears more than once in all group", childLocal, elemQName.Local),
					SubCode: ErrorCodeNotExpectedHere,
				}
			}
			return &ValidationError{
				Index:   i,
				Message: fmt.Sprintf("element %q appears more than once in all group", childLocal),
				SubCode: ErrorCodeNotExpectedHere,
			}
		}

		elementSeen[idx] = true
		if !v.elements[idx].IsOptional() {
			numRequiredSeen++
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
