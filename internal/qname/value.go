package qname

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xmlnames"
)

// ParseQNameValue parses a QName lexical value with namespace resolution.
func ParseQNameValue(lexical string, nsContext map[string]string) (QName, error) {
	trimmed := value.TrimXMLWhitespaceString(lexical)
	if trimmed == "" {
		return QName{}, fmt.Errorf("invalid QName: empty string")
	}

	prefix, local, hasPrefix, err := ParseQName(trimmed)
	if err != nil {
		return QName{}, err
	}

	var ns NamespaceURI
	if hasPrefix {
		if prefix == xmlnames.XMLPrefix {
			resolved, ok := ResolveNamespace(prefix, nsContext)
			if err := xmlnames.ValidateXMLPrefixBinding(resolved.String(), ok); err != nil {
				return QName{}, err
			}
			return QName{Namespace: NamespaceURI(xmlnames.XMLNamespace), Local: local}, nil
		}
		var ok bool
		ns, ok = ResolveNamespace(prefix, nsContext)
		if !ok {
			return QName{}, fmt.Errorf("prefix %s not found in namespace context", prefix)
		}
	} else {
		if defaultNS, ok := ResolveNamespace("", nsContext); ok {
			ns = defaultNS
		}
	}

	return QName{Namespace: ns, Local: local}, nil
}
