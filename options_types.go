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

// CompileOption configures schema loading and runtime-schema compilation.
type CompileOption interface {
	applyCompile(*compileConfig)
}

// SourceOption configures schema loading and schema XML parsing.
type SourceOption interface {
	CompileOption
	applySource(*sourceConfig)
}

// BuildOption configures immutable runtime-schema compilation.
type BuildOption interface {
	CompileOption
	applyBuild(*buildConfig)
}

// ValidateOption configures instance XML parsing and validator sessions.
type ValidateOption interface {
	applyValidate(*validateConfig)
}

type sourceConfig struct {
	allowMissingImportLocations bool
	parseLimits                 parseLimitOptions
}

type buildConfig struct {
	maxDFAStates   uint32Option
	maxOccursLimit uint32Option
}

type validateConfig struct {
	parseLimits parseLimitOptions
}

type compileConfig struct {
	source sourceConfig
	build  buildConfig
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
