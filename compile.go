package xsd

import (
	"github.com/jacoelho/xsd/internal/compile"
	"github.com/jacoelho/xsd/internal/source"
	"github.com/jacoelho/xsd/internal/validate"
)

// Engine is an immutable compiled schema validator.
type Engine struct {
	rt validate.Runtime
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
	// MaxSchemaNames caps interned schema names, including built-ins. Zero means no explicit limit.
	MaxSchemaNames int
	// MaxFiniteOccurs caps finite maxOccurs values. Zero uses the uint32 runtime cap.
	MaxFiniteOccurs uint64
	// MaxContentModelStates caps compiled content-model DFA states. Zero uses the default.
	MaxContentModelStates int
}

// Compile compiles schema sources into an immutable validation engine.
func Compile(sources ...SchemaSource) (*Engine, error) {
	return CompileWithOptions(CompileOptions{}, sources...)
}

// CompileWithOptions compiles schema sources with explicit resource limits.
func CompileWithOptions(opts CompileOptions, sources ...SchemaSource) (*Engine, error) {
	var scratch [1]source.Source
	rt, err := compile.Compile(internalCompileOptions(opts), internalSchemaSources(sources, scratch[:0]))
	if err != nil {
		return nil, err
	}
	return &Engine{rt: rt}, nil
}

func internalCompileOptions(opts CompileOptions) compile.Options {
	return compile.Options{
		MaxSchemaDepth:        opts.MaxSchemaDepth,
		MaxSchemaAttributes:   opts.MaxSchemaAttributes,
		MaxSchemaTokenBytes:   opts.MaxSchemaTokenBytes,
		MaxSchemaSourceBytes:  opts.MaxSchemaSourceBytes,
		MaxSchemaNames:        opts.MaxSchemaNames,
		MaxFiniteOccurs:       opts.MaxFiniteOccurs,
		MaxContentModelStates: opts.MaxContentModelStates,
	}
}
