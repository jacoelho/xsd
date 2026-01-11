package types

import (
	"maps"
	"slices"
)

// CopyOptions configures how schema components are copied during merge.
type CopyOptions struct {
	RemapQName      func(QName) QName
	SourceNamespace NamespaceURI
}

// NilRemap returns qname unchanged (for non-chameleon merges)
func NilRemap(qname QName) QName { return qname }

// CopyType creates a copy of a type with remapped QNames.
// Used for chameleon includes where type references need to be remapped to the target namespace.
func CopyType(typ Type, opts CopyOptions) Type {
	if typ == nil {
		return nil
	}
	switch t := typ.(type) {
	case *SimpleType:
		return t.Copy(opts)
	case *ComplexType:
		return t.Copy(opts)
	case *BuiltinType:
		// built-in types don't need remapping (they're in XSD namespace)
		return t
	default:
		return typ
	}
}

func copyAnyElement(elem *AnyElement) *AnyElement {
	if elem == nil {
		return nil
	}
	clone := *elem
	if len(elem.NamespaceList) > 0 {
		clone.NamespaceList = slices.Clone(elem.NamespaceList)
	}
	return &clone
}

func copyAnyAttribute(attr *AnyAttribute) *AnyAttribute {
	if attr == nil {
		return nil
	}
	clone := *attr
	if len(attr.NamespaceList) > 0 {
		clone.NamespaceList = slices.Clone(attr.NamespaceList)
	}
	return &clone
}

func copyQNameSlice(values []QName, remap func(QName) QName) []QName {
	if len(values) == 0 {
		return nil
	}
	out := make([]QName, len(values))
	for i, value := range values {
		out[i] = remap(value)
	}
	return out
}

func copyAttributeDecls(attrs []*AttributeDecl, opts CopyOptions) []*AttributeDecl {
	if len(attrs) == 0 {
		return nil
	}
	out := make([]*AttributeDecl, len(attrs))
	for i, attr := range attrs {
		out[i] = attr.Copy(opts)
	}
	return out
}

func copyFields(fields []Field) []Field {
	return slices.Clone(fields)
}

func copyNamespaceContext(src map[string]string) map[string]string {
	return maps.Clone(src)
}

func copyIdentityConstraints(constraints []*IdentityConstraint, opts CopyOptions) []*IdentityConstraint {
	if len(constraints) == 0 {
		return nil
	}
	out := make([]*IdentityConstraint, len(constraints))
	for i, constraint := range constraints {
		if constraint == nil {
			continue
		}
		clone := *constraint
		clone.TargetNamespace = opts.SourceNamespace
		if !constraint.ReferQName.IsZero() && constraint.ReferQName.Namespace.IsEmpty() {
			clone.ReferQName = opts.RemapQName(constraint.ReferQName)
		}
		clone.Fields = copyFields(constraint.Fields)
		clone.NamespaceContext = copyNamespaceContext(constraint.NamespaceContext)
		out[i] = &clone
	}
	return out
}

// copyParticle creates a copy of a particle with remapped QNames.
func copyParticle(particle Particle, opts CopyOptions) Particle {
	if particle == nil {
		return nil
	}
	switch p := particle.(type) {
	case *ElementDecl:
		return p.Copy(opts)
	case *ModelGroup:
		return p.Copy(opts)
	case *GroupRef:
		clone := *p
		clone.RefQName = opts.RemapQName(p.RefQName)
		return &clone
	case *AnyElement:
		return copyAnyElement(p)
	default:
		return particle
	}
}
