package types

import (
	"fmt"
	"strings"
)

// QName represents a qualified name with namespace and local part
type QName struct {
	Namespace NamespaceURI
	Local     string
}

// String returns the QName in {namespace}local format, or just local if no namespace
func (q QName) String() string {
	if q.Namespace.IsEmpty() {
		return q.Local
	}
	return "{" + q.Namespace.String() + "}" + q.Local
}

// IsZero returns true if the QName is the zero value
func (q QName) IsZero() bool {
	return q.Namespace.IsEmpty() && q.Local == ""
}

// Equal returns true if two QNames are equal
func (q QName) Equal(other QName) bool {
	return q.Namespace == other.Namespace && q.Local == other.Local
}

// SplitQName splits a QName string into prefix/local without validation.
// The caller is responsible for trimming and validating the input.
func SplitQName(name string) (prefix, local string, hasPrefix bool) {
	prefix, local, hasPrefix = strings.Cut(name, ":")
	if !hasPrefix {
		return "", name, false
	}
	return prefix, local, true
}

// ParseQName trims and validates a QName, returning prefix/local parts.
func ParseQName(name string) (prefix, local string, hasPrefix bool, err error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", "", false, fmt.Errorf("empty qname")
	}
	if !IsValidQName(trimmed) {
		return "", "", false, fmt.Errorf("invalid QName '%s'", trimmed)
	}
	prefix, local, hasPrefix = SplitQName(trimmed)
	prefix = strings.TrimSpace(prefix)
	local = strings.TrimSpace(local)
	return prefix, local, hasPrefix, nil
}
