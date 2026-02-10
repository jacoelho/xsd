package validatorgen

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	schema "github.com/jacoelho/xsd/internal/schemaanalysis"
)

// CompiledValidators stores precompiled validator artifacts used by runtime build.
type CompiledValidators = compiledValidators

// DefaultFixedValue stores canonical default/fixed metadata for runtime tables.
type DefaultFixedValue struct {
	Key    runtime.ValueKeyRef
	Ref    runtime.ValueRef
	Member runtime.ValidatorID
}

// ElementDefault returns the compiled default value for a global/local element.
func (c *compiledValidators) ElementDefault(id schema.ElemID) (DefaultFixedValue, bool) {
	if c == nil {
		return DefaultFixedValue{}, false
	}
	def, ok := c.elements.defaults.lookup(id)
	if !ok {
		return DefaultFixedValue{}, false
	}
	return toDefaultFixedValue(def), true
}

// ElementFixed returns the compiled fixed value for a global/local element.
func (c *compiledValidators) ElementFixed(id schema.ElemID) (DefaultFixedValue, bool) {
	if c == nil {
		return DefaultFixedValue{}, false
	}
	fixed, ok := c.elements.fixed.lookup(id)
	if !ok {
		return DefaultFixedValue{}, false
	}
	return toDefaultFixedValue(fixed), true
}

// AttributeDefault returns the compiled default value for an attribute declaration.
func (c *compiledValidators) AttributeDefault(id schema.AttrID) (DefaultFixedValue, bool) {
	if c == nil {
		return DefaultFixedValue{}, false
	}
	def, ok := c.attributes.defaults.lookup(id)
	if !ok {
		return DefaultFixedValue{}, false
	}
	return toDefaultFixedValue(def), true
}

// AttributeFixed returns the compiled fixed value for an attribute declaration.
func (c *compiledValidators) AttributeFixed(id schema.AttrID) (DefaultFixedValue, bool) {
	if c == nil {
		return DefaultFixedValue{}, false
	}
	fixed, ok := c.attributes.fixed.lookup(id)
	if !ok {
		return DefaultFixedValue{}, false
	}
	return toDefaultFixedValue(fixed), true
}

// AttrUseDefault returns the compiled default value for a specific attribute use.
func (c *compiledValidators) AttrUseDefault(attr *model.AttributeDecl) (DefaultFixedValue, bool) {
	if c == nil {
		return DefaultFixedValue{}, false
	}
	def, ok := c.attrUses.defaults.lookup(attr)
	if !ok {
		return DefaultFixedValue{}, false
	}
	return toDefaultFixedValue(def), true
}

// AttrUseFixed returns the compiled fixed value for a specific attribute use.
func (c *compiledValidators) AttrUseFixed(attr *model.AttributeDecl) (DefaultFixedValue, bool) {
	if c == nil {
		return DefaultFixedValue{}, false
	}
	fixed, ok := c.attrUses.fixed.lookup(attr)
	if !ok {
		return DefaultFixedValue{}, false
	}
	return toDefaultFixedValue(fixed), true
}

func toDefaultFixedValue(value compiledDefaultFixed) DefaultFixedValue {
	return DefaultFixedValue{
		Key:    value.key,
		Ref:    value.ref,
		Member: value.member,
	}
}
