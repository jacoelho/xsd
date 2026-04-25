package schemaast

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/value"
)

func (p *documentParser) parseElement(elem NodeID, global bool) (*ElementDecl, error) {
	decl := &ElementDecl{
		MinOccurs:          OccursFromInt(1),
		MaxOccurs:          OccursFromInt(1),
		NamespaceContextID: p.contextID(elem),
		SourceNamespace:    p.result.TargetNamespace,
		Global:             global,
		Origin:             p.origin(elem),
	}
	if err := p.validateElementScope(elem, global); err != nil {
		return nil, err
	}
	if err := p.parseElementNameRef(elem, decl, global); err != nil {
		return nil, err
	}
	typ, err := p.parseTypeUse(elem, true, true)
	if err != nil {
		return nil, err
	}
	decl.Type = typ
	if err := p.parseElementOccurs(elem, decl); err != nil {
		return nil, err
	}
	if err := p.parseElementDerivationAttrs(elem, decl, global); err != nil {
		return nil, err
	}
	if err := p.parseElementForm(elem, decl, global); err != nil {
		return nil, err
	}
	if err := p.validateElementReference(elem, decl); err != nil {
		return nil, err
	}
	if err := p.parseElementConstraints(elem, decl); err != nil {
		return nil, err
	}
	if err := p.parseElementChildren(elem, decl); err != nil {
		return nil, err
	}
	return decl, nil
}

func (p *documentParser) validateElementScope(elem NodeID, global bool) error {
	if global && p.hasAttr(elem, "form") {
		return fmt.Errorf("top-level element cannot have 'form' attribute")
	}
	if global {
		if err := validateElementAttributes(p.doc, elem, topLevelElementAttributeProfile.allowed, "top-level element"); err != nil {
			return err
		}
	} else if err := validateElementAttributes(p.doc, elem, localElementAttributeProfile.allowed, "local element"); err != nil {
		return err
	}
	return nil
}

func (p *documentParser) parseElementNameRef(elem NodeID, decl *ElementDecl, global bool) error {
	if p.hasAttr(elem, "name") && TrimXMLWhitespace(p.rawAttr(elem, "name")) == "" {
		return fmt.Errorf("element name attribute cannot be empty")
	}
	if name := p.attr(elem, "name"); name != "" {
		if err := validateDocumentNCName("element", name); err != nil {
			return err
		}
		ns := NamespaceEmpty
		if global {
			ns = p.result.TargetNamespace
		}
		decl.Name = QName{Namespace: ns, Local: name}
	}
	if ref := p.attr(elem, "ref"); ref != "" {
		qname, err := p.resolveQName(elem, ref, true)
		if err != nil {
			return fmt.Errorf("resolve element ref %s: %w", ref, err)
		}
		decl.Ref = qname
	}
	if !decl.Name.IsZero() && !decl.Ref.IsZero() {
		return fmt.Errorf("element cannot have both 'name' and 'ref' attributes")
	}
	if decl.Name.IsZero() && decl.Ref.IsZero() {
		return fmt.Errorf("element must have either 'name' or 'ref' attribute")
	}
	return nil
}

func (p *documentParser) parseElementOccurs(elem NodeID, decl *ElementDecl) error {
	if occ, ok, err := p.parseOccursAttr(elem, "minOccurs"); err != nil {
		return err
	} else if ok {
		decl.MinOccurs = occ
	}
	if occ, ok, err := p.parseOccursAttr(elem, "maxOccurs"); err != nil {
		return err
	} else if ok {
		decl.MaxOccurs = occ
	}
	return nil
}

func (p *documentParser) parseElementDerivationAttrs(elem NodeID, decl *ElementDecl, global bool) error {
	if p.hasAttr(elem, "final") {
		final := p.attr(elem, "final")
		if TrimXMLWhitespace(final) == "" {
			return fmt.Errorf("final attribute cannot be empty")
		}
		set, err := parseDerivationSetWithValidation(final, DerivationSet(DerivationExtension|DerivationRestriction))
		if err != nil {
			return fmt.Errorf("invalid element final attribute value '%s': %w", final, err)
		}
		decl.Final = set
	} else if global && p.result.Defaults.FinalDefault != 0 {
		decl.Final = p.result.Defaults.FinalDefault & DerivationSet(DerivationExtension|DerivationRestriction)
	}
	if p.hasAttr(elem, "block") {
		block := p.attr(elem, "block")
		if TrimXMLWhitespace(block) == "" {
			return fmt.Errorf("block attribute cannot be empty")
		}
		set, err := parseDerivationSetWithValidation(block, DerivationSet(DerivationSubstitution|DerivationExtension|DerivationRestriction))
		if err != nil {
			return fmt.Errorf("invalid element block attribute value '%s': %w", block, err)
		}
		decl.Block = set
	} else if p.result.Defaults.BlockDefault != 0 {
		decl.Block = p.result.Defaults.BlockDefault & DerivationSet(DerivationSubstitution|DerivationExtension|DerivationRestriction)
	}
	return nil
}

func (p *documentParser) parseElementForm(elem NodeID, decl *ElementDecl, global bool) error {
	if form, ok, err := p.parseForm(elem); err != nil {
		return err
	} else if ok {
		decl.Form = form
	}
	if !global && !decl.Name.IsZero() && p.localElementQualified(decl.Form) {
		decl.Name.Namespace = p.result.TargetNamespace
	}
	return nil
}

func (p *documentParser) validateElementReference(elem NodeID, decl *ElementDecl) error {
	if !decl.Ref.IsZero() {
		for _, attr := range []string{"type", "default", "fixed", "nillable", "block", "final", "form", "abstract"} {
			if p.hasAttr(elem, attr) {
				return fmt.Errorf("invalid attribute '%s' on element reference", attr)
			}
		}
		for _, child := range p.xsdChildren(elem) {
			switch p.doc.LocalName(child) {
			case "simpleType", "complexType":
				return fmt.Errorf("element reference cannot have inline %s", p.doc.LocalName(child))
			}
		}
	}
	return nil
}

func (p *documentParser) parseElementConstraints(elem NodeID, decl *ElementDecl) error {
	if p.hasAttr(elem, "default") {
		decl.Default = ValueConstraintDecl{
			Lexical:            p.rawAttr(elem, "default"),
			NamespaceContextID: decl.NamespaceContextID,
			Present:            true,
		}
	}
	if p.hasAttr(elem, "fixed") {
		decl.Fixed = ValueConstraintDecl{
			Lexical:            p.rawAttr(elem, "fixed"),
			NamespaceContextID: decl.NamespaceContextID,
			Present:            true,
		}
	}
	if subst := p.attr(elem, "substitutionGroup"); subst != "" {
		qname, err := p.resolveQName(elem, subst, true)
		if err != nil {
			return fmt.Errorf("resolve substitutionGroup %s: %w", subst, err)
		}
		decl.SubstitutionGroup = qname
	}
	abstract, err := p.parseBoolAttrDefault(elem, "abstract", false)
	if err != nil {
		return err
	}
	nillable, err := p.parseBoolAttrDefault(elem, "nillable", false)
	if err != nil {
		return err
	}
	decl.Abstract = abstract
	decl.Nillable = nillable
	return nil
}

func (p *documentParser) parseElementChildren(elem NodeID, decl *ElementDecl) error {
	var seenAnnotation, seenNonAnnotation, seenIdentity bool
	for _, child := range p.xsdChildren(elem) {
		childName := p.doc.LocalName(child)
		handled, err := handleSingleLeadingAnnotation(
			childName,
			&seenAnnotation,
			seenNonAnnotation,
			"element: at most one annotation is allowed",
			"element: annotation must appear before other elements",
		)
		if err != nil {
			return err
		}
		if handled {
			continue
		}
		seenNonAnnotation = true
		switch childName {
		case "simpleType", "complexType":
			if seenIdentity {
				return fmt.Errorf("element type definition must precede identity constraints")
			}
			continue
		case "key", "unique", "keyref":
			seenIdentity = true
			identity, err := p.parseIdentity(child)
			if err != nil {
				return err
			}
			decl.Identity = append(decl.Identity, identity)
		default:
			return fmt.Errorf("element has unexpected child element '%s'", p.doc.LocalName(child))
		}
	}
	return nil
}

func (p *documentParser) parseAttribute(elem NodeID, global bool) (*AttributeDecl, error) {
	if err := validateAnnotationOrder(p.doc, elem); err != nil {
		return nil, fmt.Errorf("parse attribute: %w", err)
	}
	if err := validateDocumentAttributeAttributes(p.doc, elem); err != nil {
		return nil, err
	}
	decl := &AttributeDecl{
		Use:                Optional,
		NamespaceContextID: p.contextID(elem),
		SourceNamespace:    p.result.TargetNamespace,
		Global:             global,
		Origin:             p.origin(elem),
	}
	if err := p.validateAttributeScope(elem, global); err != nil {
		return nil, err
	}
	if err := p.parseAttributeNameRef(elem, decl, global); err != nil {
		return nil, err
	}
	if err := p.validateAttributeReference(elem, decl); err != nil {
		return nil, err
	}
	typ, err := p.parseTypeUse(elem, false, true)
	if err != nil {
		return nil, err
	}
	decl.Type = typ
	if err := p.parseAttributeUseAndForm(elem, decl, global); err != nil {
		return nil, err
	}
	if err := p.parseAttributeConstraints(elem, decl); err != nil {
		return nil, err
	}
	if err := p.validateAttributeChildren(elem); err != nil {
		return nil, err
	}
	return decl, nil
}

func (p *documentParser) validateAttributeScope(elem NodeID, global bool) error {
	if global {
		switch {
		case p.hasAttr(elem, "form"):
			return fmt.Errorf("top-level attribute cannot have 'form' attribute")
		case p.hasAttr(elem, "use"):
			return fmt.Errorf("top-level attribute cannot have 'use' attribute")
		case p.hasAttr(elem, "ref"):
			return fmt.Errorf("top-level attribute cannot have 'ref' attribute")
		}
	}
	return nil
}

func (p *documentParser) parseAttributeNameRef(elem NodeID, decl *AttributeDecl, global bool) error {
	if p.hasAttr(elem, "name") && TrimXMLWhitespace(p.rawAttr(elem, "name")) == "" {
		return fmt.Errorf("attribute name attribute cannot be empty")
	}
	if name := p.attr(elem, "name"); name != "" {
		if err := validateDocumentNCName("attribute", name); err != nil {
			return err
		}
		if name == "xmlns" {
			return fmt.Errorf("attribute name cannot be 'xmlns'")
		}
		ns := NamespaceEmpty
		if global {
			ns = p.result.TargetNamespace
		}
		decl.Name = QName{Namespace: ns, Local: name}
	}
	if p.hasAttr(elem, "ref") && TrimXMLWhitespace(p.rawAttr(elem, "ref")) == "" {
		return fmt.Errorf("attribute ref attribute cannot be empty")
	}
	if ref := p.attr(elem, "ref"); ref != "" {
		qname, err := p.resolveQName(elem, ref, true)
		if err != nil {
			return fmt.Errorf("resolve attribute ref %s: %w", ref, err)
		}
		decl.Ref = qname
	}
	if !decl.Name.IsZero() && !decl.Ref.IsZero() {
		return fmt.Errorf("attribute cannot have both 'name' and 'ref' attributes")
	}
	if decl.Name.IsZero() && decl.Ref.IsZero() {
		return fmt.Errorf("attribute must have either 'name' or 'ref' attribute")
	}
	return nil
}

func (p *documentParser) validateAttributeReference(elem NodeID, decl *AttributeDecl) error {
	if !decl.Ref.IsZero() {
		if p.hasAttr(elem, "type") {
			return fmt.Errorf("attribute reference cannot have 'type' attribute")
		}
		if p.hasAttr(elem, "form") {
			return fmt.Errorf("attribute reference cannot have 'form' attribute")
		}
		for _, child := range p.xsdChildren(elem) {
			if p.doc.LocalName(child) == "simpleType" {
				return fmt.Errorf("attribute reference cannot have inline simpleType")
			}
		}
	}
	return nil
}

func (p *documentParser) parseAttributeUseAndForm(elem NodeID, decl *AttributeDecl, global bool) error {
	if p.hasAttr(elem, "use") {
		use := p.attr(elem, "use")
		switch use {
		case "optional":
			decl.Use = Optional
		case "required":
			decl.Use = Required
		case "prohibited":
			decl.Use = Prohibited
		default:
			return fmt.Errorf("invalid use attribute value '%s': must be 'optional', 'prohibited', or 'required'", use)
		}
	}
	if form, ok, err := p.parseForm(elem); err != nil {
		return err
	} else if ok {
		decl.Form = form
	}
	if !global && !decl.Name.IsZero() && p.localAttributeQualified(decl.Form) {
		decl.Name.Namespace = p.result.TargetNamespace
	}
	return nil
}

func (p *documentParser) parseAttributeConstraints(elem NodeID, decl *AttributeDecl) error {
	if p.hasAttr(elem, "default") {
		decl.Default = ValueConstraintDecl{
			Lexical:            p.rawAttr(elem, "default"),
			NamespaceContextID: decl.NamespaceContextID,
			Present:            true,
		}
	}
	if p.hasAttr(elem, "fixed") {
		decl.Fixed = ValueConstraintDecl{
			Lexical:            p.rawAttr(elem, "fixed"),
			NamespaceContextID: decl.NamespaceContextID,
			Present:            true,
		}
	}
	if decl.Default.Present {
		switch decl.Use {
		case Required:
			return fmt.Errorf("attribute with use='required' cannot have default value")
		case Prohibited:
			return fmt.Errorf("attribute with use='prohibited' cannot have default value")
		}
	}
	return nil
}

func (p *documentParser) validateAttributeChildren(elem NodeID) error {
	for _, child := range p.xsdChildren(elem) {
		switch p.doc.LocalName(child) {
		case "annotation", "simpleType":
			continue
		default:
			return fmt.Errorf("attribute has unexpected child element '%s'", p.doc.LocalName(child))
		}
	}
	return nil
}

func validateDocumentAttributeAttributes(doc *Document, elem NodeID) error {
	for _, attr := range doc.Attributes(elem) {
		if isXMLNSDeclaration(attr) {
			continue
		}
		if attr.NamespaceURI() == value.XSDNamespace {
			return fmt.Errorf("attribute: attribute '%s' must be unprefixed", attr.LocalName())
		}
		if attr.NamespaceURI() == "" && !attributeDeclarationProfile.allows(attr.LocalName()) {
			return fmt.Errorf("invalid attribute '%s' on <attribute> element", attr.LocalName())
		}
	}
	return nil
}

func (p *documentParser) parseTypeUse(elem NodeID, allowComplex, allowSimple bool) (TypeUse, error) {
	var typ TypeUse
	if raw := p.attr(elem, "type"); raw != "" {
		qname, err := p.resolveQName(elem, raw, true)
		if err != nil {
			return TypeUse{}, fmt.Errorf("resolve type %s: %w", raw, err)
		}
		typ.Name = qname
	}
	for _, child := range p.xsdChildren(elem) {
		switch p.doc.LocalName(child) {
		case "annotation":
			continue
		case "simpleType":
			if !allowSimple {
				return TypeUse{}, fmt.Errorf("%s cannot contain inline simpleType", p.doc.LocalName(elem))
			}
			if typ.Simple != nil {
				return TypeUse{}, fmt.Errorf("%s cannot have multiple simpleType children", p.doc.LocalName(elem))
			}
			inline, err := p.parseSimpleType(child, false)
			if err != nil {
				return TypeUse{}, err
			}
			typ.Simple = inline
		case "complexType":
			if !allowComplex {
				return TypeUse{}, fmt.Errorf("%s cannot contain inline complexType", p.doc.LocalName(elem))
			}
			if typ.Complex != nil {
				return TypeUse{}, fmt.Errorf("%s cannot have multiple complexType children", p.doc.LocalName(elem))
			}
			inline, err := p.parseComplexType(child, false)
			if err != nil {
				return TypeUse{}, err
			}
			typ.Complex = inline
		}
	}
	if typ.Simple != nil && typ.Complex != nil {
		return TypeUse{}, fmt.Errorf("%s cannot have more than one inline type definition", p.doc.LocalName(elem))
	}
	if !typ.Name.IsZero() && (typ.Simple != nil || typ.Complex != nil) {
		return TypeUse{}, fmt.Errorf("%s cannot have both 'type' attribute and inline type", p.doc.LocalName(elem))
	}
	return typ, nil
}
