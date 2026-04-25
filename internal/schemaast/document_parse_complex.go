package schemaast

import "fmt"

func (p *documentParser) parseComplexType(elem NodeID, topLevel bool) (*ComplexTypeDecl, error) {
	var decl ComplexTypeDecl
	decl.SourceNamespace = p.result.TargetNamespace
	decl.Origin = p.origin(elem)
	if name := p.attr(elem, "name"); name != "" {
		if !topLevel {
			return nil, fmt.Errorf("inline complexType cannot have 'name' attribute")
		}
		if err := validateDocumentNCName("type", name); err != nil {
			return nil, err
		}
		decl.Name = QName{Namespace: p.result.TargetNamespace, Local: name}
	}
	if topLevel && decl.Name.IsZero() {
		return nil, fmt.Errorf("complexType missing name attribute")
	}
	abstract, err := p.parseBoolAttrDefault(elem, "abstract", false)
	if err != nil {
		return nil, err
	}
	decl.Abstract = abstract
	if p.hasAttr(elem, "mixed") {
		mixed, err := p.parseBoolAttrDefault(elem, "mixed", false)
		if err != nil {
			return nil, err
		}
		decl.Mixed = mixed
		decl.MixedSet = true
	}
	if p.hasAttr(elem, "final") {
		final := p.attr(elem, "final")
		if TrimXMLWhitespace(final) == "" {
			return nil, fmt.Errorf("final attribute cannot be empty")
		}
		set, err := parseDerivationSetWithValidation(final, DerivationSet(DerivationExtension|DerivationRestriction))
		if err != nil {
			return nil, fmt.Errorf("invalid complexType final attribute value '%s': %w", final, err)
		}
		decl.Final = set
	} else if p.result.Defaults.FinalDefault != 0 {
		decl.Final = p.result.Defaults.FinalDefault & DerivationSet(DerivationExtension|DerivationRestriction)
	}
	if p.hasAttr(elem, "block") {
		block := p.attr(elem, "block")
		if TrimXMLWhitespace(block) == "" {
			return nil, fmt.Errorf("block attribute cannot be empty")
		}
		set, err := parseDerivationSetWithValidation(block, DerivationSet(DerivationExtension|DerivationRestriction))
		if err != nil {
			return nil, fmt.Errorf("invalid complexType block attribute value '%s': %w", block, err)
		}
		decl.Block = set
	} else if p.result.Defaults.BlockDefault != 0 {
		decl.Block = p.result.Defaults.BlockDefault & DerivationSet(DerivationExtension|DerivationRestriction)
	}
	var seenAnnotation, seenNonAnnotation bool
	for _, child := range p.xsdChildren(elem) {
		childName := p.doc.LocalName(child)
		handled, err := handleSingleLeadingAnnotation(
			childName,
			&seenAnnotation,
			seenNonAnnotation,
			"complexType: at most one annotation is allowed",
			"complexType: annotation must appear before other elements",
		)
		if err != nil {
			return nil, err
		}
		if handled {
			continue
		}
		seenNonAnnotation = true
		if err := p.parseComplexChild(child, &decl); err != nil {
			return nil, err
		}
	}
	return &decl, nil
}

func (p *documentParser) parseComplexChild(child NodeID, decl *ComplexTypeDecl) error {
	switch p.doc.LocalName(child) {
	case "annotation":
		return nil
	case "sequence", "choice", "all":
		return p.parseComplexParticleChild(child, decl)
	case "group":
		return p.parseComplexParticleChild(child, decl)
	case "attribute":
		return p.parseComplexAttributeChild(child, decl)
	case "attributeGroup":
		return p.parseComplexAttributeGroupChild(child, decl)
	case "anyAttribute":
		return p.parseComplexAnyAttributeChild(child, decl)
	case "simpleContent":
		return p.parseComplexContentChild(child, decl, ComplexContentSimple)
	case "complexContent":
		return p.parseComplexContentChild(child, decl, ComplexContentComplex)
	default:
		return fmt.Errorf("complexType has unexpected child element '%s'", p.doc.LocalName(child))
	}
}

func (p *documentParser) parseComplexParticleChild(child NodeID, decl *ComplexTypeDecl) error {
	if decl.Content != ComplexContentNone {
		return fmt.Errorf("complexType: element content cannot appear with simpleContent or complexContent")
	}
	if len(decl.Attributes) > 0 || len(decl.AttributeGroups) > 0 || decl.AnyAttribute != nil {
		return fmt.Errorf("complexType: content model must appear before attributes")
	}
	if decl.Particle != nil {
		return fmt.Errorf("complexType: only one content model is allowed")
	}
	particle, err := p.parseParticle(child)
	if err != nil {
		return err
	}
	decl.Particle = particle
	return nil
}

func validateComplexAttributePosition(decl *ComplexTypeDecl) error {
	if decl.Content != ComplexContentNone {
		return fmt.Errorf("complexType: attributes must be declared within simpleContent or complexContent")
	}
	if decl.AnyAttribute != nil {
		return fmt.Errorf("complexType: anyAttribute must appear after all attributes")
	}
	return nil
}

func (p *documentParser) parseComplexAttributeChild(child NodeID, decl *ComplexTypeDecl) error {
	if err := validateComplexAttributePosition(decl); err != nil {
		return err
	}
	attr, err := p.parseAttribute(child, false)
	if err != nil {
		return err
	}
	decl.Attributes = append(decl.Attributes, AttributeUseDecl{Attribute: attr})
	return nil
}

func (p *documentParser) parseComplexAttributeGroupChild(child NodeID, decl *ComplexTypeDecl) error {
	if err := validateComplexAttributePosition(decl); err != nil {
		return err
	}
	group, err := p.parseAttributeGroup(child, false)
	if err != nil {
		return err
	}
	decl.AttributeGroups = append(decl.AttributeGroups, group.Ref)
	return nil
}

func (p *documentParser) parseComplexAnyAttributeChild(child NodeID, decl *ComplexTypeDecl) error {
	if decl.Content != ComplexContentNone {
		return fmt.Errorf("complexType: attributes must be declared within simpleContent or complexContent")
	}
	if decl.AnyAttribute != nil {
		return fmt.Errorf("complexType: at most one anyAttribute is allowed")
	}
	wildcard, err := p.parseWildcard(child, false)
	if err != nil {
		return err
	}
	decl.AnyAttribute = wildcard
	return nil
}

func (p *documentParser) parseComplexContentChild(child NodeID, decl *ComplexTypeDecl, content ComplexContentKind) error {
	if decl.Particle != nil || len(decl.Attributes) > 0 || len(decl.AttributeGroups) > 0 || decl.AnyAttribute != nil {
		return fmt.Errorf("complexType: %s must be the only content model", p.doc.LocalName(child))
	}
	if decl.Content != ComplexContentNone {
		return fmt.Errorf("complexType: only one content model is allowed")
	}
	return p.parseDerivationContent(child, decl, content)
}

func (p *documentParser) parseDerivationContent(elem NodeID, decl *ComplexTypeDecl, content ComplexContentKind) error {
	decl.Content = content
	if p.hasAttr(elem, "mixed") {
		mixed, err := p.parseBoolAttrDefault(elem, "mixed", false)
		if err != nil {
			return err
		}
		decl.Mixed = mixed
		decl.MixedSet = true
	}
	var seenAnnotation, seenDerivation bool
	context := p.doc.LocalName(elem)
	for _, child := range p.xsdChildren(elem) {
		childName := p.doc.LocalName(child)
		handled, err := handleSingleLeadingAnnotation(
			childName,
			&seenAnnotation,
			seenDerivation,
			fmt.Sprintf("%s: at most one annotation is allowed", context),
			fmt.Sprintf("%s: annotation must appear before restriction or extension", context),
		)
		if err != nil {
			return err
		}
		if handled {
			continue
		}
		switch childName {
		case "annotation":
			continue
		case "extension", "restriction":
			if seenDerivation {
				return fmt.Errorf("%s must have exactly one derivation child (restriction or extension)", context)
			}
			seenDerivation = true
			if err := p.parseDerivationChild(child, childName, decl, content); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s has unexpected child element '%s'", context, childName)
		}
	}
	if !seenDerivation {
		return fmt.Errorf("%s must have exactly one derivation child (restriction or extension)", context)
	}
	return nil
}

func (p *documentParser) parseDerivationChild(child NodeID, childName string, decl *ComplexTypeDecl, content ComplexContentKind) error {
	if err := validateAnnotationOrder(p.doc, child); err != nil {
		return err
	}
	if childName == "extension" {
		decl.Derivation = ComplexDerivationExtension
	} else {
		decl.Derivation = ComplexDerivationRestriction
	}
	base, err := p.resolveQName(child, p.attr(child, "base"), true)
	if err != nil {
		return fmt.Errorf("resolve %s base: %w", childName, err)
	}
	decl.Base = base
	var state derivationBodyState
	for _, body := range p.xsdChildren(child) {
		bodyName := p.doc.LocalName(body)
		if bodyName == "annotation" {
			continue
		}
		if err := p.parseDerivationBodyChild(body, bodyName, childName, decl, content, &state); err != nil {
			return err
		}
	}
	return nil
}

type derivationBodyState struct {
	seenSimpleType    bool
	seenFacet         bool
	seenAttributeLike bool
}

func (p *documentParser) parseDerivationBodyChild(
	body NodeID,
	bodyName string,
	childName string,
	decl *ComplexTypeDecl,
	content ComplexContentKind,
	state *derivationBodyState,
) error {
	if content == ComplexContentSimple && bodyName == "simpleType" {
		return p.parseSimpleContentInlineType(body, decl, state)
	}
	if content == ComplexContentSimple && p.isFacet(body) {
		return p.parseSimpleContentFacet(body, bodyName, decl, state)
	}
	switch bodyName {
	case "attribute", "attributeGroup", "anyAttribute":
		state.seenAttributeLike = true
	case "simpleContent", "complexContent":
		return fmt.Errorf("%s has unexpected child element '%s'", childName, bodyName)
	default:
		if content == ComplexContentSimple && decl.Derivation == ComplexDerivationExtension {
			return fmt.Errorf("simpleContent extension has unexpected child element '%s'", bodyName)
		}
		if content == ComplexContentSimple && decl.Derivation == ComplexDerivationRestriction {
			return fmt.Errorf("parse facets: unknown or invalid facet '%s' (not a valid XSD 1.0 facet)", bodyName)
		}
	}
	return p.parseComplexDerivationChild(body, decl)
}

func (p *documentParser) parseSimpleContentInlineType(body NodeID, decl *ComplexTypeDecl, state *derivationBodyState) error {
	if decl.Derivation != ComplexDerivationRestriction {
		return fmt.Errorf("simpleContent extension has unexpected child element 'simpleType'")
	}
	if state.seenSimpleType || state.seenFacet || state.seenAttributeLike {
		return fmt.Errorf("simpleContent restriction: simpleType must appear before facets and attributes")
	}
	inline, err := p.parseSimpleType(body, false)
	if err != nil {
		return fmt.Errorf("parse nested simpleType: %w", err)
	}
	decl.SimpleType = inline
	state.seenSimpleType = true
	return nil
}

func (p *documentParser) parseSimpleContentFacet(body NodeID, bodyName string, decl *ComplexTypeDecl, state *derivationBodyState) error {
	if decl.Derivation != ComplexDerivationRestriction {
		return fmt.Errorf("simpleContent extension has unexpected child element '%s'", bodyName)
	}
	if state.seenAttributeLike {
		return fmt.Errorf("simpleContent restriction: facets must appear before attributes")
	}
	facet, err := p.parseFacet(body)
	if err != nil {
		return err
	}
	decl.SimpleFacets = append(decl.SimpleFacets, facet)
	state.seenFacet = true
	return nil
}

func (p *documentParser) parseComplexDerivationChild(child NodeID, decl *ComplexTypeDecl) error {
	content := decl.Content
	decl.Content = ComplexContentNone
	err := p.parseComplexChild(child, decl)
	decl.Content = content
	return err
}

func (p *documentParser) isFacet(elem NodeID) bool {
	switch p.doc.LocalName(elem) {
	case "length", "minLength", "maxLength", "pattern", "enumeration",
		"whiteSpace", "maxInclusive", "maxExclusive", "minInclusive",
		"minExclusive", "totalDigits", "fractionDigits":
		return true
	default:
		return false
	}
}
