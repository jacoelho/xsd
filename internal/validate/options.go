// Package validate owns XML instance validation concerns.
package validate

import "github.com/jacoelho/xsd/xsderrors"

const (
	defaultMaxErrors                       = 100
	defaultMaxIdentityScopes               = 10_000
	defaultMaxIdentityEntries              = 100_000
	defaultMaxIdentityTupleBytes           = 4 << 10
	defaultMaxSchemaLocationNamespaces     = 256
	defaultMaxSchemaLocationNamespaceBytes = 64 << 10
	defaultMaxInstanceDepth                = 256
	defaultMaxInstanceAttributes           = 4_096
	defaultMaxInstanceTextBytes            = 4 << 20
	defaultMaxInstanceTokenBytes           = 4 << 20
	defaultMaxInstanceBytes                = 64 << 20
)

// Options controls instance validation limits.
type Options struct {
	MaxErrors                       int
	MaxIdentityScopes               int
	MaxIdentityEntries              int
	MaxIdentityTupleBytes           int64
	MaxSchemaLocationNamespaces     int
	MaxSchemaLocationNamespaceBytes int64
	MaxInstanceDepth                int
	MaxInstanceAttributes           int
	MaxInstanceTextBytes            int64
	MaxInstanceTokenBytes           int64
	MaxInstanceBytes                int64
}

// Limits is the normalized internal form of Options.
type Limits struct {
	Errors                       int
	IdentityScopes               int
	IdentityEntries              int
	IdentityTupleBytes           int64
	SchemaLocationNamespaces     int
	SchemaLocationNamespaceBytes int64
	InstanceDepth                int
	InstanceAttributes           int
	InstanceTextBytes            int64
	InstanceTokenBytes           int64
	InstanceBytes                int64
}

// NormalizeOptions validates options and returns runtime limits.
func NormalizeOptions(opts Options) (Limits, error) {
	if opts.MaxErrors < 0 {
		return Limits{}, optionError("MaxErrors cannot be negative")
	}
	if opts.MaxIdentityScopes < 0 {
		return Limits{}, optionError("MaxIdentityScopes cannot be negative")
	}
	if opts.MaxIdentityEntries < 0 {
		return Limits{}, optionError("MaxIdentityEntries cannot be negative")
	}
	if opts.MaxIdentityTupleBytes < 0 {
		return Limits{}, optionError("MaxIdentityTupleBytes cannot be negative")
	}
	if opts.MaxSchemaLocationNamespaces < 0 {
		return Limits{}, optionError("MaxSchemaLocationNamespaces cannot be negative")
	}
	if opts.MaxSchemaLocationNamespaceBytes < 0 {
		return Limits{}, optionError("MaxSchemaLocationNamespaceBytes cannot be negative")
	}
	if opts.MaxInstanceDepth < 0 {
		return Limits{}, optionError("MaxInstanceDepth cannot be negative")
	}
	if opts.MaxInstanceAttributes < 0 {
		return Limits{}, optionError("MaxInstanceAttributes cannot be negative")
	}
	if opts.MaxInstanceTextBytes < 0 {
		return Limits{}, optionError("MaxInstanceTextBytes cannot be negative")
	}
	if opts.MaxInstanceTokenBytes < 0 {
		return Limits{}, optionError("MaxInstanceTokenBytes cannot be negative")
	}
	if opts.MaxInstanceBytes < 0 {
		return Limits{}, optionError("MaxInstanceBytes cannot be negative")
	}
	return Limits{
		Errors:                       intLimitOrDefault(opts.MaxErrors, defaultMaxErrors),
		IdentityScopes:               intLimitOrDefault(opts.MaxIdentityScopes, defaultMaxIdentityScopes),
		IdentityEntries:              intLimitOrDefault(opts.MaxIdentityEntries, defaultMaxIdentityEntries),
		IdentityTupleBytes:           byteLimitOrDefault(opts.MaxIdentityTupleBytes, defaultMaxIdentityTupleBytes),
		SchemaLocationNamespaces:     intLimitOrDefault(opts.MaxSchemaLocationNamespaces, defaultMaxSchemaLocationNamespaces),
		SchemaLocationNamespaceBytes: byteLimitOrDefault(opts.MaxSchemaLocationNamespaceBytes, defaultMaxSchemaLocationNamespaceBytes),
		InstanceDepth:                intLimitOrDefault(opts.MaxInstanceDepth, defaultMaxInstanceDepth),
		InstanceAttributes:           intLimitOrDefault(opts.MaxInstanceAttributes, defaultMaxInstanceAttributes),
		InstanceTextBytes:            byteLimitOrDefault(opts.MaxInstanceTextBytes, defaultMaxInstanceTextBytes),
		InstanceTokenBytes:           byteLimitOrDefault(opts.MaxInstanceTokenBytes, defaultMaxInstanceTokenBytes),
		InstanceBytes:                byteLimitOrDefault(opts.MaxInstanceBytes, defaultMaxInstanceBytes),
	}, nil
}

func intLimitOrDefault(value, def int) int {
	if value == 0 {
		return def
	}
	return value
}

func byteLimitOrDefault(value, def int64) int64 {
	if value == 0 {
		return def
	}
	return value
}

func optionError(msg string) error {
	return xsderrors.Validation(xsderrors.CodeValidationOption, 0, 0, "", msg)
}
