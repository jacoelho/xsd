package schema

import (
	"errors"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/vocab"
)

// simpleType is compiler-owned schema declaration metadata. It holds no
// value structure: value.Builder owns the base, list item, union members,
// variety, primitive, builtin kind and whitespace of every simple type, and
// compiler reads of a completed type go through Builder.TypeView and
// Builder.UnionHasFacets. Only the type being compiled is read from its
// simpleTypeDraft.
type simpleType struct {
	Name    QName
	Final   DerivationMask
	Missing bool
	Scope   declarationScope

	// Union admission derives this from completed members; restrictions preserve it.
	unionContainsList bool
}

// simpleTypeDraft is a simple type under construction: its declaration and
// the value specification completeSimpleType hands to value.Builder.Complete.
// A draft lives only between compiling a simple type and completing it; the
// build stores the declaration alone.
type simpleTypeDraft struct {
	spec value.TypeSpec
	decl simpleType
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

// missingSimpleType creates the unavailable-type sentinel. Missing keeps it
// unavailable to every caller; its value record is a collapsed xs:string
// restriction only so that its reserved ID is complete and the builder can
// seal.
func missingSimpleType(name QName) simpleTypeDraft {
	return simpleTypeDraft{
		decl: simpleType{Name: name, Missing: true},
		spec: missingValueTypeSpec(),
	}
}

func missingValueTypeSpec() value.TypeSpec {
	return value.TypeSpec{
		Variety: value.Atomic, Primitive: value.PrimitiveString,
		Whitespace: value.WhitespaceCollapse, WhitespacePresent: true,
		Base: value.BuiltinType(value.PrimitiveString), ListItem: value.NoType,
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
		return newBuildDefect("simple type final mask is invalid")
	}
	//nolint:exhaustive // only simple-type derivation kinds are valid here.
	switch derivation {
	case DerivationRestriction, derivationList, derivationUnion:
	default:
		return newBuildDefect("simple type final derivation is invalid")
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
