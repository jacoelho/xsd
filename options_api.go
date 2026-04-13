package xsd

type parseLimitKind uint8

const (
	parseLimitNone parseLimitKind = iota
	parseLimitDepth
	parseLimitAttrs
	parseLimitTokenSize
	parseLimitQNameInternEntries
)

type buildLimitKind uint8

const (
	buildLimitNone buildLimitKind = iota
	buildLimitDFAStates
	buildLimitOccurs
)

type sourceOptionKind uint8

const (
	sourceOptionNone sourceOptionKind = iota
	sourceOptionAllowMissingImportLocations
	sourceOptionParseLimit
)

// SourceOptionValue is a source option token returned by source option funcs.
// Pass it to NewSourceSet, SourceSet.WithOptions, or Compile/CompileFile.
type SourceOptionValue struct {
	kind      sourceOptionKind
	parseKind parseLimitKind
	value     int
}

// BuildOptionValue is a build option token returned by build option funcs.
// Pass it to PreparedSchema.Build, SourceSet.Build, or Compile/CompileFile.
type BuildOptionValue struct {
	kind  buildLimitKind
	value uint32
}

// ValidateOptionValue is a validate option token returned by validate option funcs.
// Pass it to Schema.NewValidator.
type ValidateOptionValue struct {
	kind  parseLimitKind
	value int
}

// AllowMissingImportLocations skips imports that omit schemaLocation or fail
// with not-found when missing import locations are allowed.
func AllowMissingImportLocations() SourceOptionValue {
	return SourceOptionValue{kind: sourceOptionAllowMissingImportLocations}
}

// SchemaMaxDepth sets the schema XML max depth limit (0 uses default).
func SchemaMaxDepth(value int) SourceOptionValue {
	return SourceOptionValue{kind: sourceOptionParseLimit, parseKind: parseLimitDepth, value: value}
}

// SchemaMaxAttrs sets the schema XML max attributes limit (0 uses default).
func SchemaMaxAttrs(value int) SourceOptionValue {
	return SourceOptionValue{kind: sourceOptionParseLimit, parseKind: parseLimitAttrs, value: value}
}

// SchemaMaxTokenSize sets the schema XML max token size limit (0 uses default).
func SchemaMaxTokenSize(value int) SourceOptionValue {
	return SourceOptionValue{kind: sourceOptionParseLimit, parseKind: parseLimitTokenSize, value: value}
}

// SchemaMaxQNameInternEntries sets the schema QName interning cache size (0 leaves xmlstream default).
func SchemaMaxQNameInternEntries(value int) SourceOptionValue {
	return SourceOptionValue{kind: sourceOptionParseLimit, parseKind: parseLimitQNameInternEntries, value: value}
}

// MaxDFAStates sets the runtime DFA state limit (0 uses default).
func MaxDFAStates(value uint32) BuildOptionValue {
	return BuildOptionValue{kind: buildLimitDFAStates, value: value}
}

// MaxOccursLimit sets the runtime maxOccurs compilation limit (0 uses default).
func MaxOccursLimit(value uint32) BuildOptionValue {
	return BuildOptionValue{kind: buildLimitOccurs, value: value}
}

// InstanceMaxDepth sets the instance XML max depth limit (0 uses default).
func InstanceMaxDepth(value int) ValidateOptionValue {
	return ValidateOptionValue{kind: parseLimitDepth, value: value}
}

// InstanceMaxAttrs sets the instance XML max attributes limit (0 uses default).
func InstanceMaxAttrs(value int) ValidateOptionValue {
	return ValidateOptionValue{kind: parseLimitAttrs, value: value}
}

// InstanceMaxTokenSize sets the instance XML max token size limit (0 uses default).
func InstanceMaxTokenSize(value int) ValidateOptionValue {
	return ValidateOptionValue{kind: parseLimitTokenSize, value: value}
}

// InstanceMaxQNameInternEntries sets the instance QName interning cache size (0 leaves xmlstream default).
func InstanceMaxQNameInternEntries(value int) ValidateOptionValue {
	return ValidateOptionValue{kind: parseLimitQNameInternEntries, value: value}
}

func (o SourceOptionValue) applyCompile(cfg *compileConfig) {
	o.applySource(&cfg.source)
}

func (o SourceOptionValue) applySource(cfg *sourceConfig) {
	switch o.kind {
	case sourceOptionAllowMissingImportLocations:
		cfg.allowMissingImportLocations = true
	case sourceOptionParseLimit:
		switch o.parseKind {
		case parseLimitDepth:
			cfg.parseLimits.maxDepth = intOption{value: o.value, set: true}
		case parseLimitAttrs:
			cfg.parseLimits.maxAttrs = intOption{value: o.value, set: true}
		case parseLimitTokenSize:
			cfg.parseLimits.maxTokenSize = intOption{value: o.value, set: true}
		case parseLimitQNameInternEntries:
			cfg.parseLimits.maxQNameInternEntries = intOption{value: o.value, set: true}
		}
	}
}

func (o BuildOptionValue) applyCompile(cfg *compileConfig) {
	o.applyBuild(&cfg.build)
}

func (o BuildOptionValue) applyBuild(cfg *buildConfig) {
	switch o.kind {
	case buildLimitDFAStates:
		cfg.maxDFAStates = uint32Option{value: o.value, set: true}
	case buildLimitOccurs:
		cfg.maxOccursLimit = uint32Option{value: o.value, set: true}
	}
}

func (o ValidateOptionValue) applyValidate(cfg *validateConfig) {
	switch o.kind {
	case parseLimitDepth:
		cfg.parseLimits.maxDepth = intOption{value: o.value, set: true}
	case parseLimitAttrs:
		cfg.parseLimits.maxAttrs = intOption{value: o.value, set: true}
	case parseLimitTokenSize:
		cfg.parseLimits.maxTokenSize = intOption{value: o.value, set: true}
	case parseLimitQNameInternEntries:
		cfg.parseLimits.maxQNameInternEntries = intOption{value: o.value, set: true}
	}
}

func applyCompileOptions(cfg *compileConfig, opts []CompileOption) {
	for _, opt := range opts {
		if opt != nil {
			opt.applyCompile(cfg)
		}
	}
}

func applySourceOptions(cfg *sourceConfig, opts []SourceOption) {
	for _, opt := range opts {
		if opt != nil {
			opt.applySource(cfg)
		}
	}
}

func applyBuildOptions(cfg *buildConfig, opts []BuildOption) {
	for _, opt := range opts {
		if opt != nil {
			opt.applyBuild(cfg)
		}
	}
}

func applyValidateOptions(cfg *validateConfig, opts []ValidateOption) {
	for _, opt := range opts {
		if opt != nil {
			opt.applyValidate(cfg)
		}
	}
}
