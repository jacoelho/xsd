package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

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

func validateElementConstraints(doc *xsdxml.Document, elem xsdxml.NodeID, elementName string, schema *Schema) error {
	if hasIDAttribute(doc, elem) {
		idAttr := doc.GetAttribute(elem, "id")
		if err := validateIDAttribute(idAttr, elementName, schema); err != nil {
			return err
		}
	}
	if err := validateOnlyAnnotationChildren(doc, elem, elementName); err != nil {
		return err
	}
	return nil
}
