package xsd

import "github.com/jacoelho/xsd/pkg/xmlstream"

type intOption struct {
	value int
	set   bool
}

func (o intOption) resolved() int {
	if !o.set {
		return 0
	}
	return o.value
}

type uint32Option struct {
	value uint32
	set   bool
}

func (o uint32Option) resolved() uint32 {
	if !o.set {
		return 0
	}
	return o.value
}

type parseLimitOptions struct {
	maxDepth              intOption
	maxAttrs              intOption
	maxTokenSize          intOption
	maxQNameInternEntries intOption
}

// SourceOptions configures schema loading and schema XML parsing.
type SourceOptions struct {
	allowMissingImportLocations bool
	parseLimits                 parseLimitOptions
}

// BuildOptions configures immutable runtime-schema compilation.
type BuildOptions struct {
	maxDFAStates   uint32Option
	maxOccursLimit uint32Option
}

// ValidateOptions configures instance XML parsing and validator sessions.
type ValidateOptions struct {
	parseLimits parseLimitOptions
}

type resolvedSourceOptions struct {
	schemaLimits                xmlParseLimits
	allowMissingImportLocations bool
}

type resolvedBuildOptions struct {
	maxDFAStates   uint32
	maxOccursLimit uint32
}

type resolvedValidateOptions struct {
	instanceParseOptions []xmlstream.Option
	instanceLimits       xmlParseLimits
}
