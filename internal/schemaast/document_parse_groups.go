package schemaast

import "fmt"

func (p *documentParser) parseGroup(elem NodeID, topLevel bool) (*GroupDecl, error) {
	group := &GroupDecl{
		MinOccurs:       OccursFromInt(1),
		MaxOccurs:       OccursFromInt(1),
		SourceNamespace: p.result.TargetNamespace,
		Origin:          p.origin(elem),
	}
	if name := p.attr(elem, "name"); name != "" {
		if err := validateDocumentNCName("group", name); err != nil {
			return nil, err
		}
		group.Name = QName{Namespace: p.result.TargetNamespace, Local: name}
	}
	if topLevel {
		if err := validateElementAttributes(
			p.doc,
			elem,
			validAttributeNames[attrSetTopLevelGroup],
			"top-level group (only id, name allowed)",
		); err != nil {
			return nil, err
		}
	}
	if ref := p.attr(elem, "ref"); ref != "" {
		qname, err := p.resolveQName(elem, ref, true)
		if err != nil {
			return nil, fmt.Errorf("resolve group ref %s: %w", ref, err)
		}
		group.Ref = qname
	}
	if occ, ok, err := p.parseOccursAttr(elem, "minOccurs"); err != nil {
		return nil, err
	} else if ok {
		group.MinOccurs = occ
	}
	if occ, ok, err := p.parseOccursAttr(elem, "maxOccurs"); err != nil {
		return nil, err
	} else if ok {
		group.MaxOccurs = occ
	}
	if topLevel && group.Name.IsZero() {
		return nil, fmt.Errorf("group missing name attribute")
	}
	var seenAnnotation, seenNonAnnotation bool
	var seenModelGroup bool
	for _, child := range p.xsdChildren(elem) {
		childName := p.doc.LocalName(child)
		handled, err := handleSingleLeadingAnnotation(
			childName,
			&seenAnnotation,
			seenNonAnnotation,
			fmt.Sprintf("group '%s': at most one annotation is allowed", group.Name.Local),
			fmt.Sprintf("group '%s': annotation must appear before other elements", group.Name.Local),
		)
		if err != nil {
			return nil, err
		}
		if handled {
			continue
		}

		switch childName {
		case "sequence", "choice", "all":
			if seenModelGroup {
				return nil, fmt.Errorf("group '%s': exactly one model group (all, choice, or sequence) is allowed", group.Name.Local)
			}
			seenModelGroup = true
			seenNonAnnotation = true
			particle, err := p.parseParticle(child)
			if err != nil {
				return nil, err
			}
			if topLevel && (!particle.Min.IsOne() || !particle.Max.IsOne()) {
				return nil, fmt.Errorf("group '%s' must have minOccurs='1' and maxOccurs='1' (got minOccurs=%s, maxOccurs=%s)",
					group.Name.Local, particle.Min, particle.Max)
			}
			group.Particle = particle
		default:
			return nil, fmt.Errorf("group has unexpected child element '%s'", childName)
		}
	}
	if topLevel && group.Ref.IsZero() && !seenModelGroup {
		return nil, fmt.Errorf("group '%s' must contain exactly one model group (all, choice, or sequence)", group.Name.Local)
	}
	return group, nil
}

func (p *documentParser) parseAttributeGroup(elem NodeID, topLevel bool) (*AttributeGroupDecl, error) {
	group := &AttributeGroupDecl{
		SourceNamespace: p.result.TargetNamespace,
		Origin:          p.origin(elem),
	}
	if name := p.attr(elem, "name"); name != "" {
		if err := validateDocumentNCName("attributeGroup", name); err != nil {
			return nil, err
		}
		group.Name = QName{Namespace: p.result.TargetNamespace, Local: name}
	}
	if ref := p.attr(elem, "ref"); ref != "" {
		qname, err := p.resolveQName(elem, ref, true)
		if err != nil {
			return nil, fmt.Errorf("resolve attributeGroup ref %s: %w", ref, err)
		}
		group.Ref = qname
	}
	if topLevel && group.Name.IsZero() {
		return nil, fmt.Errorf("attributeGroup missing name attribute")
	}
	var seenAnnotation, seenNonAnnotation bool
	for _, child := range p.xsdChildren(elem) {
		childName := p.doc.LocalName(child)
		handled, err := handleSingleLeadingAnnotation(
			childName,
			&seenAnnotation,
			seenNonAnnotation,
			fmt.Sprintf("attributeGroup '%s': at most one annotation is allowed", group.Name.Local),
			fmt.Sprintf("attributeGroup '%s': annotation must appear before other elements", group.Name.Local),
		)
		if err != nil {
			return nil, err
		}
		if handled {
			continue
		}
		if !group.Ref.IsZero() {
			return nil, fmt.Errorf("attributeGroup: unexpected child element '%s'", childName)
		}

		switch childName {
		case "attribute":
			seenNonAnnotation = true
			attr, err := p.parseAttribute(child, false)
			if err != nil {
				return nil, err
			}
			group.Attributes = append(group.Attributes, AttributeUseDecl{Attribute: attr})
		case "attributeGroup":
			seenNonAnnotation = true
			ref, err := p.parseAttributeGroup(child, false)
			if err != nil {
				return nil, err
			}
			group.AttributeGroups = append(group.AttributeGroups, ref.Ref)
		case "anyAttribute":
			seenNonAnnotation = true
			any, err := p.parseWildcard(child, false)
			if err != nil {
				return nil, err
			}
			group.AnyAttribute = any
		default:
			return nil, fmt.Errorf("attributeGroup has unexpected child element '%s'", childName)
		}
	}
	return group, nil
}

func (p *documentParser) parseParticle(elem NodeID) (*ParticleDecl, error) {
	particle := &ParticleDecl{
		Min: OccursFromInt(1),
		Max: OccursFromInt(1),
	}
	switch p.doc.LocalName(elem) {
	case "sequence":
		particle.Kind = ParticleSequence
	case "choice":
		particle.Kind = ParticleChoice
	case "all":
		particle.Kind = ParticleAll
	case "element":
		element, err := p.parseElement(elem, false)
		if err != nil {
			return nil, err
		}
		return &ParticleDecl{
			Kind:    ParticleElement,
			Element: element,
			Min:     element.MinOccurs,
			Max:     element.MaxOccurs,
		}, nil
	case "group":
		group, err := p.parseGroup(elem, false)
		if err != nil {
			return nil, err
		}
		return &ParticleDecl{
			Kind:     ParticleGroup,
			GroupRef: group.Ref,
			Min:      group.MinOccurs,
			Max:      group.MaxOccurs,
		}, nil
	case "any":
		any, err := p.parseWildcard(elem, true)
		if err != nil {
			return nil, err
		}
		return &ParticleDecl{
			Kind:     ParticleWildcard,
			Wildcard: any,
			Min:      any.MinOccurs,
			Max:      any.MaxOccurs,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported particle '%s'", p.doc.LocalName(elem))
	}
	if err := validateElementAttributes(
		p.doc,
		elem,
		validAttributeNames[attrSetModelGroup],
		fmt.Sprintf("<%s> (only id, minOccurs, maxOccurs allowed)", p.doc.LocalName(elem)),
	); err != nil {
		return nil, err
	}
	if occ, ok, err := p.parseOccursAttr(elem, "minOccurs"); err != nil {
		return nil, err
	} else if ok {
		particle.Min = occ
	}
	if occ, ok, err := p.parseOccursAttr(elem, "maxOccurs"); err != nil {
		return nil, err
	} else if ok {
		particle.Max = occ
	}
	var seenAnnotation, seenNonAnnotation bool
	groupName := p.doc.LocalName(elem)
	for _, child := range p.xsdChildren(elem) {
		childName := p.doc.LocalName(child)
		handled, err := handleSingleLeadingAnnotation(
			childName,
			&seenAnnotation,
			seenNonAnnotation,
			fmt.Sprintf("%s: at most one annotation is allowed", groupName),
			fmt.Sprintf("%s: annotation must appear before other elements", groupName),
		)
		if err != nil {
			return nil, err
		}
		if handled {
			continue
		}
		seenNonAnnotation = true
		if particle.Kind == ParticleAll && childName != "element" {
			return nil, fmt.Errorf("xs:all cannot contain %s references (only element declarations are allowed)", childName)
		}
		if childName == "all" {
			return nil, fmt.Errorf("xs:all cannot be nested inside %s", groupName)
		}
		switch childName {
		case "element", "group", "choice", "sequence", "all", "any":
			parsed, err := p.parseParticle(child)
			if err != nil {
				return nil, err
			}
			particle.Children = append(particle.Children, *parsed)
		default:
			return nil, fmt.Errorf("%s has unexpected child element '%s'", groupName, childName)
		}
	}
	return particle, nil
}

func (p *documentParser) parseWildcard(elem NodeID, withOccurs bool) (*WildcardDecl, error) {
	allowed := validAttributeNames[attrSetAnyAttribute]
	if withOccurs {
		allowed = validAttributeNames[attrSetAnyElement]
	}
	if err := validateAllowedAttributes(p.doc, elem, p.doc.LocalName(elem), allowed); err != nil {
		return nil, err
	}
	namespaceAttr := p.attr(elem, "namespace")
	if !p.hasAttr(elem, "namespace") {
		namespaceAttr = "##any"
	} else if namespaceAttr == "" {
		namespaceAttr = "##local"
	}
	nsConstraint, nsList, err := parseNamespaceConstraint(namespaceAttr)
	if err != nil {
		return nil, fmt.Errorf("parse namespace constraint: %w", err)
	}
	any := &WildcardDecl{
		TargetNamespace: p.result.TargetNamespace,
		Namespace:       nsConstraint,
		NamespaceList:   nsList,
		ProcessContents: Strict,
		MinOccurs:       OccursFromInt(1),
		MaxOccurs:       OccursFromInt(1),
	}
	if p.hasAttr(elem, "processContents") && TrimXMLWhitespace(p.rawAttr(elem, "processContents")) == "" {
		return nil, fmt.Errorf("processContents attribute cannot be empty")
	}
	switch pc := p.attr(elem, "processContents"); pc {
	case "", "strict":
		any.ProcessContents = Strict
	case "lax":
		any.ProcessContents = Lax
	case "skip":
		any.ProcessContents = Skip
	default:
		return nil, fmt.Errorf("invalid processContents value '%s': must be 'strict', 'lax', or 'skip'", pc)
	}
	if withOccurs {
		if occ, ok, err := p.parseOccursAttr(elem, "minOccurs"); err != nil {
			return nil, err
		} else if ok {
			any.MinOccurs = occ
		}
		if occ, ok, err := p.parseOccursAttr(elem, "maxOccurs"); err != nil {
			return nil, err
		} else if ok {
			any.MaxOccurs = occ
		}
	}
	var seenAnnotation, seenNonAnnotation bool
	wildcardName := p.doc.LocalName(elem)
	for _, child := range p.xsdChildren(elem) {
		childName := p.doc.LocalName(child)
		handled, err := handleSingleLeadingAnnotation(
			childName,
			&seenAnnotation,
			seenNonAnnotation,
			fmt.Sprintf("%s: at most one annotation is allowed", wildcardName),
			fmt.Sprintf("%s: annotation must appear before other elements", wildcardName),
		)
		if err != nil {
			return nil, err
		}
		if handled {
			continue
		}
		seenNonAnnotation = true
		return nil, fmt.Errorf("%s has unexpected child element '%s'", wildcardName, childName)
	}
	return any, nil
}
