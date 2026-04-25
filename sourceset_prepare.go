package xsd

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/compiler"
)

func buildSchema(prepared *compiler.Prepared, buildOpts resolvedBuildOptions, validateDefaults resolvedValidateOptions) (*Schema, error) {
	if prepared == nil {
		return nil, fmt.Errorf("build prepared schema: nil prepared schema")
	}

	rt, err := prepared.Build(toCompileConfig(buildOpts))
	if err != nil {
		return nil, fmt.Errorf("build prepared schema: build runtime: %w", err)
	}
	return newSchema(rt, validateDefaults), nil
}

func toCompileConfig(opts resolvedBuildOptions) compiler.BuildConfig {
	return compiler.BuildConfig{
		MaxDFAStates:   opts.maxDFAStates,
		MaxOccursLimit: opts.maxOccursLimit,
	}
}
