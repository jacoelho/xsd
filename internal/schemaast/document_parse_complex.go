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
	case "group":
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
	case "attribute":
		if decl.Content != ComplexContentNone {
			return fmt.Errorf("complexType: attributes must be declared within simpleContent or complexContent")
		}
		if decl.AnyAttribute != nil {
			return fmt.Errorf("complexType: anyAttribute must appear after all attributes")
		}
		attr, err := p.parseAttribute(child, false)
		if err != nil {
			return err
		}
		decl.Attributes = append(decl.Attributes, AttributeUseDecl{Attribute: attr})
	case "attributeGroup":
		if decl.Content != ComplexContentNone {
			return fmt.Errorf("complexType: attributes must be declared within simpleContent or complexContent")
		}
		if decl.AnyAttribute != nil {
			return fmt.Errorf("complexType: anyAttribute must appear after all attributes")
		}
		group, err := p.parseAttributeGroup(child, false)
		if err != nil {
			return err
		}
		decl.AttributeGroups = append(decl.AttributeGroups, group.Ref)
	case "anyAttribute":
		if decl.Content != ComplexContentNone {
			return fmt.Errorf("complexType: attributes must be declared within simpleContent or complexContent")
		}
		if decl.AnyAttribute != nil {
			return fmt.Errorf("complexType: at most one anyAttribute is allowed")
		}
		any, err := p.parseWildcard(child, false)
		if err != nil {
			return err
		}
		decl.AnyAttribute = any
	case "simpleContent":
		if decl.Particle != nil || len(decl.Attributes) > 0 || len(decl.AttributeGroups) > 0 || decl.AnyAttribute != nil {
			return fmt.Errorf("complexType: simpleContent must be the only content model")
		}
		if decl.Content != ComplexContentNone {
			return fmt.Errorf("complexType: only one content model is allowed")
		}
		return p.parseDerivationContent(child, decl, ComplexContentSimple)
	case "complexContent":
		if decl.Particle != nil || len(decl.Attributes) > 0 || len(decl.AttributeGroups) > 0 || decl.AnyAttribute != nil {
			return fmt.Errorf("complexType: complexContent must be the only content model")
		}
		if decl.Content != ComplexContentNone {
			return fmt.Errorf("complexType: only one content model is allowed")
		}
		return p.parseDerivationContent(child, decl, ComplexContentComplex)
	default:
		return fmt.Errorf("complexType has unexpected child element '%s'", p.doc.LocalName(child))
	}
	return nil
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
			var seenSimpleType, seenFacet, seenAttributeLike bool
			for _, body := range p.xsdChildren(child) {
				bodyName := p.doc.LocalName(body)
				if bodyName == "annotation" {
					continue
				}
				if content == ComplexContentSimple && bodyName == "simpleType" {
					if decl.Derivation != ComplexDerivationRestriction {
						return fmt.Errorf("simpleContent extension has unexpected child element 'simpleType'")
					}
					if seenSimpleType || seenFacet || seenAttributeLike {
						return fmt.Errorf("simpleContent restriction: simpleType must appear before facets and attributes")
					}
					inline, err := p.parseSimpleType(body, false)
					if err != nil {
						return fmt.Errorf("parse nested simpleType: %w", err)
					}
					decl.SimpleType = inline
					seenSimpleType = true
					continue
				}
				if content == ComplexContentSimple && p.isFacet(body) {
					if decl.Derivation != ComplexDerivationRestriction {
						return fmt.Errorf("simpleContent extension has unexpected child element '%s'", bodyName)
					}
					if seenAttributeLike {
						return fmt.Errorf("simpleContent restriction: facets must appear before attributes")
					}
					facet, err := p.parseFacet(body)
					if err != nil {
						return err
					}
					decl.SimpleFacets = append(decl.SimpleFacets, facet)
					seenFacet = true
					continue
				}
				switch bodyName {
				case "attribute", "attributeGroup", "anyAttribute":
					seenAttributeLike = true
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
				if err := p.parseComplexDerivationChild(body, decl); err != nil {
					return err
				}
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
