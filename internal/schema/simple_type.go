package schema

import (
	"errors"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
)

// simpleType is compiler-owned schema declaration metadata. ValueSpec is the
// sole owner of all simple-type semantics and derivation edges; schema keeps
// only declaration facts and derived construction metadata.
type simpleType struct {
	ValueSpec value.TypeSpec
	Name      QName
	Final     DerivationMask
	Missing   bool
	Scope     declarationScope

	// Union admission derives this from completed members; restrictions preserve it.
	unionContainsList bool
	// The completed value definition owns this effective-facet projection.
	unionHasFacets bool
}

// simpleTypeByID returns the declaration at id when it is within types.
func simpleTypeByID(types []simpleType, id SimpleTypeID) (*simpleType, bool) {
	if !validSimpleTypeID(id, len(types)) {
		return nil, false
	}
	return &types[id], true
}

// usableSimpleType returns a present, non-missing declaration at id.
func usableSimpleType(types []simpleType, id SimpleTypeID) (*simpleType, bool) {
	st, ok := simpleTypeByID(types, id)
	if !ok || st.Missing {
		return nil, false
	}
	return st, true
}

// missingSimpleTypeLocalName returns the local name used for unavailable types.
func missingSimpleTypeLocalName() string { return "missing" }

// missingSimpleType creates the unavailable-type sentinel declaration.
func missingSimpleType(name QName, base SimpleTypeID) simpleType {
	return simpleType{
		Name:    name,
		Missing: true,
		ValueSpec: value.TypeSpec{
			Variety: value.Atomic, Primitive: value.PrimitiveString,
			Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
			Base: base, ListItem: value.NoType,
		},
	}
}

// orderedFacetStep only records duplicate/exclusive source facets. Typed
// bound ordering and derivation are checked by value.Builder.
type orderedFacetStep struct {
	MinInclusive bool
	MinExclusive bool
	MaxInclusive bool
	MaxExclusive bool
}

// validateOrderedFacetStep rejects duplicate inclusive/exclusive bounds in a
// single source restriction step.
func validateOrderedFacetStep(step orderedFacetStep) error {
	if step.MinInclusive && step.MinExclusive {
		return errors.New("minInclusive and minExclusive cannot both be specified")
	}
	if step.MaxInclusive && step.MaxExclusive {
		return errors.New("maxInclusive and maxExclusive cannot both be specified")
	}
	return nil
}

// validateSimpleTypeFinalAllows checks whether derivation is allowed by final.
func validateSimpleTypeFinalAllows(final, derivation DerivationMask) error {
	if !validSimpleFinalMask(final) {
		return errors.New("simple type final mask is invalid")
	}
	//nolint:exhaustive // only simple-type derivation kinds are valid here.
	switch derivation {
	case DerivationRestriction, derivationList, derivationUnion:
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
	case derivationList:
		return vocab.XSDElemList
	case derivationUnion:
		return vocab.XSDElemUnion
	default:
		return "derivation"
	}
}
