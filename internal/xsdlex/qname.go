package xsdlex

import (
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/value"
)

// NamespaceURI represents a namespace URI.
type NamespaceURI = string

// NamespaceEmpty represents an empty namespace URI (no namespace).
const NamespaceEmpty NamespaceURI = ""

// ResolveNamespace looks up a prefix in a namespace context map.
func ResolveNamespace(prefix string, context map[string]string) (NamespaceURI, bool) {
	if context == nil {
		return NamespaceEmpty, false
	}
	uri, ok := context[prefix]
	if !ok {
		return NamespaceEmpty, false
	}
	return uri, true
}

// QName represents a qualified name with namespace and local part.
type QName struct {
	Namespace NamespaceURI
	Local     string
}

// Is reports whether the QName matches the namespace and local name.
func (q QName) Is(namespace, local string) bool {
	return q.Namespace == namespace && q.Local == local
}

// HasLocal reports whether the local name matches, ignoring namespace.
func (q QName) HasLocal(local string) bool {
	return q.Local == local
}

// String returns the QName in {namespace}local format, or just local if no namespace.
func (q QName) String() string {
	if q.Namespace == NamespaceEmpty {
		return q.Local
	}
	return "{" + q.Namespace + "}" + q.Local
}

// IsZero returns true if the QName is the zero value.
func (q QName) IsZero() bool {
	return q.Namespace == NamespaceEmpty && q.Local == ""
}

// Equal returns true if two QNames are equal.
func (q QName) Equal(other QName) bool {
	return q.Namespace == other.Namespace && q.Local == other.Local
}

// SplitQName splits a QName string into prefix/local without validation.
func SplitQName(name string) (prefix, local string, hasPrefix bool) {
	prefix, local, hasPrefix = strings.Cut(name, ":")
	if !hasPrefix {
		return "", name, false
	}
	return prefix, local, true
}

// ParseQName trims and validates a QName, returning prefix/local parts.
func ParseQName(name string) (prefix, local string, hasPrefix bool, err error) {
	trimmed := value.TrimXMLWhitespaceString(name)
	if trimmed == "" {
		return "", "", false, fmt.Errorf("empty qname")
	}
	if !IsValidQName(trimmed) {
		return "", "", false, fmt.Errorf("invalid QName '%s'", trimmed)
	}
	prefix, local, hasPrefix = SplitQName(trimmed)
	prefix = value.TrimXMLWhitespaceString(prefix)
	local = value.TrimXMLWhitespaceString(local)
	return prefix, local, hasPrefix, nil
}

// IsValidNCName returns true if the string is a valid NCName.
func IsValidNCName(s string) bool {
	return value.ValidateNCName([]byte(s)) == nil
}

// IsValidQName returns true if the string is a valid QName.
func IsValidQName(s string) bool {
	return value.ValidateQName([]byte(s)) == nil
}

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
		if prefix == value.XMLPrefix {
			resolved, ok := ResolveNamespace(prefix, nsContext)
			if err := value.ValidateXMLPrefixBinding(resolved, ok); err != nil {
				return QName{}, err
			}
			return QName{Namespace: NamespaceURI(value.XMLNamespace), Local: local}, nil
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
