package schema

import (
	"errors"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/xsderrors"
)

var errBareNotationTypeUse = errors.New("NOTATION type use requires enumeration")

// checkNotationTypeUse admits an actual value type, not an atomic restriction
// base or an enumeration literal. Constructor edges are admitted independently,
// so completed list and union types require no repeated descendant walk.
func checkNotationTypeUse(build *schemaBuild, id SimpleTypeID) error {
	if !ValidSimpleTypeID(id, len(build.SimpleTypes)) || !ValidSimpleTypeID(id, len(build.simpleTypeUnavailable)) {
		return xsderrors.InternalInvariant("NOTATION use references invalid simple type")
	}
	if build.simpleTypeUnavailable[id] {
		return nil
	}
	if build.valueBuilder == nil {
		return xsderrors.InternalInvariant("NOTATION use has no value builder")
	}
	bare, ok := build.valueBuilder.NotationWithoutEnumeration(id)
	if !ok {
		return xsderrors.InternalInvariant("NOTATION use has no completed value definition")
	}
	if bare {
		return errBareNotationTypeUse
	}
	return nil
}

func (c *compiler) admitNotationTypeUse(node *schemaNode, id SimpleTypeID) error {
	err := checkNotationTypeUse(&c.rt, id)
	if errors.Is(err, errBareNotationTypeUse) {
		err = xsderrors.SchemaCompile(xsderrors.CodeSchemaFacet, err.Error())
	}
	return withSchemaCompileLocation(node, err)
}

func validateNotationTypeUse(build *schemaBuild, id SimpleTypeID) error {
	if err := checkNotationTypeUse(build, id); err != nil {
		return schemaBuildValidationError(err)
	}
	return nil
}

func validateNotationConstructorUses(build *schemaBuild, id SimpleTypeID) error {
	if id < value.BuiltinTypeCount {
		return nil
	}
	uses, ok := build.valueBuilder.ConstructorUses(id)
	if !ok {
		return xsderrors.InternalInvariant("NOTATION constructor has no value definition")
	}
	for i := range uses.Len() {
		member, valid := uses.At(i)
		if !valid {
			return xsderrors.InternalInvariant("NOTATION constructor references invalid member")
		}
		if err := validateNotationConstructorEdge(build, member); err != nil {
			return err
		}
	}
	return nil
}

func validateNotationConstructorEdge(build *schemaBuild, id SimpleTypeID) error {
	if err := build.spendTypeDerivationWork(1); err != nil {
		return err
	}
	return validateNotationTypeUse(build, id)
}
