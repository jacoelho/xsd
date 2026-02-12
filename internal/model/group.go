package model

import (
	"slices"

	"github.com/jacoelho/xsd/internal/occurs"
)

// Particle represents a content model particle
type Particle interface {
	MinOcc() occurs.Occurs
	MaxOcc() occurs.Occurs
}

// GroupKind represents the kind of model group in XSD content models.
type GroupKind int

const (
	// Sequence indicates elements must appear in the specified order.
	Sequence GroupKind = iota
	// Choice indicates exactly one of the elements must appear.
	Choice
	// AllGroup indicates all elements may appear in any order, each at most once.
	AllGroup
)

// ModelGroup represents sequence, choice, or all groups
type ModelGroup struct {
	SourceNamespace NamespaceURI
	Particles       []Particle
	MinOccurs       occurs.Occurs
	MaxOccurs       occurs.Occurs
	Kind            GroupKind
}

// MinOcc implements Particle interface
func (m *ModelGroup) MinOcc() occurs.Occurs {
	return m.MinOccurs
}

// MaxOcc implements Particle interface
func (m *ModelGroup) MaxOcc() occurs.Occurs {
	return m.MaxOccurs
}

// Copy creates a copy of the model group with remapped QNames.
func (m *ModelGroup) Copy(opts CopyOptions) *ModelGroup {
	if existing, ok := opts.lookupModelGroup(m); ok {
		return existing
	}
	clone := *m
	opts.rememberModelGroup(m, &clone)
	clone.SourceNamespace = sourceNamespace(m.SourceNamespace, opts)
	if len(m.Particles) > 0 {
		clone.Particles = make([]Particle, len(m.Particles))
		for i, particle := range m.Particles {
			clone.Particles[i] = copyParticle(particle, opts)
		}
	}
	return &clone
}

// Content represents element content.
// SimpleContent and ComplexContent return derivation info; others return nil.
type Content interface {
	isContent()
	BaseTypeQName() QName
	ExtensionDef() *Extension
	RestrictionDef() *Restriction
	Copy(opts CopyOptions) Content
}

// SimpleContent represents simple content in a complex type
type SimpleContent struct {
	Extension   *Extension
	Restriction *Restriction
	Base        QName
}

func (s *SimpleContent) isContent() {}

// ExtensionDef returns the extension if present.
func (s *SimpleContent) ExtensionDef() *Extension { return s.Extension }

// RestrictionDef returns the restriction if present.
func (s *SimpleContent) RestrictionDef() *Restriction { return s.Restriction }

// BaseTypeQName returns the base type QName from Extension or Restriction.
func (s *SimpleContent) BaseTypeQName() QName {
	if !s.Base.IsZero() {
		return s.Base
	}
	if s.Extension != nil {
		return s.Extension.Base
	}
	if s.Restriction != nil {
		return s.Restriction.Base
	}
	return QName{}
}

// Copy creates a copy of the simple content with remapped QNames.
func (s *SimpleContent) Copy(opts CopyOptions) Content {
	clone := *s
	if !s.Base.IsZero() && s.Base.Namespace == "" {
		clone.Base = opts.RemapQName(s.Base)
	}
	if s.Extension != nil {
		extCopy := copyExtension(s.Extension, opts)
		clone.Extension = extCopy
	}
	if s.Restriction != nil {
		restrCopy := copyRestriction(s.Restriction, opts)
		clone.Restriction = restrCopy
	}
	return &clone
}

// ComplexContent represents complex content
type ComplexContent struct {
	Extension   *Extension
	Restriction *Restriction
	Base        QName
	Mixed       bool
	// MixedSpecified reports whether mixed was explicitly set on complexContent.
	MixedSpecified bool
}

func (c *ComplexContent) isContent() {}

// ExtensionDef returns the extension if present.
func (c *ComplexContent) ExtensionDef() *Extension { return c.Extension }

// RestrictionDef returns the restriction if present.
func (c *ComplexContent) RestrictionDef() *Restriction { return c.Restriction }

// BaseTypeQName returns the base type QName from Extension or Restriction.
func (c *ComplexContent) BaseTypeQName() QName {
	if !c.Base.IsZero() {
		return c.Base
	}
	if c.Extension != nil {
		return c.Extension.Base
	}
	if c.Restriction != nil {
		return c.Restriction.Base
	}
	return QName{}
}

// Copy creates a copy of the complex content with remapped QNames.
func (c *ComplexContent) Copy(opts CopyOptions) Content {
	clone := *c
	if !c.Base.IsZero() && c.Base.Namespace == "" {
		clone.Base = opts.RemapQName(c.Base)
	}
	if c.Extension != nil {
		extCopy := copyExtension(c.Extension, opts)
		clone.Extension = extCopy
	}
	if c.Restriction != nil {
		restrCopy := copyRestriction(c.Restriction, opts)
		clone.Restriction = restrCopy
	}
	return &clone
}

// ElementContent represents element-only or mixed content
type ElementContent struct {
	Particle Particle
}

func (e *ElementContent) isContent() {}

// BaseTypeQName returns an empty base QName for element content.
func (e *ElementContent) BaseTypeQName() QName { return QName{} }

// ExtensionDef returns nil for element content.
func (e *ElementContent) ExtensionDef() *Extension { return nil }

// RestrictionDef returns nil for element content.
func (e *ElementContent) RestrictionDef() *Restriction { return nil }

// Copy creates a copy of the element content with remapped particles.
func (e *ElementContent) Copy(opts CopyOptions) Content {
	clone := *e
	if e.Particle != nil {
		clone.Particle = copyParticle(e.Particle, opts)
	}
	return &clone
}

// EmptyContent represents empty content
type EmptyContent struct{}

func (e *EmptyContent) isContent() {}

// BaseTypeQName returns an empty base QName for empty content.
func (e *EmptyContent) BaseTypeQName() QName { return QName{} }

// ExtensionDef returns nil for empty content.
func (e *EmptyContent) ExtensionDef() *Extension { return nil }

// RestrictionDef returns nil for empty content.
func (e *EmptyContent) RestrictionDef() *Restriction { return nil }

// Copy creates a copy of the empty content.
func (e *EmptyContent) Copy(_ CopyOptions) Content {
	return &EmptyContent{}
}

func copyExtension(ext *Extension, opts CopyOptions) *Extension {
	if ext == nil {
		return nil
	}
	clone := *ext
	clone.Attributes = copyAttributeDecls(ext.Attributes, opts)
	if !ext.Base.IsZero() && ext.Base.Namespace == "" {
		clone.Base = opts.RemapQName(ext.Base)
	}
	clone.AttrGroups = copyQNameSlice(ext.AttrGroups, opts.RemapQName)
	clone.AnyAttribute = copyAnyAttribute(ext.AnyAttribute, opts)
	return &clone
}

func copyRestriction(restriction *Restriction, opts CopyOptions) *Restriction {
	if restriction == nil {
		return nil
	}
	clone := *restriction
	clone.Attributes = copyAttributeDecls(restriction.Attributes, opts)
	if !restriction.Base.IsZero() && restriction.Base.Namespace == "" {
		clone.Base = opts.RemapQName(restriction.Base)
	}
	clone.AttrGroups = copyQNameSlice(restriction.AttrGroups, opts.RemapQName)
	if len(restriction.Facets) > 0 {
		clone.Facets = copyRestrictionFacets(restriction.Facets, opts)
	}
	if restriction.Particle != nil {
		clone.Particle = copyParticle(restriction.Particle, opts)
	}
	clone.AnyAttribute = copyAnyAttribute(restriction.AnyAttribute, opts)
	if restriction.SimpleType != nil {
		clone.SimpleType = restriction.SimpleType.Copy(opts)
	}
	return &clone
}

func copyRestrictionFacets(facets []any, opts CopyOptions) []any {
	if len(facets) == 0 {
		return nil
	}
	needsRemap := isChameleonRemap(opts)
	if !needsRemap {
		return slices.Clone(facets)
	}
	out := make([]any, len(facets))
	for i, facet := range facets {
		if enum, ok := facet.(*Enumeration); ok {
			out[i] = copyEnumerationFacet(enum, opts)
			continue
		}
		out[i] = facet
	}
	return out
}

func copyEnumerationFacet(enum *Enumeration, opts CopyOptions) *Enumeration {
	if enum == nil {
		return nil
	}
	clone := NewEnumeration(enum.values)
	if contexts := enum.ValueContexts(); len(contexts) > 0 {
		copied := make([]map[string]string, len(contexts))
		for i, ctx := range contexts {
			copied[i] = copyValueNamespaceContext(ctx, opts)
		}
		clone.SetValueContexts(copied)
	}
	if isChameleonRemap(opts) {
		clone.SetQNameValues(nil)
	} else if qnames := enum.QNameValues(); len(qnames) > 0 {
		clone.SetQNameValues(slices.Clone(qnames))
	}
	if enum.sealed.Load() {
		clone.Seal()
	}
	return clone
}

// GroupRef represents a placeholder for a group reference that will be resolved later
// This allows forward references and references to groups in imported schemas
type GroupRef struct {
	RefQName  QName
	MinOccurs occurs.Occurs
	MaxOccurs occurs.Occurs
}

// MinOcc implements Particle interface
func (g *GroupRef) MinOcc() occurs.Occurs {
	return g.MinOccurs
}

// MaxOcc implements Particle interface
func (g *GroupRef) MaxOcc() occurs.Occurs {
	return g.MaxOccurs
}
