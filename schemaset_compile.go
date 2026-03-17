package xsd

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/compiler"
	"github.com/jacoelho/xsd/internal/validator"
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
	return compileEntries(s.entries, s.loadOpts, runtimeOverride)
}

func compileEntries(entries []schemaSetEntry, loadOpts LoadOptions, runtimeOverride *RuntimeOptions) (*Schema, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("compile schema set: no schema roots added")
	}

	if runtimeOverride != nil {
		loadOpts.runtime = *runtimeOverride
	}
	resolvedLoad, runtimeOpts, err := loadOpts.withDefaults()
	if err != nil {
		return nil, fmt.Errorf("compile schema set: %w", err)
	}

	prepared, err := prepareEntries(entries, resolvedLoad)
	if err != nil {
		return nil, fmt.Errorf("compile schema set: %w", err)
	}
	rt, err := prepared.Build(toCompileConfig(runtimeOpts))
	if err != nil {
		return nil, fmt.Errorf("compile schema set: build runtime: %w", err)
	}
	return &Schema{engine: validator.NewEngine(rt, runtimeOpts.instanceParseOptions...)}, nil
}

func toCompileConfig(opts resolvedRuntimeOptions) compiler.BuildConfig {
	return compiler.BuildConfig{
		MaxDFAStates:   opts.maxDFAStates,
		MaxOccursLimit: opts.maxOccursLimit,
	}
}
