package parser

import (
	"fmt"

	qnamelex "github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
)

type defaultNamespacePolicy uint8

const (
	useDefaultNamespace defaultNamespacePolicy = iota
	forceEmptyNamespace
)

func resolveQNameWithPolicy(
	doc *xsdxml.Document,
	qname string,
	elem xsdxml.NodeID,
	schema *Schema,
	policy defaultNamespacePolicy,
) (types.QName, error) {
	prefix, local, hasPrefix, err := qnamelex.ParseQName(qname)
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
