package xsd

import (
	"sync"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator"
)

// Schema wraps a compiled runtime schema with convenience methods.
type Schema struct {
	rt               *runtime.Schema
	defaultValidator func() *Validator
	validateDefaults resolvedValidateOptions
}

// Validator validates XML documents against one compiled schema.
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
	defaultValidator := sync.OnceValue(func() *Validator {
		return newValidator(rt, validateDefaults)
	})
	return &Schema{
		rt:               rt,
		defaultValidator: defaultValidator,
		validateDefaults: validateDefaults,
	}
}
