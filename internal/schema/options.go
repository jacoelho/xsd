// Package schema compiles streamed XSD declarations into immutable validation programs.
package schema

import "github.com/jacoelho/xsd/xsderrors"

const (
	defaultMaxSchemaDepth             = 256
	defaultMaxSchemaAttributes        = 256
	defaultMaxSchemaTokenBytes        = int64(4 << 20)
	defaultMaxSchemaSourceBytes       = int64(64 << 20)
	defaultMaxSchemaSources           = 1024
	defaultMaxSchemaTotalBytes        = int64(256 << 20)
	defaultMaxSchemaReferences        = 16_384
	defaultMaxSchemaDependencySteps   = 1_000_000
	defaultMaxSchemaTargetContexts    = 4096
	defaultMaxSchemaInstantiatedNodes = 1_000_000
	defaultMaxContentModelStates      = 16_384
	// Counter determinization can perform many scalar configuration operations
	// per retained DFA state; keep the work cap proportional to the state cap.
	defaultMaxContentModelAnalysisSteps  = defaultMaxContentModelStates * 1024
	defaultMaxSubstitutionClosureEntries = 1_000_000
	// defaultMaxSchemaPatternWork bounds regular-expression matching per
	// compiler literal, in the units documented on xsdregex.MatchOptions.
	defaultMaxSchemaPatternWork = uint64(1 << 29)
	// defaultMaxSchemaTotalPatternWork is the per-literal default x 16; the
	// same headroom ratio as the instance plane.
	defaultMaxSchemaTotalPatternWork   = uint64(1 << 33)
	defaultMaxSimpleUnionMemberEntries = 1_000_000
	// defaultMaxSchemaPatternRetainedBytes is derived from a byte target, not
	// from the per-pattern caps; docs/RESOURCE_LIMITS.md gives the derivation.
	defaultMaxSchemaPatternRetainedBytes = int64(64 << 20)
)

// Options controls schema compilation resource limits.
type Options struct {
	MaxSchemaDepth      int
	MaxSchemaAttributes int
	// MaxSchemaTokenBytes caps the bytes the schema XML reader holds for one
	// token (names, attribute values, PI and text payload, construction
	// scratch), measured lexically, and the character data accumulated in
	// one element across text tokens. It also scales literal evaluation work.
	MaxSchemaTokenBytes           int64
	MaxSchemaSourceBytes          int64
	MaxSchemaSources              int
	MaxSchemaTotalBytes           int64
	MaxSchemaReferences           int
	MaxSchemaDependencySteps      int
	MaxSchemaTargetContexts       int
	MaxSchemaInstantiatedNodes    int
	MaxSchemaNames                int
	MaxFiniteOccurs               uint64
	MaxContentModelStates         int
	MaxContentModelAnalysisSteps  int
	MaxSubstitutionClosureEntries int
	MaxSimpleUnionMemberEntries   int
	MaxSchemaPatternWork          uint64
	MaxSchemaTotalPatternWork     uint64
	MaxSchemaPatternRetainedBytes int64
}

// Limits is the normalized internal form of Options.
type Limits struct {
	MaxSchemaDepth                int
	MaxSchemaAttributes           int
	MaxSchemaTokenBytes           int64
	MaxSchemaSourceBytes          int64
	MaxSchemaSources              int
	MaxSchemaTotalBytes           int64
	MaxSchemaReferences           int
	MaxSchemaDependencySteps      int
	MaxSchemaTargetContexts       int
	MaxSchemaInstantiatedNodes    int
	MaxSchemaNames                int
	MaxContentModelStates         int
	MaxContentModelAnalysisSteps  int
	MaxSubstitutionClosureEntries int
	MaxSimpleUnionMemberEntries   int
	MaxFiniteOccurs               uint64
	MaxSchemaPatternWork          uint64
	MaxSchemaTotalPatternWork     uint64
	MaxSchemaPatternRetainedBytes int64
}

// NormalizeOptions validates options and fills default limits.
func NormalizeOptions(opts Options) (Limits, error) {
	var limits Limits
	if err := normalizeSchemaSourceLimits(opts, &limits); err != nil {
		return Limits{}, err
	}
	if err := normalizeSchemaGraphLimits(opts, &limits); err != nil {
		return Limits{}, err
	}
	if err := normalizeCompilationLimits(opts, &limits); err != nil {
		return Limits{}, err
	}
	limits.MaxFiniteOccurs = opts.MaxFiniteOccurs
	var err error
	if limits.MaxSchemaPatternRetainedBytes, err = byteLimitOrDefault("MaxSchemaPatternRetainedBytes", opts.MaxSchemaPatternRetainedBytes, defaultMaxSchemaPatternRetainedBytes); err != nil {
		return Limits{}, err
	}
	return limits, nil
}

func normalizeSchemaSourceLimits(opts Options, limits *Limits) error {
	var err error
	if limits.MaxSchemaDepth, err = limitOrDefault("MaxSchemaDepth", opts.MaxSchemaDepth, defaultMaxSchemaDepth); err != nil {
		return err
	}
	if limits.MaxSchemaAttributes, err = limitOrDefault("MaxSchemaAttributes", opts.MaxSchemaAttributes, defaultMaxSchemaAttributes); err != nil {
		return err
	}
	if limits.MaxSchemaTokenBytes, err = byteLimitOrDefault("MaxSchemaTokenBytes", opts.MaxSchemaTokenBytes, defaultMaxSchemaTokenBytes); err != nil {
		return err
	}
	if limits.MaxSchemaSourceBytes, err = byteLimitOrDefault("MaxSchemaSourceBytes", opts.MaxSchemaSourceBytes, defaultMaxSchemaSourceBytes); err != nil {
		return err
	}
	if limits.MaxSchemaSources, err = limitOrDefault("MaxSchemaSources", opts.MaxSchemaSources, defaultMaxSchemaSources); err != nil {
		return err
	}
	if limits.MaxSchemaTotalBytes, err = byteLimitOrDefault("MaxSchemaTotalBytes", opts.MaxSchemaTotalBytes, defaultMaxSchemaTotalBytes); err != nil {
		return err
	}
	return nil
}

func normalizeSchemaGraphLimits(opts Options, limits *Limits) error {
	var err error
	if limits.MaxSchemaReferences, err = limitOrDefault("MaxSchemaReferences", opts.MaxSchemaReferences, defaultMaxSchemaReferences); err != nil {
		return err
	}
	if limits.MaxSchemaDependencySteps, err = limitOrDefault("MaxSchemaDependencySteps", opts.MaxSchemaDependencySteps, defaultMaxSchemaDependencySteps); err != nil {
		return err
	}
	if limits.MaxSchemaTargetContexts, err = limitOrDefault("MaxSchemaTargetContexts", opts.MaxSchemaTargetContexts, defaultMaxSchemaTargetContexts); err != nil {
		return err
	}
	if limits.MaxSchemaInstantiatedNodes, err = limitOrDefault("MaxSchemaInstantiatedNodes", opts.MaxSchemaInstantiatedNodes, defaultMaxSchemaInstantiatedNodes); err != nil {
		return err
	}
	if opts.MaxSchemaNames < 0 {
		return limitError("MaxSchemaNames cannot be negative")
	}
	limits.MaxSchemaNames = opts.MaxSchemaNames
	return nil
}

func normalizeCompilationLimits(opts Options, limits *Limits) error {
	var err error
	if limits.MaxContentModelStates, err = limitOrDefault("MaxContentModelStates", opts.MaxContentModelStates, defaultMaxContentModelStates); err != nil {
		return err
	}
	if limits.MaxContentModelAnalysisSteps, err = limitOrDefault("MaxContentModelAnalysisSteps", opts.MaxContentModelAnalysisSteps, defaultMaxContentModelAnalysisSteps); err != nil {
		return err
	}
	if limits.MaxSubstitutionClosureEntries, err = limitOrDefault("MaxSubstitutionClosureEntries", opts.MaxSubstitutionClosureEntries, defaultMaxSubstitutionClosureEntries); err != nil {
		return err
	}
	if limits.MaxSimpleUnionMemberEntries, err = limitOrDefault("MaxSimpleUnionMemberEntries", opts.MaxSimpleUnionMemberEntries, defaultMaxSimpleUnionMemberEntries); err != nil {
		return err
	}
	limits.MaxSchemaPatternWork = uintLimitOrDefault(opts.MaxSchemaPatternWork, defaultMaxSchemaPatternWork)
	limits.MaxSchemaTotalPatternWork = uintLimitOrDefault(opts.MaxSchemaTotalPatternWork, defaultMaxSchemaTotalPatternWork)
	return nil
}

func limitOrDefault(name string, value, def int) (int, error) {
	if value < 0 {
		return 0, limitError(name + " cannot be negative")
	}
	if value == 0 {
		return def, nil
	}
	return value, nil
}

func uintLimitOrDefault(value, def uint64) uint64 {
	if value == 0 {
		return def
	}
	return value
}

func byteLimitOrDefault(name string, value, def int64) (int64, error) {
	if value < 0 {
		return 0, limitError(name + " cannot be negative")
	}
	if value == 0 {
		return def, nil
	}
	return value, nil
}

func limitError(msg string) error {
	return xsderrors.SchemaCompile(xsderrors.CodeSchemaLimit, msg)
}
