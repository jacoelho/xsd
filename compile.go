package xsd

import (
	"context"

	"github.com/jacoelho/xsd/internal/compile"
	"github.com/jacoelho/xsd/internal/runtime"
)

// Engine is an immutable compiled schema validator.
type Engine struct {
	rt *runtime.Schema
}

// CompileOptions controls schema compilation resource limits.
type CompileOptions struct {
	// MaxSchemaDepth caps nested schema XML elements. Zero uses the default.
	MaxSchemaDepth int
	// MaxSchemaAttributes caps attributes on one schema XML element. Zero uses the default.
	MaxSchemaAttributes int
	// MaxSchemaTokenBytes caps retained schema XML token payloads. Zero uses the default.
	MaxSchemaTokenBytes int64
	// MaxSchemaSourceBytes caps bytes read from each schema source. Zero uses the default.
	MaxSchemaSourceBytes int64
	// MaxSchemaSources caps explicit source descriptors and distinct resolver-loaded
	// source identities admitted to one compilation. Zero uses the default.
	MaxSchemaSources int
	// MaxSchemaTotalBytes caps aggregate bytes read across all schema sources. Zero uses the default.
	MaxSchemaTotalBytes int64
	// MaxSchemaReferences caps include/import references processed during compilation. Zero uses the default.
	MaxSchemaReferences int
	// MaxSchemaTargetContexts caps distinct source/effective-target-namespace contexts,
	// including primary and chameleon-derived contexts. Zero uses the default.
	MaxSchemaTargetContexts int
	// MaxSchemaInstantiatedNodes caps aggregate raw schema nodes across all target contexts. Zero uses the default.
	MaxSchemaInstantiatedNodes int
	// MaxSchemaNames caps interned schema names, including built-ins. Zero means no explicit limit.
	MaxSchemaNames int
	// MaxFiniteOccurs caps finite maxOccurs values. Zero uses the uint32 runtime cap.
	MaxFiniteOccurs uint64
	// MaxContentModelStates caps compiled content-model DFA states. Zero uses the default.
	MaxContentModelStates int
	// MaxSubstitutionClosureEntries caps aggregate transitive substitution-group relationships. Zero uses the default.
	MaxSubstitutionClosureEntries int
	// MaxSimpleUnionMemberEntries caps aggregate flattened simple-union members. Zero uses the default.
	MaxSimpleUnionMemberEntries int
}

// Compile compiles schema sources into an immutable validation engine. ctx must
// be non-nil; cancellation is cooperative at callback, read, and batch boundaries.
func Compile(ctx context.Context, sources ...SchemaSource) (*Engine, error) {
	return CompileWithOptions(ctx, CompileOptions{}, sources...)
}

// CompileWithOptions compiles schema sources with explicit resource limits.
func CompileWithOptions(ctx context.Context, opts CompileOptions, sources ...SchemaSource) (*Engine, error) {
	rt, err := compile.CompileMappedSources(ctx, internalCompileOptions(opts), sources, internalSchemaSource)
	if err != nil {
		return nil, err
	}
	return &Engine{rt: rt}, nil
}

func internalCompileOptions(opts CompileOptions) compile.Options {
	return compile.Options{
		MaxSchemaDepth:                opts.MaxSchemaDepth,
		MaxSchemaAttributes:           opts.MaxSchemaAttributes,
		MaxSchemaTokenBytes:           opts.MaxSchemaTokenBytes,
		MaxSchemaSourceBytes:          opts.MaxSchemaSourceBytes,
		MaxSchemaSources:              opts.MaxSchemaSources,
		MaxSchemaTotalBytes:           opts.MaxSchemaTotalBytes,
		MaxSchemaReferences:           opts.MaxSchemaReferences,
		MaxSchemaTargetContexts:       opts.MaxSchemaTargetContexts,
		MaxSchemaInstantiatedNodes:    opts.MaxSchemaInstantiatedNodes,
		MaxSchemaNames:                opts.MaxSchemaNames,
		MaxFiniteOccurs:               opts.MaxFiniteOccurs,
		MaxContentModelStates:         opts.MaxContentModelStates,
		MaxSubstitutionClosureEntries: opts.MaxSubstitutionClosureEntries,
		MaxSimpleUnionMemberEntries:   opts.MaxSimpleUnionMemberEntries,
	}
}
