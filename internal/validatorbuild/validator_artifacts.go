package validatorbuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/complexplan"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/runtime"
)

// DefaultFixedValue stores canonical default/fixed metadata for runtime tables.
type DefaultFixedValue struct {
	Key    runtime.ValueKeyRef
	Ref    runtime.ValueRef
	Member runtime.ValidatorID
}

// ElementDefault returns the compiled default value for a global/local element.
func (a *ValidatorArtifacts) ElementDefault(id analysis.ElemID) (DefaultFixedValue, bool) {
	if a == nil {
		return DefaultFixedValue{}, false
	}
	value, ok := a.elements.defaults.lookup(id)
	return toDefaultFixedValue(value), ok
}

// ElementFixed returns the compiled fixed value for a global/local element.
func (a *ValidatorArtifacts) ElementFixed(id analysis.ElemID) (DefaultFixedValue, bool) {
	if a == nil {
		return DefaultFixedValue{}, false
	}
	value, ok := a.elements.fixed.lookup(id)
	return toDefaultFixedValue(value), ok
}

// AttributeDefault returns the compiled default value for an attribute declaration.
func (a *ValidatorArtifacts) AttributeDefault(id analysis.AttrID) (DefaultFixedValue, bool) {
	if a == nil {
		return DefaultFixedValue{}, false
	}
	value, ok := a.attributes.defaults.lookup(id)
	return toDefaultFixedValue(value), ok
}

// AttributeFixed returns the compiled fixed value for an attribute declaration.
func (a *ValidatorArtifacts) AttributeFixed(id analysis.AttrID) (DefaultFixedValue, bool) {
	if a == nil {
		return DefaultFixedValue{}, false
	}
	value, ok := a.attributes.fixed.lookup(id)
	return toDefaultFixedValue(value), ok
}

// AttrUseDefault returns the compiled default value for a specific attribute use.
func (a *ValidatorArtifacts) AttrUseDefault(attr *model.AttributeDecl) (DefaultFixedValue, bool) {
	if a == nil {
		return DefaultFixedValue{}, false
	}
	value, ok := a.attrUses.defaults.lookup(attr)
	return toDefaultFixedValue(value), ok
}

// AttrUseFixed returns the compiled fixed value for a specific attribute use.
func (a *ValidatorArtifacts) AttrUseFixed(attr *model.AttributeDecl) (DefaultFixedValue, bool) {
	if a == nil {
		return DefaultFixedValue{}, false
	}
	value, ok := a.attrUses.fixed.lookup(attr)
	return toDefaultFixedValue(value), ok
}

func toDefaultFixedValue(value compiledDefaultFixed) DefaultFixedValue {
	return DefaultFixedValue{
		Key:    value.key,
		Ref:    value.ref,
		Member: value.member,
	}
}

// Compile builds runtime validator artifacts from prepared schema state.
func Compile(
	sch *parser.Schema,
	reg *analysis.Registry,
	complexTypes *complexplan.ComplexTypes,
) (*ValidatorArtifacts, error) {
	if sch == nil {
		return nil, fmt.Errorf("runtime build: schema is nil")
	}
	if reg == nil {
		return nil, fmt.Errorf("runtime build: registry is nil")
	}
	if complexTypes == nil {
		return nil, fmt.Errorf("runtime build: complex types are nil")
	}
	compiled, err := compileValidatorArtifactsWithPlan(sch, reg, complexTypes)
	if err != nil {
		return nil, fmt.Errorf("runtime build: compile validators: %w", err)
	}
	return compiled, nil
}
