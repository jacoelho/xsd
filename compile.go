package xsd

import (
	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/validate"
)

// Engine validates against one immutable compiled schema and is safe for concurrent use.
// Instance location hints cannot add components to that schema.
// Only Compile and CompileWithOptions create a usable Engine: a zero or nil
// Engine fails every call with xsderrors.CodeValidationSession, after any
// option error.
type Engine struct {
	pool *validate.SessionPool
}

// CompileOptions controls schema compilation resource limits.
type CompileOptions struct {
	// MaxSchemaDepth caps nested schema XML elements. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaDepth int
	// MaxSchemaAttributes caps attributes on one schema XML element. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaAttributes int
	// MaxSchemaTokenBytes caps the bytes held for one schema token and the text accumulated in one element. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaTokenBytes int64
	// MaxSchemaSourceBytes caps bytes read from each schema source. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaSourceBytes int64
	// MaxSchemaSources caps the explicit sources and the distinct schema documents of one compilation. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaSources int
	// MaxSchemaTotalBytes caps aggregate bytes read from schema sources. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaTotalBytes int64
	// MaxSchemaReferences caps include/import references processed during compilation. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaReferences int
	// MaxSchemaDependencySteps caps aggregate schema-graph expansion and dependency-resolution work. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaDependencySteps int
	// MaxSchemaTargetContexts caps distinct source/effective-target-namespace contexts. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaTargetContexts int
	// MaxSchemaInstantiatedNodes caps aggregate schema node occurrences across effective target contexts. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaInstantiatedNodes int
	// MaxSchemaNames caps interned schema names. Zero means no explicit limit.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaNames int
	// MaxFiniteOccurs caps finite maxOccurs values. Zero uses the uint32 runtime cap.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxFiniteOccurs uint64
	// MaxContentModelStates caps compiled content-model DFA states. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxContentModelStates int
	// MaxContentModelAnalysisSteps caps content-model traversal, determinization and ambiguity-analysis work. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxContentModelAnalysisSteps int
	// MaxSubstitutionClosureEntries caps aggregate transitive substitution-group relationships. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSubstitutionClosureEntries int
	// MaxSimpleUnionMemberEntries caps aggregate flattened simple-union members. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSimpleUnionMemberEntries int
	// MaxSchemaPatternWork caps regular-expression matching work for one schema literal. Zero uses the finite default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaPatternWork uint64
	// MaxSchemaTotalPatternWork caps regular-expression matching work summed over one compilation. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaTotalPatternWork uint64
	// MaxSchemaPatternRetainedBytes caps the estimated memory retained by compiled regular expressions. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaPatternRetainedBytes int64
	// MaxSchemaPatternAnalysisSteps caps the automaton-construction work of compiling regular expressions, summed over one compilation. Zero uses the default.
	// See docs/RESOURCE_LIMITS.md#compilation.
	MaxSchemaPatternAnalysisSteps int
}

// Compile compiles schema sources into a concurrently reusable validation engine.
func Compile(sources ...SchemaSource) (*Engine, error) {
	return CompileWithOptions(CompileOptions{}, sources...)
}

// CompileWithOptions compiles schema sources with explicit resource limits.
func CompileWithOptions(opts CompileOptions, sources ...SchemaSource) (*Engine, error) {
	rt, err := xsdSchema.CompileMappedSources(xsdSchema.Options(opts), sources, internalSchemaSource)
	if err != nil {
		return nil, err
	}
	return &Engine{pool: validate.NewSessionPool(rt)}, nil
}
