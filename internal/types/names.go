package types

import (
	"fmt"
	"strings"
)

// TypeName represents the local name of an XSD type
// Using a typed string prevents mixing type names with other strings
type TypeName string

// Built-in XSD type name constants
const (
	// Complex type
	TypeNameAnyType TypeName = "anyType"

	// Base simple type (base of all simple types)
	TypeNameAnySimpleType TypeName = "anySimpleType"

	// Primitive types (19 total)
	TypeNameString       TypeName = "string"
	TypeNameBoolean      TypeName = "boolean"
	TypeNameDecimal      TypeName = "decimal"
	TypeNameFloat        TypeName = "float"
	TypeNameDouble       TypeName = "double"
	TypeNameDuration     TypeName = "duration"
	TypeNameDateTime     TypeName = "dateTime"
	TypeNameTime         TypeName = "time"
	TypeNameDate         TypeName = "date"
	TypeNameGYearMonth   TypeName = "gYearMonth"
	TypeNameGYear        TypeName = "gYear"
	TypeNameGMonthDay    TypeName = "gMonthDay"
	TypeNameGDay         TypeName = "gDay"
	TypeNameGMonth       TypeName = "gMonth"
	TypeNameHexBinary    TypeName = "hexBinary"
	TypeNameBase64Binary TypeName = "base64Binary"
	TypeNameAnyURI       TypeName = "anyURI"
	TypeNameQName        TypeName = "QName"
	TypeNameNOTATION     TypeName = "NOTATION"

	// Derived string types
	TypeNameNormalizedString TypeName = "normalizedString"
	TypeNameToken            TypeName = "token"
	TypeNameLanguage         TypeName = "language"
	TypeNameName             TypeName = "Name"
	TypeNameNCName           TypeName = "NCName"
	TypeNameID               TypeName = "ID"
	TypeNameIDREF            TypeName = "IDREF"
	TypeNameIDREFS           TypeName = "IDREFS"
	TypeNameENTITY           TypeName = "ENTITY"
	TypeNameENTITIES         TypeName = "ENTITIES"
	TypeNameNMTOKEN          TypeName = "NMTOKEN"
	TypeNameNMTOKENS         TypeName = "NMTOKENS"

	// Derived numeric types
	TypeNameInteger            TypeName = "integer"
	TypeNameLong               TypeName = "long"
	TypeNameInt                TypeName = "int"
	TypeNameShort              TypeName = "short"
	TypeNameByte               TypeName = "byte"
	TypeNameNonNegativeInteger TypeName = "nonNegativeInteger"
	TypeNamePositiveInteger    TypeName = "positiveInteger"
	TypeNameUnsignedLong       TypeName = "unsignedLong"
	TypeNameUnsignedInt        TypeName = "unsignedInt"
	TypeNameUnsignedShort      TypeName = "unsignedShort"
	TypeNameUnsignedByte       TypeName = "unsignedByte"
	TypeNameNegativeInteger    TypeName = "negativeInteger"
	TypeNameNonPositiveInteger TypeName = "nonPositiveInteger"
)

// String returns the string representation of the type name
func (tn TypeName) String() string {
	return string(tn)
}

// NamespaceURI represents a namespace URI
// This is a newtype over string to provide type safety for namespace URIs
type NamespaceURI string

// NamespaceEmpty represents an empty namespace URI (no namespace)
const NamespaceEmpty NamespaceURI = ""

// String returns the namespace URI as a string
func (ns NamespaceURI) String() string {
	return string(ns)
}

// IsEmpty returns true if the namespace URI is empty
func (ns NamespaceURI) IsEmpty() bool {
	return ns == NamespaceEmpty
}

// Equal returns true if two namespace URIs are equal
func (ns NamespaceURI) Equal(other NamespaceURI) bool {
	return ns == other
}

// ResolveNamespace looks up a prefix in a namespace context map.
func ResolveNamespace(prefix string, context map[string]string) (NamespaceURI, bool) {
	if context == nil {
		return NamespaceEmpty, false
	}
	uri, ok := context[prefix]
	if !ok {
		return NamespaceEmpty, false
	}
	return NamespaceURI(uri), true
}

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

// SplitQName splits a QName string into prefix/local without schemacheck.
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

// IsValidNCName returns true if the string is a valid NCName (non-colonized name)
// NCName must not be empty, must not contain colons, must start with a NameStartChar,
// and subsequent characters must be NameChars (XML 1.0 spec)
func IsValidNCName(s string) bool {
	return validateNCName(s) == nil
}

// IsValidQName returns true if the string is a valid QName.
// QName must not be empty, may contain at most one colon, and each part must be a valid NCName.
func IsValidQName(s string) bool {
	return validateQName(s) == nil
}
