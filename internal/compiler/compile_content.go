package compiler

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func (c *Compiler) compileContentModel(complexType *types.ComplexType) (*grammar.CompiledContentModel, error) {
	content := complexType.Content()
	if content == nil {
		return &grammar.CompiledContentModel{Empty: true}, nil
	}

	switch cnt := content.(type) {
	case *types.EmptyContent:
		return &grammar.CompiledContentModel{Empty: true}, nil

	case *types.ElementContent:
		if cnt.Particle == nil {
			return &grammar.CompiledContentModel{Empty: true}, nil
		}
		minOccurs := types.OccursFromInt(1)
		var mg *types.ModelGroup
		if typedMG, ok := cnt.Particle.(*types.ModelGroup); ok {
			mg = typedMG
			minOccurs = mg.MinOccurs
		}
		particles, err := c.flattenParticles(cnt.Particle)
		if err != nil {
			return nil, err
		}
		// if flattening produces no particles (e.g., empty group), mark as empty
		if len(particles) == 0 {
			if mg != nil && mg.Kind == types.Choice && len(mg.Particles) == 0 {
				return &grammar.CompiledContentModel{
					Kind:      mg.Kind,
					RejectAll: true,
					Mixed:     complexType.EffectiveMixed(),
					MinOccurs: minOccurs,
				}, nil
			}
			return &grammar.CompiledContentModel{Empty: true, Mixed: complexType.EffectiveMixed()}, nil
		}
		return &grammar.CompiledContentModel{
			Kind:      c.getGroupKind(cnt.Particle),
			Particles: particles,
			Mixed:     complexType.EffectiveMixed(),
			MinOccurs: minOccurs,
		}, nil

	case *types.ComplexContent:
		return c.compileComplexContent(cnt, complexType.EffectiveMixed())

	case *types.SimpleContent:
		// simple content - no element content model
		return &grammar.CompiledContentModel{Empty: true}, nil
	}

	return &grammar.CompiledContentModel{Empty: true}, nil
}

func (c *Compiler) flattenParticles(particle types.Particle) ([]*grammar.CompiledParticle, error) {
	// inline ModelGroups are tree-structured (no pointer cycles) from parser
	expandedGroups := make(map[types.QName]bool)
	return c.flattenParticle(particle, expandedGroups)
}

func (c *Compiler) flattenParticle(particle types.Particle, expandedGroups map[types.QName]bool) ([]*grammar.CompiledParticle, error) {
	switch p := particle.(type) {
	case *types.ModelGroup:
		var children []*grammar.CompiledParticle
		for _, child := range p.Particles {
			flattened, err := c.flattenParticle(child, expandedGroups)
			if err != nil {
				return nil, err
			}
			children = append(children, flattened...)
		}
		if len(children) > 0 {
			return []*grammar.CompiledParticle{{
				Kind:      grammar.ParticleGroup,
				MinOccurs: p.MinOccurs,
				MaxOccurs: p.MaxOccurs,
				GroupKind: p.Kind,
				Children:  children,
			}}, nil
		}
		return nil, nil

	case *types.ElementDecl:
		compiled := &grammar.CompiledParticle{
			Kind:        grammar.ParticleElement,
			MinOccurs:   p.MinOccurs,
			MaxOccurs:   p.MaxOccurs,
			IsReference: p.IsReference,
		}
		if c.schema != nil {
			if capMax, ok := c.schema.ParticleRestrictionCaps[p]; ok {
				if compiled.MaxOccurs.IsUnbounded() || compiled.MaxOccurs.Cmp(capMax) > 0 {
					if compiled.MinOccurs.Cmp(capMax) <= 0 {
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
				compiledElem, err := c.compileElement(p.Name, elemDecl, elementScopeTopLevel)
				if err != nil {
					return nil, fmt.Errorf("compile element %s: %w", p.Name, err)
				}
				compiled.Element = compiledElem
			}
		} else {
			// local element - compile directly from the particle's ElementDecl.
			compiledElem, err := c.compileElement(p.Name, p, elementScopeLocal)
			if err != nil {
				return nil, fmt.Errorf("compile element %s: %w", p.Name, err)
			}
			compiled.Element = compiledElem
		}
		return []*grammar.CompiledParticle{compiled}, nil

	case *types.GroupRef:
		// cycle detection via QName - check if already expanding this group
		if expandedGroups[p.RefQName] {
			// circular group reference detected - return empty result
			return nil, nil
		}
		expandedGroups[p.RefQName] = true
		defer func() { delete(expandedGroups, p.RefQName) }()

		// expand group reference
		if group, ok := c.schema.Groups[p.RefQName]; ok {
			if p.MaxOccurs.IsZero() {
				return nil, nil
			}
			var children []*grammar.CompiledParticle
			for _, child := range group.Particles {
				flattened, err := c.flattenParticle(child, expandedGroups)
				if err != nil {
					return nil, err
				}
				children = append(children, flattened...)
			}
			if len(children) == 0 {
				return nil, nil
			}
			return []*grammar.CompiledParticle{{
				Kind:      grammar.ParticleGroup,
				MinOccurs: p.MinOccurs,
				MaxOccurs: p.MaxOccurs,
				GroupKind: group.Kind,
				Children:  children,
			}}, nil
		}
		return nil, nil

	case *types.AnyElement:
		return []*grammar.CompiledParticle{{
			Kind:      grammar.ParticleWildcard,
			MinOccurs: p.MinOccurs,
			MaxOccurs: p.MaxOccurs,
			Wildcard:  p,
		}}, nil
	}

	return nil, nil
}

func (c *Compiler) compileComplexContent(cc *types.ComplexContent, mixed bool) (*grammar.CompiledContentModel, error) {
	cm := &grammar.CompiledContentModel{
		Mixed: mixed,
	}

	particle := c.contentParticle(cc)
	if particle == nil {
		cm.Empty = true
		return cm, nil
	}

	cm.Kind = c.getGroupKind(particle)
	particles, err := c.flattenParticles(particle)
	if err != nil {
		return nil, err
	}
	cm.Particles = particles
	if len(cm.Particles) > 0 {
		return cm, nil
	}
	if mg, ok := particle.(*types.ModelGroup); ok && mg.Kind == types.Choice && len(mg.Particles) == 0 {
		cm.RejectAll = true
		cm.MinOccurs = mg.MinOccurs
		return cm, nil
	}
	cm.Empty = true

	return cm, nil
}

func (c *Compiler) contentParticle(cc *types.ComplexContent) types.Particle {
	if cc.Extension != nil && cc.Extension.Particle != nil {
		return cc.Extension.Particle
	}
	if cc.Restriction != nil && cc.Restriction.Particle != nil {
		return cc.Restriction.Particle
	}
	return nil
}

func (c *Compiler) buildAutomaton(compiledType *grammar.CompiledType) error {
	if compiledType.ContentModel == nil || compiledType.ContentModel.Empty || compiledType.ContentModel.RejectAll {
		return nil
	}

	// for all groups, use simple array-based validation instead of DFA
	// this correctly handles missing required elements, duplicates, and any order
	if compiledType.ContentModel.Kind == types.AllGroup {
		elements := c.collectAllGroupElements(compiledType.ContentModel.Particles)
		if len(elements) == 0 {
			compiledType.ContentModel.Empty = true
			compiledType.ContentModel.AllElements = nil
			return nil
		}
		compiledType.ContentModel.AllElements = elements
		return nil
	}

	elementFormDefault := types.FormUnqualified
	if c.grammar.ElementFormDefault == parser.Qualified {
		elementFormDefault = types.FormQualified
	}
	automaton, err := grammar.BuildAutomaton(compiledType.ContentModel.Particles, c.grammar.TargetNamespace, elementFormDefault)
	if err != nil {
		return fmt.Errorf("type %s: automaton build failed: %w", compiledType.QName, err)
	}
	compiledType.ContentModel.Automaton = automaton

	return nil
}

func (c *Compiler) collectAllGroupElements(particles []*grammar.CompiledParticle) []*grammar.AllGroupElement {
	var elements []*grammar.AllGroupElement
	for _, p := range particles {
		switch p.Kind {
		case grammar.ParticleElement:
			// per XSD spec: maxOccurs="0" means the particle contributes no schema component
			// and is ignored during validation (Section 5.4 of validation-rules.md)
			if p.Element != nil && !p.MaxOccurs.IsZero() {
				elements = append(elements, &grammar.AllGroupElement{
					Element:           p.Element,
					Optional:          p.MinOccurs.IsZero(),
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
	decls := make(map[types.QName]*grammar.CompiledElement)
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
	if len(decls) == 0 {
		return nil
	}
	return decls
}

func (c *Compiler) flattenSequenceParticles(particles, out []*grammar.CompiledParticle) ([]*grammar.CompiledParticle, bool) {
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
