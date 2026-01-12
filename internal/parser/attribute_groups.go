package parser

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

type attributeUses struct {
	anyAttribute *types.AnyAttribute
	attributes   []*types.AttributeDecl
	attrGroups   []types.QName
}

func parseAttributeUses(doc *xsdxml.Document, children []xsdxml.NodeID, schema *Schema, context string) (attributeUses, error) {
	uses := attributeUses{
		attributes: []*types.AttributeDecl{},
		attrGroups: []types.QName{},
	}
	hasAnyAttribute := false

	for _, child := range children {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation":
			continue
		case "attribute":
			if hasAnyAttribute {
				return uses, fmt.Errorf("%s: anyAttribute must appear after all attributes", context)
			}
			attr, err := parseAttribute(doc, child, schema)
			if err != nil {
				return uses, fmt.Errorf("parse attribute in %s: %w", context, err)
			}
			uses.attributes = append(uses.attributes, attr)
		case "attributeGroup":
			if hasAnyAttribute {
				return uses, fmt.Errorf("%s: anyAttribute must appear after all attributes", context)
			}
			if err := validateElementConstraints(doc, child, "attributeGroup", schema); err != nil {
				return uses, err
			}
			ref := doc.GetAttribute(child, "ref")
			if ref == "" {
				return uses, fmt.Errorf("attributeGroup reference missing ref attribute")
			}
			refQName, err := resolveQName(doc, ref, child, schema)
			if err != nil {
				return uses, fmt.Errorf("resolve attributeGroup ref %s: %w", ref, err)
			}
			uses.attrGroups = append(uses.attrGroups, refQName)
		case "anyAttribute":
			if hasAnyAttribute {
				return uses, fmt.Errorf("%s: at most one anyAttribute is allowed", context)
			}
			hasAnyAttribute = true
			anyAttr, err := parseAnyAttribute(doc, child, schema)
			if err != nil {
				return uses, fmt.Errorf("parse anyAttribute in %s: %w", context, err)
			}
			uses.anyAttribute = anyAttr
		}
	}

	return uses, nil
}

// parseTopLevelAttributeGroup parses a top-level <attributeGroup> definition
// Content model: (annotation?, ((attribute | attributeGroup)*, anyAttribute?))
func parseTopLevelAttributeGroup(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) error {
	name := getNameAttr(doc, elem)
	if name == "" {
		return fmt.Errorf("attributeGroup missing name attribute")
	}

	if hasIDAttribute(doc, elem) {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, "attributeGroup", schema); err != nil {
			return err
		}
	}

	attrGroup := &types.AttributeGroup{
		Name: types.QName{
			Namespace: schema.TargetNamespace,
			Local:     name,
		},
		Attributes:      []*types.AttributeDecl{},
		AttrGroups:      []types.QName{},
		SourceNamespace: schema.TargetNamespace,
	}

	hasAnnotation := false
	hasNonAnnotation := false
	hasAnyAttribute := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return fmt.Errorf("attributeGroup '%s': at most one annotation is allowed", name)
			}
			if hasNonAnnotation {
				return fmt.Errorf("attributeGroup '%s': annotation must appear before other elements", name)
			}
			hasAnnotation = true

		case "attribute":
			hasNonAnnotation = true
			attr, err := parseAttribute(doc, child, schema)
			if err != nil {
				return fmt.Errorf("attributeGroup: parse attribute: %w", err)
			}
			attrGroup.Attributes = append(attrGroup.Attributes, attr)

		case "attributeGroup":
			hasNonAnnotation = true
			if doc.HasAttribute(child, "name") {
				return fmt.Errorf("attributeGroup reference cannot have 'name' attribute")
			}
			if err := validateElementConstraints(doc, child, "attributeGroup", schema); err != nil {
				return err
			}
			ref := doc.GetAttribute(child, "ref")
			if ref == "" {
				return fmt.Errorf("attributeGroup reference missing ref attribute")
			}
			refQName, err := resolveQName(doc, ref, child, schema)
			if err != nil {
				return fmt.Errorf("resolve attributeGroup ref %s: %w", ref, err)
			}
			attrGroup.AttrGroups = append(attrGroup.AttrGroups, refQName)

		case "anyAttribute":
			hasNonAnnotation = true
			if hasAnyAttribute {
				return fmt.Errorf("attributeGroup '%s': at most one anyAttribute is allowed", name)
			}
			hasAnyAttribute = true
			anyAttr, err := parseAnyAttribute(doc, child, schema)
			if err != nil {
				return fmt.Errorf("parse anyAttribute in attributeGroup: %w", err)
			}
			attrGroup.AnyAttribute = anyAttr

		case "key", "keyref", "unique":
			return fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", doc.LocalName(child))
		default:
			return fmt.Errorf("invalid child element <%s> in <attributeGroup> declaration", doc.LocalName(child))
		}
	}

	qname := types.QName{
		Namespace: schema.TargetNamespace,
		Local:     name,
	}
	if _, exists := schema.AttributeGroups[qname]; exists {
		return fmt.Errorf("attributeGroup %s already defined", qname)
	}
	schema.AttributeGroups[qname] = attrGroup
	return nil
}

// parseAnyAttribute parses an <anyAttribute> wildcard
// Content model: (annotation?)
func parseAnyAttribute(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.AnyAttribute, error) {
	if doc.GetAttribute(elem, "notNamespace") != "" {
		return nil, fmt.Errorf("notNamespace attribute is not supported in XSD 1.0 (XSD 1.1 feature)")
	}
	if doc.GetAttribute(elem, "notQName") != "" {
		return nil, fmt.Errorf("notQName attribute is not supported in XSD 1.0 (XSD 1.1 feature)")
	}

	for _, attr := range doc.Attributes(elem) {
		attrName := attr.LocalName()
		if attrName == "xmlns" || strings.HasPrefix(attrName, "xmlns:") {
			continue
		}
		if attr.NamespaceURI() == "" && !validAttributeNames[attrSetAnyAttribute][attrName] {
			return nil, fmt.Errorf("invalid attribute '%s' on <anyAttribute> element (XSD 1.0 only allows: namespace, processContents)", attrName)
		}
	}

	if hasIDAttribute(doc, elem) {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, "anyAttribute", schema); err != nil {
			return nil, err
		}
	}

	hasAnnotation := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation":
			if hasAnnotation {
				return nil, fmt.Errorf("anyAttribute: at most one annotation is allowed")
			}
			hasAnnotation = true
		default:
			return nil, fmt.Errorf("anyAttribute: unexpected child element '%s'", doc.LocalName(child))
		}
	}

	anyAttr := &types.AnyAttribute{
		ProcessContents: types.Strict,
		TargetNamespace: schema.TargetNamespace,
	}

	namespaceAttr := doc.GetAttribute(elem, "namespace")
	hasNamespaceAttr := false
	for _, attr := range doc.Attributes(elem) {
		if attr.LocalName() == "namespace" && attr.NamespaceURI() == "" {
			hasNamespaceAttr = true
			break
		}
	}
	if !hasNamespaceAttr {
		namespaceAttr = "##any"
	} else if namespaceAttr == "" {
		namespaceAttr = "##local"
	}

	nsConstraint, nsList, err := parseNamespaceConstraint(namespaceAttr, schema)
	if err != nil {
		return nil, fmt.Errorf("parse namespace constraint: %w", err)
	}
	anyAttr.Namespace = nsConstraint
	anyAttr.NamespaceList = nsList

	processContents := doc.GetAttribute(elem, "processContents")
	hasProcessContents := false
	for _, attr := range doc.Attributes(elem) {
		if attr.LocalName() == "processContents" && attr.NamespaceURI() == "" {
			hasProcessContents = true
			break
		}
	}
	if hasProcessContents && processContents == "" {
		return nil, fmt.Errorf("processContents attribute cannot be empty")
	}

	switch processContents {
	case "strict":
		anyAttr.ProcessContents = types.Strict
	case "lax":
		anyAttr.ProcessContents = types.Lax
	case "skip":
		anyAttr.ProcessContents = types.Skip
	case "":
		anyAttr.ProcessContents = types.Strict
	default:
		return nil, fmt.Errorf("invalid processContents value '%s': must be 'strict', 'lax', or 'skip'", processContents)
	}

	return anyAttr, nil
}

// parseIdentityConstraint parses a key, keyref, or unique constraint
func parseIdentityConstraint(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.IdentityConstraint, error) {
	name := getNameAttr(doc, elem)
	if name == "" {
		return nil, fmt.Errorf("identity constraint missing name attribute")
	}

	if hasIDAttribute(doc, elem) {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, doc.LocalName(elem), schema); err != nil {
			return nil, err
		}
	}

	nsContext := namespaceContextForElement(doc, elem, schema)

	constraint := &types.IdentityConstraint{
		Name:             name,
		TargetNamespace:  schema.TargetNamespace,
		Fields:           []types.Field{},
		NamespaceContext: nsContext,
	}

	switch doc.LocalName(elem) {
	case "key":
		constraint.Type = types.KeyConstraint
	case "keyref":
		constraint.Type = types.KeyRefConstraint
	case "unique":
		constraint.Type = types.UniqueConstraint
	default:
		return nil, fmt.Errorf("unknown identity constraint type: %s", doc.LocalName(elem))
	}

	refer := doc.GetAttribute(elem, "refer")
	if refer != "" {
		referQName, err := resolveIdentityConstraintQName(doc, refer, elem, schema)
		if err != nil {
			return nil, fmt.Errorf("resolve refer QName %s: %w", refer, err)
		}
		constraint.ReferQName = referQName
	} else if constraint.Type == types.KeyRefConstraint {
		return nil, fmt.Errorf("keyref missing refer attribute")
	}

	annotationCount := 0
	seenSelector := false
	seenField := false

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "annotation":
			if seenSelector || seenField {
				return nil, fmt.Errorf("identity constraint '%s': annotation must appear before selector and field", name)
			}
			annotationCount++
			if annotationCount > 1 {
				return nil, fmt.Errorf("identity constraint '%s': at most one annotation allowed", name)
			}

		case "selector":
			if seenSelector {
				return nil, fmt.Errorf("identity constraint '%s': only one selector allowed", name)
			}
			xpath := doc.GetAttribute(child, "xpath")
			if xpath == "" {
				return nil, fmt.Errorf("selector missing xpath attribute")
			}
			if err := validateAllowedAttributes(doc, child, "selector", validAttributeNames[attrSetIdentityConstraint]); err != nil {
				return nil, err
			}
			if err := validateElementConstraints(doc, child, "selector", schema); err != nil {
				return nil, err
			}
			constraint.Selector = types.Selector{XPath: xpath}
			seenSelector = true

		case "field":
			if !seenSelector {
				return nil, fmt.Errorf("identity constraint '%s': selector must appear before field", name)
			}
			xpath := doc.GetAttribute(child, "xpath")
			if xpath == "" {
				return nil, fmt.Errorf("field missing xpath attribute")
			}
			if err := validateAllowedAttributes(doc, child, "field", validAttributeNames[attrSetIdentityConstraint]); err != nil {
				return nil, err
			}
			if err := validateElementConstraints(doc, child, "field", schema); err != nil {
				return nil, err
			}
			constraint.Fields = append(constraint.Fields, types.Field{XPath: xpath})
			seenField = true
		}
	}

	if constraint.Selector.XPath == "" {
		return nil, fmt.Errorf("identity constraint missing selector")
	}

	if len(constraint.Fields) == 0 {
		return nil, fmt.Errorf("identity constraint missing fields")
	}

	return constraint, nil
}

func validateAllowedAttributes(doc *xsdxml.Document, elem xsdxml.NodeID, elementName string, allowed map[string]bool) error {
	for _, attr := range doc.Attributes(elem) {
		if attr.NamespaceURI() == xsdxml.XMLNSNamespace || attr.LocalName() == "xmlns" {
			continue
		}
		if attr.NamespaceURI() != "" {
			if attr.NamespaceURI() == xsdxml.XSDNamespace {
				return fmt.Errorf("%s: attribute '%s' must be unprefixed", elementName, attr.LocalName())
			}
			continue
		}
		if !allowed[attr.LocalName()] {
			return fmt.Errorf("%s: unexpected attribute '%s'", elementName, attr.LocalName())
		}
	}
	return nil
}

// parseDerivationSetWithValidation parses and validates a derivation set.
// Returns an error if any token is not a valid derivation method.
// Per XSD spec, #all cannot be combined with other values.
func parseDerivationSetWithValidation(value string, allowed types.DerivationSet) (types.DerivationSet, error) {
	var set types.DerivationSet
	hasAll := false
	for token := range strings.FieldsSeq(value) {
		if hasAll {
			return set, fmt.Errorf("derivation set cannot combine '#all' with other values")
		}
		switch token {
		case "extension":
			if !allowed.Has(types.DerivationExtension) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationExtension)
		case "restriction":
			if !allowed.Has(types.DerivationRestriction) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationRestriction)
		case "list":
			if !allowed.Has(types.DerivationList) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationList)
		case "union":
			if !allowed.Has(types.DerivationUnion) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationUnion)
		case "substitution":
			if !allowed.Has(types.DerivationSubstitution) {
				return set, fmt.Errorf("invalid derivation method '%s'", token)
			}
			set = set.Add(types.DerivationSubstitution)
		case "#all":
			if set != 0 {
				return set, fmt.Errorf("derivation set cannot combine '#all' with other values")
			}
			set = allowed
			hasAll = true
		default:
			return set, fmt.Errorf("invalid derivation method '%s'", token)
		}
	}
	return set, nil
}
