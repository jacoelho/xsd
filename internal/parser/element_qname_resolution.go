package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	qnamelex "github.com/jacoelho/xsd/internal/qname"
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
) (model.QName, error) {
	prefix, local, hasPrefix, err := qnamelex.ParseQName(qname)
	if err != nil {
		return model.QName{}, err
	}

	namespace, err := namespaceFromPrefixPolicy(doc, elem, schema, qname, prefix, hasPrefix, policy)
	if err != nil {
		return model.QName{}, err
	}
	if err := validateQNameNamespace(schema, namespace); err != nil {
		return model.QName{}, err
	}
	return model.QName{Namespace: namespace, Local: local}, nil
}

func namespaceFromPrefixPolicy(
	doc *xsdxml.Document,
	elem xsdxml.NodeID,
	schema *Schema,
	qname string,
	prefix string,
	hasPrefix bool,
	policy defaultNamespacePolicy,
) (model.NamespaceURI, error) {
	if !hasPrefix {
		if policy == forceEmptyNamespace {
			return model.NamespaceEmpty, nil
		}
		defaultNS := namespaceForPrefix(doc, elem, schema, "")
		if defaultNS != "" {
			return defaultNS, nil
		}
		return model.NamespaceEmpty, nil
	}

	namespaceStr := namespaceForPrefix(doc, elem, schema, prefix)
	if namespaceStr == "" {
		return model.NamespaceURI(""), fmt.Errorf("undefined namespace prefix '%s' in '%s'", prefix, qname)
	}
	return namespaceStr, nil
}
