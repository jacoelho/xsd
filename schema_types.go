package xsd

import (
	"sync"

	"github.com/jacoelho/xsd/internal/model"
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

// QName is a public qualified name with namespace and local part.
type QName = model.QName

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
