package compile

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/xsderrors"
)

// AttributeUseMergeRuntime supplies compile-time type derivation and wildcard
// metadata needed to merge attribute uses.
type AttributeUseMergeRuntime interface {
	runtime.TypeDerivationRuntime
	runtime.AttributeWildcardRuntime
}

// AttributeUseMergeResult tells callers how to mirror an internal merge into
// their concrete attribute-use storage.
type AttributeUseMergeResult struct {
	Index    int
	Appended bool
}

// AttributeUseChildKind classifies an XSD child in an attribute-use container.
type AttributeUseChildKind uint8

const (
	// AttributeUseChildIgnored is a child handled by another parent grammar.
	AttributeUseChildIgnored AttributeUseChildKind = iota
	// AttributeUseChildAttribute is an xs:attribute child.
	AttributeUseChildAttribute
	// AttributeUseChildGroup is an xs:attributeGroup child.
	AttributeUseChildGroup
	// AttributeUseChildWildcard is an xs:anyAttribute child.
	AttributeUseChildWildcard
)

// AttributeUseMerger owns compile-time duplicate, restriction, and wildcard
// admission policy for attribute uses.
type AttributeUseMerger struct {
	seen              map[runtime.QName]int
	inheritedWildcard runtime.WildcardID
	mode              AttributeMergeMode
}

// NewAttributeUseMerger creates a merger seeded with inherited attribute uses.
func NewAttributeUseMerger(
	inherited []runtime.AttributeUse,
	inheritedWildcard runtime.WildcardID,
	mode AttributeMergeMode,
) AttributeUseMerger {
	seen := make(map[runtime.QName]int, len(inherited))
	for i := range inherited {
		seen[inherited[i].Name] = i
	}
	return AttributeUseMerger{
		seen:              seen,
		inheritedWildcard: inheritedWildcard,
		mode:              mode,
	}
}

// Add merges use and returns the concrete storage operation callers must apply.
func (m *AttributeUseMerger) Add(rt AttributeUseMergeRuntime, uses []runtime.AttributeUse, use runtime.AttributeUse) (AttributeUseMergeResult, error) {
	if i, ok := m.seen[use.Name]; ok {
		if i >= len(uses) {
			return AttributeUseMergeResult{}, xsderrors.InternalInvariant("attribute use merger index outside concrete use set")
		}
		if m.mode != AttributeMergeRestriction && !uses[i].Prohibited && !use.Prohibited {
			return AttributeUseMergeResult{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaDuplicate, "duplicate attribute use")
		}
		if m.mode == AttributeMergeRestriction {
			base := runtime.NewAttributeUseRestrictionValidationForUse(uses[i])
			derived := runtime.NewAttributeUseRestrictionValidationForUse(use)
			if err := runtime.ValidateAttributeUseRestriction(rt, base, derived); err != nil {
				return AttributeUseMergeResult{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, err.Error())
			}
		}
		return AttributeUseMergeResult{Index: i}, nil
	}
	if m.mode == AttributeMergeRestriction && !use.Prohibited {
		if !m.inheritedWildcardAllows(rt, use.Name) {
			return AttributeUseMergeResult{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaInvalidAttribute, "new restricted attribute is not allowed by base wildcard")
		}
	}
	m.seen[use.Name] = len(uses)
	return AttributeUseMergeResult{Index: len(uses), Appended: true}, nil
}

// ClassifyAttributeUseChild returns the compile action for an XSD child local
// name inside an attribute-use container.
func ClassifyAttributeUseChild(local string) AttributeUseChildKind {
	switch local {
	case attributeChild:
		return AttributeUseChildAttribute
	case attributeGroup:
		return AttributeUseChildGroup
	case anyAttribute:
		return AttributeUseChildWildcard
	default:
		return AttributeUseChildIgnored
	}
}

func (m *AttributeUseMerger) inheritedWildcardAllows(rt AttributeUseMergeRuntime, name runtime.QName) bool {
	if m.inheritedWildcard == runtime.NoWildcard {
		return false
	}
	wildcard, ok := rt.Wildcard(m.inheritedWildcard)
	return ok && runtime.WildcardAllowsNamespace(wildcard, name.Namespace)
}

// RemoveProhibitedAttributeUses removes prohibited uses from the final
// attribute-use set while preserving the order of admitted uses.
func RemoveProhibitedAttributeUses(uses []runtime.AttributeUse) []runtime.AttributeUse {
	out := uses[:0]
	for _, use := range uses {
		if !use.Prohibited {
			out = append(out, use)
		}
	}
	return out
}
