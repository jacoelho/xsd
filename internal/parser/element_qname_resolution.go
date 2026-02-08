package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

type defaultNamespacePolicy uint8

const (
	useDefaultNamespace defaultNamespacePolicy = iota
	forceEmptyNamespace
)

// resolveQName resolves a QName for TYPE references using namespace prefix mappings.
// For unprefixed QNames in XSD type attribute values (type, base, itemType, memberTypes, etc.):
// 1. If a default namespace (xmlns="...") is declared -> use that namespace
// 2. Otherwise -> empty namespace (no namespace)
// This follows the XSD spec's QName resolution rules.
func resolveQName(doc *xsdxml.Document, qname string, elem xsdxml.NodeID, schema *Schema) (types.QName, error) {
	return resolveQNameWithPolicy(doc, qname, elem, schema, useDefaultNamespace)
}

// resolveElementQName resolves a QName for ELEMENT references (ref, substitutionGroup).
// Element references use the same namespace resolution rules as type references.
func resolveElementQName(doc *xsdxml.Document, qname string, elem xsdxml.NodeID, schema *Schema) (types.QName, error) {
	return resolveQName(doc, qname, elem, schema)
}

// resolveIdentityConstraintQName resolves a QName for identity constraint references.
// Identity constraints use standard QName resolution.
func resolveIdentityConstraintQName(doc *xsdxml.Document, qname string, elem xsdxml.NodeID, schema *Schema) (types.QName, error) {
	return resolveQName(doc, qname, elem, schema)
}

// resolveAttributeRefQName resolves a QName for ATTRIBUTE references.
// Per XSD, unprefixed attribute references are in no namespace (ignore default namespaces).
func resolveAttributeRefQName(doc *xsdxml.Document, qname string, elem xsdxml.NodeID, schema *Schema) (types.QName, error) {
	return resolveQNameWithPolicy(doc, qname, elem, schema, forceEmptyNamespace)
}

func resolveQNameWithPolicy(
	doc *xsdxml.Document,
	qname string,
	elem xsdxml.NodeID,
	schema *Schema,
	policy defaultNamespacePolicy,
) (types.QName, error) {
	prefix, local, hasPrefix, err := types.ParseQName(qname)
	if err != nil {
		return types.QName{}, err
	}

	namespace, err := namespaceFromPrefixPolicy(doc, elem, schema, qname, prefix, hasPrefix, policy)
	if err != nil {
		return types.QName{}, err
	}
	if err := validateQNameNamespace(schema, namespace); err != nil {
		return types.QName{}, err
	}
	return types.QName{Namespace: namespace, Local: local}, nil
}

func namespaceFromPrefixPolicy(
	doc *xsdxml.Document,
	elem xsdxml.NodeID,
	schema *Schema,
	qname string,
	prefix string,
	hasPrefix bool,
	policy defaultNamespacePolicy,
) (types.NamespaceURI, error) {
	if !hasPrefix {
		if policy == forceEmptyNamespace {
			return types.NamespaceEmpty, nil
		}
		defaultNS := namespaceForPrefix(doc, elem, schema, "")
		if defaultNS != "" {
			return types.NamespaceURI(defaultNS), nil
		}
		return types.NamespaceEmpty, nil
	}

	namespaceStr := namespaceForPrefix(doc, elem, schema, prefix)
	if namespaceStr == "" {
		return types.NamespaceURI(""), fmt.Errorf("undefined namespace prefix '%s' in '%s'", prefix, qname)
	}
	return types.NamespaceURI(namespaceStr), nil
}
