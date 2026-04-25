package schemaast

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/value"
)

func (p *documentParser) parseNotation(elem NodeID) (*NotationDecl, error) {
	if err := validateAllowedAttributes(p.doc, elem, "notation", validNotationAttributes); err != nil {
		return nil, err
	}
	name := p.attr(elem, "name")
	if name == "" {
		return nil, fmt.Errorf("notation missing name attribute")
	}
	if err := validateDocumentNCName("notation", name); err != nil {
		return nil, err
	}
	public := p.attr(elem, "public")
	system := p.attr(elem, "system")
	if public == "" && system == "" {
		return nil, fmt.Errorf("notation must have either 'public' or 'system' attribute")
	}
	if TrimXMLWhitespace(string(p.doc.DirectTextContentBytes(elem))) != "" {
		return nil, fmt.Errorf("notation must not contain character data")
	}
	var seenAnnotation bool
	for _, child := range p.doc.Children(elem) {
		childName := p.doc.LocalName(child)
		if p.doc.NamespaceURI(child) != value.XSDNamespace || childName != "annotation" {
			return nil, fmt.Errorf("notation '%s': unexpected child element '%s'", name, childName)
		}
		if seenAnnotation {
			return nil, fmt.Errorf("notation '%s': at most one annotation is allowed", name)
		}
		seenAnnotation = true
	}
	return &NotationDecl{
		Name:            QName{Namespace: p.result.TargetNamespace, Local: name},
		Public:          public,
		System:          system,
		SourceNamespace: p.result.TargetNamespace,
		Origin:          p.origin(elem),
	}, nil
}

func (p *documentParser) parseIdentity(elem NodeID) (IdentityDecl, error) {
	var identity IdentityDecl
	name := p.attr(elem, "name")
	if name == "" {
		return IdentityDecl{}, fmt.Errorf("identity constraint missing name attribute")
	}
	if err := validateDocumentNCName("identity constraint", name); err != nil {
		return IdentityDecl{}, err
	}
	identity.Name = QName{Namespace: p.result.TargetNamespace, Local: name}
	identity.NamespaceContextID = p.contextID(elem)
	kind := p.doc.LocalName(elem)
	switch kind {
	case "key":
		if p.attr(elem, "refer") != "" {
			return IdentityDecl{}, fmt.Errorf("identity constraint %q: 'refer' attribute is only allowed on keyref constraints", name)
		}
		if err := validateElementAttributes(p.doc, elem, identityConstraintAttributeProfile.allowed, "key"); err != nil {
			return IdentityDecl{}, err
		}
		identity.Kind = IdentityKey
	case "unique":
		if p.attr(elem, "refer") != "" {
			return IdentityDecl{}, fmt.Errorf("identity constraint %q: 'refer' attribute is only allowed on keyref constraints", name)
		}
		if err := validateElementAttributes(p.doc, elem, identityConstraintAttributeProfile.allowed, "unique"); err != nil {
			return IdentityDecl{}, err
		}
		identity.Kind = IdentityUnique
	case "keyref":
		if err := validateElementAttributes(p.doc, elem, keyrefAttributeProfile.allowed, "keyref"); err != nil {
			return IdentityDecl{}, err
		}
		identity.Kind = IdentityKeyref
		refer := p.attr(elem, "refer")
		if refer == "" {
			return IdentityDecl{}, fmt.Errorf("keyref missing refer attribute")
		}
		qname, err := p.resolveQName(elem, refer, true)
		if err != nil {
			return IdentityDecl{}, fmt.Errorf("resolve keyref refer %s: %w", refer, err)
		}
		identity.Refer = qname
	}
	var seenAnnotation, seenNonAnnotation, seenSelector bool
	for _, child := range p.xsdChildren(elem) {
		childName := p.doc.LocalName(child)
		handled, err := handleSingleLeadingAnnotation(
			childName,
			&seenAnnotation,
			seenNonAnnotation,
			fmt.Sprintf("identity constraint %q: at most one annotation is allowed", identity.Name.Local),
			fmt.Sprintf("identity constraint %q: annotation must appear before selector and field", identity.Name.Local),
		)
		if err != nil {
			return IdentityDecl{}, err
		}
		if handled {
			continue
		}
		seenNonAnnotation = true
		switch childName {
		case "selector":
			if seenSelector {
				return IdentityDecl{}, fmt.Errorf("identity constraint %q: only one selector allowed", identity.Name.Local)
			}
			if err := validateElementAttributes(p.doc, child, validAttributeNames[attrSetIdentityConstraint], "selector"); err != nil {
				return IdentityDecl{}, err
			}
			if err := validateOnlyAnnotationChildren(p.doc, child, "selector"); err != nil {
				return IdentityDecl{}, err
			}
			identity.Selector = p.attr(child, "xpath")
			if identity.Selector == "" {
				return IdentityDecl{}, fmt.Errorf("selector missing xpath attribute")
			}
			seenSelector = true
		case "field":
			if !seenSelector {
				return IdentityDecl{}, fmt.Errorf("identity constraint %q: selector must appear before field", identity.Name.Local)
			}
			if err := validateElementAttributes(p.doc, child, validAttributeNames[attrSetIdentityConstraint], "field"); err != nil {
				return IdentityDecl{}, err
			}
			if err := validateOnlyAnnotationChildren(p.doc, child, "field"); err != nil {
				return IdentityDecl{}, err
			}
			if p.attr(child, "xpath") == "" {
				return IdentityDecl{}, fmt.Errorf("field missing xpath attribute")
			}
			field := p.attr(child, "xpath")
			identity.Fields = append(identity.Fields, field)
		default:
			return IdentityDecl{}, fmt.Errorf("identity constraint has unexpected child element '%s'", p.doc.LocalName(child))
		}
	}
	if identity.Selector == "" {
		return IdentityDecl{}, fmt.Errorf("identity constraint missing selector")
	}
	if len(identity.Fields) == 0 {
		return IdentityDecl{}, fmt.Errorf("identity constraint missing fields")
	}
	return identity, nil
}
