package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

var (
	validTopLevelElementAttributes = map[string]bool{
		"id":                true,
		"name":              true,
		"type":              true,
		"default":           true,
		"fixed":             true,
		"nillable":          true,
		"abstract":          true,
		"block":             true,
		"final":             true,
		"substitutionGroup": true,
	}
	validElementReferenceAttributes = map[string]bool{
		"id":        true,
		"ref":       true,
		"minOccurs": true,
		"maxOccurs": true,
	}
	validLocalElementAttributes = map[string]bool{
		"id":        true,
		"name":      true,
		"type":      true,
		"minOccurs": true,
		"maxOccurs": true,
		"default":   true,
		"fixed":     true,
		"nillable":  true,
		"block":     true,
		"form":      true,
		"ref":       true,
	}
)

type elementAttrScan struct {
	defaultVal       string
	ref              string
	name             string
	typ              string
	minOccurs        string
	maxOccurs        string
	invalidRefAttr   string
	fixedVal         string
	nillable         string
	block            string
	form             string
	invalidLocalAttr string
	id               string
	hasRef           bool
	hasType          bool
	hasMinOccurs     bool
	hasMaxOccurs     bool
	hasDefault       bool
	hasFixed         bool
	hasNillable      bool
	hasBlock         bool
	hasForm          bool
	hasAbstract      bool
	hasFinal         bool
	hasName          bool
	hasID            bool
}

func scanElementAttributes(doc *xsdxml.Document, elem xsdxml.NodeID) elementAttrScan {
	var attrs elementAttrScan
	for _, attr := range doc.Attributes(elem) {
		if attr.NamespaceURI() == xsdxml.XMLNSNamespace || attr.NamespaceURI() == "xmlns" || attr.LocalName() == "xmlns" {
			continue
		}
		if attr.NamespaceURI() != "" {
			continue
		}
		attrName := attr.LocalName()
		switch attrName {
		case "id":
			attrs.hasID = true
			attrs.id = attr.Value()
		case "ref":
			if !attrs.hasRef {
				attrs.hasRef = true
				attrs.ref = attr.Value()
			}
		case "name":
			if !attrs.hasName {
				attrs.hasName = true
				attrs.name = attr.Value()
			}
		case "type":
			if !attrs.hasType {
				attrs.hasType = true
				attrs.typ = attr.Value()
			}
		case "minOccurs":
			if !attrs.hasMinOccurs {
				attrs.hasMinOccurs = true
				attrs.minOccurs = attr.Value()
			}
		case "maxOccurs":
			if !attrs.hasMaxOccurs {
				attrs.hasMaxOccurs = true
				attrs.maxOccurs = attr.Value()
			}
		case "default":
			if !attrs.hasDefault {
				attrs.hasDefault = true
				attrs.defaultVal = attr.Value()
			}
		case "fixed":
			if !attrs.hasFixed {
				attrs.hasFixed = true
				attrs.fixedVal = attr.Value()
			}
		case "nillable":
			if !attrs.hasNillable {
				attrs.hasNillable = true
				attrs.nillable = attr.Value()
			}
		case "block":
			if !attrs.hasBlock {
				attrs.hasBlock = true
				attrs.block = attr.Value()
			}
		case "form":
			if !attrs.hasForm {
				attrs.hasForm = true
				attrs.form = attr.Value()
			}
		case "abstract":
			attrs.hasAbstract = true
		case "final":
			attrs.hasFinal = true
		}

		if attr.NamespaceURI() != "" {
			continue
		}
		if attrs.invalidRefAttr == "" && !validElementReferenceAttributes[attrName] {
			attrs.invalidRefAttr = attrName
		}
		if attrs.invalidLocalAttr == "" && !validLocalElementAttributes[attrName] {
			attrs.invalidLocalAttr = attrName
		}
	}
	return attrs
}

// makeAnyType creates an anyType ComplexType.
// Per XSD 1.0 spec, anyType has mixed content allowing any elements and text.
func makeAnyType() types.Type {
	return types.NewAnyTypeComplexType()
}

// parseTopLevelElement parses a top-level element declaration
func parseTopLevelElement(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) error {
	name := getNameAttr(doc, elem)
	if name == "" {
		return fmt.Errorf("element missing name attribute")
	}

	if err := validateElementAttributes(doc, elem, validTopLevelElementAttributes, "top-level element"); err != nil {
		return err
	}

	if hasIDAttribute(doc, elem) {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, "element", schema); err != nil {
			return err
		}
	}

	// 'form' attribute only applies to local element declarations
	if doc.HasAttribute(elem, "form") {
		return fmt.Errorf("top-level element cannot have 'form' attribute")
	}

	// validate annotation order: if present, must be first child
	if err := validateAnnotationOrder(doc, elem); err != nil {
		return err
	}
	if err := validateElementChildrenOrder(doc, elem); err != nil {
		return err
	}

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation", "complexType", "simpleType", "key", "keyref", "unique":
			// allowed.
		default:
			return fmt.Errorf("invalid child element <%s> in <element> declaration", doc.LocalName(child))
		}
	}

	// validate: cannot have both default and fixed
	if doc.HasAttribute(elem, "default") && doc.HasAttribute(elem, "fixed") {
		return fmt.Errorf("element cannot have both 'default' and 'fixed' attributes")
	}

	var hasInlineType bool
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) == xsdxml.XSDNamespace {
			if doc.LocalName(child) == "complexType" || doc.LocalName(child) == "simpleType" {
				hasInlineType = true
				break
			}
		}
	}

	// validate: cannot have both type attribute and inline type definition
	if doc.GetAttribute(elem, "type") != "" && hasInlineType {
		return fmt.Errorf("element cannot have both 'type' attribute and inline type definition")
	}

	decl := &types.ElementDecl{
		Name: types.QName{
			Namespace: schema.TargetNamespace,
			Local:     name,
		},
		MinOccurs:       types.OccursFromInt(1),
		MaxOccurs:       types.OccursFromInt(1),
		SourceNamespace: schema.TargetNamespace,
		Form:            types.FormQualified, // global elements are always qualified
	}

	if typeName := doc.GetAttribute(elem, "type"); typeName != "" {
		typeQName, err := resolveQName(doc, typeName, elem, schema)
		if err != nil {
			return fmt.Errorf("resolve type %s: %w", typeName, err)
		}

		// check if it's a built-in type
		if builtinType := types.GetBuiltinNS(typeQName.Namespace, typeQName.Local); builtinType != nil {
			// anyType is special - it's a complex type, not a simple type
			if typeQName.Local == "anyType" {
				decl.Type = makeAnyType()
			} else {
				decl.Type = builtinType
			}
		} else {
			// will be resolved later in a second pass
			// for now, create a placeholder
			decl.Type = types.NewPlaceholderSimpleType(typeQName)
		}
		decl.TypeExplicit = true
	} else {
		for _, child := range doc.Children(elem) {
			if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
				continue
			}

			switch doc.LocalName(child) {
			case "complexType":
				if doc.GetAttribute(child, "name") != "" {
					return fmt.Errorf("inline complexType cannot have 'name' attribute")
				}
				ct, err := parseInlineComplexType(doc, child, schema)
				if err != nil {
					return fmt.Errorf("parse inline complexType: %w", err)
				}
				decl.Type = ct
				decl.TypeExplicit = true
			case "simpleType":
				if doc.GetAttribute(child, "name") != "" {
					return fmt.Errorf("inline simpleType cannot have 'name' attribute")
				}
				st, err := parseInlineSimpleType(doc, child, schema)
				if err != nil {
					return fmt.Errorf("parse inline simpleType: %w", err)
				}
				decl.Type = st
				decl.TypeExplicit = true
			}
		}
		// if no inline type was found, default to anyType
		if decl.Type == nil {
			decl.Type = makeAnyType()
		}
	}

	if ok, value, err := parseBoolAttribute(doc, elem, "nillable"); err != nil {
		return err
	} else if ok {
		decl.Nillable = value
	}

	if ok, value, err := parseBoolAttribute(doc, elem, "abstract"); err != nil {
		return err
	} else if ok {
		decl.Abstract = value
	}

	if doc.HasAttribute(elem, "default") {
		decl.Default = doc.GetAttribute(elem, "default")
		decl.HasDefault = true
		decl.DefaultContext = namespaceContextForElement(doc, elem, schema)
	}

	// fixed attribute may have an empty value (fixed=""), so check for presence
	if doc.HasAttribute(elem, "fixed") {
		decl.Fixed = doc.GetAttribute(elem, "fixed")
		decl.HasFixed = true
		decl.FixedContext = namespaceContextForElement(doc, elem, schema)
	}

	// parse block attribute (space-separated list: substitution, extension, restriction, #all)
	if doc.HasAttribute(elem, "block") {
		blockAttr := doc.GetAttribute(elem, "block")
		if blockAttr == "" {
			decl.Block = 0
		} else {
			block, err := parseDerivationSetWithValidation(blockAttr, types.DerivationSet(types.DerivationSubstitution|types.DerivationExtension|types.DerivationRestriction))
			if err != nil {
				return fmt.Errorf("invalid block attribute value '%s': %w", blockAttr, err)
			}
			decl.Block = block
		}
	} else if schema.BlockDefault != 0 {
		decl.Block = schema.BlockDefault & types.DerivationSet(types.DerivationSubstitution|types.DerivationExtension|types.DerivationRestriction)
	}

	// parse final attribute (space-separated list: extension, restriction, #all).
	// element final does not allow substitution; W3C tests (elemF004/006/007/008) expect invalid.
	if doc.HasAttribute(elem, "final") {
		finalAttr := doc.GetAttribute(elem, "final")
		if finalAttr == "" {
			decl.Final = 0
		} else {
			final, err := parseDerivationSetWithValidation(finalAttr, types.DerivationSet(types.DerivationExtension|types.DerivationRestriction))
			if err != nil {
				return fmt.Errorf("invalid final attribute value '%s': %w", finalAttr, err)
			}
			decl.Final = final
		}
	} else if schema.FinalDefault != 0 {
		decl.Final = schema.FinalDefault & types.DerivationSet(types.DerivationExtension|types.DerivationRestriction)
	}

	if subGroup := doc.GetAttribute(elem, "substitutionGroup"); subGroup != "" {
		// use resolveElementQName for element references (not type references)
		subGroupQName, err := resolveElementQName(doc, subGroup, elem, schema)
		if err != nil {
			return fmt.Errorf("resolve substitutionGroup %s: %w", subGroup, err)
		}
		decl.SubstitutionGroup = subGroupQName

		// add to schema's substitution groups map
		if schema.SubstitutionGroups[subGroupQName] == nil {
			schema.SubstitutionGroups[subGroupQName] = []types.QName{}
		}
		schema.SubstitutionGroups[subGroupQName] = append(schema.SubstitutionGroups[subGroupQName], decl.Name)
	}

	// parse identity constraints (key, keyref, unique)
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "key", "keyref", "unique":
			constraint, err := parseIdentityConstraint(doc, child, schema)
			if err != nil {
				return fmt.Errorf("parse identity constraint: %w", err)
			}
			decl.Constraints = append(decl.Constraints, constraint)
		}
	}

	decl, err := types.NewElementDeclFromParsed(decl)
	if err != nil {
		return err
	}

	if _, exists := schema.ElementDecls[decl.Name]; exists {
		return fmt.Errorf("duplicate element declaration: '%s'", decl.Name)
	}
	schema.ElementDecls[decl.Name] = decl
	return nil
}

// parseElement parses an element reference or declaration within a content model
func parseElement(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) (*types.ElementDecl, error) {
	attrs := scanElementAttributes(doc, elem)

	if attrs.hasID {
		if err := validateIDAttribute(attrs.id, "element", schema); err != nil {
			return nil, err
		}
	}

	if attrs.ref != "" && attrs.name != "" {
		return nil, fmt.Errorf("element cannot have both 'name' and 'ref' attributes")
	}

	if attrs.ref != "" {
		return parseElementReference(doc, elem, schema, &attrs)
	}

	return parseLocalElement(doc, elem, schema, &attrs)
}

func parseElementReference(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema, attrs *elementAttrScan) (*types.ElementDecl, error) {
	if err := validateElementReferenceAttributes(doc, elem, attrs); err != nil {
		return nil, err
	}

	refQName, err := resolveElementQName(doc, attrs.ref, elem, schema)
	if err != nil {
		return nil, fmt.Errorf("resolve ref %s: %w", attrs.ref, err)
	}

	minOccurs, maxOccurs, err := parseElementOccurs(attrs)
	if err != nil {
		return nil, err
	}

	decl := &types.ElementDecl{
		Name:        refQName,
		MinOccurs:   minOccurs,
		MaxOccurs:   maxOccurs,
		IsReference: true,
	}
	parsed, err := types.NewElementDeclFromParsed(decl)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func parseLocalElement(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema, attrs *elementAttrScan) (*types.ElementDecl, error) {
	if attrs.name == "" {
		return nil, fmt.Errorf("element missing name and ref")
	}
	if attrs.invalidLocalAttr != "" {
		return nil, fmt.Errorf("invalid attribute '%s' on local element", attrs.invalidLocalAttr)
	}
	if err := validateLocalElementChildren(doc, elem); err != nil {
		return nil, err
	}
	if err := validateLocalElementAttributes(attrs); err != nil {
		return nil, err
	}

	hasInlineType := elementHasInlineType(doc, elem)
	if attrs.typ != "" && hasInlineType {
		return nil, fmt.Errorf("element cannot have both 'type' attribute and inline type definition")
	}

	effectiveForm, elementNamespace, err := resolveLocalElementForm(attrs, schema)
	if err != nil {
		return nil, err
	}

	minOccurs, maxOccurs, err := parseElementOccurs(attrs)
	if err != nil {
		return nil, err
	}

	decl := &types.ElementDecl{
		Name: types.QName{
			Namespace: elementNamespace,
			Local:     attrs.name,
		},
		SourceNamespace: schema.TargetNamespace,
		MinOccurs:       minOccurs,
		MaxOccurs:       maxOccurs,
	}
	decl.TypeExplicit = attrs.hasType || hasInlineType
	if effectiveForm == Qualified {
		decl.Form = types.FormQualified
	} else {
		decl.Form = types.FormUnqualified
	}

	typ, err := resolveElementType(doc, elem, schema, attrs)
	if err != nil {
		return nil, err
	}
	decl.Type = typ

	err = applyElementConstraints(doc, elem, schema, attrs, decl)
	if err != nil {
		return nil, err
	}

	parsed, err := types.NewElementDeclFromParsed(decl)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func validateElementReferenceAttributes(doc *xsdxml.Document, elem xsdxml.NodeID, attrs *elementAttrScan) error {
	if attrs.invalidRefAttr != "" {
		return fmt.Errorf("invalid attribute '%s' on element reference", attrs.invalidRefAttr)
	}
	if err := validateOnlyAnnotationChildren(doc, elem, "element"); err != nil {
		return err
	}
	if attrs.hasType {
		return fmt.Errorf("element reference cannot have 'type' attribute")
	}
	if attrs.hasDefault {
		return fmt.Errorf("element reference cannot have 'default' attribute")
	}
	if attrs.hasFixed {
		return fmt.Errorf("element reference cannot have 'fixed' attribute")
	}
	if attrs.hasNillable {
		return fmt.Errorf("element reference cannot have 'nillable' attribute")
	}
	if attrs.hasBlock {
		return fmt.Errorf("element reference cannot have 'block' attribute")
	}
	if attrs.hasFinal {
		return fmt.Errorf("element reference cannot have 'final' attribute")
	}
	if attrs.hasForm {
		return fmt.Errorf("element reference cannot have 'form' attribute")
	}
	if attrs.hasAbstract {
		return fmt.Errorf("element reference cannot have 'abstract' attribute")
	}
	return nil
}

func validateLocalElementAttributes(attrs *elementAttrScan) error {
	if attrs.hasAbstract {
		return fmt.Errorf("local element cannot have 'abstract' attribute (only global elements can be abstract)")
	}
	if attrs.hasFinal {
		return fmt.Errorf("local element cannot have 'final' attribute (only global elements can be final)")
	}
	if attrs.hasDefault && attrs.hasFixed {
		return fmt.Errorf("element cannot have both 'default' and 'fixed' attributes")
	}
	return nil
}

func validateLocalElementChildren(doc *xsdxml.Document, elem xsdxml.NodeID) error {
	if err := validateAnnotationOrder(doc, elem); err != nil {
		return err
	}
	if err := validateElementChildrenOrder(doc, elem); err != nil {
		return err
	}

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation", "complexType", "simpleType", "key", "keyref", "unique":
			continue
		default:
			return fmt.Errorf("invalid child element <%s> in <element> declaration", doc.LocalName(child))
		}
	}

	return nil
}

func elementHasInlineType(doc *xsdxml.Document, elem xsdxml.NodeID) bool {
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) == xsdxml.XSDNamespace {
			name := doc.LocalName(child)
			if name == "complexType" || name == "simpleType" {
				return true
			}
		}
	}
	return false
}

func resolveLocalElementForm(attrs *elementAttrScan, schema *Schema) (Form, types.NamespaceURI, error) {
	var effectiveForm Form
	if formAttr := attrs.form; formAttr != "" {
		formAttr = types.ApplyWhiteSpace(formAttr, types.WhiteSpaceCollapse)
		switch formAttr {
		case "qualified":
			effectiveForm = Qualified
		case "unqualified":
			effectiveForm = Unqualified
		default:
			return Unqualified, "", fmt.Errorf("invalid form attribute value '%s': must be 'qualified' or 'unqualified'", formAttr)
		}
	} else {
		effectiveForm = schema.ElementFormDefault
	}

	var elementNamespace types.NamespaceURI
	if effectiveForm == Qualified {
		elementNamespace = schema.TargetNamespace
	}

	return effectiveForm, elementNamespace, nil
}

func parseElementOccurs(attrs *elementAttrScan) (types.Occurs, types.Occurs, error) {
	minOccurs := types.OccursFromInt(1)
	if attrs.hasMinOccurs {
		var err error
		minOccurs, err = parseOccursValue("minOccurs", attrs.minOccurs)
		if err != nil {
			return types.OccursFromInt(0), types.OccursFromInt(0), err
		}
	}
	maxOccurs := types.OccursFromInt(1)
	if attrs.hasMaxOccurs {
		var err error
		maxOccurs, err = parseOccursValue("maxOccurs", attrs.maxOccurs)
		if err != nil {
			return types.OccursFromInt(0), types.OccursFromInt(0), err
		}
	}
	return minOccurs, maxOccurs, nil
}

func resolveElementType(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema, attrs *elementAttrScan) (types.Type, error) {
	if typeName := attrs.typ; typeName != "" {
		typeQName, err := resolveQName(doc, typeName, elem, schema)
		if err != nil {
			return nil, fmt.Errorf("resolve type %s: %w", typeName, err)
		}

		if builtinType := types.GetBuiltinNS(typeQName.Namespace, typeQName.Local); builtinType != nil {
			if typeQName.Local == "anyType" {
				return makeAnyType(), nil
			}
			return builtinType, nil
		}
		return types.NewPlaceholderSimpleType(typeQName), nil
	}

	var typ types.Type
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "complexType":
			if doc.GetAttribute(child, "name") != "" {
				return nil, fmt.Errorf("inline complexType cannot have 'name' attribute")
			}
			ct, err := parseInlineComplexType(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse inline complexType: %w", err)
			}
			typ = ct
		case "simpleType":
			if doc.GetAttribute(child, "name") != "" {
				return nil, fmt.Errorf("inline simpleType cannot have 'name' attribute")
			}
			st, err := parseInlineSimpleType(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse inline simpleType: %w", err)
			}
			typ = st
		}
	}

	if typ == nil {
		typ = makeAnyType()
	}

	return typ, nil
}

func applyElementConstraints(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema, attrs *elementAttrScan, decl *types.ElementDecl) error {
	if attrs.hasNillable {
		value, err := parseBoolValue("nillable", attrs.nillable)
		if err != nil {
			return err
		}
		decl.Nillable = value
	}

	if attrs.hasDefault {
		decl.Default = attrs.defaultVal
		decl.HasDefault = true
		decl.DefaultContext = namespaceContextForElement(doc, elem, schema)
	}

	if attrs.hasFixed {
		decl.Fixed = attrs.fixedVal
		decl.HasFixed = true
		decl.FixedContext = namespaceContextForElement(doc, elem, schema)
	}

	if attrs.hasBlock {
		blockAttr := attrs.block
		if blockAttr == "" {
			decl.Block = 0
		} else {
			block, err := parseDerivationSetWithValidation(blockAttr, types.DerivationSet(types.DerivationSubstitution|types.DerivationExtension|types.DerivationRestriction))
			if err != nil {
				return fmt.Errorf("invalid block attribute value '%s': %w", blockAttr, err)
			}
			decl.Block = block
		}
	} else if schema.BlockDefault != 0 {
		decl.Block = schema.BlockDefault & types.DerivationSet(types.DerivationSubstitution|types.DerivationExtension|types.DerivationRestriction)
	}

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "key", "keyref", "unique":
			constraint, err := parseIdentityConstraint(doc, child, schema)
			if err != nil {
				return fmt.Errorf("parse identity constraint: %w", err)
			}
			decl.Constraints = append(decl.Constraints, constraint)
		}
	}

	return nil
}
