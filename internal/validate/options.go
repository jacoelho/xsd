// Package validate owns XML instance validation concerns.
package validate

import "github.com/jacoelho/xsd/xsderrors"

const (
	defaultMaxSchemaLocationNamespaces     = 256
	defaultMaxSchemaLocationNamespaceBytes = 64 << 10
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
	maxSchemaLocationNamespaces := opts.MaxSchemaLocationNamespaces
	if maxSchemaLocationNamespaces == 0 {
		maxSchemaLocationNamespaces = defaultMaxSchemaLocationNamespaces
	}
	maxSchemaLocationNamespaceBytes := opts.MaxSchemaLocationNamespaceBytes
	if maxSchemaLocationNamespaceBytes == 0 {
		maxSchemaLocationNamespaceBytes = defaultMaxSchemaLocationNamespaceBytes
	}
	return Limits{
		Errors:                       opts.MaxErrors,
		IdentityScopes:               opts.MaxIdentityScopes,
		IdentityEntries:              opts.MaxIdentityEntries,
		IdentityTupleBytes:           opts.MaxIdentityTupleBytes,
		SchemaLocationNamespaces:     maxSchemaLocationNamespaces,
		SchemaLocationNamespaceBytes: maxSchemaLocationNamespaceBytes,
		InstanceDepth:                opts.MaxInstanceDepth,
		InstanceAttributes:           opts.MaxInstanceAttributes,
		InstanceTextBytes:            opts.MaxInstanceTextBytes,
		InstanceTokenBytes:           opts.MaxInstanceTokenBytes,
	}, nil
}

func optionError(msg string) error {
	return xsderrors.Validation(xsderrors.CodeValidationOption, 0, 0, "", msg)
}
