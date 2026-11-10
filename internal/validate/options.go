// Package validate owns XML instance validation concerns.
package validate

import (
	xsdValue "github.com/jacoelho/xsd/internal/value"
	"github.com/jacoelho/xsd/internal/xmlstream"
)

const (
	defaultMaxErrors             = 100
	defaultMaxIdentityScopes     = 10_000
	defaultMaxIdentityEntries    = 100_000
	defaultMaxIdentityTupleBytes = 4 << 10
	defaultMaxInstanceDepth      = 256
	defaultMaxInstanceAttributes = 4_096
	defaultMaxInstanceTextBytes  = 4 << 20
	defaultMaxInstanceTokenBytes = 4 << 20
	defaultMaxInstanceBytes      = 64 << 20
	// defaultMaxInstanceValueWork is the finite per-value evaluation ceiling.
	// Keep this literal in the validation owner so it does not depend on schema
	// construction limits or the builtin type count.
	defaultMaxInstanceValueWork uint64 = 4_194_502_132_335
	// defaultMaxInstancePatternWork bounds regular-expression matching CPU per
	// value, in the units documented on xsdregex.MatchOptions. It keeps the
	// measured worst case (a near-cap NFA against 4 MiB of text) near 1.2 s.
	defaultMaxInstancePatternWork uint64 = 1 << 29
	// defaultMaxInstanceTotalPatternWork bounds pattern work summed over one
	// document. It is the per-value default times the number of maximum-size
	// text values a maximum-size document can hold, so a document made
	// entirely of values at the per-value default stays admitted. At the
	// measured worst case of about 1 s per value this is about 16 s.
	defaultMaxInstanceTotalPatternWork uint64 = defaultMaxInstancePatternWork * (defaultMaxInstanceBytes / defaultMaxInstanceTextBytes)
)

// Options controls instance validation limits.
type Options struct {
	MaxErrors                   int
	MaxIdentityScopes           int
	MaxIdentityEntries          int
	MaxIdentityTupleBytes       int64
	MaxInstanceDepth            int
	MaxInstanceAttributes       int
	MaxInstanceTextBytes        int64
	MaxInstanceTokenBytes       int64
	MaxInstanceBytes            int64
	MaxInstanceValueWork        uint64
	MaxInstancePatternWork      uint64
	MaxInstanceTotalPatternWork uint64
}

// Limits is the normalized internal form of Options.
type Limits struct {
	Errors                   int
	IdentityScopes           int
	IdentityEntries          int
	IdentityTupleBytes       int64
	InstanceDepth            int
	InstanceAttributes       int
	InstanceTextBytes        int64
	InstanceTokenBytes       int64
	InstanceBytes            int64
	InstanceValueWork        uint64
	InstancePatternWork      uint64
	InstanceTotalPatternWork uint64
}

// evaluation projects the work limits onto the value program. total is the
// calling session's document account; it is passed by pointer at each call so
// every evaluation draws on the live account.
func (l Limits) evaluation(total *xsdValue.PatternAccount) xsdValue.EvaluationLimits {
	return xsdValue.EvaluationLimits{Work: l.InstanceValueWork, PatternWork: l.InstancePatternWork, PatternTotal: total}
}

// readerConfig projects the instance syntax limits onto the stream reader.
func (l Limits) readerConfig() xmlstream.Config {
	return xmlstream.Config{
		Limits: xmlstream.Limits{
			MaxInputBytes: l.InstanceBytes,
			MaxTokenBytes: l.InstanceTokenBytes,
			MaxAttrs:      l.InstanceAttributes,
			MaxDepth:      l.InstanceDepth,
		},
		LazyAttrValues: true,
	}
}

// NormalizeOptions validates options and returns runtime limits.
func NormalizeOptions(opts Options) (Limits, error) {
	if err := validateOptions(opts); err != nil {
		return Limits{}, err
	}
	return Limits{
		Errors:                   intLimitOrDefault(opts.MaxErrors, defaultMaxErrors),
		IdentityScopes:           intLimitOrDefault(opts.MaxIdentityScopes, defaultMaxIdentityScopes),
		IdentityEntries:          intLimitOrDefault(opts.MaxIdentityEntries, defaultMaxIdentityEntries),
		IdentityTupleBytes:       byteLimitOrDefault(opts.MaxIdentityTupleBytes, defaultMaxIdentityTupleBytes),
		InstanceDepth:            intLimitOrDefault(opts.MaxInstanceDepth, defaultMaxInstanceDepth),
		InstanceAttributes:       intLimitOrDefault(opts.MaxInstanceAttributes, defaultMaxInstanceAttributes),
		InstanceTextBytes:        byteLimitOrDefault(opts.MaxInstanceTextBytes, defaultMaxInstanceTextBytes),
		InstanceTokenBytes:       byteLimitOrDefault(opts.MaxInstanceTokenBytes, defaultMaxInstanceTokenBytes),
		InstanceBytes:            byteLimitOrDefault(opts.MaxInstanceBytes, defaultMaxInstanceBytes),
		InstanceValueWork:        uintLimitOrDefault(opts.MaxInstanceValueWork, defaultMaxInstanceValueWork),
		InstancePatternWork:      uintLimitOrDefault(opts.MaxInstancePatternWork, defaultMaxInstancePatternWork),
		InstanceTotalPatternWork: uintLimitOrDefault(opts.MaxInstanceTotalPatternWork, defaultMaxInstanceTotalPatternWork),
	}, nil
}

func validateOptions(opts Options) error {
	values := [...]struct {
		name  string
		value int64
	}{
		{"MaxErrors", int64(opts.MaxErrors)},
		{"MaxIdentityScopes", int64(opts.MaxIdentityScopes)},
		{"MaxIdentityEntries", int64(opts.MaxIdentityEntries)},
		{"MaxIdentityTupleBytes", opts.MaxIdentityTupleBytes},
		{"MaxInstanceDepth", int64(opts.MaxInstanceDepth)},
		{optionMaxInstanceAttributes, int64(opts.MaxInstanceAttributes)},
		{"MaxInstanceTextBytes", opts.MaxInstanceTextBytes},
		{optionMaxInstanceTokenBytes, opts.MaxInstanceTokenBytes},
		{optionMaxInstanceBytes, opts.MaxInstanceBytes},
	}
	for _, option := range values {
		if option.value < 0 {
			return optionError(optionNegativeMessage(option.name, option.value))
		}
	}
	return nil
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

func uintLimitOrDefault(value, def uint64) uint64 {
	if value == 0 {
		return def
	}
	return value
}
