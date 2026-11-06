package schema

import (
	"github.com/jacoelho/xsd/xsderrors"
)

// attributeMergeMode identifies how local attribute uses combine with inherited
// uses during compile-time derivation.
type attributeMergeMode uint8

const (
	// attributeMergeInvalid is not a valid attribute derivation operation.
	attributeMergeInvalid attributeMergeMode = iota
	// attributeMergeDirect appends locally declared uses without inheriting a
	// wildcard.
	attributeMergeDirect
	// attributeMergeExtension appends local uses and unions local and inherited
	// wildcards.
	attributeMergeExtension
	// attributeMergeRestriction validates local uses and wildcard declarations
	// as restrictions of inherited attributes.
	attributeMergeRestriction
)

// schemaAttributeWildcardRuntime supplies compile-time wildcard metadata and stores
// wildcard values produced by intersection or union.
type schemaAttributeWildcardRuntime interface {
	Wildcard(id WildcardID) (wildcardRuntime, bool)
	AddWildcard(w wildcardRuntime) (WildcardID, error)
}

// attributeWildcardBuilder owns compile-time attribute-wildcard construction.
type attributeWildcardBuilder struct {
	wildcard          WildcardID
	inheritedWildcard WildcardID
	mode              attributeMergeMode
}

// newAttributeWildcardBuilder creates a builder for local attribute wildcard
// declarations over an optional inherited wildcard.
func newAttributeWildcardBuilder(inherited WildcardID, mode attributeMergeMode) attributeWildcardBuilder {
	return attributeWildcardBuilder{
		wildcard:          NoWildcard,
		inheritedWildcard: inherited,
		mode:              mode,
	}
}

// declared returns the wildcard produced by local anyAttribute and attribute
// group declarations before extension inheritance is applied.
func (b *attributeWildcardBuilder) declared() WildcardID {
	return b.wildcard
}

// addGroup merges an attribute group's wildcard into the local declaration.
func (b *attributeWildcardBuilder) addGroup(rt schemaAttributeWildcardRuntime, id WildcardID) error {
	if id == NoWildcard {
		return nil
	}
	wildcard, err := requiredAttributeWildcard(rt, id)
	if err != nil {
		return err
	}
	process := wildcard.Process
	if b.wildcard != NoWildcard {
		current, err := requiredAttributeWildcard(rt, b.wildcard)
		if err != nil {
			return err
		}
		process = current.Process
	}
	return b.add(rt, id, process)
}

// addAnyAttribute merges a local anyAttribute wildcard into the local
// declaration.
func (b *attributeWildcardBuilder) addAnyAttribute(rt schemaAttributeWildcardRuntime, id WildcardID) error {
	if id == NoWildcard {
		return nil
	}
	wildcard, err := requiredAttributeWildcard(rt, id)
	if err != nil {
		return err
	}
	return b.add(rt, id, wildcard.Process)
}

func (b *attributeWildcardBuilder) add(rt schemaAttributeWildcardRuntime, id WildcardID, process ProcessContents) error {
	if b.wildcard == NoWildcard {
		b.wildcard = id
		return nil
	}
	current, err := requiredAttributeWildcard(rt, b.wildcard)
	if err != nil {
		return err
	}
	next, err := requiredAttributeWildcard(rt, id)
	if err != nil {
		return err
	}
	intersection, err := intersectWildcard(current, next, process)
	if err != nil {
		return xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, err.Error())
	}
	intersectionID, err := rt.AddWildcard(intersection)
	if err != nil {
		return err
	}
	b.wildcard = intersectionID
	return nil
}

// finish returns the wildcard produced by the builder's derivation operation.
func (b *attributeWildcardBuilder) finish(rt schemaAttributeWildcardRuntime) (WildcardID, error) {
	switch b.mode {
	case attributeMergeDirect:
		return b.wildcard, nil
	case attributeMergeExtension:
		return b.finishExtension(rt)
	case attributeMergeRestriction:
		return b.finishRestriction(rt)
	case attributeMergeInvalid:
		return NoWildcard, xsderrors.InternalInvariant("invalid attribute merge mode")
	default:
		return NoWildcard, xsderrors.InternalInvariant("unknown attribute merge mode")
	}
}

func (b *attributeWildcardBuilder) finishExtension(rt schemaAttributeWildcardRuntime) (WildcardID, error) {
	if b.inheritedWildcard == NoWildcard {
		return b.wildcard, nil
	}
	if b.wildcard == NoWildcard {
		return b.inheritedWildcard, nil
	}
	declared, err := requiredAttributeWildcard(rt, b.wildcard)
	if err != nil {
		return NoWildcard, err
	}
	inherited, err := requiredAttributeWildcard(rt, b.inheritedWildcard)
	if err != nil {
		return NoWildcard, err
	}
	union, err := unionWildcard(declared, inherited, declared.Process)
	if err != nil {
		return NoWildcard, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, err.Error())
	}
	return rt.AddWildcard(union)
}

func (b *attributeWildcardBuilder) finishRestriction(rt schemaAttributeWildcardRuntime) (WildcardID, error) {
	if b.wildcard == NoWildcard {
		return NoWildcard, nil
	}
	if b.inheritedWildcard == NoWildcard {
		return NoWildcard, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "attribute wildcard restriction requires base wildcard")
	}
	declared, err := requiredAttributeWildcard(rt, b.wildcard)
	if err != nil {
		return NoWildcard, err
	}
	inherited, err := requiredAttributeWildcard(rt, b.inheritedWildcard)
	if err != nil {
		return NoWildcard, err
	}
	if !wildcardNamespaceSubset(declared, inherited) {
		return NoWildcard, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "attribute wildcard restriction is not subset of base")
	}
	return b.wildcard, nil
}

// schemaAttributeWildcardDerivation returns the provenance kind stored on the
// compiled attribute-use set.
func schemaAttributeWildcardDerivation(mode attributeMergeMode) (attributeWildcardDerivation, error) {
	switch mode {
	case attributeMergeDirect:
		return attributeWildcardNone, nil
	case attributeMergeExtension:
		return attributeWildcardExtension, nil
	case attributeMergeRestriction:
		return attributeWildcardRestriction, nil
	case attributeMergeInvalid:
		return attributeWildcardNone, xsderrors.InternalInvariant("invalid attribute merge mode")
	default:
		return attributeWildcardNone, xsderrors.InternalInvariant("unknown attribute merge mode")
	}
}

func requiredAttributeWildcard(rt schemaAttributeWildcardRuntime, id WildcardID) (wildcardRuntime, error) {
	wildcard, ok := rt.Wildcard(id)
	if !ok {
		return wildcardRuntime{}, xsderrors.InternalInvariant("attribute wildcard references missing wildcard")
	}
	return wildcard, nil
}
