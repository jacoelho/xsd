package schema

import "github.com/jacoelho/xsd/xsderrors"

type restrictionNormalizer struct {
	validator contentRestrictionValidator
	models    restrictionModelView
}

// compareNormalizedRestriction compares over a view that borrows unchanged
// original models and owns only replacement nodes. When normalization added
// replacements, the comparison gets a private range/emptiability analysis and
// wildcard cache so replacement IDs never reach the caller's reusable caches;
// work still charges the caller's sticky budget.
func (v contentRestrictionValidator) compareNormalizedRestriction(base, derived contentParticle) error {
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
		analysis, err := newContentModelAnalysis(v.rt, v.spend)
		if err != nil {
			return err
		}
		analysis.models = &restrictionModelView{original: n.models.original, added: n.models.added}
		comparison.analysis = analysis
		comparison.wildcards = make(map[contentModelID]bool)
	}
	if derived.Kind == particleModel && n.models.ignorable(derived.Model) {
		return comparison.validatePointlessRestriction(base)
	}
	return comparison.compareParticleRestriction(base, derived)
}

// validatePointlessRestriction decides a derived {content type} particle that
// clause 2.2.1 of Particle Valid (Restriction) finds pointless: it leaves no
// particle to compare, and restricts only an emptiable base particle, as
// accepted W3C test mgE014 expects.
func (v contentRestrictionValidator) validatePointlessRestriction(base contentParticle) error {
	emptiable, err := v.analysis.ParticleEmptiable(base)
	if err != nil {
		return err
	}
	if !emptiable {
		return contentRestrictionMismatch("pointless restriction of base that is not emptiable")
	}
	return nil
}

func (n *restrictionNormalizer) particle(p contentParticle, depth int) (contentParticle, error) {
	if err := n.validator.charge(); err != nil {
		return contentParticle{}, err
	}
	if depth > maxComponentDependencyDepth {
		return contentParticle{}, xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, "content restriction normalization exceeds depth limit")
	}
	switch p.Kind {
	case particleElement:
		return n.element(p)
	case particleModel:
		return n.model(p, depth)
	case particleWildcard:
		return p, nil
	default:
		return contentParticle{}, contentRestrictionInvariant("content restriction references invalid particle kind")
	}
}

func (n *restrictionNormalizer) element(p contentParticle) (contentParticle, error) {
	v := n.validator
	if !v.rt.Substitutions.HasMembers(p.Element) {
		return p, nil
	}
	decl := v.rt.Elements[p.Element]
	if decl.Scope != declarationScopeGlobal {
		return p, nil
	}
	var members []contentParticle
	var err error
	v.rt.Substitutions.ForEachMember(p.Element, func(id ElementID) bool {
		err = n.appendActualMember(&members, p.Element, id)
		return err == nil
	})
	if err = v.finish(err); err != nil {
		return contentParticle{}, err
	}
	if len(members) == 0 {
		return p, nil
	}
	members, err = n.insertConcreteHead(members, p.Element)
	if err != nil {
		return contentParticle{}, err
	}
	// These are literal members of the actual group, not new head references.
	// Expanding them again could regenerate their enclosing choice forever.
	return n.substitutionChoice(p.Occurs, members)
}

func (n *restrictionNormalizer) appendActualMember(members *[]contentParticle, head, id ElementID) error {
	v := n.validator
	if err := v.charge(); err != nil {
		return err
	}
	if !validElementID(id, len(v.rt.Elements)) {
		return contentRestrictionInvariant("substitution group references missing element")
	}
	member, allowed := v.SubstitutionMemberByName(head, v.rt.Elements[id].Name)
	if !allowed || member != id {
		return v.finish(nil)
	}

	return n.appendLiteralMember(members, id)
}

// Generated groups are in expanded-name order (substitutionTable.ForEachMember),
// including the concrete head, so the work spent before the first failing
// member does not depend on the order names were interned. Comparison spans waive only generated order; explicit source
// choice boundaries remain ordered.
func (n *restrictionNormalizer) insertConcreteHead(members []contentParticle, head ElementID) ([]contentParticle, error) {
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
	members[position] = elementParticle(head, occurrence{Min: 1, Max: 1})
	return members, nil
}

func (n *restrictionNormalizer) headInsertionIndex(members []contentParticle, head ElementID) (int, error) {
	elements := n.validator.rt.Elements
	for i, member := range members {
		if err := n.validator.charge(); err != nil {
			return 0, err
		}
		if compareQNameText(&n.validator.rt.Names, elements[head].Name, elements[member.Element].Name) < 0 {
			return i, nil
		}
	}
	return len(members), nil
}

func (n *restrictionNormalizer) appendLiteralMember(members *[]contentParticle, id ElementID) error {
	if err := n.validator.charge(); err != nil {
		return err
	}
	*members = append(*members, elementParticle(id, occurrence{Min: 1, Max: 1}))
	return nil
}

func (n *restrictionNormalizer) model(p contentParticle, depth int) (contentParticle, error) {
	m, ok := n.models.model(p.Model)
	if !ok {
		return contentParticle{}, contentRestrictionInvariant("content restriction references missing content model")
	}
	if m.Kind == modelEmpty {
		return p, nil
	}
	// A root enters with a unit particle and carries its occurrence on its
	// record; every other model a particle names has a unit record.
	occurs := p.Occurs
	switch {
	case occurs.IsExactlyOne():
		occurs = m.Occurs
	case !m.Occurs.IsExactlyOne():
		return contentParticle{}, contentRestrictionInvariant("model particle names a model whose record repeats it")
	}
	if pointlessSingleton(m, occurs) {
		return n.particle(m.Particles[0], depth+1)
	}
	return n.modelBody(p, m, occurs, depth)
}

func pointlessSingleton(m contentModelNode, occurs occurrence) bool {
	return len(m.Particles) == 1 && occurs.IsExactlyOne() && (m.Kind == modelSequence || m.Kind == modelChoice || m.Kind == modelAll)
}

func (n *restrictionNormalizer) modelBody(original contentParticle, m contentModelNode, occurs occurrence, depth int) (contentParticle, error) {
	children, err := n.children(m, depth)
	if err != nil {
		return contentParticle{}, err
	}

	if !children.changed && len(children.particles) != 0 {
		// Borrow both original occurrence layers; moving model bounds onto the
		// particle as well would count them twice during comparison.
		return original, nil
	}
	return n.declaredGroup(m, occurs, children)
}

// childDisposition is what clause 2.2 does with a normalized child in its
// parent group.
type childDisposition uint8

const (
	childKept childDisposition = iota
	// childIgnored drops an empty group.
	childIgnored
	// childSpliced replaces a 1..1 group of the parent's compositor by its
	// members.
	childSpliced
)

type normalizedRestrictionChild struct {
	members     []contentParticle
	particle    contentParticle
	disposition childDisposition
}

func (c normalizedRestrictionChild) unchanged(original contentParticle) bool {
	return c.particle == original && c.disposition == childKept
}

type normalizedRestrictionChildren struct {
	particles []contentParticle
	order     restrictionChoiceOrder
	changed   bool
}

func (n *restrictionNormalizer) children(m contentModelNode, depth int) (normalizedRestrictionChildren, error) {
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

func (n *restrictionNormalizer) replaceChild(result *normalizedRestrictionChildren, parent contentModelNode, index int, child normalizedRestrictionChild) error {
	var err error
	if !result.changed {
		result.particles, err = n.copyPrefix(parent.Particles, index)
		if err != nil {
			return err
		}
		result.changed = true
	}
	if parent.Kind == modelChoice && child.disposition == childSpliced {
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
		return n.appendChildChoiceSpan(result, restrictionChoiceSpan{end: len(child.members)})
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

func (n *restrictionNormalizer) copyPrefix(original []contentParticle, end int) ([]contentParticle, error) {
	if err := n.validator.spend(len(original)); err != nil {
		return nil, err
	}
	particles := make([]contentParticle, end, len(original))
	copy(particles, original[:end])
	return particles, nil
}

func (n *restrictionNormalizer) child(original contentParticle, parent modelKind, depth int) (normalizedRestrictionChild, error) {
	p, err := n.particle(original, depth+1)
	child := normalizedRestrictionChild{particle: p}
	if err != nil || p.Kind != particleModel {
		return child, err
	}
	body, ok := n.models.model(p.Model)
	if !ok {
		return normalizedRestrictionChild{}, contentRestrictionInvariant("normalized content model is missing")
	}
	if body.Kind == modelEmpty || n.models.ignorable(p.Model) {
		child.disposition = childIgnored
		return child, nil
	}
	if p.Occurs.IsExactlyOne() && body.Occurs.IsExactlyOne() && body.Kind == parent && (parent == modelSequence || parent == modelChoice) {
		child.members = body.Particles
		child.disposition = childSpliced
	}
	return child, nil
}

func (n *restrictionNormalizer) appendChild(particles []contentParticle, child normalizedRestrictionChild) ([]contentParticle, error) {
	switch child.disposition {
	case childIgnored:
		return particles, nil
	case childSpliced:
		if err := n.validator.spend(len(child.members)); err != nil {
			return nil, err
		}
		return append(particles, child.members...), nil
	case childKept:
	}
	if err := n.validator.charge(); err != nil {
		return nil, err
	}
	return append(particles, child.particle), nil
}

// declaredGroup is the normalized form of the declared group m once its
// pointless children are ignored (Particle Valid (Restriction) clause 2.2). m
// was not a pointless singleton as declared, so it stays a group of its
// declared kind whatever the number of children left, as
// docs/spec/INTERPRETATIONS.md records. A group left with no children that
// emptyGroup finds pointless is marked ignorable: its parent ignores it, and
// validatePointlessRestriction decides the derived particle of a
// {content type}, which has no parent.
func (n *restrictionNormalizer) declaredGroup(m contentModelNode, occurs occurrence, children normalizedRestrictionChildren) (contentParticle, error) {
	term, err := n.validator.analysis.modelTermCountRange(m)
	if err != nil {
		return contentParticle{}, err
	}
	return n.addGroup(restrictionModel{
		model:     contentModelNode{Kind: m.Kind, Occurs: occurrence{Min: 1, Max: 1}, Particles: children.particles},
		order:     children.order,
		term:      term,
		ignorable: len(children.particles) == 0 && emptyGroup(m, occurs, term),
	}, occurs)
}

// emptyGroup reports whether the declared group m, left with no children, is
// ignored in its parent (Particle Valid (Restriction) clause 2.2.1), as
// docs/spec/INTERPRETATIONS.md records for groups declared empty and for
// required empty choices.
func emptyGroup(m contentModelNode, occurs occurrence, term countRange) bool {
	return term.repeated(occurs).emptiable() && (len(m.Particles) == 0 || occurs.IsExactlyOne())
}

// substitutionChoice is the choice clause 2.1 generates for a head with an
// actual substitution group. It is not declared, so clause 2.2 finds it
// pointless when it has one member (I-027 in docs/spec/INTERPRETATIONS.md).
func (n *restrictionNormalizer) substitutionChoice(occurs occurrence, members []contentParticle) (contentParticle, error) {
	if len(members) == 1 && occurs.IsExactlyOne() {
		return members[0], nil
	}
	body := contentModelNode{Kind: modelChoice, Occurs: occurrence{Min: 1, Max: 1}, Particles: members}
	term, err := n.validator.analysis.modelTermCountRange(body)
	if err != nil {
		return contentParticle{}, err
	}
	return n.addGroup(restrictionModel{model: body, order: restrictionChoiceOrder{unordered: true}, term: term}, occurs)
}

func (n *restrictionNormalizer) addGroup(model restrictionModel, occurs occurrence) (contentParticle, error) {
	if err := n.validator.charge(); err != nil {
		return contentParticle{}, err
	}
	id, err := nextContentModelID(len(n.models.original) + len(n.models.added))
	if err != nil {
		return contentParticle{}, err
	}
	n.models.added = append(n.models.added, model)
	return modelParticle(id, occurs), nil
}
