package xsd

import "github.com/jacoelho/xsd/internal/xmlstream"

// XMLConfig configures XML parsing limits. Zero values use defaults.
type XMLConfig struct {
	MaxDepth             int
	MaxAttrs             int
	MaxTokenSize         int
	MaxNameInternEntries int
}

// SourceConfig configures schema source loading and schema XML parsing.
type SourceConfig struct {
	XML                         XMLConfig
	AllowMissingImportLocations bool
}

// BuildConfig configures runtime schema construction. Zero values use defaults.
type BuildConfig struct {
	MaxDFAStates   uint32
	MaxOccursLimit uint32
}

// ValidateConfig configures instance XML parsing.
type ValidateConfig struct {
	XML XMLConfig
	// FastValidation lowers per-parse overhead by disabling
	// line/column tracking and entity expansion.
	FastValidation bool
}

// CompileConfig configures schema loading, runtime construction, and default validation.
type CompileConfig struct {
	Source   SourceConfig
	Build    BuildConfig
	Validate ValidateConfig
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
