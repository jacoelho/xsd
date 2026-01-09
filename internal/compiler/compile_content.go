package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/grammar"
	xsdschema "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *Compiler) compileContentModel(ct *types.ComplexType) *grammar.CompiledContentModel {
	content := ct.Content()
	if content == nil {
		return &grammar.CompiledContentModel{Empty: true}
	}

	switch cnt := content.(type) {
	case *types.EmptyContent:
		return &grammar.CompiledContentModel{Empty: true}

	case *types.ElementContent:
		if cnt.Particle == nil {
			return &grammar.CompiledContentModel{Empty: true}
		}
		minOccurs := 1
		var mg *types.ModelGroup
		if typedMG, ok := cnt.Particle.(*types.ModelGroup); ok {
			mg = typedMG
			minOccurs = mg.MinOccurs
		}
		particles := c.flattenParticles(cnt.Particle)
		// if flattening produces no particles (e.g., empty group), mark as empty
		if len(particles) == 0 {
			if mg != nil && mg.Kind == types.Choice && len(mg.Particles) == 0 {
				return &grammar.CompiledContentModel{
					Kind:      mg.Kind,
					RejectAll: true,
					Mixed:     ct.Mixed(),
					MinOccurs: minOccurs,
				}
			}
			return &grammar.CompiledContentModel{Empty: true, Mixed: ct.Mixed()}
		}
		return &grammar.CompiledContentModel{
			Kind:      c.getGroupKind(cnt.Particle),
			Particles: particles,
			Mixed:     ct.Mixed(),
			MinOccurs: minOccurs,
		}

	case *types.ComplexContent:
		return c.compileComplexContent(cnt, ct.Mixed())

	case *types.SimpleContent:
		// simple content - no element content model
		return &grammar.CompiledContentModel{Empty: true}
	}

	return &grammar.CompiledContentModel{Empty: true}
}

func (c *Compiler) flattenParticles(particle types.Particle) []*grammar.CompiledParticle {
	// inline ModelGroups are tree-structured (no pointer cycles) from parser
	expandedGroups := make(map[types.QName]bool)
	return c.flattenParticle(particle, expandedGroups)
}

func (c *Compiler) flattenParticle(particle types.Particle, expandedGroups map[types.QName]bool) []*grammar.CompiledParticle {
	switch p := particle.(type) {
	case *types.ModelGroup:
		var children []*grammar.CompiledParticle
		for _, child := range p.Particles {
			children = append(children, c.flattenParticle(child, expandedGroups)...)
		}
		if len(children) > 0 {
			return []*grammar.CompiledParticle{{
				Kind:      grammar.ParticleGroup,
				MinOccurs: p.MinOccurs,
				MaxOccurs: p.MaxOccurs,
				GroupKind: p.Kind,
				Children:  children,
			}}
		}
		return nil

	case *types.ElementDecl:
		compiled := &grammar.CompiledParticle{
			Kind:        grammar.ParticleElement,
			MinOccurs:   p.MinOccurs,
			MaxOccurs:   p.MaxOccurs,
			IsReference: p.IsReference,
		}
		if c.schema != nil {
			if capMax, ok := c.schema.ParticleRestrictionCaps[p]; ok && capMax >= 0 {
				if compiled.MaxOccurs == types.UnboundedOccurs || compiled.MaxOccurs > capMax {
					if compiled.MinOccurs <= capMax {
						compiled.MaxOccurs = capMax
					}
				}
			}
		}
		if p.IsReference {
			// reference to a top-level element - reuse the global compiled element.
			if elem, ok := c.elements[p.Name]; ok {
				compiled.Element = elem
			} else if elemDecl, ok := c.schema.ElementDecls[p.Name]; ok {
				if compiledElem, err := c.compileElement(p.Name, elemDecl, true); err == nil {
					compiled.Element = compiledElem
				}
			}
		} else {
			// local element - compile directly from the particle's ElementDecl.
			if compiledElem, err := c.compileElement(p.Name, p, false); err == nil {
				compiled.Element = compiledElem
			}
		}
		return []*grammar.CompiledParticle{compiled}

	case *types.GroupRef:
		// cycle detection via QName - check if already expanding this group
		if expandedGroups[p.RefQName] {
			// circular group reference detected - return empty result
			return nil
		}
		expandedGroups[p.RefQName] = true
		defer func() { delete(expandedGroups, p.RefQName) }()

		// expand group reference
		if group, ok := c.schema.Groups[p.RefQName]; ok {
			if p.MaxOccurs == 0 {
				return nil
			}
			var children []*grammar.CompiledParticle
			for _, child := range group.Particles {
				children = append(children, c.flattenParticle(child, expandedGroups)...)
			}
			if len(children) == 0 {
				return nil
			}
			return []*grammar.CompiledParticle{{
				Kind:      grammar.ParticleGroup,
				MinOccurs: p.MinOccurs,
				MaxOccurs: p.MaxOccurs,
				GroupKind: group.Kind,
				Children:  children,
			}}
		}
		return nil

	case *types.AnyElement:
		return []*grammar.CompiledParticle{{
			Kind:      grammar.ParticleWildcard,
			MinOccurs: p.MinOccurs,
			MaxOccurs: p.MaxOccurs,
			Wildcard:  p,
		}}
	}

	return nil
}

func (c *Compiler) compileComplexContent(cc *types.ComplexContent, mixed bool) *grammar.CompiledContentModel {
	cm := &grammar.CompiledContentModel{
		Mixed: mixed || cc.Mixed,
	}

	if cc.Extension != nil && cc.Extension.Particle != nil {
		cm.Kind = c.getGroupKind(cc.Extension.Particle)
		cm.Particles = c.flattenParticles(cc.Extension.Particle)
		// if flattening produces no particles (e.g., empty group), mark as empty
		if len(cm.Particles) == 0 {
			if mg, ok := cc.Extension.Particle.(*types.ModelGroup); ok && mg.Kind == types.Choice && len(mg.Particles) == 0 {
				cm.RejectAll = true
				cm.MinOccurs = mg.MinOccurs
			} else {
				cm.Empty = true
			}
		}
	} else if cc.Restriction != nil && cc.Restriction.Particle != nil {
		cm.Kind = c.getGroupKind(cc.Restriction.Particle)
		cm.Particles = c.flattenParticles(cc.Restriction.Particle)
		// if flattening produces no particles (e.g., empty group), mark as empty
		if len(cm.Particles) == 0 {
			if mg, ok := cc.Restriction.Particle.(*types.ModelGroup); ok && mg.Kind == types.Choice && len(mg.Particles) == 0 {
				cm.RejectAll = true
				cm.MinOccurs = mg.MinOccurs
			} else {
				cm.Empty = true
			}
		}
	} else {
		cm.Empty = true
	}

	return cm
}

func (c *Compiler) buildAutomaton(ct *grammar.CompiledType) error {
	if ct.ContentModel == nil || ct.ContentModel.Empty || ct.ContentModel.RejectAll {
		return nil
	}

	// for all groups, use simple array-based validation instead of DFA
	// this correctly handles missing required elements, duplicates, and any order
	if ct.ContentModel.Kind == types.AllGroup {
		elements := c.buildAllGroupElements(ct.ContentModel.Particles)
		if len(elements) == 0 {
			ct.ContentModel.Empty = true
			ct.ContentModel.AllElements = nil
			return nil
		}
		ct.ContentModel.AllElements = elements
		return nil
	}

	elementFormQualified := c.grammar.ElementFormDefault == xsdschema.Qualified
	automaton, err := grammar.BuildAutomaton(ct.ContentModel.Particles, c.grammar.TargetNamespace, elementFormQualified)
	if err != nil {
		return fmt.Errorf("type %s: automaton build failed: %w", ct.QName, err)
	}
	ct.ContentModel.Automaton = automaton

	return nil
}

func (c *Compiler) buildAllGroupElements(particles []*grammar.CompiledParticle) []*grammar.AllGroupElement {
	return c.collectAllGroupElements(particles)
}

func (c *Compiler) collectAllGroupElements(particles []*grammar.CompiledParticle) []*grammar.AllGroupElement {
	var elements []*grammar.AllGroupElement
	for _, p := range particles {
		switch p.Kind {
		case grammar.ParticleElement:
			// per XSD spec: maxOccurs="0" means the particle contributes no schema component
			// and is ignored during validation (Section 5.4 of validation-rules.md)
			if p.Element != nil && p.MaxOccurs != 0 {
				elements = append(elements, &grammar.AllGroupElement{
					Element:           p.Element,
					Optional:          p.MinOccurs == 0,
					AllowSubstitution: p.IsReference,
				})
			}
		case grammar.ParticleGroup:
			elements = append(elements, c.collectAllGroupElements(p.Children)...)
		}
	}
	return elements
}

func (c *Compiler) populateContentModelCaches(cm *grammar.CompiledContentModel) {
	if cm == nil || len(cm.Particles) == 0 {
		return
	}
	cm.ElementIndex = c.indexContentModelElements(cm.Particles)
	cm.SimpleSequence, cm.IsSimpleSequence = c.flattenSequenceParticles(cm.Particles, nil)
}

func (c *Compiler) indexContentModelElements(particles []*grammar.CompiledParticle) map[types.QName]*grammar.CompiledElement {
	var decls map[types.QName]*grammar.CompiledElement
	var walk func(items []*grammar.CompiledParticle)
	walk = func(items []*grammar.CompiledParticle) {
		for _, particle := range items {
			if particle == nil {
				continue
			}
			switch particle.Kind {
			case grammar.ParticleElement:
				if particle.Element == nil {
					continue
				}
				qname := particle.Element.EffectiveQName
				if qname.IsZero() {
					qname = particle.Element.QName
				}
				if decls == nil {
					decls = make(map[types.QName]*grammar.CompiledElement)
				}
				if existing, ok := decls[qname]; ok && existing != particle.Element {
					decls[qname] = nil
					continue
				}
				decls[qname] = particle.Element
			case grammar.ParticleGroup:
				walk(particle.Children)
			}
		}
	}
	walk(particles)
	return decls
}

func (c *Compiler) flattenSequenceParticles(particles []*grammar.CompiledParticle, out []*grammar.CompiledParticle) ([]*grammar.CompiledParticle, bool) {
	for _, particle := range particles {
		if particle == nil {
			return nil, false
		}
		switch particle.Kind {
		case grammar.ParticleElement:
			out = append(out, particle)
		case grammar.ParticleGroup:
			if particle.GroupKind != types.Sequence {
				return nil, false
			}
			var ok bool
			out, ok = c.flattenSequenceParticles(particle.Children, out)
			if !ok {
				return nil, false
			}
		case grammar.ParticleWildcard:
			return nil, false
		default:
			return nil, false
		}
	}
	return out, true
}
