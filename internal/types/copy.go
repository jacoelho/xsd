package types

import (
	"maps"
	"slices"
)

// CopyOptions configures how schema components are copied during merge.
type CopyOptions struct {
	RemapQName              func(QName) QName
	memo                    *copyMemo
	SourceNamespace         NamespaceURI
	PreserveSourceNamespace bool
}

type copyMemo struct {
	simpleTypes  map[*SimpleType]*SimpleType
	complexTypes map[*ComplexType]*ComplexType
	modelGroups  map[*ModelGroup]*ModelGroup
	elementDecls map[*ElementDecl]*ElementDecl
}

// WithGraphMemo enables cycle-safe graph copy memoization for copy operations.
func WithGraphMemo(opts CopyOptions) CopyOptions {
	if opts.memo == nil {
		opts.memo = &copyMemo{}
	}
	return opts
}

func (opts CopyOptions) lookupSimpleType(src *SimpleType) (*SimpleType, bool) {
	if src == nil || opts.memo == nil || opts.memo.simpleTypes == nil {
		return nil, false
	}
	dst, ok := opts.memo.simpleTypes[src]
	return dst, ok
}

func (opts CopyOptions) rememberSimpleType(src, dst *SimpleType) {
	if src == nil || dst == nil || opts.memo == nil {
		return
	}
	if opts.memo.simpleTypes == nil {
		opts.memo.simpleTypes = make(map[*SimpleType]*SimpleType)
	}
	opts.memo.simpleTypes[src] = dst
}

func (opts CopyOptions) lookupComplexType(src *ComplexType) (*ComplexType, bool) {
	if src == nil || opts.memo == nil || opts.memo.complexTypes == nil {
		return nil, false
	}
	dst, ok := opts.memo.complexTypes[src]
	return dst, ok
}

func (opts CopyOptions) rememberComplexType(src, dst *ComplexType) {
	if src == nil || dst == nil || opts.memo == nil {
		return
	}
	if opts.memo.complexTypes == nil {
		opts.memo.complexTypes = make(map[*ComplexType]*ComplexType)
	}
	opts.memo.complexTypes[src] = dst
}

func (opts CopyOptions) lookupModelGroup(src *ModelGroup) (*ModelGroup, bool) {
	if src == nil || opts.memo == nil || opts.memo.modelGroups == nil {
		return nil, false
	}
	dst, ok := opts.memo.modelGroups[src]
	return dst, ok
}

func (opts CopyOptions) rememberModelGroup(src, dst *ModelGroup) {
	if src == nil || dst == nil || opts.memo == nil {
		return
	}
	if opts.memo.modelGroups == nil {
		opts.memo.modelGroups = make(map[*ModelGroup]*ModelGroup)
	}
	opts.memo.modelGroups[src] = dst
}

func (opts CopyOptions) lookupElementDecl(src *ElementDecl) (*ElementDecl, bool) {
	if src == nil || opts.memo == nil || opts.memo.elementDecls == nil {
		return nil, false
	}
	dst, ok := opts.memo.elementDecls[src]
	return dst, ok
}

func (opts CopyOptions) rememberElementDecl(src, dst *ElementDecl) {
	if src == nil || dst == nil || opts.memo == nil {
		return
	}
	if opts.memo.elementDecls == nil {
		opts.memo.elementDecls = make(map[*ElementDecl]*ElementDecl)
	}
	opts.memo.elementDecls[src] = dst
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

func copyAnyElement(elem *AnyElement, opts CopyOptions) *AnyElement {
	if elem == nil {
		return nil
	}
	clone := *elem
	if !opts.PreserveSourceNamespace && clone.TargetNamespace.IsEmpty() && !opts.SourceNamespace.IsEmpty() {
		clone.TargetNamespace = opts.SourceNamespace
	}
	if len(elem.NamespaceList) > 0 {
		clone.NamespaceList = slices.Clone(elem.NamespaceList)
	}
	return &clone
}

func copyAnyAttribute(attr *AnyAttribute, opts CopyOptions) *AnyAttribute {
	if attr == nil {
		return nil
	}
	clone := *attr
	if !opts.PreserveSourceNamespace && clone.TargetNamespace.IsEmpty() && !opts.SourceNamespace.IsEmpty() {
		clone.TargetNamespace = opts.SourceNamespace
	}
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

func copyValueNamespaceContext(src map[string]string, opts CopyOptions) map[string]string {
	if src == nil {
		return nil
	}
	clone := maps.Clone(src)
	if !isChameleonRemap(opts) {
		return clone
	}
	clone[""] = opts.SourceNamespace.String()
	return clone
}

func isChameleonRemap(opts CopyOptions) bool {
	if opts.RemapQName == nil || opts.SourceNamespace.IsEmpty() {
		return false
	}
	remapped := opts.RemapQName(QName{Local: "x"})
	return remapped.Namespace == opts.SourceNamespace && !remapped.Namespace.IsEmpty()
}

func copyIdentityConstraints(constraints []*IdentityConstraint, opts CopyOptions) []*IdentityConstraint {
	if len(constraints) == 0 {
		return nil
	}
	out := make([]*IdentityConstraint, 0, len(constraints))
	for _, constraint := range constraints {
		if constraint == nil {
			continue
		}
		clone := *constraint
		if opts.PreserveSourceNamespace {
			clone.TargetNamespace = constraint.TargetNamespace
		} else {
			clone.TargetNamespace = opts.SourceNamespace
		}
		if !constraint.ReferQName.IsZero() && constraint.ReferQName.Namespace.IsEmpty() {
			clone.ReferQName = opts.RemapQName(constraint.ReferQName)
		}
		clone.Fields = slices.Clone(constraint.Fields)
		clone.NamespaceContext = maps.Clone(constraint.NamespaceContext)
		out = append(out, &clone)
	}
	if len(out) == 0 {
		return nil
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
		return copyAnyElement(p, opts)
	default:
		return particle
	}
}

func sourceNamespace(original NamespaceURI, opts CopyOptions) NamespaceURI {
	if opts.PreserveSourceNamespace {
		return original
	}
	return opts.SourceNamespace
}
