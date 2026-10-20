package schema

import "github.com/jacoelho/xsd/xsderrors"

type restrictionNormalizer struct {
	validator contentRestrictionValidator
	models    restrictionModelView
}

func (v contentRestrictionValidator) compareNormalizedRestriction(base, derived Particle) error {
	n := restrictionNormalizer{validator: v, models: restrictionModelView{original: v.rt.Models}}
	base, err := n.particle(base, 0)
	if err != nil {
		return err
	}
	derived, err = n.particle(derived, 0)
	if err != nil {
		return err
	}
	comparison := v
	if len(n.models.added) != 0 {
		analysis, err := NewContentModelAnalysis(v.rt, v.spend)
		if err != nil {
			return err
		}
		analysis.models = &restrictionModelView{original: n.models.original, added: n.models.added}
		comparison.analysis = analysis
		comparison.wildcards = make(map[ContentModelID]bool)
	}
	return comparison.compareParticleRestriction(base, derived)
}

func (n *restrictionNormalizer) particle(p Particle, depth int) (Particle, error) {
	if err := n.validator.charge(); err != nil {
		return Particle{}, err
	}
	if depth > maxComponentDependencyDepth {
		return Particle{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content restriction normalization exceeds depth limit")
	}
	switch p.Kind {
	case ParticleElement:
		return n.element(p)
	case ParticleModel:
		return n.model(p, depth)
	case ParticleWildcard:
		return p, nil
	default:
		return Particle{}, contentRestrictionInvariant("content restriction references invalid particle kind")
	}
}

func (n *restrictionNormalizer) element(p Particle) (Particle, error) {
	v := n.validator
	if !v.rt.Substitutions.HasMembers(p.Element) {
		return p, nil
	}
	decl := v.rt.Elements[p.Element]
	if decl.Scope != DeclarationScopeGlobal {
		return p, nil
	}
	var members []Particle
	var err error
	v.rt.Substitutions.ForEachMember(p.Element, func(id ElementID) bool {
		err = n.appendActualMember(&members, p.Element, id)
		return err == nil
	})
	if err = v.finish(err); err != nil {
		return Particle{}, err
	}
	if len(members) == 0 {
		return p, nil
	}
	members, err = n.insertConcreteHead(members, p.Element)
	if err != nil {
		return Particle{}, err
	}
	// These are literal members of the actual group, not new head references.
	// Expanding them again could regenerate their enclosing choice forever.
	return n.group(ModelChoice, p.Occurs, members, restrictionChoiceOrder{unordered: true})
}

func (n *restrictionNormalizer) appendActualMember(members *[]Particle, head, id ElementID) error {
	v := n.validator
	if err := v.charge(); err != nil {
		return err
	}
	if !ValidElementID(id, len(v.rt.Elements)) {
		return contentRestrictionInvariant("substitution group references missing element")
	}
	member, allowed := v.SubstitutionMemberByName(head, v.rt.Elements[id].Name)
	if !allowed || member != id {
		return v.finish(nil)
	}

	return n.appendLiteralMember(members, id)
}

// Generated groups retain deterministic QName storage order, including the
// concrete head. Comparison spans waive only generated order; explicit source
// choice boundaries remain ordered.
func (n *restrictionNormalizer) insertConcreteHead(members []Particle, head ElementID) ([]Particle, error) {
	if n.validator.rt.Elements[head].Abstract {
		return members, nil
	}
	position, err := n.headInsertionIndex(members, head)
	if err != nil {
		return nil, err
	}
	if err := n.appendLiteralMember(&members, head); err != nil {
		return nil, err
	}
	if err := n.validator.spend(len(members) - 1 - position); err != nil {
		return nil, err
	}
	copy(members[position+1:], members[position:len(members)-1])
	members[position] = ElementParticle(head, Occurrence{Min: 1, Max: 1})
	return members, nil
}

func (n *restrictionNormalizer) headInsertionIndex(members []Particle, head ElementID) (int, error) {
	elements := n.validator.rt.Elements
	for i, member := range members {
		if err := n.validator.charge(); err != nil {
			return 0, err
		}
		if compareQName(elements[head].Name, elements[member.Element].Name) < 0 {
			return i, nil
		}
	}
	return len(members), nil
}

func (n *restrictionNormalizer) appendLiteralMember(members *[]Particle, id ElementID) error {
	if err := n.validator.charge(); err != nil {
		return err
	}
	*members = append(*members, ElementParticle(id, Occurrence{Min: 1, Max: 1}))
	return nil
}

func (n *restrictionNormalizer) model(p Particle, depth int) (Particle, error) {
	m, ok := n.models.model(p.Model)
	if !ok {
		return Particle{}, contentRestrictionInvariant("content restriction references missing content model")
	}
	if m.Kind == ModelEmpty || m.Kind == ModelAny {
		return p, nil
	}
	occurs := p.Occurs
	if occurs.IsExactlyOne() {
		occurs = m.Occurs
	} else if !m.Occurs.IsExactlyOne() {
		// Source lowering normally moves nested bounds to the particle. Retain
		// two nonunit layers rather than multiplying away structural guards.
		inner, err := n.model(ModelParticle(p.Model, Occurrence{Min: 1, Max: 1}), depth+1)
		if err != nil {
			return Particle{}, err
		}
		if err := n.validator.charge(); err != nil {
			return Particle{}, err
		}
		return n.group(ModelSequence, p.Occurs, []Particle{inner}, restrictionChoiceOrder{})
	}
	if pointlessSingleton(m, occurs) {
		return n.particle(m.Particles[0], depth+1)
	}
	return n.modelBody(p, m, occurs, depth)
}

func pointlessSingleton(m ContentModel, occurs Occurrence) bool {
	return len(m.Particles) == 1 && occurs.IsExactlyOne() && (m.Kind == ModelSequence || m.Kind == ModelChoice || m.Kind == ModelAll)
}

func (n *restrictionNormalizer) modelBody(original Particle, m ContentModel, occurs Occurrence, depth int) (Particle, error) {
	children, err := n.children(m, depth)
	if err != nil {
		return Particle{}, err
	}

	if !children.changed && len(children.particles) != 0 && (m.Kind != ModelAll || len(children.particles) != 1) {
		// Borrow both original occurrence layers; moving model bounds onto the
		// particle as well would count them twice during comparison.
		return original, nil
	}
	return n.group(m.Kind, occurs, children.particles, children.order)
}

type normalizedRestrictionChild struct {
	flattened []Particle
	particle  Particle
	omitted   bool
}

func (c normalizedRestrictionChild) unchanged(original Particle) bool {
	return c.particle == original && c.flattened == nil && !c.omitted
}

type normalizedRestrictionChildren struct {
	particles []Particle
	order     restrictionChoiceOrder
	changed   bool
}

func (n *restrictionNormalizer) children(m ContentModel, depth int) (normalizedRestrictionChildren, error) {
	result := normalizedRestrictionChildren{particles: m.Particles}
	for i, original := range m.Particles {
		child, err := n.child(original, m.Kind, depth)
		if err != nil {
			return normalizedRestrictionChildren{}, err
		}
		if !result.changed && child.unchanged(original) {
			continue
		}
		if err := n.replaceChild(&result, m, i, child); err != nil {
			return normalizedRestrictionChildren{}, err
		}
	}
	return result, nil
}

func (n *restrictionNormalizer) replaceChild(result *normalizedRestrictionChildren, parent ContentModel, index int, child normalizedRestrictionChild) error {
	var err error
	if !result.changed {
		result.particles, err = n.copyPrefix(parent.Particles, index)
		if err != nil {
			return err
		}
		result.changed = true
	}
	if parent.Kind == ModelChoice && child.flattened != nil {
		if err = n.appendChildChoiceSpans(result, child); err != nil {
			return err
		}
	}
	result.particles, err = n.appendChild(result.particles, child)
	return err
}

func (n *restrictionNormalizer) appendChildChoiceSpans(result *normalizedRestrictionChildren, child normalizedRestrictionChild) error {
	order := n.models.choiceOrder(child.particle.Model)
	if order.unordered {
		return n.appendChildChoiceSpan(result, restrictionChoiceSpan{end: len(child.flattened)})
	}
	for _, span := range order.spans {
		if err := n.appendChildChoiceSpan(result, span); err != nil {
			return err
		}
	}
	return nil
}

func (n *restrictionNormalizer) appendChildChoiceSpan(result *normalizedRestrictionChildren, span restrictionChoiceSpan) error {
	if err := n.validator.charge(); err != nil {
		return err
	}
	offset := len(result.particles)
	result.order.spans = append(result.order.spans, restrictionChoiceSpan{start: offset + span.start, end: offset + span.end})
	return nil
}

func (n *restrictionNormalizer) copyPrefix(original []Particle, end int) ([]Particle, error) {
	if err := n.validator.spend(len(original)); err != nil {
		return nil, err
	}
	particles := make([]Particle, end, len(original))
	copy(particles, original[:end])
	return particles, nil
}

func (n *restrictionNormalizer) child(original Particle, parent ModelKind, depth int) (normalizedRestrictionChild, error) {
	p, err := n.particle(original, depth+1)
	child := normalizedRestrictionChild{particle: p}
	if err != nil || p.Kind != ParticleModel {
		return child, err
	}
	body, ok := n.models.model(p.Model)
	if !ok {
		return normalizedRestrictionChild{}, contentRestrictionInvariant("normalized content model is missing")
	}
	if body.Kind == ModelEmpty {
		child.omitted = true
		return child, nil
	}
	if p.Occurs.IsExactlyOne() && body.Occurs.IsExactlyOne() && body.Kind == parent && (parent == ModelSequence || parent == ModelChoice) {
		child.flattened = body.Particles
	}
	return child, nil
}

func (n *restrictionNormalizer) appendChild(particles []Particle, child normalizedRestrictionChild) ([]Particle, error) {
	if child.omitted {
		return particles, nil
	}
	if child.flattened != nil {
		if err := n.validator.spend(len(child.flattened)); err != nil {
			return nil, err
		}
		return append(particles, child.flattened...), nil
	}
	if err := n.validator.charge(); err != nil {
		return nil, err
	}
	return append(particles, child.particle), nil
}

func (n *restrictionNormalizer) group(kind ModelKind, occurs Occurrence, particles []Particle, order restrictionChoiceOrder) (Particle, error) {
	if len(particles) == 1 {
		if occurs.IsExactlyOne() {
			return particles[0], nil
		}
		if kind == ModelAll {
			p := particles[0]
			p.Occurs = MultiplyOccurrence(p.Occurs, occurs)
			return p, nil
		}
	}
	body := ContentModel{Kind: kind, Occurs: Occurrence{Min: 1, Max: 1}, Particles: particles}
	if len(particles) == 0 && (kind != ModelChoice || occurs.Min == 0) {
		body = ContentModel{Kind: ModelEmpty}
		occurs = Occurrence{Min: 1, Max: 1}
	}
	if err := n.validator.charge(); err != nil {
		return Particle{}, err
	}
	id, err := NextContentModelID(len(n.models.original) + len(n.models.added))
	if err != nil {
		return Particle{}, err
	}
	n.models.added = append(n.models.added, restrictionModel{model: body, order: order})
	return ModelParticle(id, occurs), nil
}
