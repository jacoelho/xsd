package runtimebuild

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/contentmodel"
	"github.com/jacoelho/xsd/internal/runtime"
	"github.com/jacoelho/xsd/internal/schemair"
)

// Config configures runtime-schema lowering.
type Config struct {
	MaxDFAStates   uint32
	MaxOccursLimit uint32
}

// Input contains immutable IR for runtime lowering.
type Input struct {
	Schema *schemair.Schema
	Config Config
}

func validateBuildInputs(input Input) error {
	if input.Schema == nil {
		return fmt.Errorf("runtime build: schema ir is nil")
	}
	return nil
}

// Build lowers prepared schema artifacts into an immutable runtime schema.
func Build(input Input) (*runtime.Schema, error) {
	if err := validateBuildInputs(input); err != nil {
		return nil, err
	}
	maxOccursLimit := input.Config.MaxOccursLimit
	if maxOccursLimit == 0 {
		maxOccursLimit = defaultMaxOccursLimit
	}
	artifacts, err := buildRuntimeArtifacts(input.Schema)
	if err != nil {
		return nil, err
	}

	builder := &schemaBuilder{
		limits:      contentmodel.Limits{MaxDFAStates: input.Config.MaxDFAStates},
		builder:     runtime.NewBuilder(),
		schema:      input.Schema,
		artifacts:   artifacts,
		complexIDs:  make(map[runtime.TypeID]uint32),
		wildcardIDs: make(map[schemair.WildcardID]runtime.WildcardID),
		maxOccurs:   maxOccursLimit,
	}
	rt, err := builder.build()
	if err != nil {
		return nil, err
	}
	return rt, nil
}
