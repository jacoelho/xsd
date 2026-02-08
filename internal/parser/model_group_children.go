package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseModelGroupChildParticle(doc *xsdxml.Document, child xsdxml.NodeID, schema *Schema, parentKind types.GroupKind, parentName string) (types.Particle, error) {
	childName := doc.LocalName(child)
	switch childName {
	case "element":
		el, err := parseElement(doc, child, schema)
		if err != nil {
			return nil, fmt.Errorf("parse element in %s: %w", parentName, err)
		}
		return el, nil
	case "sequence", "choice", "all":
		if parentKind == types.AllGroup {
			return nil, fmt.Errorf("xs:all cannot contain model groups (only element declarations are allowed)")
		}
		group, err := parseModelGroup(doc, child, schema)
		if err != nil {
			return nil, fmt.Errorf("parse %s in %s: %w", childName, parentName, err)
		}
		return group, nil
	case "group":
		if parentKind == types.AllGroup {
			return nil, fmt.Errorf("xs:all cannot contain group references (only element declarations are allowed)")
		}
		return parseModelGroupGroupRef(doc, child, schema)
	case "any":
		if parentKind == types.AllGroup {
			return nil, fmt.Errorf("xs:all cannot contain any wildcards (only element declarations are allowed)")
		}
		anyElem, err := parseAnyElement(doc, child, schema)
		if err != nil {
			return nil, fmt.Errorf("parse any element: %w", err)
		}
		return anyElem, nil
	case "key", "keyref", "unique":
		return nil, fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", childName)
	case "attribute", "attributeGroup", "anyAttribute":
		return nil, fmt.Errorf("%s cannot appear inside %s (attributes must be declared at complexType level, not inside content model groups)", childName, parentName)
	default:
		return nil, fmt.Errorf("%s: unexpected child element <%s>", parentName, childName)
	}
}

func parseModelGroupGroupRef(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (types.Particle, error) {
	if err := validateElementConstraints(doc, elem, "group", schema); err != nil {
		return nil, err
	}
	ref := doc.GetAttribute(elem, "ref")
	if ref == "" {
		return nil, fmt.Errorf("group reference missing ref attribute")
	}
	refQName, err := resolveQName(doc, ref, elem, schema)
	if err != nil {
		return nil, fmt.Errorf("resolve group ref %s: %w", ref, err)
	}
	minOccurs, err := parseOccursAttr(doc, elem, "minOccurs")
	if err != nil {
		return nil, err
	}
	maxOccurs, err := parseOccursAttr(doc, elem, "maxOccurs")
	if err != nil {
		return nil, err
	}
	return &types.GroupRef{RefQName: refQName, MinOccurs: minOccurs, MaxOccurs: maxOccurs}, nil
}
