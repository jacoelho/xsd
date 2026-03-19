package xsd

import (
	"sync"

	"github.com/jacoelho/xsd/internal/qname"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/validator"
)

// Schema wraps a compiled runtime schema with convenience methods.
type Schema struct {
	rt                   *runtime.Schema
	defaultValidator     *Validator
	validateDefaults     resolvedValidateOptions
	defaultValidatorOnce sync.Once
}

// Validator validates XML documents against one compiled schema.
type Validator struct {
	engine *validator.Engine
}

// QName is a public qualified name with namespace and local part.
type QName = qname.QName

func newSchema(rt *runtime.Schema, validateDefaults resolvedValidateOptions) *Schema {
	if rt == nil {
		return &Schema{}
	}
	return &Schema{
		rt:               rt,
		validateDefaults: validateDefaults,
	}
}
