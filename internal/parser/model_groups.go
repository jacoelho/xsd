package parser

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/num"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

func parseModelGroup(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.ModelGroup, error) {
	var kind types.GroupKind
	switch doc.LocalName(elem) {
	case "sequence":
		kind = types.Sequence
	case "choice":
		kind = types.Choice
	case "all":
		kind = types.AllGroup
	default:
		return nil, fmt.Errorf("unknown model group: %s", doc.LocalName(elem))
	}

	var (
		idAttr        string
		minOccursAttr string
		maxOccursAttr string
		hasID         bool
		hasMinOccurs  bool
		hasMaxOccurs  bool
	)
	for _, attr := range doc.Attributes(elem) {
		attrName := attr.LocalName()
		if attr.NamespaceURI() == "" {
			if !validAttributeNames[attrSetModelGroup][attrName] {
				return nil, fmt.Errorf("invalid attribute '%s' on <%s> (only id, minOccurs, maxOccurs allowed)", attrName, doc.LocalName(elem))
			}
			switch attrName {
			case "id":
				hasID = true
				idAttr = attr.Value()
			case "minOccurs":
				if !hasMinOccurs {
					hasMinOccurs = true
					minOccursAttr = attr.Value()
				}
				if minOccursAttr == "" {
					return nil, fmt.Errorf("%s: minOccurs attribute cannot be empty", doc.LocalName(elem))
				}
			case "maxOccurs":
				if !hasMaxOccurs {
					hasMaxOccurs = true
					maxOccursAttr = attr.Value()
				}
				if maxOccursAttr == "" {
					return nil, fmt.Errorf("%s: maxOccurs attribute cannot be empty", doc.LocalName(elem))
				}
			}
		}
	}

	if hasID {
		if err := validateIDAttribute(idAttr, doc.LocalName(elem), schema); err != nil {
			return nil, err
		}
	}

	minOccurs, err := parseOccursAttr(doc, elem, "minOccurs")
	if err != nil {
		return nil, err
	}
	maxOccurs, err := parseOccursAttr(doc, elem, "maxOccurs")
	if err != nil {
		return nil, err
	}
	if kind == types.AllGroup {
		if !minOccurs.IsZero() && !minOccurs.IsOne() {
			return nil, fmt.Errorf("xs:all must have minOccurs='0' or '1'")
		}
		if !maxOccurs.IsOne() {
			return nil, fmt.Errorf("xs:all must have maxOccurs='1'")
		}
	}

	mg := &types.ModelGroup{
		Kind:      kind,
		Particles: []types.Particle{},
		MinOccurs: minOccurs,
		MaxOccurs: maxOccurs,
	}

	hasAnnotation := false
	hasNonAnnotation := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return nil, fmt.Errorf("%s: at most one annotation is allowed", doc.LocalName(elem))
			}
			if hasNonAnnotation {
				return nil, fmt.Errorf("%s: annotation must appear before other elements", doc.LocalName(elem))
			}
			hasAnnotation = true
			continue

		case "element":
			hasNonAnnotation = true
			el, err := parseElement(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse element in %s: %w", doc.LocalName(elem), err)
			}
			mg.Particles = append(mg.Particles, el)

		case "sequence", "choice", "all":
			if kind == types.AllGroup {
				return nil, fmt.Errorf("xs:all cannot contain model groups (only element declarations are allowed)")
			}
			hasNonAnnotation = true
			group, err := parseModelGroup(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse %s in %s: %w", doc.LocalName(child), doc.LocalName(elem), err)
			}
			mg.Particles = append(mg.Particles, group)

		case "group":
			if kind == types.AllGroup {
				return nil, fmt.Errorf("xs:all cannot contain group references (only element declarations are allowed)")
			}
			hasNonAnnotation = true
			if err := validateElementConstraints(doc, child, "group", schema); err != nil {
				return nil, err
			}
			ref := doc.GetAttribute(child, "ref")
			if ref == "" {
				return nil, fmt.Errorf("group reference missing ref attribute")
			}
			refQName, err := resolveQName(doc, ref, child, schema)
			if err != nil {
				return nil, fmt.Errorf("resolve group ref %s: %w", ref, err)
			}
			minOccurs, err := parseOccursAttr(doc, child, "minOccurs")
			if err != nil {
				return nil, err
			}
			maxOccurs, err := parseOccursAttr(doc, child, "maxOccurs")
			if err != nil {
				return nil, err
			}
			groupRef := &types.GroupRef{
				RefQName:  refQName,
				MinOccurs: minOccurs,
				MaxOccurs: maxOccurs,
			}
			mg.Particles = append(mg.Particles, groupRef)

		case "any":
			hasNonAnnotation = true
			if kind == types.AllGroup {
				return nil, fmt.Errorf("xs:all cannot contain any wildcards (only element declarations are allowed)")
			}
			anyElem, err := parseAnyElement(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse any element: %w", err)
			}
			mg.Particles = append(mg.Particles, anyElem)

		case "key", "keyref", "unique":
			return nil, fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", doc.LocalName(child))

		case "attribute", "attributeGroup", "anyAttribute":
			return nil, fmt.Errorf("%s cannot appear inside %s (attributes must be declared at complexType level, not inside content model groups)", doc.LocalName(child), doc.LocalName(elem))
		default:
			return nil, fmt.Errorf("%s: unexpected child element <%s>", doc.LocalName(elem), doc.LocalName(child))
		}
	}

	return mg, nil
}

// parseTopLevelGroup parses a top-level <group> definition
// Content model: (annotation?, (all | choice | sequence))
func parseTopLevelGroup(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) error {
	name := getNameAttr(doc, elem)
	if name == "" {
		return fmt.Errorf("group missing name attribute")
	}

	for _, attr := range doc.Attributes(elem) {
		if attr.NamespaceURI() != "" {
			continue
		}
		attrName := attr.LocalName()
		if !validAttributeNames[attrSetTopLevelGroup][attrName] {
			return fmt.Errorf("invalid attribute '%s' on top-level group (only id, name allowed)", attrName)
		}
	}

	if err := validateOptionalID(doc, elem, "group", schema); err != nil {
		return err
	}

	qname := types.QName{
		Namespace: schema.TargetNamespace,
		Local:     name,
	}
	if _, exists := schema.Groups[qname]; exists {
		return fmt.Errorf("duplicate group definition: '%s'", name)
	}

	hasAnnotation := false
	hasModelGroup := false
	var mg *types.ModelGroup

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return fmt.Errorf("group '%s': at most one annotation is allowed", name)
			}
			if hasModelGroup {
				return fmt.Errorf("group '%s': annotation must appear before model group", name)
			}
			hasAnnotation = true

		case "sequence", "choice", "all":
			if hasModelGroup {
				return fmt.Errorf("group '%s': exactly one model group (all, choice, or sequence) is allowed", name)
			}
			var err error
			mg, err = parseModelGroup(doc, child, schema)
			if err != nil {
				return fmt.Errorf("parse model group: %w", err)
			}
			hasModelGroup = true
		case "key", "keyref", "unique":
			return fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", doc.LocalName(child))
		}
	}

	if mg == nil {
		return fmt.Errorf("group '%s' must contain exactly one model group (all, choice, or sequence)", name)
	}

	mg.SourceNamespace = schema.TargetNamespace
	schema.Groups[qname] = mg
	schema.addGlobalDecl(GlobalDeclGroup, qname)
	return nil
}

// parseAnyElement parses an <any> wildcard element
// Content model: (annotation?)
func parseAnyElement(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.AnyElement, error) {
	nsConstraint, nsList, processContents, parseErr := parseWildcardConstraints(
		doc,
		elem,
		"any",
		"namespace, processContents, minOccurs, maxOccurs",
		validAttributeNames[attrSetAnyElement],
	)
	if parseErr != nil {
		return nil, parseErr
	}

	if err := validateOptionalID(doc, elem, "any", schema); err != nil {
		return nil, err
	}

	hasAnnotation := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return nil, fmt.Errorf("any: at most one annotation is allowed")
			}
			hasAnnotation = true
		default:
			return nil, fmt.Errorf("any: unexpected child element '%s'", doc.LocalName(child))
		}
	}

	minOccursAttr := doc.GetAttribute(elem, "minOccurs")
	maxOccursAttr := doc.GetAttribute(elem, "maxOccurs")
	for _, attr := range doc.Attributes(elem) {
		if attr.LocalName() == "minOccurs" && attr.NamespaceURI() == "" && minOccursAttr == "" {
			return nil, fmt.Errorf("minOccurs attribute cannot be empty")
		}
		if attr.LocalName() == "maxOccurs" && attr.NamespaceURI() == "" && maxOccursAttr == "" {
			return nil, fmt.Errorf("maxOccurs attribute cannot be empty")
		}
	}
	if err := validateOccursValue(minOccursAttr); err != nil {
		return nil, fmt.Errorf("invalid minOccurs value '%s': %w", minOccursAttr, err)
	}
	if err := validateOccursValueAllowUnbounded(maxOccursAttr); err != nil {
		return nil, fmt.Errorf("invalid maxOccurs value '%s': %w", maxOccursAttr, err)
	}

	minOccurs, err := parseOccursAttr(doc, elem, "minOccurs")
	if err != nil {
		return nil, err
	}
	maxOccurs, err := parseOccursAttr(doc, elem, "maxOccurs")
	if err != nil {
		return nil, err
	}
	anyElem := &types.AnyElement{
		MinOccurs:       minOccurs,
		MaxOccurs:       maxOccurs,
		ProcessContents: processContents,
		TargetNamespace: schema.TargetNamespace,
	}
	anyElem.Namespace = nsConstraint
	anyElem.NamespaceList = nsList

	return anyElem, nil
}

func validateOccursValue(value string) error {
	if value == "" {
		return nil
	}
	if value == "unbounded" {
		return fmt.Errorf("occurs value must be a non-negative integer")
	}
	return validateOccursInteger(value)
}

func validateOccursValueAllowUnbounded(value string) error {
	if value == "" || value == "unbounded" {
		return nil
	}
	return validateOccursInteger(value)
}

func validateOccursInteger(value string) error {
	if strings.HasPrefix(value, "-") {
		return fmt.Errorf("occurs value must be a non-negative integer")
	}
	if value == "" {
		return fmt.Errorf("occurs value must be a non-negative integer")
	}
	intVal, perr := num.ParseInt([]byte(value))
	if perr != nil || intVal.Sign < 0 {
		return fmt.Errorf("occurs value must be a non-negative integer")
	}
	return nil
}

// parseNamespaceConstraint parses a namespace constraint value
func parseNamespaceConstraint(value string) (types.NamespaceConstraint, []types.NamespaceURI, error) {
	switch value {
	case "##any":
		return types.NSCAny, nil, nil
	case "##other":
		return types.NSCOther, nil, nil
	case "##targetNamespace":
		return types.NSCTargetNamespace, nil, nil
	case "##local":
		return types.NSCLocal, nil, nil
	}

	var resultList []types.NamespaceURI
	seen := false
	for ns := range types.FieldsXMLWhitespaceSeq(value) {
		seen = true
		if strings.HasPrefix(ns, "##") && !validNamespaceConstraintTokens[ns] {
			if ns == "##any" || ns == "##other" {
				return 0, nil, fmt.Errorf("invalid namespace constraint: %s cannot appear in a namespace list (must be used alone)", ns)
			}
			return 0, nil, fmt.Errorf("invalid namespace constraint: unknown special token %s (must be one of: ##any, ##other, ##targetNamespace, ##local)", ns)
		}

		switch ns {
		case "##targetNamespace":
			resultList = append(resultList, types.NamespaceTargetPlaceholder)
		case "##local":
			resultList = append(resultList, types.NamespaceEmpty)
		default:
			resultList = append(resultList, types.NamespaceURI(ns))
		}
	}
	if !seen {
		return 0, nil, fmt.Errorf("invalid namespace constraint: empty namespace list")
	}

	return types.NSCList, resultList, nil
}
