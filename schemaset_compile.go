package xsd

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/set"
)

// Compile compiles all added schema roots using set load options.
func (s *SchemaSet) Compile() (*Schema, error) {
	return s.compileWithRuntimeOverride(nil)
}

// CompileWithRuntimeOptions compiles all added roots with explicit runtime options.
func (s *SchemaSet) CompileWithRuntimeOptions(opts RuntimeOptions) (*Schema, error) {
	return s.compileWithRuntimeOverride(&opts)
}

func (s *SchemaSet) compileWithRuntimeOverride(runtimeOverride *RuntimeOptions) (*Schema, error) {
	if s == nil {
		return nil, fmt.Errorf("compile schema set: nil set")
	}
	if len(s.entries) == 0 {
		return nil, fmt.Errorf("compile schema set: no schema roots added")
	}

	loadOpts := s.loadOpts
	if runtimeOverride != nil {
		loadOpts.runtime = *runtimeOverride
	}
	resolvedLoad, runtimeOpts, err := loadOpts.withDefaults()
	if err != nil {
		return nil, fmt.Errorf("compile schema set: %w", err)
	}

	prepared, err := s.prepareResolved(resolvedLoad)
	if err != nil {
		return nil, fmt.Errorf("compile schema set: %w", err)
	}
	rt, err := prepared.BuildRuntime(toCompileConfig(runtimeOpts))
	if err != nil {
		return nil, fmt.Errorf("compile schema set: build runtime: %w", err)
	}
	return &Schema{engine: newEngine(rt, runtimeOpts.instanceParseOptions...)}, nil
}

func toCompileConfig(opts resolvedRuntimeOptions) set.CompileConfig {
	return set.CompileConfig{
		MaxDFAStates:   opts.maxDFAStates,
		MaxOccursLimit: opts.maxOccursLimit,
	}
}
