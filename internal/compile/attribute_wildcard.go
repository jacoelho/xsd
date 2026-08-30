package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

// AttributeMergeMode identifies how local attribute uses combine with inherited
// uses during compile-time derivation.
type AttributeMergeMode uint8

const (
	// AttributeMergeInvalid is not a valid attribute derivation operation.
	AttributeMergeInvalid AttributeMergeMode = iota
	// AttributeMergeDirect appends locally declared uses without inheriting a
	// wildcard.
	AttributeMergeDirect
	// AttributeMergeExtension appends local uses and unions local and inherited
	// wildcards.
	AttributeMergeExtension
	// AttributeMergeRestriction validates local uses and wildcard declarations
	// as restrictions of inherited attributes.
	AttributeMergeRestriction
)

// AttributeWildcardRuntime supplies compile-time wildcard metadata and stores
// wildcard values produced by intersection or union.
type AttributeWildcardRuntime interface {
	Wildcard(id runtime.WildcardID) (runtime.Wildcard, bool)
	AddWildcard(w runtime.Wildcard) (runtime.WildcardID, error)
}

// AttributeWildcardBuilder owns compile-time attribute-wildcard construction.
type AttributeWildcardBuilder struct {
	wildcard          runtime.WildcardID
	inheritedWildcard runtime.WildcardID
	mode              AttributeMergeMode
}

// NewAttributeWildcardBuilder creates a builder for local attribute wildcard
// declarations over an optional inherited wildcard.
func NewAttributeWildcardBuilder(inherited runtime.WildcardID, mode AttributeMergeMode) AttributeWildcardBuilder {
	return AttributeWildcardBuilder{
		wildcard:          runtime.NoWildcard,
		inheritedWildcard: inherited,
		mode:              mode,
	}
}

// Declared returns the wildcard produced by local anyAttribute and attribute
// group declarations before extension inheritance is applied.
func (b *AttributeWildcardBuilder) Declared() runtime.WildcardID {
	return b.wildcard
}

// AddGroup merges an attribute group's wildcard into the local declaration.
func (b *AttributeWildcardBuilder) AddGroup(rt AttributeWildcardRuntime, id runtime.WildcardID) error {
	if id == runtime.NoWildcard {
		return nil
	}
	wildcard, err := requiredAttributeWildcard(rt, id)
	if err != nil {
		return err
	}
	process := wildcard.Process
	if b.wildcard != runtime.NoWildcard {
		current, err := requiredAttributeWildcard(rt, b.wildcard)
		if err != nil {
			return err
		}
		process = current.Process
	}
	return b.add(rt, id, process)
}

// AddAnyAttribute merges a local anyAttribute wildcard into the local
// declaration.
func (b *AttributeWildcardBuilder) AddAnyAttribute(rt AttributeWildcardRuntime, id runtime.WildcardID) error {
	if id == runtime.NoWildcard {
		return nil
	}
	wildcard, err := requiredAttributeWildcard(rt, id)
	if err != nil {
		return err
	}
	return b.add(rt, id, wildcard.Process)
}

func (b *AttributeWildcardBuilder) add(rt AttributeWildcardRuntime, id runtime.WildcardID, process runtime.ProcessContents) error {
	if b.wildcard == runtime.NoWildcard {
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
	intersection, err := runtime.IntersectWildcard(current, next, process)
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

// Finish returns the wildcard produced by the builder's derivation operation.
func (b *AttributeWildcardBuilder) Finish(rt AttributeWildcardRuntime) (runtime.WildcardID, error) {
	switch b.mode {
	case AttributeMergeDirect:
		return b.wildcard, nil
	case AttributeMergeExtension:
		return b.finishExtension(rt)
	case AttributeMergeRestriction:
		return b.finishRestriction(rt)
	case AttributeMergeInvalid:
		return runtime.NoWildcard, xsderrors.InternalInvariant("invalid attribute merge mode")
	default:
		return runtime.NoWildcard, xsderrors.InternalInvariant("unknown attribute merge mode")
	}
}

func (b *AttributeWildcardBuilder) finishExtension(rt AttributeWildcardRuntime) (runtime.WildcardID, error) {
	if b.inheritedWildcard == runtime.NoWildcard {
		return b.wildcard, nil
	}
	if b.wildcard == runtime.NoWildcard {
		return b.inheritedWildcard, nil
	}
	declared, err := requiredAttributeWildcard(rt, b.wildcard)
	if err != nil {
		return runtime.NoWildcard, err
	}
	inherited, err := requiredAttributeWildcard(rt, b.inheritedWildcard)
	if err != nil {
		return runtime.NoWildcard, err
	}
	union, err := runtime.UnionWildcard(declared, inherited, declared.Process)
	if err != nil {
		return runtime.NoWildcard, xsderrors.SchemaCompile(xsderrors.CodeSchemaContentModel, err.Error())
	}
	return rt.AddWildcard(union)
}

func (b *AttributeWildcardBuilder) finishRestriction(rt AttributeWildcardRuntime) (runtime.WildcardID, error) {
	if b.wildcard == runtime.NoWildcard {
		return runtime.NoWildcard, nil
	}
	if b.inheritedWildcard == runtime.NoWildcard {
		return runtime.NoWildcard, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "attribute wildcard restriction requires base wildcard")
	}
	declared, err := requiredAttributeWildcard(rt, b.wildcard)
	if err != nil {
		return runtime.NoWildcard, err
	}
	inherited, err := requiredAttributeWildcard(rt, b.inheritedWildcard)
	if err != nil {
		return runtime.NoWildcard, err
	}
	if !runtime.WildcardSubset(declared, inherited) {
		return runtime.NoWildcard, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "attribute wildcard restriction is not subset of base")
	}
	return b.wildcard, nil
}

// AttributeWildcardDerivation returns the provenance kind stored on the
// compiled attribute-use set.
func AttributeWildcardDerivation(mode AttributeMergeMode) (runtime.AttributeWildcardDerivation, error) {
	switch mode {
	case AttributeMergeDirect:
		return runtime.AttributeWildcardNone, nil
	case AttributeMergeExtension:
		return runtime.AttributeWildcardExtension, nil
	case AttributeMergeRestriction:
		return runtime.AttributeWildcardRestriction, nil
	case AttributeMergeInvalid:
		return runtime.AttributeWildcardNone, xsderrors.InternalInvariant("invalid attribute merge mode")
	default:
		return runtime.AttributeWildcardNone, xsderrors.InternalInvariant("unknown attribute merge mode")
	}
}

func requiredAttributeWildcard(rt AttributeWildcardRuntime, id runtime.WildcardID) (runtime.Wildcard, error) {
	wildcard, ok := rt.Wildcard(id)
	if !ok {
		return runtime.Wildcard{}, xsderrors.InternalInvariant("attribute wildcard references missing wildcard")
	}
	return wildcard, nil
}
