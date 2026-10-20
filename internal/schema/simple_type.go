package schema

import (
	"errors"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
)

// SimpleType is compiler-owned schema declaration metadata. ValueSpec is the
// sole owner of all simple-type semantics and derivation edges; schema keeps
// only declaration facts and derived construction metadata.
type SimpleType struct {
	ValueSpec value.TypeSpec
	Name      QName
	Final     DerivationMask
	Missing   bool
	Scope     DeclarationScope

	// Union admission derives this from completed members; restrictions preserve it.
	unionContainsList bool
	// The completed value definition owns this effective-facet projection.
	unionHasFacets bool
}

// SimpleTypeByID returns the declaration at id when it is within types.
func SimpleTypeByID(types []SimpleType, id SimpleTypeID) (*SimpleType, bool) {
	if !ValidSimpleTypeID(id, len(types)) {
		return nil, false
	}
	return &types[id], true
}

// UsableSimpleType returns a present, non-missing declaration at id.
func UsableSimpleType(types []SimpleType, id SimpleTypeID) (*SimpleType, bool) {
	st, ok := SimpleTypeByID(types, id)
	if !ok || st.Missing {
		return nil, false
	}
	return st, true
}

// MissingSimpleTypeLocalName returns the local name used for unavailable types.
func MissingSimpleTypeLocalName() string { return "missing" }

// MissingSimpleType creates the unavailable-type sentinel declaration.
func MissingSimpleType(name QName, base SimpleTypeID) SimpleType {
	return SimpleType{
		Name:    name,
		Missing: true,
		ValueSpec: value.TypeSpec{
			Variety: value.Atomic, Primitive: value.PrimitiveString,
			Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
			Base: base, ListItem: value.NoType,
		},
	}
}

// ValidWhitespaceRestriction reports whether next preserves the base type's
// whiteSpace restriction.
func ValidWhitespaceRestriction(base, next value.WhitespaceMode) bool {
	if base == value.WhitespaceCollapse {
		return next == value.WhitespaceCollapse
	}
	if base == value.WhitespaceReplace {
		return next != value.WhitespacePreserve
	}
	return true
}

// OrderedFacetStep only records duplicate/exclusive source facets. Typed
// bound ordering and derivation are checked by value.Builder.
type OrderedFacetStep struct {
	MinInclusive bool
	MinExclusive bool
	MaxInclusive bool
	MaxExclusive bool
}

// ValidateOrderedFacetStep rejects duplicate inclusive/exclusive bounds in a
// single source restriction step.
func ValidateOrderedFacetStep(step OrderedFacetStep) error {
	if step.MinInclusive && step.MinExclusive {
		return errors.New("minInclusive and minExclusive cannot both be specified")
	}
	if step.MaxInclusive && step.MaxExclusive {
		return errors.New("maxInclusive and maxExclusive cannot both be specified")
	}
	return nil
}

// ValidateSimpleTypeFinalAllows checks whether derivation is allowed by final.
func ValidateSimpleTypeFinalAllows(final, derivation DerivationMask) error {
	if !ValidSimpleFinalMask(final) {
		return errors.New("simple type final mask is invalid")
	}
	//nolint:exhaustive // only simple-type derivation kinds are valid here.
	switch derivation {
	case DerivationRestriction, DerivationList, DerivationUnion:
	default:
		return errors.New("simple type final derivation is invalid")
	}
	if final&derivation != 0 {
		return errors.New("simple type final blocks " + simpleTypeFinalDerivationName(derivation))
	}
	return nil
}

func simpleTypeFinalDerivationName(derivation DerivationMask) string {
	//nolint:exhaustive // callers validate the simple derivation kind first.
	switch derivation {
	case DerivationRestriction:
		return vocab.XSDElemRestriction
	case DerivationList:
		return vocab.XSDElemList
	case DerivationUnion:
		return vocab.XSDElemUnion
	default:
		return "derivation"
	}
}
