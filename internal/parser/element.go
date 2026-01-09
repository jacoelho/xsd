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
	id         string
	ref        string
	name       string
	typ        string
	minOccurs  string
	maxOccurs  string
	defaultVal string
	fixedVal   string
	nillable   string
	block      string
	form       string

	hasID        bool
	hasRef       bool
	hasName      bool
	hasType      bool
	hasMinOccurs bool
	hasMaxOccurs bool
	hasDefault   bool
	hasFixed     bool
	hasNillable  bool
	hasBlock     bool
	hasForm      bool
	hasAbstract  bool
	hasFinal     bool

	invalidRefAttr   string
	invalidLocalAttr string
}

func scanElementAttributes(doc *xsdxml.Document, elem xsdxml.NodeID) elementAttrScan {
	var attrs elementAttrScan
	for _, attr := range doc.Attributes(elem) {
		if attr.NamespaceURI() == xsdxml.XMLNSNamespace || attr.NamespaceURI() == "xmlns" || attr.LocalName() == "xmlns" {
			continue
		}
		attrName := attr.LocalName()
		switch attrName {
		case "id":
			if attr.NamespaceURI() == "" {
				attrs.hasID = true
				attrs.id = attr.Value()
			}
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
	ct := &types.ComplexType{
		QName: types.QName{
			Namespace: xsdxml.XSDNamespace,
			Local:     "anyType",
		},
	}
	ct.SetContent(&types.EmptyContent{})
	ct.SetMixed(true)
	return ct
}

// parseTopLevelElement parses a top-level element declaration
func parseTopLevelElement(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) error {
	name := getAttr(doc, elem, "name")
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
	if doc.GetAttribute(elem, "default") != "" && doc.GetAttribute(elem, "fixed") != "" {
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
		MinOccurs:       1,
		MaxOccurs:       1,
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
			decl.Type = &types.SimpleType{
				QName: typeQName,
			}
		}
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
			case "simpleType":
				if doc.GetAttribute(child, "name") != "" {
					return fmt.Errorf("inline simpleType cannot have 'name' attribute")
				}
				st, err := parseInlineSimpleType(doc, child, schema)
				if err != nil {
					return fmt.Errorf("parse inline simpleType: %w", err)
				}
				decl.Type = st
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

	if defaultVal := doc.GetAttribute(elem, "default"); defaultVal != "" {
		decl.Default = defaultVal
	}

	// fixed attribute may have an empty value (fixed=""), so check for presence
	if doc.HasAttribute(elem, "fixed") {
		decl.Fixed = doc.GetAttribute(elem, "fixed")
		decl.HasFixed = true
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
	ref := attrs.ref
	name := attrs.name

	if attrs.hasID {
		if err := validateIDAttribute(attrs.id, "element", schema); err != nil {
			return nil, err
		}
	}

	// validate: cannot have both name and ref
	if ref != "" && name != "" {
		return nil, fmt.Errorf("element cannot have both 'name' and 'ref' attributes")
	}

	// check if it's a reference
	if ref != "" {
		if attrs.invalidRefAttr != "" {
			return nil, fmt.Errorf("invalid attribute '%s' on element reference", attrs.invalidRefAttr)
		}
		if err := validateOnlyAnnotationChildren(doc, elem, "element"); err != nil {
			return nil, err
		}
		// validate: element references cannot have certain attributes
		// per XSD spec, when ref is present, these are forbidden:
		// name, type, default, fixed, nillable, block, form, substitutionGroup, abstract
		if attrs.hasType {
			return nil, fmt.Errorf("element reference cannot have 'type' attribute")
		}
		if attrs.hasDefault {
			return nil, fmt.Errorf("element reference cannot have 'default' attribute")
		}
		if attrs.hasFixed {
			return nil, fmt.Errorf("element reference cannot have 'fixed' attribute")
		}
		if attrs.hasNillable {
			return nil, fmt.Errorf("element reference cannot have 'nillable' attribute")
		}
		if attrs.hasBlock {
			return nil, fmt.Errorf("element reference cannot have 'block' attribute")
		}
		if attrs.hasForm {
			return nil, fmt.Errorf("element reference cannot have 'form' attribute")
		}
		if attrs.hasAbstract {
			return nil, fmt.Errorf("element reference cannot have 'abstract' attribute")
		}

		// for element references, use resolveElementQName (doesn't check built-in types)
		refQName, err := resolveElementQName(doc, ref, elem, schema)
		if err != nil {
			return nil, fmt.Errorf("resolve ref %s: %w", ref, err)
		}

		minOccurs, err := parseOccursValue("minOccurs", attrs.minOccurs, attrs.hasMinOccurs, 1)
		if err != nil {
			return nil, err
		}
		maxOccurs, err := parseOccursValue("maxOccurs", attrs.maxOccurs, attrs.hasMaxOccurs, 1)
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

	// local element declaration
	if name == "" {
		return nil, fmt.Errorf("element missing name and ref")
	}

	if attrs.invalidLocalAttr != "" {
		return nil, fmt.Errorf("invalid attribute '%s' on local element", attrs.invalidLocalAttr)
	}

	if err := validateAnnotationOrder(doc, elem); err != nil {
		return nil, err
	}
	if err := validateElementChildrenOrder(doc, elem); err != nil {
		return nil, err
	}

	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation", "complexType", "simpleType", "key", "keyref", "unique":
			// allowed.
		default:
			return nil, fmt.Errorf("invalid child element <%s> in <element> declaration", doc.LocalName(child))
		}
	}

	// validate: abstract attribute is only allowed on global elements
	// per XSD spec, local elements cannot have the abstract attribute
	if attrs.hasAbstract {
		return nil, fmt.Errorf("local element cannot have 'abstract' attribute (only global elements can be abstract)")
	}

	// validate: final attribute is only allowed on global elements
	if attrs.hasFinal {
		return nil, fmt.Errorf("local element cannot have 'final' attribute (only global elements can be final)")
	}

	// validate: cannot have both default and fixed
	if attrs.defaultVal != "" && attrs.fixedVal != "" {
		return nil, fmt.Errorf("element cannot have both 'default' and 'fixed' attributes")
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
	if attrs.typ != "" && hasInlineType {
		return nil, fmt.Errorf("element cannot have both 'type' attribute and inline type definition")
	}

	// per XSD spec: local elements use the form attribute if present,
	// otherwise use schema's elementFormDefault (which defaults to unqualified)
	var effectiveForm Form
	if formAttr := attrs.form; formAttr != "" {
		switch formAttr {
		case "qualified":
			effectiveForm = Qualified
		case "unqualified":
			effectiveForm = Unqualified
		default:
			return nil, fmt.Errorf("invalid form attribute value '%s': must be 'qualified' or 'unqualified'", formAttr)
		}
	} else {
		effectiveForm = schema.ElementFormDefault
	}

	// for unqualified elements, the namespace is empty (no namespace)
	// for qualified elements, the namespace is the target namespace
	var elementNamespace types.NamespaceURI
	if effectiveForm == Qualified {
		elementNamespace = schema.TargetNamespace
	}

	decl := &types.ElementDecl{
		Name: types.QName{
			Namespace: elementNamespace,
			Local:     name,
		},
		SourceNamespace: schema.TargetNamespace,
	}
	minOccurs, err := parseOccursValue("minOccurs", attrs.minOccurs, attrs.hasMinOccurs, 1)
	if err != nil {
		return nil, err
	}
	maxOccurs, err := parseOccursValue("maxOccurs", attrs.maxOccurs, attrs.hasMaxOccurs, 1)
	if err != nil {
		return nil, err
	}
	decl.MinOccurs = minOccurs
	decl.MaxOccurs = maxOccurs

	if effectiveForm == Qualified {
		decl.Form = types.FormQualified
	} else {
		decl.Form = types.FormUnqualified
	}

	if typeName := attrs.typ; typeName != "" {
		typeQName, err := resolveQName(doc, typeName, elem, schema)
		if err != nil {
			return nil, fmt.Errorf("resolve type %s: %w", typeName, err)
		}

		if builtinType := types.GetBuiltinNS(typeQName.Namespace, typeQName.Local); builtinType != nil {
			// anyType is special - it's a complex type, not a simple type
			if typeQName.Local == "anyType" {
				decl.Type = makeAnyType()
			} else {
				decl.Type = builtinType
			}
		} else {
			decl.Type = &types.SimpleType{
				QName: typeQName,
			}
		}
	} else {
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
				decl.Type = ct
			case "simpleType":
				if doc.GetAttribute(child, "name") != "" {
					return nil, fmt.Errorf("inline simpleType cannot have 'name' attribute")
				}
				st, err := parseInlineSimpleType(doc, child, schema)
				if err != nil {
					return nil, fmt.Errorf("parse inline simpleType: %w", err)
				}
				decl.Type = st
			}
		}
		if decl.Type == nil {
			decl.Type = makeAnyType()
		}
	}

	if ok, value, err := parseBoolValue("nillable", attrs.nillable, attrs.hasNillable); err != nil {
		return nil, err
	} else if ok {
		decl.Nillable = value
	}

	if defaultVal := attrs.defaultVal; defaultVal != "" {
		decl.Default = defaultVal
	}

	// fixed attribute may have an empty value (fixed=""), so check for presence
	if attrs.hasFixed {
		decl.Fixed = attrs.fixedVal
		decl.HasFixed = true
	}

	// parse block attribute (space-separated list: substitution, extension, restriction, #all)
	if attrs.hasBlock {
		blockAttr := attrs.block
		if blockAttr == "" {
			decl.Block = 0
		} else {
			block, err := parseDerivationSetWithValidation(blockAttr, types.DerivationSet(types.DerivationSubstitution|types.DerivationExtension|types.DerivationRestriction))
			if err != nil {
				return nil, fmt.Errorf("invalid block attribute value '%s': %w", blockAttr, err)
			}
			decl.Block = block
		}
	} else if schema.BlockDefault != 0 {
		decl.Block = schema.BlockDefault & types.DerivationSet(types.DerivationSubstitution|types.DerivationExtension|types.DerivationRestriction)
	}

	// parse identity constraints (key, keyref, unique) for local elements
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		switch doc.LocalName(child) {
		case "key", "keyref", "unique":
			constraint, err := parseIdentityConstraint(doc, child, schema)
			if err != nil {
				return nil, fmt.Errorf("parse identity constraint: %w", err)
			}
			decl.Constraints = append(decl.Constraints, constraint)
		}
	}

	parsed, err := types.NewElementDeclFromParsed(decl)
	if err != nil {
		return nil, err
	}
	return parsed, nil
}

func validateElementAttributes(doc *xsdxml.Document, elem xsdxml.NodeID, validAttributes map[string]bool, context string) error {
	for _, attr := range doc.Attributes(elem) {
		if attr.NamespaceURI() == xsdxml.XMLNSNamespace || attr.NamespaceURI() == "xmlns" || attr.LocalName() == "xmlns" {
			continue
		}
		if attr.NamespaceURI() != "" {
			continue
		}
		if !validAttributes[attr.LocalName()] {
			return fmt.Errorf("invalid attribute '%s' on %s", attr.LocalName(), context)
		}
	}
	return nil
}

func namespaceForPrefix(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema, prefix string) string {
	for current := elem; current != xsdxml.InvalidNode; current = doc.Parent(current) {
		for _, attr := range doc.Attributes(current) {
			isXMLNSAttr := attr.NamespaceURI() == xsdxml.XMLNSNamespace ||
				(attr.NamespaceURI() == "" && attr.LocalName() == "xmlns")
			if !isXMLNSAttr {
				continue
			}
			if prefix == "" {
				if attr.LocalName() == "xmlns" {
					return attr.Value()
				}
				continue
			}
			if attr.LocalName() == prefix {
				return attr.Value()
			}
		}
	}

	if schema.NamespaceDecls != nil {
		if prefix == "" {
			if ns, ok := schema.NamespaceDecls[""]; ok {
				return ns
			}
		} else if ns, ok := schema.NamespaceDecls[prefix]; ok {
			return ns
		}
	}

	switch prefix {
	case "xs", "xsd":
		return xsdxml.XSDNamespace
	case "xml":
		return xsdxml.XMLNamespace
	default:
		return ""
	}
}

func namespaceContextForElement(doc *xsdxml.Document, elem xsdxml.NodeID, schema *Schema) map[string]string {
	context := make(map[string]string)
	for current := elem; current != xsdxml.InvalidNode; current = doc.Parent(current) {
		for _, attr := range doc.Attributes(current) {
			ns := attr.NamespaceURI()
			local := attr.LocalName()
			if ns != xsdxml.XMLNSNamespace && (ns != "" || local != "xmlns") {
				continue
			}
			prefix := local
			if prefix == "xmlns" {
				prefix = ""
			}
			if _, exists := context[prefix]; !exists {
				context[prefix] = attr.Value()
			}
		}
	}

	if schema != nil {
		for prefix, uri := range schema.NamespaceDecls {
			if _, exists := context[prefix]; !exists {
				context[prefix] = uri
			}
		}
	}

	if _, exists := context["xml"]; !exists {
		context["xml"] = xsdxml.XMLNamespace
	}

	return context
}

// resolveQName resolves a QName for TYPE references using namespace prefix mappings.
// For unprefixed QNames in XSD type attribute values (type, base, itemType, memberTypes, etc.):
// 1. Built-in XSD types (string, int, etc.) -> XSD namespace
// 2. If a default namespace (xmlns="...") is declared -> use that namespace
// 3. Otherwise -> empty namespace (no namespace)
// This follows the XSD spec's QName resolution rules.
func resolveQName(doc *xsdxml.Document, qname string, elem xsdxml.NodeID, schema *Schema) (types.QName, error) {
	prefix, local, hasPrefix, err := types.ParseQName(qname)
	if err != nil {
		return types.QName{}, err
	}

	var namespace types.NamespaceURI
	if !hasPrefix {
		// no prefix - check if it's a built-in type first
		if types.GetBuiltin(types.TypeName(local)) != nil {
			// built-in type - use XSD namespace
			namespace = types.XSDNamespace
		} else {
			// check for default namespace (xmlns="...") in scope
			defaultNS := namespaceForPrefix(doc, elem, schema, "")
			// if default namespace is XSD namespace, treat as no namespace
			// (XSD types are handled above, non-XSD names in XSD namespace don't exist)
			// this is the strict spec behavior that W3C tests expect.
			if defaultNS == xsdxml.XSDNamespace {
				namespace = ""
			} else if defaultNS != "" {
				namespace = types.NamespaceURI(defaultNS)
			} else {
				// no default namespace - per XSD spec, unprefixed QNames resolve to no namespace
				// (not target namespace). The spec explicitly states that targetNamespace is NOT
				// used implicitly in QName resolution.
				// however, when there's no targetNamespace, types are in no namespace, so
				// unprefixed references naturally resolve to no namespace (which is correct).
				// when there's a targetNamespace, unprefixed references must resolve to no namespace
				// (not target namespace), which makes them invalid if they reference types in
				// the target namespace (unless there's a default namespace binding).
				namespace = types.NamespaceEmpty
			}
		}
	} else {
		namespaceStr := namespaceForPrefix(doc, elem, schema, prefix)
		if namespaceStr == "" {
			return types.QName{}, fmt.Errorf("undefined namespace prefix '%s' in '%s'", prefix, qname)
		}
		namespace = types.NamespaceURI(namespaceStr)
	}

	return types.QName{
		Namespace: namespace,
		Local:     local,
	}, nil
}

// resolveQNameWithoutBuiltin resolves a QName using namespace prefixes without
// applying built-in type shortcuts.
func resolveQNameWithoutBuiltin(doc *xsdxml.Document, qname string, elem xsdxml.NodeID, schema *Schema) (types.QName, error) {
	prefix, local, hasPrefix, err := types.ParseQName(qname)
	if err != nil {
		return types.QName{}, err
	}

	var namespace types.NamespaceURI
	if !hasPrefix {
		// no prefix - check for default namespace (xmlns="...") in scope
		defaultNS := namespaceForPrefix(doc, elem, schema, "")
		// if default namespace is XSD namespace, treat as no namespace
		if defaultNS == xsdxml.XSDNamespace {
			namespace = ""
		} else if defaultNS != "" {
			namespace = types.NamespaceURI(defaultNS)
		} else {
			// no default namespace - per XSD spec, unprefixed QNames resolve to no namespace.
			namespace = types.NamespaceEmpty
		}
	} else {
		namespaceStr := namespaceForPrefix(doc, elem, schema, prefix)
		if namespaceStr == "" {
			return types.QName{}, fmt.Errorf("undefined namespace prefix '%s' in '%s'", prefix, qname)
		}
		namespace = types.NamespaceURI(namespaceStr)
	}

	return types.QName{
		Namespace: namespace,
		Local:     local,
	}, nil
}

// resolveElementQName resolves a QName for ELEMENT references (ref, substitutionGroup).
// Unlike resolveQName for types, this does NOT check for built-in type names
// because element references never refer to built-in types.
func resolveElementQName(doc *xsdxml.Document, qname string, elem xsdxml.NodeID, schema *Schema) (types.QName, error) {
	return resolveQNameWithoutBuiltin(doc, qname, elem, schema)
}

// resolveIdentityConstraintQName resolves a QName for identity constraint references.
// Identity constraints use standard QName resolution without built-in type shortcuts.
func resolveIdentityConstraintQName(doc *xsdxml.Document, qname string, elem xsdxml.NodeID, schema *Schema) (types.QName, error) {
	return resolveQNameWithoutBuiltin(doc, qname, elem, schema)
}

// resolveAttributeRefQName resolves a QName for ATTRIBUTE references.
// QName values in schema attributes use standard XML namespace resolution:
// - Prefixed names use the declared namespace for that prefix
// - Unprefixed names use the default namespace if declared, otherwise no namespace
func resolveAttributeRefQName(doc *xsdxml.Document, qname string, elem xsdxml.NodeID, schema *Schema) (types.QName, error) {
	prefix, local, hasPrefix, err := types.ParseQName(qname)
	if err != nil {
		return types.QName{}, err
	}

	var namespace types.NamespaceURI
	if !hasPrefix {
		// no prefix - check for default namespace (xmlns="...")
		defaultNS := namespaceForPrefix(doc, elem, schema, "")
		// if default namespace is XSD namespace, treat as no namespace
		if defaultNS == xsdxml.XSDNamespace {
			namespace = ""
		} else if defaultNS != "" {
			namespace = types.NamespaceURI(defaultNS)
		}
		// if no default namespace, namespace stays empty
	} else {
		namespaceStr := namespaceForPrefix(doc, elem, schema, prefix)
		if namespaceStr == "" {
			return types.QName{}, fmt.Errorf("undefined namespace prefix '%s' in '%s'", prefix, qname)
		}
		namespace = types.NamespaceURI(namespaceStr)
	}

	return types.QName{
		Namespace: namespace,
		Local:     local,
	}, nil
}

// validateAnnotationOrder checks that annotation (if present) is the first XSD child element.
// Per XSD spec, annotation must appear first in element, attribute, complexType, simpleType, etc.
func validateAnnotationOrder(doc *xsdxml.Document, elem xsdxml.NodeID) error {
	seenNonAnnotation := false
	annotationCount := 0
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}

		if doc.LocalName(child) == "annotation" {
			if seenNonAnnotation {
				return fmt.Errorf("annotation must be first child element, found after other XSD elements")
			}
			annotationCount++
			if annotationCount > 1 {
				return fmt.Errorf("at most one annotation element is allowed")
			}
		} else {
			seenNonAnnotation = true
		}
	}
	return nil
}

// validateElementChildrenOrder checks that identity constraints follow any inline type definition.
// Per XSD 1.0, element content model is: (annotation?, (simpleType|complexType)?, (unique|key|keyref)*).
func validateElementChildrenOrder(doc *xsdxml.Document, elem xsdxml.NodeID) error {
	seenType := false
	seenConstraint := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		switch doc.LocalName(child) {
		case "annotation":
			continue
		case "simpleType", "complexType":
			if seenConstraint {
				return fmt.Errorf("element type definition must precede identity constraints")
			}
			if seenType {
				return fmt.Errorf("element cannot have more than one inline type definition")
			}
			seenType = true
		case "unique", "key", "keyref":
			seenConstraint = true
		}
	}
	return nil
}

func validateOnlyAnnotationChildren(doc *xsdxml.Document, elem xsdxml.NodeID, elementName string) error {
	seenAnnotation := false
	for _, child := range doc.Children(elem) {
		if doc.NamespaceURI(child) != xsdxml.XSDNamespace {
			continue
		}
		if doc.LocalName(child) == "annotation" {
			if seenAnnotation {
				return fmt.Errorf("%s: at most one annotation is allowed", elementName)
			}
			seenAnnotation = true
			continue
		}
		return fmt.Errorf("%s: unexpected child element '%s'", elementName, doc.LocalName(child))
	}
	return nil
}
