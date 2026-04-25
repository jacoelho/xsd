package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/runtimebuild/valuebuild"
	"github.com/jacoelho/xsd/internal/schemair"
)

type defaultFixedValue struct {
	Key    runtime.ValueKeyRef
	Ref    runtime.ValueRef
	Member runtime.ValidatorID
}

type runtimeArtifacts struct {
	TypeValidators    map[schemair.TypeID]runtime.ValidatorID
	BuiltinValidators map[string]runtime.ValidatorID
	TextValidators    map[schemair.TypeID]runtime.ValidatorID
	ElementDefaults   map[schemair.ElementID]defaultFixedValue
	ElementFixed      map[schemair.ElementID]defaultFixedValue
	AttributeDefaults map[schemair.AttributeID]defaultFixedValue
	AttributeFixed    map[schemair.AttributeID]defaultFixedValue
	AttrUseDefaults   map[schemair.AttributeUseID]defaultFixedValue
	AttrUseFixed      map[schemair.AttributeUseID]defaultFixedValue
	Validators        runtime.ValidatorsBundle
	Enums             runtime.EnumTable
	Facets            []runtime.FacetInstr
	Patterns          []runtime.Pattern
	Values            runtime.ValueBlob
}

func buildRuntimeArtifacts(schema *schemair.Schema) (runtimeArtifacts, error) {
	if schema == nil {
		return runtimeArtifacts{}, fmt.Errorf("runtime build: schema ir is nil")
	}
	compiled, err := valuebuild.Compile(schema)
	if err != nil {
		return runtimeArtifacts{}, err
	}
	artifacts := runtimeArtifacts{
		TypeValidators:    make(map[schemair.TypeID]runtime.ValidatorID, runtimeValidatorCap(len(compiled.TypeValidators))),
		BuiltinValidators: make(map[string]runtime.ValidatorID, runtimeValidatorCap(len(schema.BuiltinTypes))),
		TextValidators:    make(map[schemair.TypeID]runtime.ValidatorID),
		ElementDefaults:   make(map[schemair.ElementID]defaultFixedValue),
		ElementFixed:      make(map[schemair.ElementID]defaultFixedValue),
		AttributeDefaults: make(map[schemair.AttributeID]defaultFixedValue),
		AttributeFixed:    make(map[schemair.AttributeID]defaultFixedValue),
		AttrUseDefaults:   make(map[schemair.AttributeUseID]defaultFixedValue),
		AttrUseFixed:      make(map[schemair.AttributeUseID]defaultFixedValue),
		Validators:        compiled.Validators,
		Enums:             compiled.Enums,
		Facets:            compiled.Facets,
		Patterns:          compiled.Patterns,
		Values:            compiled.Values,
	}
	for id, validator := range compiled.TypeValidators {
		artifacts.TypeValidators[id] = validator
	}
	for local, validator := range compiled.BuiltinValidators {
		artifacts.BuiltinValidators[local] = validator
	}
	for id, validator := range compiled.TextValidators {
		artifacts.TextValidators[id] = validator
	}
	for id, def := range compiled.ElementDefaults {
		artifacts.ElementDefaults[id] = defaultFixed(def)
	}
	for id, fixed := range compiled.ElementFixed {
		artifacts.ElementFixed[id] = defaultFixed(fixed)
	}
	for id, def := range compiled.AttributeDefaults {
		artifacts.AttributeDefaults[id] = defaultFixed(def)
	}
	for id, fixed := range compiled.AttributeFixed {
		artifacts.AttributeFixed[id] = defaultFixed(fixed)
	}
	for id, def := range compiled.AttrUseDefaults {
		artifacts.AttrUseDefaults[id] = defaultFixed(def)
	}
	for id, fixed := range compiled.AttrUseFixed {
		artifacts.AttrUseFixed[id] = defaultFixed(fixed)
	}
	return artifacts, nil
}

func runtimeValidatorCap(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

func defaultFixed(value valuebuild.DefaultFixedValue) defaultFixedValue {
	return defaultFixedValue{
		Key:    value.Key,
		Ref:    value.Ref,
		Member: value.Member,
	}
}
