package xsd

import (
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator"
)

// Schema wraps a compiled runtime schema with convenience methods.
type Schema struct {
	rt               *runtime.Schema
	defaultEngine    *validator.Engine
	validateDefaults resolvedValidateOptions
}

// Validator validates XML documents against one compiled schema and is safe for concurrent use.
type Validator struct {
	engine *validator.Engine
}

// NamespaceURI identifies an XML namespace URI.
type NamespaceURI string

// LocalName identifies a local XML name.
type LocalName string

// Name is a public namespace-qualified name.
type Name struct {
	Namespace NamespaceURI
	Local     LocalName
}

// IsZero reports whether the name has no namespace and no local part.
func (n Name) IsZero() bool {
	return n.Namespace == "" && n.Local == ""
}

// String formats the name as local or {namespace}local.
func (n Name) String() string {
	if n.Namespace == "" {
		return string(n.Local)
	}
	return "{" + string(n.Namespace) + "}" + string(n.Local)
}

func newSchema(rt *runtime.Schema, validateDefaults resolvedValidateOptions) *Schema {
	if rt == nil {
		return &Schema{}
	}
	return &Schema{
		rt:               rt,
		defaultEngine:    validator.NewEngine(rt, validateDefaults.instanceParseOptions...),
		validateDefaults: validateDefaults,
	}
}
