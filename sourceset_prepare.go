package xsd

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/compiler"
)

func preparePreparedSchema(req compileRequest) (*PreparedSchema, error) {
	prepared, err := req.prepare()
	if err != nil {
		return nil, err
	}
	return &PreparedSchema{prepared: prepared}, nil
}

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
