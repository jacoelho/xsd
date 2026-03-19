package xsd

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/compiler"
)

func prepareEntries(entries []sourceEntry, source resolvedSourceOptions) (*compiler.Prepared, error) {
	if len(entries) == 1 {
		entry := entries[0]
		return compiler.PrepareRoots(compiler.LoadConfig{
			FS:                          entry.fsys,
			Location:                    entry.location,
			Resolver:                    entry.resolver,
			AllowMissingImportLocations: source.allowMissingImportLocations,
			SchemaParseOptions:          source.schemaLimits.options(),
		})
	}

	roots := make([]compiler.Root, 0, len(entries))
	for _, entry := range entries {
		roots = append(roots, compiler.Root{
			FS:       entry.fsys,
			Location: entry.location,
		})
	}
	return compiler.PrepareRoots(compiler.LoadConfig{
		Roots:                       roots,
		AllowMissingImportLocations: source.allowMissingImportLocations,
		SchemaParseOptions:          source.schemaLimits.options(),
	})
}

func preparePreparedSchema(entries []sourceEntry, sourceOpts SourceOptions) (*PreparedSchema, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("prepare source set: no schema roots added")
	}

	source, err := sourceOpts.withDefaults()
	if err != nil {
		return nil, fmt.Errorf("prepare source set: %w", err)
	}

	prepared, err := prepareEntries(entries, source)
	if err != nil {
		return nil, fmt.Errorf("prepare source set: %w", err)
	}
	return &PreparedSchema{prepared: prepared}, nil
}

func buildSchema(prepared *compiler.Prepared, buildOpts BuildOptions, validateDefaults resolvedValidateOptions) (*Schema, error) {
	if prepared == nil {
		return nil, fmt.Errorf("build prepared schema: nil prepared schema")
	}

	rt, err := prepared.Build(toCompileConfig(buildOpts.withDefaults()))
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
