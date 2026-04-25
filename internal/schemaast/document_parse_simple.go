package schemaast

import (
	"fmt"
	"strconv"
)

func (p *documentParser) parseSimpleType(elem NodeID, topLevel bool) (*SimpleTypeDecl, error) {
	if err := validateAnnotationOrder(p.doc, elem); err != nil {
		return nil, err
	}
	var decl SimpleTypeDecl
	decl.SourceNamespace = p.result.TargetNamespace
	decl.Origin = p.origin(elem)
	if name := p.attr(elem, "name"); name != "" {
		if !topLevel {
			return nil, fmt.Errorf("inline simpleType cannot have 'name' attribute")
		}
		if err := validateDocumentNCName("type", name); err != nil {
			return nil, err
		}
		decl.Name = QName{Namespace: p.result.TargetNamespace, Local: name}
	}
	if topLevel && decl.Name.IsZero() {
		return nil, fmt.Errorf("simpleType missing name attribute")
	}
	if p.hasAttr(elem, "final") {
		final := p.attr(elem, "final")
		if TrimXMLWhitespace(final) == "" {
			return nil, fmt.Errorf("final attribute cannot be empty")
		}
		set, err := parseDerivationSetWithValidation(final, DerivationSet(DerivationRestriction|DerivationList|DerivationUnion))
		if err != nil {
			return nil, fmt.Errorf("invalid simpleType final attribute value '%s': %w", final, err)
		}
		decl.Final = set
	} else if p.result.Defaults.FinalDefault != 0 {
		decl.Final = p.result.Defaults.FinalDefault & DerivationSet(DerivationRestriction|DerivationList|DerivationUnion)
	}

	for _, child := range p.xsdChildren(elem) {
		switch p.doc.LocalName(child) {
		case "annotation":
			continue
		case "restriction":
			if decl.Kind != SimpleDerivationNone {
				return nil, fmt.Errorf("simpleType has multiple derivation children")
			}
			if err := p.parseSimpleRestriction(child, &decl); err != nil {
				return nil, err
			}
		case "list":
			if decl.Kind != SimpleDerivationNone {
				return nil, fmt.Errorf("simpleType has multiple derivation children")
			}
			if err := p.parseSimpleList(child, &decl); err != nil {
				return nil, err
			}
		case "union":
			if decl.Kind != SimpleDerivationNone {
				return nil, fmt.Errorf("simpleType has multiple derivation children")
			}
			if err := p.parseSimpleUnion(child, &decl); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("simpleType has unexpected child element '%s'", p.doc.LocalName(child))
		}
	}
	if decl.Kind == SimpleDerivationNone {
		return nil, fmt.Errorf("simpleType must have exactly one derivation child")
	}
	return &decl, nil
}

func (p *documentParser) parseSimpleRestriction(elem NodeID, decl *SimpleTypeDecl) error {
	if err := validateAnnotationOrder(p.doc, elem); err != nil {
		return err
	}
	decl.Kind = SimpleDerivationRestriction
	if base := p.attr(elem, "base"); base != "" {
		qname, err := p.resolveQName(elem, base, true)
		if err != nil {
			return fmt.Errorf("resolve restriction base %s: %w", base, err)
		}
		decl.Base = qname
	}
	for _, child := range p.xsdChildren(elem) {
		switch p.doc.LocalName(child) {
		case "annotation":
			continue
		case "simpleType":
			if !decl.Base.IsZero() {
				return fmt.Errorf("restriction cannot have both base attribute and inline simpleType child")
			}
			if decl.InlineBase != nil {
				return fmt.Errorf("restriction cannot have multiple simpleType children")
			}
			inline, err := p.parseSimpleType(child, false)
			if err != nil {
				return err
			}
			decl.InlineBase = inline
		default:
			if !p.isFacet(child) {
				return fmt.Errorf("parse facets: unknown or invalid facet '%s' (not a valid XSD 1.0 facet)", p.doc.LocalName(child))
			}
			facet, err := p.parseFacet(child)
			if err != nil {
				return err
			}
			decl.Facets = append(decl.Facets, facet)
		}
	}
	if decl.Base.IsZero() && decl.InlineBase == nil {
		return fmt.Errorf("restriction missing base attribute and inline simpleType")
	}
	return nil
}

func (p *documentParser) parseSimpleList(elem NodeID, decl *SimpleTypeDecl) error {
	if err := validateAnnotationOrder(p.doc, elem); err != nil {
		return err
	}
	decl.Kind = SimpleDerivationList
	if itemType := p.attr(elem, "itemType"); itemType != "" {
		qname, err := p.resolveQName(elem, itemType, true)
		if err != nil {
			return fmt.Errorf("resolve list itemType %s: %w", itemType, err)
		}
		decl.ItemType = qname
	}
	for _, child := range p.xsdChildren(elem) {
		switch p.doc.LocalName(child) {
		case "annotation":
			continue
		case "simpleType":
			if !decl.ItemType.IsZero() {
				return fmt.Errorf("list cannot have both itemType attribute and inline simpleType child")
			}
			if decl.InlineItem != nil {
				return fmt.Errorf("list cannot have multiple simpleType children")
			}
			inline, err := p.parseSimpleType(child, false)
			if err != nil {
				return err
			}
			decl.InlineItem = inline
		default:
			return fmt.Errorf("list has unexpected child element '%s'", p.doc.LocalName(child))
		}
	}
	if decl.ItemType.IsZero() && decl.InlineItem == nil {
		return fmt.Errorf("list must have either itemType attribute or inline simpleType child")
	}
	return nil
}

func (p *documentParser) parseSimpleUnion(elem NodeID, decl *SimpleTypeDecl) error {
	if err := validateAnnotationOrder(p.doc, elem); err != nil {
		return err
	}
	decl.Kind = SimpleDerivationUnion
	if p.hasAttr(elem, "memberTypes") && TrimXMLWhitespace(p.rawAttr(elem, "memberTypes")) == "" {
		return fmt.Errorf("union memberTypes attribute cannot be empty")
	}
	if members := p.attr(elem, "memberTypes"); members != "" {
		for member := range FieldsXMLWhitespaceSeq(members) {
			qname, err := p.resolveQName(elem, member, true)
			if err != nil {
				return fmt.Errorf("resolve union memberType %s: %w", member, err)
			}
			decl.MemberTypes = append(decl.MemberTypes, qname)
		}
	}
	for _, child := range p.xsdChildren(elem) {
		switch p.doc.LocalName(child) {
		case "annotation":
			continue
		case "simpleType":
			inline, err := p.parseSimpleType(child, false)
			if err != nil {
				return err
			}
			decl.InlineMembers = append(decl.InlineMembers, *inline)
		default:
			return fmt.Errorf("union has unexpected child element '%s'", p.doc.LocalName(child))
		}
	}
	if len(decl.MemberTypes) == 0 && len(decl.InlineMembers) == 0 {
		return fmt.Errorf("union must have at least one member type")
	}
	return nil
}

func (p *documentParser) parseFacet(elem NodeID) (FacetDecl, error) {
	name := p.doc.LocalName(elem)
	if err := validateOnlyAnnotationChildren(p.doc, elem, name); err != nil {
		return FacetDecl{}, err
	}
	lexical := p.rawAttr(elem, "value")
	if err := validateDocumentFacetLexical(name, lexical); err != nil {
		return FacetDecl{}, err
	}
	fixed, err := p.parseBoolAttrDefault(elem, "fixed", false)
	if err != nil {
		return FacetDecl{}, err
	}
	return FacetDecl{
		Name:               name,
		Lexical:            lexical,
		Fixed:              fixed,
		NamespaceContextID: p.contextID(elem),
	}, nil
}

func validateDocumentFacetLexical(name, lexical string) error {
	switch name {
	case "length", "minLength", "maxLength", "fractionDigits":
		value, err := parseDocumentFacetInt(name, lexical)
		if err != nil {
			return err
		}
		if value < 0 {
			return fmt.Errorf("%s value must be non-negative, got %d", name, value)
		}
	case "totalDigits":
		value, err := parseDocumentFacetInt(name, lexical)
		if err != nil {
			return err
		}
		if value <= 0 {
			return fmt.Errorf("totalDigits value must be positive, got %d", value)
		}
	}
	return nil
}

func parseDocumentFacetInt(name, lexical string) (int, error) {
	if lexical == "" {
		return 0, fmt.Errorf("%s facet missing value", name)
	}
	value, err := strconv.Atoi(lexical)
	if err != nil {
		return 0, fmt.Errorf("invalid %s value: %w", name, err)
	}
	return value, nil
}

func validateDocumentNCName(kind, name string) error {
	if IsValidNCName(name) {
		return nil
	}
	return fmt.Errorf("invalid %s name '%s': must be a valid NCName", kind, name)
}
