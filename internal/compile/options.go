// Package compile owns schema compilation concerns.
package compile

import "github.com/jacoelho/xsd/xsderrors"

const (
	defaultMaxSchemaDepth             = 256
	defaultMaxSchemaAttributes        = 256
	defaultMaxSchemaTokenBytes        = int64(4 << 20)
	defaultMaxSchemaSourceBytes       = int64(64 << 20)
	defaultMaxSchemaSources           = 1024
	defaultMaxSchemaTotalBytes        = int64(256 << 20)
	defaultMaxSchemaReferences        = 16_384
	defaultMaxSchemaTargetContexts    = 4096
	defaultMaxSchemaInstantiatedNodes = 1_000_000
	defaultMaxContentModelStates      = 16_384
)

// Options controls schema compilation resource limits.
type Options struct {
	MaxSchemaDepth             int
	MaxSchemaAttributes        int
	MaxSchemaTokenBytes        int64
	MaxSchemaSourceBytes       int64
	MaxSchemaSources           int
	MaxSchemaTotalBytes        int64
	MaxSchemaReferences        int
	MaxSchemaTargetContexts    int
	MaxSchemaInstantiatedNodes int
	MaxSchemaNames             int
	MaxFiniteOccurs            uint64
	MaxContentModelStates      int
}

// Limits is the normalized internal form of Options.
type Limits struct {
	MaxSchemaDepth             int
	MaxSchemaAttributes        int
	MaxSchemaTokenBytes        int64
	MaxSchemaSourceBytes       int64
	MaxSchemaSources           int
	MaxSchemaTotalBytes        int64
	MaxSchemaReferences        int
	MaxSchemaTargetContexts    int
	MaxSchemaInstantiatedNodes int
	MaxSchemaNames             int
	MaxContentModelStates      int
	MaxFiniteOccurs            uint64
}

// NormalizeOptions validates options and fills default limits.
func NormalizeOptions(opts Options) (Limits, error) {
	depth, err := limitOrDefault("MaxSchemaDepth", opts.MaxSchemaDepth, defaultMaxSchemaDepth)
	if err != nil {
		return Limits{}, err
	}
	attrs, err := limitOrDefault("MaxSchemaAttributes", opts.MaxSchemaAttributes, defaultMaxSchemaAttributes)
	if err != nil {
		return Limits{}, err
	}
	tokenBytes, err := byteLimitOrDefault("MaxSchemaTokenBytes", opts.MaxSchemaTokenBytes, defaultMaxSchemaTokenBytes)
	if err != nil {
		return Limits{}, err
	}
	sourceBytes, err := byteLimitOrDefault("MaxSchemaSourceBytes", opts.MaxSchemaSourceBytes, defaultMaxSchemaSourceBytes)
	if err != nil {
		return Limits{}, err
	}
	sources, err := limitOrDefault("MaxSchemaSources", opts.MaxSchemaSources, defaultMaxSchemaSources)
	if err != nil {
		return Limits{}, err
	}
	totalBytes, err := byteLimitOrDefault("MaxSchemaTotalBytes", opts.MaxSchemaTotalBytes, defaultMaxSchemaTotalBytes)
	if err != nil {
		return Limits{}, err
	}
	references, err := limitOrDefault("MaxSchemaReferences", opts.MaxSchemaReferences, defaultMaxSchemaReferences)
	if err != nil {
		return Limits{}, err
	}
	targetContexts, err := limitOrDefault("MaxSchemaTargetContexts", opts.MaxSchemaTargetContexts, defaultMaxSchemaTargetContexts)
	if err != nil {
		return Limits{}, err
	}
	instantiatedNodes, err := limitOrDefault("MaxSchemaInstantiatedNodes", opts.MaxSchemaInstantiatedNodes, defaultMaxSchemaInstantiatedNodes)
	if err != nil {
		return Limits{}, err
	}
	if opts.MaxSchemaNames < 0 {
		return Limits{}, limitError("MaxSchemaNames cannot be negative")
	}
	modelStates, err := limitOrDefault("MaxContentModelStates", opts.MaxContentModelStates, defaultMaxContentModelStates)
	if err != nil {
		return Limits{}, err
	}
	return Limits{
		MaxSchemaDepth:             depth,
		MaxSchemaAttributes:        attrs,
		MaxSchemaTokenBytes:        tokenBytes,
		MaxSchemaSourceBytes:       sourceBytes,
		MaxSchemaSources:           sources,
		MaxSchemaTotalBytes:        totalBytes,
		MaxSchemaReferences:        references,
		MaxSchemaTargetContexts:    targetContexts,
		MaxSchemaInstantiatedNodes: instantiatedNodes,
		MaxSchemaNames:             opts.MaxSchemaNames,
		MaxContentModelStates:      modelStates,
		MaxFiniteOccurs:            opts.MaxFiniteOccurs,
	}, nil
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
