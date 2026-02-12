package xsd

// NewLoadOptions returns a default, valid load options value.
func NewLoadOptions() LoadOptions {
	return LoadOptions{}
}

// NewRuntimeOptions returns a default, valid runtime options value.
func NewRuntimeOptions() RuntimeOptions {
	return RuntimeOptions{}
}

// RuntimeOptions returns the runtime options embedded in the load options.
func (o LoadOptions) RuntimeOptions() RuntimeOptions {
	return o.runtime
}

// Validate validates load options values.
func (o LoadOptions) Validate() error {
	_, _, err := o.withDefaults()
	return err
}

// Validate validates runtime options values.
func (o RuntimeOptions) Validate() error {
	_, err := o.withDefaults()
	return err
}

// WithAllowMissingImportLocations controls whether imports without schemaLocation are skipped.
func (o LoadOptions) WithAllowMissingImportLocations(value bool) LoadOptions {
	o.allowMissingImportLocations = value
	return o
}

// WithSchemaMaxDepth sets the schema XML max depth limit (0 uses default).
func (o LoadOptions) WithSchemaMaxDepth(value int) LoadOptions {
	o.schemaMaxDepth = intOption{value: value, set: true}
	return o
}

// WithSchemaMaxAttrs sets the schema XML max attributes limit (0 uses default).
func (o LoadOptions) WithSchemaMaxAttrs(value int) LoadOptions {
	o.schemaMaxAttrs = intOption{value: value, set: true}
	return o
}

// WithSchemaMaxTokenSize sets the schema XML max token size limit (0 uses default).
func (o LoadOptions) WithSchemaMaxTokenSize(value int) LoadOptions {
	o.schemaMaxTokenSize = intOption{value: value, set: true}
	return o
}

// WithSchemaMaxQNameInternEntries sets the schema QName interning cache size (0 leaves xmlstream default).
func (o LoadOptions) WithSchemaMaxQNameInternEntries(value int) LoadOptions {
	o.schemaMaxQNameInternEntries = intOption{value: value, set: true}
	return o
}

// WithRuntimeOptions sets all runtime options in one call.
func (o LoadOptions) WithRuntimeOptions(value RuntimeOptions) LoadOptions {
	o.runtime = value
	return o
}

// WithMaxDFAStates sets the runtime DFA state limit (0 uses default).
func (o RuntimeOptions) WithMaxDFAStates(value uint32) RuntimeOptions {
	o.maxDFAStates = uint32Option{value: value, set: true}
	return o
}

// WithMaxOccursLimit sets the runtime maxOccurs compilation limit (0 uses default).
func (o RuntimeOptions) WithMaxOccursLimit(value uint32) RuntimeOptions {
	o.maxOccursLimit = uint32Option{value: value, set: true}
	return o
}

// WithInstanceMaxDepth sets the instance XML max depth limit (0 uses default).
func (o RuntimeOptions) WithInstanceMaxDepth(value int) RuntimeOptions {
	o.instanceMaxDepth = intOption{value: value, set: true}
	return o
}

// WithInstanceMaxAttrs sets the instance XML max attributes limit (0 uses default).
func (o RuntimeOptions) WithInstanceMaxAttrs(value int) RuntimeOptions {
	o.instanceMaxAttrs = intOption{value: value, set: true}
	return o
}

// WithInstanceMaxTokenSize sets the instance XML max token size limit (0 uses default).
func (o RuntimeOptions) WithInstanceMaxTokenSize(value int) RuntimeOptions {
	o.instanceMaxTokenSize = intOption{value: value, set: true}
	return o
}

// WithInstanceMaxQNameInternEntries sets the instance QName interning cache size (0 leaves xmlstream default).
func (o RuntimeOptions) WithInstanceMaxQNameInternEntries(value int) RuntimeOptions {
	o.instanceMaxQNameInternEntries = intOption{value: value, set: true}
	return o
}
