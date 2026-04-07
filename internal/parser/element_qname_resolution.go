package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
)

type defaultNamespacePolicy uint8

const (
	useDefaultNamespace defaultNamespacePolicy = iota
	forceEmptyNamespace
)

func resolveQNameWithPolicy(
	doc *Document,
	rawQName string,
	elem NodeID,
	schema *Schema,
	policy defaultNamespacePolicy,
) (model.QName, error) {
	prefix, local, hasPrefix, err := model.ParseQName(rawQName)
	if err != nil {
		return model.QName{}, err
	}

	namespace, err := namespaceFromPrefixPolicy(doc, elem, schema, rawQName, prefix, hasPrefix, policy)
	if err != nil {
		return model.QName{}, err
	}
	if err := validateQNameNamespace(schema, namespace); err != nil {
		return model.QName{}, err
	}
	return model.QName{Namespace: namespace, Local: local}, nil
}

func namespaceFromPrefixPolicy(
	doc *Document,
	elem NodeID,
	schema *Schema,
	rawQName string,
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
		return model.NamespaceURI(""), fmt.Errorf("undefined namespace prefix '%s' in '%s'", prefix, rawQName)
	}
	return namespaceStr, nil
}
