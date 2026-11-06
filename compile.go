package xsd

import (
	xsdSchema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/validate"
)

// Engine validates against one immutable compiled schema and is safe for concurrent use.
// Instance location hints cannot add components to that schema.
type Engine struct {
	pool *validate.SessionPool
}

// CompileOptions controls schema compilation resource limits.
type CompileOptions struct {
	// MaxSchemaDepth caps nested schema XML elements. Zero uses the default.
	MaxSchemaDepth int
	// MaxSchemaAttributes caps attributes on one schema XML element. Zero uses the default.
	MaxSchemaAttributes int
	// MaxSchemaTokenBytes caps the bytes the schema XML reader holds for one
	// token (names, attribute values, PI and text payload, construction
	// scratch), measured lexically, and the character data accumulated in
	// one element across text tokens. It also scales literal evaluation
	// work. Zero uses the default.
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
	// MaxSchemaDependencySteps caps aggregate schema-graph expansion, target-context
	// propagation, and component-dependency resolution work. Zero uses the default.
	MaxSchemaDependencySteps int
	// MaxSchemaTargetContexts caps distinct source/effective-target-namespace contexts,
	// including primary and chameleon-derived contexts. Zero uses the default.
	MaxSchemaTargetContexts int
	// MaxSchemaInstantiatedNodes caps aggregate schema node occurrences across effective target contexts. Zero uses the default.
	MaxSchemaInstantiatedNodes int
	// MaxSchemaNames caps interned schema names, including built-ins. Zero means no explicit limit.
	MaxSchemaNames int
	// MaxFiniteOccurs caps finite maxOccurs values. Zero uses the uint32 runtime cap.
	MaxFiniteOccurs uint64
	// MaxContentModelStates caps compiled content-model DFA states. Zero uses the default.
	MaxContentModelStates int
	// MaxContentModelAnalysisSteps caps content-model traversal, determinization, and ambiguity-analysis work. Zero uses the default.
	MaxContentModelAnalysisSteps int
	// MaxSubstitutionClosureEntries caps aggregate transitive substitution-group relationships. Zero uses the default.
	MaxSubstitutionClosureEntries int
	// MaxSimpleUnionMemberEntries caps aggregate flattened simple-union members. Zero uses the default.
	MaxSimpleUnionMemberEntries int
	// MaxSchemaPatternWork caps regular-expression matching work for one schema
	// literal (enumeration, bound, fixed or default value), across all pattern
	// facets, list items and union members. Units are defined in
	// docs/RESOURCE_LIMITS.md (Pattern matching cost). Zero uses the finite
	// default.
	MaxSchemaPatternWork uint64
	// MaxSchemaTotalPatternWork caps regular-expression matching work summed
	// over every schema-literal evaluation of one compilation; a facet literal
	// may be charged for both its base-type check and facet compilation; units are in
	// docs/RESOURCE_LIMITS.md (Pattern matching cost). Zero uses the default.
	MaxSchemaTotalPatternWork uint64
	// MaxSchemaPatternRetainedBytes caps the estimated memory retained by
	// compiled regular expressions, summed over the distinct pattern facets of
	// one compilation; identical pattern text is compiled and charged once.
	// The estimate is defined in docs/RESOURCE_LIMITS.md. Each pattern is
	// separately limited by fixed per-pattern bounds. Zero uses the default.
	MaxSchemaPatternRetainedBytes int64
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
