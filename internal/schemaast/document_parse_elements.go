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
	if global && p.hasAttr(elem, "form") {
		return nil, fmt.Errorf("top-level element cannot have 'form' attribute")
	}
	if global {
		if err := validateElementAttributes(p.doc, elem, topLevelElementAttributeProfile.allowed, "top-level element"); err != nil {
			return nil, err
		}
	} else if err := validateElementAttributes(p.doc, elem, localElementAttributeProfile.allowed, "local element"); err != nil {
		return nil, err
	}
	if p.hasAttr(elem, "name") && TrimXMLWhitespace(p.rawAttr(elem, "name")) == "" {
		return nil, fmt.Errorf("element name attribute cannot be empty")
	}
	if name := p.attr(elem, "name"); name != "" {
		if err := validateDocumentNCName("element", name); err != nil {
			return nil, err
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
			return nil, fmt.Errorf("resolve element ref %s: %w", ref, err)
		}
		decl.Ref = qname
	}
	if !decl.Name.IsZero() && !decl.Ref.IsZero() {
		return nil, fmt.Errorf("element cannot have both 'name' and 'ref' attributes")
	}
	if decl.Name.IsZero() && decl.Ref.IsZero() {
		return nil, fmt.Errorf("element must have either 'name' or 'ref' attribute")
	}
	if typ, err := p.parseTypeUse(elem, true, true); err != nil {
		return nil, err
	} else {
		decl.Type = typ
	}
	if occ, ok, err := p.parseOccursAttr(elem, "minOccurs"); err != nil {
		return nil, err
	} else if ok {
		decl.MinOccurs = occ
	}
	if occ, ok, err := p.parseOccursAttr(elem, "maxOccurs"); err != nil {
		return nil, err
	} else if ok {
		decl.MaxOccurs = occ
	}
	if p.hasAttr(elem, "final") {
		final := p.attr(elem, "final")
		if TrimXMLWhitespace(final) == "" {
			return nil, fmt.Errorf("final attribute cannot be empty")
		}
		set, err := parseDerivationSetWithValidation(final, DerivationSet(DerivationExtension|DerivationRestriction))
		if err != nil {
			return nil, fmt.Errorf("invalid element final attribute value '%s': %w", final, err)
		}
		decl.Final = set
	} else if global && p.result.Defaults.FinalDefault != 0 {
		decl.Final = p.result.Defaults.FinalDefault & DerivationSet(DerivationExtension|DerivationRestriction)
	}
	if p.hasAttr(elem, "block") {
		block := p.attr(elem, "block")
		if TrimXMLWhitespace(block) == "" {
			return nil, fmt.Errorf("block attribute cannot be empty")
		}
		set, err := parseDerivationSetWithValidation(block, DerivationSet(DerivationSubstitution|DerivationExtension|DerivationRestriction))
		if err != nil {
			return nil, fmt.Errorf("invalid element block attribute value '%s': %w", block, err)
		}
		decl.Block = set
	} else if p.result.Defaults.BlockDefault != 0 {
		decl.Block = p.result.Defaults.BlockDefault & DerivationSet(DerivationSubstitution|DerivationExtension|DerivationRestriction)
	}
	if form, ok, err := p.parseForm(elem); err != nil {
		return nil, err
	} else if ok {
		decl.Form = form
	}
	if !global && !decl.Name.IsZero() && p.localElementQualified(decl.Form) {
		decl.Name.Namespace = p.result.TargetNamespace
	}
	if !decl.Ref.IsZero() {
		for _, attr := range []string{"type", "default", "fixed", "nillable", "block", "final", "form", "abstract"} {
			if p.hasAttr(elem, attr) {
				return nil, fmt.Errorf("invalid attribute '%s' on element reference", attr)
			}
		}
		for _, child := range p.xsdChildren(elem) {
			switch p.doc.LocalName(child) {
			case "simpleType", "complexType":
				return nil, fmt.Errorf("element reference cannot have inline %s", p.doc.LocalName(child))
			}
		}
	}
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
			return nil, fmt.Errorf("resolve substitutionGroup %s: %w", subst, err)
		}
		decl.SubstitutionGroup = qname
	}
	abstract, err := p.parseBoolAttrDefault(elem, "abstract", false)
	if err != nil {
		return nil, err
	}
	nillable, err := p.parseBoolAttrDefault(elem, "nillable", false)
	if err != nil {
		return nil, err
	}
	decl.Abstract = abstract
	decl.Nillable = nillable
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
			return nil, err
		}
		if handled {
			continue
		}
		seenNonAnnotation = true
		switch childName {
		case "simpleType", "complexType":
			if seenIdentity {
				return nil, fmt.Errorf("element type definition must precede identity constraints")
			}
			continue
		case "key", "unique", "keyref":
			seenIdentity = true
			identity, err := p.parseIdentity(child)
			if err != nil {
				return nil, err
			}
			decl.Identity = append(decl.Identity, identity)
		default:
			return nil, fmt.Errorf("element has unexpected child element '%s'", p.doc.LocalName(child))
		}
	}
	return decl, nil
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
	if global {
		switch {
		case p.hasAttr(elem, "form"):
			return nil, fmt.Errorf("top-level attribute cannot have 'form' attribute")
		case p.hasAttr(elem, "use"):
			return nil, fmt.Errorf("top-level attribute cannot have 'use' attribute")
		case p.hasAttr(elem, "ref"):
			return nil, fmt.Errorf("top-level attribute cannot have 'ref' attribute")
		}
	}
	if p.hasAttr(elem, "name") && TrimXMLWhitespace(p.rawAttr(elem, "name")) == "" {
		return nil, fmt.Errorf("attribute name attribute cannot be empty")
	}
	if name := p.attr(elem, "name"); name != "" {
		if err := validateDocumentNCName("attribute", name); err != nil {
			return nil, err
		}
		if name == "xmlns" {
			return nil, fmt.Errorf("attribute name cannot be 'xmlns'")
		}
		ns := NamespaceEmpty
		if global {
			ns = p.result.TargetNamespace
		}
		decl.Name = QName{Namespace: ns, Local: name}
	}
	if p.hasAttr(elem, "ref") && TrimXMLWhitespace(p.rawAttr(elem, "ref")) == "" {
		return nil, fmt.Errorf("attribute ref attribute cannot be empty")
	}
	if ref := p.attr(elem, "ref"); ref != "" {
		qname, err := p.resolveQName(elem, ref, true)
		if err != nil {
			return nil, fmt.Errorf("resolve attribute ref %s: %w", ref, err)
		}
		decl.Ref = qname
	}
	if !decl.Name.IsZero() && !decl.Ref.IsZero() {
		return nil, fmt.Errorf("attribute cannot have both 'name' and 'ref' attributes")
	}
	if decl.Name.IsZero() && decl.Ref.IsZero() {
		return nil, fmt.Errorf("attribute must have either 'name' or 'ref' attribute")
	}
	if !decl.Ref.IsZero() {
		if p.hasAttr(elem, "type") {
			return nil, fmt.Errorf("attribute reference cannot have 'type' attribute")
		}
		if p.hasAttr(elem, "form") {
			return nil, fmt.Errorf("attribute reference cannot have 'form' attribute")
		}
		for _, child := range p.xsdChildren(elem) {
			if p.doc.LocalName(child) == "simpleType" {
				return nil, fmt.Errorf("attribute reference cannot have inline simpleType")
			}
		}
	}
	if typ, err := p.parseTypeUse(elem, false, true); err != nil {
		return nil, err
	} else {
		decl.Type = typ
	}
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
			return nil, fmt.Errorf("invalid use attribute value '%s': must be 'optional', 'prohibited', or 'required'", use)
		}
	}
	if form, ok, err := p.parseForm(elem); err != nil {
		return nil, err
	} else if ok {
		decl.Form = form
	}
	if !global && !decl.Name.IsZero() && p.localAttributeQualified(decl.Form) {
		decl.Name.Namespace = p.result.TargetNamespace
	}
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
			return nil, fmt.Errorf("attribute with use='required' cannot have default value")
		case Prohibited:
			return nil, fmt.Errorf("attribute with use='prohibited' cannot have default value")
		}
	}
	for _, child := range p.xsdChildren(elem) {
		switch p.doc.LocalName(child) {
		case "annotation", "simpleType":
			continue
		default:
			return nil, fmt.Errorf("attribute has unexpected child element '%s'", p.doc.LocalName(child))
		}
	}
	return decl, nil
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
	if !typ.Name.IsZero() && (typ.Simple != nil || typ.Complex != nil) {
		return TypeUse{}, fmt.Errorf("%s cannot have both 'type' attribute and inline type", p.doc.LocalName(elem))
	}
	return typ, nil
}
