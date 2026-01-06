package parser

import (
	"fmt"
	"strings"

	xsdschema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// makeAnyType creates an anyType ComplexType.
// Per XSD 1.0 spec, anyType has mixed content allowing any elements and text.
func makeAnyType() types.Type {
	ct := &types.ComplexType{
		QName: types.QName{
			Namespace: xml.XSDNamespace,
			Local:     "anyType",
		},
	}
	ct.SetContent(&types.EmptyContent{})
	ct.SetMixed(true)
	return ct
}

// parseTopLevelElement parses a top-level element declaration
func parseTopLevelElement(elem xml.Element, schema *xsdschema.Schema) error {
	name := getAttr(elem, "name")
	if name == "" {
		return fmt.Errorf("element missing name attribute")
	}

	validAttributes := map[string]bool{
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
	if err := validateElementAttributes(elem, validAttributes, "top-level element"); err != nil {
		return err
	}

	if hasIDAttribute(elem) {
		idAttr := elem.GetAttribute("id")
		if err := validateIDAttribute(idAttr, "element", schema); err != nil {
			return err
		}
	}

	// 'form' attribute only applies to local element declarations
	if elem.HasAttribute("form") {
		return fmt.Errorf("top-level element cannot have 'form' attribute")
	}

	// Validate annotation order: if present, must be first child
	if err := validateAnnotationOrder(elem); err != nil {
		return err
	}
	if err := validateElementChildrenOrder(elem); err != nil {
		return err
	}

	for _, child := range elem.Children() {
		if child.NamespaceURI() != xml.XSDNamespace {
			continue
		}
		switch child.LocalName() {
		case "annotation", "complexType", "simpleType", "key", "keyref", "unique":
			// Allowed.
		default:
			return fmt.Errorf("invalid child element <%s> in <element> declaration", child.LocalName())
		}
	}

	// Validate: cannot have both default and fixed
	if elem.GetAttribute("default") != "" && elem.GetAttribute("fixed") != "" {
		return fmt.Errorf("element cannot have both 'default' and 'fixed' attributes")
	}

	var hasInlineType bool
	for _, child := range elem.Children() {
		if child.NamespaceURI() == xml.XSDNamespace {
			if child.LocalName() == "complexType" || child.LocalName() == "simpleType" {
				hasInlineType = true
				break
			}
		}
	}

	// Validate: cannot have both type attribute and inline type definition
	if elem.GetAttribute("type") != "" && hasInlineType {
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
		Form:            types.FormQualified, // Global elements are always qualified
	}

	if typeName := elem.GetAttribute("type"); typeName != "" {
		typeQName, err := resolveQName(typeName, elem, schema)
		if err != nil {
			return fmt.Errorf("resolve type %s: %w", typeName, err)
		}

		// Check if it's a built-in type
		if builtinType := types.GetBuiltinNS(typeQName.Namespace, typeQName.Local); builtinType != nil {
			// anyType is special - it's a complex type, not a simple type
			if typeQName.Local == "anyType" {
				decl.Type = makeAnyType()
			} else {
				decl.Type = builtinType
			}
		} else {
			// Will be resolved later in a second pass
			// For now, create a placeholder
			decl.Type = &types.SimpleType{
				QName: typeQName,
			}
		}
	} else {
		for _, child := range elem.Children() {
			if child.NamespaceURI() != xml.XSDNamespace {
				continue
			}

			switch child.LocalName() {
			case "complexType":
				if child.GetAttribute("name") != "" {
					return fmt.Errorf("inline complexType cannot have 'name' attribute")
				}
				ct, err := parseInlineComplexType(child, schema)
				if err != nil {
					return fmt.Errorf("parse inline complexType: %w", err)
				}
				decl.Type = ct
			case "simpleType":
				if child.GetAttribute("name") != "" {
					return fmt.Errorf("inline simpleType cannot have 'name' attribute")
				}
				st, err := parseInlineSimpleType(child, schema)
				if err != nil {
					return fmt.Errorf("parse inline simpleType: %w", err)
				}
				decl.Type = st
			}
		}
		// If no inline type was found, default to anyType
		if decl.Type == nil {
			decl.Type = makeAnyType()
		}
	}

	if ok, value, err := parseBoolAttribute(elem, "nillable"); err != nil {
		return err
	} else if ok {
		decl.Nillable = value
	}

	if ok, value, err := parseBoolAttribute(elem, "abstract"); err != nil {
		return err
	} else if ok {
		decl.Abstract = value
	}

	if defaultVal := elem.GetAttribute("default"); defaultVal != "" {
		decl.Default = defaultVal
	}

	// fixed attribute may have an empty value (fixed=""), so check for presence
	if elem.HasAttribute("fixed") {
		decl.Fixed = elem.GetAttribute("fixed")
		decl.HasFixed = true
	}

	// Parse block attribute (space-separated list: substitution, extension, restriction, #all)
	if elem.HasAttribute("block") {
		blockAttr := elem.GetAttribute("block")
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

	// Parse final attribute (space-separated list: extension, restriction, #all).
	// Element final does not allow substitution; W3C tests (elemF004/006/007/008) expect invalid.
	if elem.HasAttribute("final") {
		finalAttr := elem.GetAttribute("final")
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

	if subGroup := elem.GetAttribute("substitutionGroup"); subGroup != "" {
		// Use resolveElementQName for element references (not type references)
		subGroupQName, err := resolveElementQName(subGroup, elem, schema)
		if err != nil {
			return fmt.Errorf("resolve substitutionGroup %s: %w", subGroup, err)
		}
		decl.SubstitutionGroup = subGroupQName

		// Add to schema's substitution groups map
		if schema.SubstitutionGroups[subGroupQName] == nil {
			schema.SubstitutionGroups[subGroupQName] = []types.QName{}
		}
		schema.SubstitutionGroups[subGroupQName] = append(schema.SubstitutionGroups[subGroupQName], decl.Name)
	}

	// Parse identity constraints (key, keyref, unique)
	for _, child := range elem.Children() {
		if child.NamespaceURI() != xml.XSDNamespace {
			continue
		}

		switch child.LocalName() {
		case "key", "keyref", "unique":
			constraint, err := parseIdentityConstraint(child, schema)
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
func parseElement(elem xml.Element, schema *xsdschema.Schema) (*types.ElementDecl, error) {
	ref := elem.GetAttribute("ref")
	name := elem.GetAttribute("name")

	if hasIDAttribute(elem) {
		idAttr := elem.GetAttribute("id")
		if err := validateIDAttribute(idAttr, "element", schema); err != nil {
			return nil, err
		}
	}

	// Validate: cannot have both name and ref
	if ref != "" && name != "" {
		return nil, fmt.Errorf("element cannot have both 'name' and 'ref' attributes")
	}

	// Check if it's a reference
	if ref != "" {
		validAttributes := map[string]bool{
			"id":        true,
			"ref":       true,
			"minOccurs": true,
			"maxOccurs": true,
		}
		if err := validateElementAttributes(elem, validAttributes, "element reference"); err != nil {
			return nil, err
		}
		if err := validateOnlyAnnotationChildren(elem, "element"); err != nil {
			return nil, err
		}
		// Validate: element references cannot have certain attributes
		// Per XSD spec, when ref is present, these are forbidden:
		// name, type, default, fixed, nillable, block, form, substitutionGroup, abstract
		if elem.HasAttribute("type") {
			return nil, fmt.Errorf("element reference cannot have 'type' attribute")
		}
		if elem.HasAttribute("default") {
			return nil, fmt.Errorf("element reference cannot have 'default' attribute")
		}
		if elem.HasAttribute("fixed") {
			return nil, fmt.Errorf("element reference cannot have 'fixed' attribute")
		}
		if elem.HasAttribute("nillable") {
			return nil, fmt.Errorf("element reference cannot have 'nillable' attribute")
		}
		if elem.HasAttribute("block") {
			return nil, fmt.Errorf("element reference cannot have 'block' attribute")
		}
		if elem.HasAttribute("form") {
			return nil, fmt.Errorf("element reference cannot have 'form' attribute")
		}
		if elem.HasAttribute("abstract") {
			return nil, fmt.Errorf("element reference cannot have 'abstract' attribute")
		}

		// For element references, use resolveElementQName (doesn't check built-in types)
		refQName, err := resolveElementQName(ref, elem, schema)
		if err != nil {
			return nil, fmt.Errorf("resolve ref %s: %w", ref, err)
		}

		minOccurs, err := parseOccursAttr(elem, "minOccurs", 1)
		if err != nil {
			return nil, err
		}
		maxOccurs, err := parseOccursAttr(elem, "maxOccurs", 1)
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

	// Local element declaration
	if name == "" {
		return nil, fmt.Errorf("element missing name and ref")
	}

	validAttributes := map[string]bool{
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
	if err := validateElementAttributes(elem, validAttributes, "local element"); err != nil {
		return nil, err
	}

	if err := validateAnnotationOrder(elem); err != nil {
		return nil, err
	}
	if err := validateElementChildrenOrder(elem); err != nil {
		return nil, err
	}

	for _, child := range elem.Children() {
		if child.NamespaceURI() != xml.XSDNamespace {
			continue
		}
		switch child.LocalName() {
		case "annotation", "complexType", "simpleType", "key", "keyref", "unique":
			// Allowed.
		default:
			return nil, fmt.Errorf("invalid child element <%s> in <element> declaration", child.LocalName())
		}
	}

	// Validate: abstract attribute is only allowed on global elements
	// Per XSD spec, local elements cannot have the abstract attribute
	if elem.HasAttribute("abstract") {
		return nil, fmt.Errorf("local element cannot have 'abstract' attribute (only global elements can be abstract)")
	}

	// Validate: final attribute is only allowed on global elements
	if elem.HasAttribute("final") {
		return nil, fmt.Errorf("local element cannot have 'final' attribute (only global elements can be final)")
	}

	// Validate: cannot have both default and fixed
	if elem.GetAttribute("default") != "" && elem.GetAttribute("fixed") != "" {
		return nil, fmt.Errorf("element cannot have both 'default' and 'fixed' attributes")
	}

	var hasInlineType bool
	for _, child := range elem.Children() {
		if child.NamespaceURI() == xml.XSDNamespace {
			if child.LocalName() == "complexType" || child.LocalName() == "simpleType" {
				hasInlineType = true
				break
			}
		}
	}

	// Validate: cannot have both type attribute and inline type definition
	if elem.GetAttribute("type") != "" && hasInlineType {
		return nil, fmt.Errorf("element cannot have both 'type' attribute and inline type definition")
	}

	// Per XSD spec: local elements use the form attribute if present,
	// otherwise use schema's elementFormDefault (which defaults to unqualified)
	var effectiveForm xsdschema.Form
	if formAttr := elem.GetAttribute("form"); formAttr != "" {
		switch formAttr {
		case "qualified":
			effectiveForm = xsdschema.Qualified
		case "unqualified":
			effectiveForm = xsdschema.Unqualified
		default:
			return nil, fmt.Errorf("invalid form attribute value '%s': must be 'qualified' or 'unqualified'", formAttr)
		}
	} else {
		effectiveForm = schema.ElementFormDefault
	}

	// For unqualified elements, the namespace is empty (no namespace)
	// For qualified elements, the namespace is the target namespace
	var elementNamespace types.NamespaceURI
	if effectiveForm == xsdschema.Qualified {
		elementNamespace = schema.TargetNamespace
	}

	decl := &types.ElementDecl{
		Name: types.QName{
			Namespace: elementNamespace,
			Local:     name,
		},
		SourceNamespace: schema.TargetNamespace,
	}
	minOccurs, err := parseOccursAttr(elem, "minOccurs", 1)
	if err != nil {
		return nil, err
	}
	maxOccurs, err := parseOccursAttr(elem, "maxOccurs", 1)
	if err != nil {
		return nil, err
	}
	decl.MinOccurs = minOccurs
	decl.MaxOccurs = maxOccurs

	if effectiveForm == xsdschema.Qualified {
		decl.Form = types.FormQualified
	} else {
		decl.Form = types.FormUnqualified
	}

	if typeName := elem.GetAttribute("type"); typeName != "" {
		typeQName, err := resolveQName(typeName, elem, schema)
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
		for _, child := range elem.Children() {
			if child.NamespaceURI() != xml.XSDNamespace {
				continue
			}

			switch child.LocalName() {
			case "complexType":
				if child.GetAttribute("name") != "" {
					return nil, fmt.Errorf("inline complexType cannot have 'name' attribute")
				}
				ct, err := parseInlineComplexType(child, schema)
				if err != nil {
					return nil, fmt.Errorf("parse inline complexType: %w", err)
				}
				decl.Type = ct
			case "simpleType":
				if child.GetAttribute("name") != "" {
					return nil, fmt.Errorf("inline simpleType cannot have 'name' attribute")
				}
				st, err := parseInlineSimpleType(child, schema)
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

	if ok, value, err := parseBoolAttribute(elem, "nillable"); err != nil {
		return nil, err
	} else if ok {
		decl.Nillable = value
	}

	if defaultVal := elem.GetAttribute("default"); defaultVal != "" {
		decl.Default = defaultVal
	}

	// fixed attribute may have an empty value (fixed=""), so check for presence
	if elem.HasAttribute("fixed") {
		decl.Fixed = elem.GetAttribute("fixed")
		decl.HasFixed = true
	}

	// Parse block attribute (space-separated list: substitution, extension, restriction, #all)
	if elem.HasAttribute("block") {
		blockAttr := elem.GetAttribute("block")
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

	// Parse identity constraints (key, keyref, unique) for local elements
	for _, child := range elem.Children() {
		if child.NamespaceURI() != xml.XSDNamespace {
			continue
		}

		switch child.LocalName() {
		case "key", "keyref", "unique":
			constraint, err := parseIdentityConstraint(child, schema)
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

func validateElementAttributes(elem xml.Element, validAttributes map[string]bool, context string) error {
	for _, attr := range elem.Attributes() {
		if attr.NamespaceURI() == xml.XMLNSNamespace || attr.NamespaceURI() == "xmlns" || attr.LocalName() == "xmlns" {
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

func namespaceForPrefix(elem xml.Element, schema *xsdschema.Schema, prefix string) string {
	for current := elem; current != nil; current = current.Parent() {
		for _, attr := range current.Attributes() {
			isXMLNSAttr := attr.NamespaceURI() == xml.XMLNSNamespace ||
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
		return xml.XSDNamespace
	case "xml":
		return xml.XMLNamespace
	default:
		return ""
	}
}

func namespaceContextForElement(elem xml.Element, schema *xsdschema.Schema) map[string]string {
	context := make(map[string]string)
	for current := elem; current != nil; current = current.Parent() {
		for _, attr := range current.Attributes() {
			if attr.NamespaceURI() != xml.XMLNSNamespace && !(attr.NamespaceURI() == "" && attr.LocalName() == "xmlns") {
				continue
			}
			prefix := attr.LocalName()
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
		context["xml"] = xml.XMLNamespace
	}

	return context
}

// resolveQName resolves a QName for TYPE references using namespace prefix mappings.
// For unprefixed QNames in XSD type attribute values (type, base, itemType, memberTypes, etc.):
// 1. Built-in XSD types (string, int, etc.) -> XSD namespace
// 2. If a default namespace (xmlns="...") is declared -> use that namespace
// 3. Otherwise -> empty namespace (no namespace)
// This follows the XSD spec's QName resolution rules.
func resolveQName(qname string, elem xml.Element, schema *xsdschema.Schema) (types.QName, error) {
	if qname == "" {
		return types.QName{}, fmt.Errorf("empty qname")
	}

	// Trim whitespace from the QName value per XSD spec
	qname = strings.TrimSpace(qname)
	if !types.IsValidQName(qname) {
		return types.QName{}, fmt.Errorf("invalid QName '%s'", qname)
	}

	prefix := ""
	local := qname
	for i, r := range qname {
		if r == ':' {
			prefix = strings.TrimSpace(qname[:i])
			local = strings.TrimSpace(qname[i+1:])
			break
		}
	}

	var namespace types.NamespaceURI
	if prefix == "" {
		// No prefix - check if it's a built-in type first
		if types.GetBuiltin(types.TypeName(local)) != nil {
			// Built-in type - use XSD namespace
			namespace = types.XSDNamespace
		} else {
			// Check for default namespace (xmlns="...") in scope
			defaultNS := namespaceForPrefix(elem, schema, "")
			// If default namespace is XSD namespace, treat as no namespace
			// (XSD types are handled above, non-XSD names in XSD namespace don't exist)
			// This is the strict spec behavior that W3C tests expect.
			if defaultNS == xml.XSDNamespace {
				namespace = ""
			} else if defaultNS != "" {
				namespace = types.NamespaceURI(defaultNS)
			} else {
				// No default namespace - per XSD spec, unprefixed QNames resolve to no namespace
				// (not target namespace). The spec explicitly states that targetNamespace is NOT
				// used implicitly in QName resolution.
				// However, when there's no targetNamespace, types are in no namespace, so
				// unprefixed references naturally resolve to no namespace (which is correct).
				// When there's a targetNamespace, unprefixed references must resolve to no namespace
				// (not target namespace), which makes them invalid if they reference types in
				// the target namespace (unless there's a default namespace binding).
				namespace = types.NamespaceEmpty
			}
		}
	} else {
		namespaceStr := namespaceForPrefix(elem, schema, prefix)
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
func resolveQNameWithoutBuiltin(qname string, elem xml.Element, schema *xsdschema.Schema) (types.QName, error) {
	if qname == "" {
		return types.QName{}, fmt.Errorf("empty qname")
	}

	// Trim whitespace from the QName value per XSD spec
	qname = strings.TrimSpace(qname)
	if !types.IsValidQName(qname) {
		return types.QName{}, fmt.Errorf("invalid QName '%s'", qname)
	}

	prefix := ""
	local := qname
	for i, r := range qname {
		if r == ':' {
			prefix = strings.TrimSpace(qname[:i])
			local = strings.TrimSpace(qname[i+1:])
			break
		}
	}

	var namespace types.NamespaceURI
	if prefix == "" {
		// No prefix - check for default namespace (xmlns="...") in scope
		defaultNS := namespaceForPrefix(elem, schema, "")
		// If default namespace is XSD namespace, treat as no namespace
		if defaultNS == xml.XSDNamespace {
			namespace = ""
		} else if defaultNS != "" {
			namespace = types.NamespaceURI(defaultNS)
		} else {
			// No default namespace - per XSD spec, unprefixed QNames resolve to no namespace.
			namespace = types.NamespaceEmpty
		}
	} else {
		namespaceStr := namespaceForPrefix(elem, schema, prefix)
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
func resolveElementQName(qname string, elem xml.Element, schema *xsdschema.Schema) (types.QName, error) {
	return resolveQNameWithoutBuiltin(qname, elem, schema)
}

// resolveIdentityConstraintQName resolves a QName for identity constraint references.
// Identity constraints use standard QName resolution without built-in type shortcuts.
func resolveIdentityConstraintQName(qname string, elem xml.Element, schema *xsdschema.Schema) (types.QName, error) {
	return resolveQNameWithoutBuiltin(qname, elem, schema)
}

// resolveAttributeRefQName resolves a QName for ATTRIBUTE references.
// QName values in schema attributes use standard XML namespace resolution:
// - Prefixed names use the declared namespace for that prefix
// - Unprefixed names use the default namespace if declared, otherwise no namespace
func resolveAttributeRefQName(qname string, elem xml.Element, schema *xsdschema.Schema) (types.QName, error) {
	if qname == "" {
		return types.QName{}, fmt.Errorf("empty qname")
	}

	// Trim whitespace from the QName value per XSD spec
	qname = strings.TrimSpace(qname)
	if !types.IsValidQName(qname) {
		return types.QName{}, fmt.Errorf("invalid QName '%s'", qname)
	}

	prefix := ""
	local := qname
	for i, r := range qname {
		if r == ':' {
			prefix = strings.TrimSpace(qname[:i])
			local = strings.TrimSpace(qname[i+1:])
			break
		}
	}

	var namespace types.NamespaceURI
	if prefix == "" {
		// No prefix - check for default namespace (xmlns="...")
		defaultNS := namespaceForPrefix(elem, schema, "")
		// If default namespace is XSD namespace, treat as no namespace
		if defaultNS == xml.XSDNamespace {
			namespace = ""
		} else if defaultNS != "" {
			namespace = types.NamespaceURI(defaultNS)
		}
		// If no default namespace, namespace stays empty
	} else {
		namespaceStr := namespaceForPrefix(elem, schema, prefix)
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
func validateAnnotationOrder(elem xml.Element) error {
	seenNonAnnotation := false
	annotationCount := 0
	for _, child := range elem.Children() {
		if child.NamespaceURI() != xml.XSDNamespace {
			continue
		}

		if child.LocalName() == "annotation" {
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
func validateElementChildrenOrder(elem xml.Element) error {
	seenType := false
	seenConstraint := false
	for _, child := range elem.Children() {
		if child.NamespaceURI() != xml.XSDNamespace {
			continue
		}
		switch child.LocalName() {
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

func validateOnlyAnnotationChildren(elem xml.Element, elementName string) error {
	seenAnnotation := false
	for _, child := range elem.Children() {
		if child.NamespaceURI() != xml.XSDNamespace {
			continue
		}
		if child.LocalName() == "annotation" {
			if seenAnnotation {
				return fmt.Errorf("%s: at most one annotation is allowed", elementName)
			}
			seenAnnotation = true
			continue
		}
		return fmt.Errorf("%s: unexpected child element '%s'", elementName, child.LocalName())
	}
	return nil
}
