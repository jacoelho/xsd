package xsd

// NewSourceOptions returns a default, valid source options value.
func NewSourceOptions() SourceOptions {
	return SourceOptions{}
}

// NewBuildOptions returns a default, valid build options value.
func NewBuildOptions() BuildOptions {
	return BuildOptions{}
}

// NewValidateOptions returns a default, valid validate options value.
func NewValidateOptions() ValidateOptions {
	return ValidateOptions{}
}

// Validate validates source options values.
func (o SourceOptions) Validate() error {
	_, err := o.withDefaults()
	return err
}

// Validate validates build options values.
func (o BuildOptions) Validate() error {
	_ = o.withDefaults()
	return nil
}

// Validate validates validate options values.
func (o ValidateOptions) Validate() error {
	_, err := o.withDefaults()
	return err
}

// WithAllowMissingImportLocations controls whether imports without schemaLocation are skipped.
func (o SourceOptions) WithAllowMissingImportLocations(value bool) SourceOptions {
	o.allowMissingImportLocations = value
	return o
}

// WithSchemaMaxDepth sets the schema XML max depth limit (0 uses default).
func (o SourceOptions) WithSchemaMaxDepth(value int) SourceOptions {
	o.parseLimits.maxDepth = intOption{value: value, set: true}
	return o
}

// WithSchemaMaxAttrs sets the schema XML max attributes limit (0 uses default).
func (o SourceOptions) WithSchemaMaxAttrs(value int) SourceOptions {
	o.parseLimits.maxAttrs = intOption{value: value, set: true}
	return o
}

// WithSchemaMaxTokenSize sets the schema XML max token size limit (0 uses default).
func (o SourceOptions) WithSchemaMaxTokenSize(value int) SourceOptions {
	o.parseLimits.maxTokenSize = intOption{value: value, set: true}
	return o
}

// WithSchemaMaxQNameInternEntries sets the schema QName interning cache size (0 leaves xmlstream default).
func (o SourceOptions) WithSchemaMaxQNameInternEntries(value int) SourceOptions {
	o.parseLimits.maxQNameInternEntries = intOption{value: value, set: true}
	return o
}

// WithMaxDFAStates sets the runtime DFA state limit (0 uses default).
func (o BuildOptions) WithMaxDFAStates(value uint32) BuildOptions {
	o.maxDFAStates = uint32Option{value: value, set: true}
	return o
}

// WithMaxOccursLimit sets the runtime maxOccurs compilation limit (0 uses default).
func (o BuildOptions) WithMaxOccursLimit(value uint32) BuildOptions {
	o.maxOccursLimit = uint32Option{value: value, set: true}
	return o
}

// WithInstanceMaxDepth sets the instance XML max depth limit (0 uses default).
func (o ValidateOptions) WithInstanceMaxDepth(value int) ValidateOptions {
	o.parseLimits.maxDepth = intOption{value: value, set: true}
	return o
}

// WithInstanceMaxAttrs sets the instance XML max attributes limit (0 uses default).
func (o ValidateOptions) WithInstanceMaxAttrs(value int) ValidateOptions {
	o.parseLimits.maxAttrs = intOption{value: value, set: true}
	return o
}

// WithInstanceMaxTokenSize sets the instance XML max token size limit (0 uses default).
func (o ValidateOptions) WithInstanceMaxTokenSize(value int) ValidateOptions {
	o.parseLimits.maxTokenSize = intOption{value: value, set: true}
	return o
}

// WithInstanceMaxQNameInternEntries sets the instance QName interning cache size (0 leaves xmlstream default).
func (o ValidateOptions) WithInstanceMaxQNameInternEntries(value int) ValidateOptions {
	o.parseLimits.maxQNameInternEntries = intOption{value: value, set: true}
	return o
}
