package xsd

import (
	"fmt"

	"github.com/jacoelho/xsd/pkg/xmlstream"
)

// LoadOptions configures schema loading and default runtime compilation.
type LoadOptions struct {
	runtime                     RuntimeOptions
	allowMissingImportLocations bool
	schemaMaxDepth              int
	schemaMaxAttrs              int
	schemaMaxTokenSize          int
	schemaMaxQNameInternEntries int
}

// RuntimeOptions configures runtime compilation and instance XML limits.
type RuntimeOptions struct {
	maxDFAStates                  uint32
	maxOccursLimit                uint32
	instanceMaxDepth              int
	instanceMaxAttrs              int
	instanceMaxTokenSize          int
	instanceMaxQNameInternEntries int
}

type resolvedLoadOptions struct {
	schemaLimits                xmlParseLimits
	allowMissingImportLocations bool
}

type resolvedRuntimeOptions struct {
	instanceParseOptions []xmlstream.Option
	maxDFAStates         uint32
	maxOccursLimit       uint32
}

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
	o.schemaMaxDepth = value
	return o
}

// WithSchemaMaxAttrs sets the schema XML max attributes limit (0 uses default).
func (o LoadOptions) WithSchemaMaxAttrs(value int) LoadOptions {
	o.schemaMaxAttrs = value
	return o
}

// WithSchemaMaxTokenSize sets the schema XML max token size limit (0 uses default).
func (o LoadOptions) WithSchemaMaxTokenSize(value int) LoadOptions {
	o.schemaMaxTokenSize = value
	return o
}

// WithSchemaMaxQNameInternEntries sets the schema QName interning cache size (0 leaves xmlstream default).
func (o LoadOptions) WithSchemaMaxQNameInternEntries(value int) LoadOptions {
	o.schemaMaxQNameInternEntries = value
	return o
}

// WithRuntimeOptions sets all runtime options in one call.
func (o LoadOptions) WithRuntimeOptions(value RuntimeOptions) LoadOptions {
	o.runtime = value
	return o
}

// WithMaxDFAStates sets the runtime DFA state limit (0 uses default).
func (o LoadOptions) WithMaxDFAStates(value uint32) LoadOptions {
	o.runtime.maxDFAStates = value
	return o
}

// WithMaxOccursLimit sets the runtime maxOccurs compilation limit (0 uses default).
func (o LoadOptions) WithMaxOccursLimit(value uint32) LoadOptions {
	o.runtime.maxOccursLimit = value
	return o
}

// WithInstanceMaxDepth sets the instance XML max depth limit (0 uses default).
func (o LoadOptions) WithInstanceMaxDepth(value int) LoadOptions {
	o.runtime.instanceMaxDepth = value
	return o
}

// WithInstanceMaxAttrs sets the instance XML max attributes limit (0 uses default).
func (o LoadOptions) WithInstanceMaxAttrs(value int) LoadOptions {
	o.runtime.instanceMaxAttrs = value
	return o
}

// WithInstanceMaxTokenSize sets the instance XML max token size limit (0 uses default).
func (o LoadOptions) WithInstanceMaxTokenSize(value int) LoadOptions {
	o.runtime.instanceMaxTokenSize = value
	return o
}

// WithInstanceMaxQNameInternEntries sets the instance QName interning cache size (0 leaves xmlstream default).
func (o LoadOptions) WithInstanceMaxQNameInternEntries(value int) LoadOptions {
	o.runtime.instanceMaxQNameInternEntries = value
	return o
}

// WithMaxDFAStates sets the runtime DFA state limit (0 uses default).
func (o RuntimeOptions) WithMaxDFAStates(value uint32) RuntimeOptions {
	o.maxDFAStates = value
	return o
}

// WithMaxOccursLimit sets the runtime maxOccurs compilation limit (0 uses default).
func (o RuntimeOptions) WithMaxOccursLimit(value uint32) RuntimeOptions {
	o.maxOccursLimit = value
	return o
}

// WithInstanceMaxDepth sets the instance XML max depth limit (0 uses default).
func (o RuntimeOptions) WithInstanceMaxDepth(value int) RuntimeOptions {
	o.instanceMaxDepth = value
	return o
}

// WithInstanceMaxAttrs sets the instance XML max attributes limit (0 uses default).
func (o RuntimeOptions) WithInstanceMaxAttrs(value int) RuntimeOptions {
	o.instanceMaxAttrs = value
	return o
}

// WithInstanceMaxTokenSize sets the instance XML max token size limit (0 uses default).
func (o RuntimeOptions) WithInstanceMaxTokenSize(value int) RuntimeOptions {
	o.instanceMaxTokenSize = value
	return o
}

// WithInstanceMaxQNameInternEntries sets the instance QName interning cache size (0 leaves xmlstream default).
func (o RuntimeOptions) WithInstanceMaxQNameInternEntries(value int) RuntimeOptions {
	o.instanceMaxQNameInternEntries = value
	return o
}

func (o LoadOptions) withDefaults() (resolvedLoadOptions, resolvedRuntimeOptions, error) {
	schemaLimits, err := resolveXMLParseLimits(
		o.schemaMaxDepth,
		o.schemaMaxAttrs,
		o.schemaMaxTokenSize,
		o.schemaMaxQNameInternEntries,
	)
	if err != nil {
		return resolvedLoadOptions{}, resolvedRuntimeOptions{}, fmt.Errorf("schema xml limits: %w", err)
	}
	runtimeOpts, err := o.runtime.withDefaults()
	if err != nil {
		return resolvedLoadOptions{}, resolvedRuntimeOptions{}, fmt.Errorf("runtime options: %w", err)
	}
	return resolvedLoadOptions{
		allowMissingImportLocations: o.allowMissingImportLocations,
		schemaLimits:                schemaLimits,
	}, runtimeOpts, nil
}

func (o RuntimeOptions) withDefaults() (resolvedRuntimeOptions, error) {
	instanceLimits, err := resolveXMLParseLimits(
		o.instanceMaxDepth,
		o.instanceMaxAttrs,
		o.instanceMaxTokenSize,
		o.instanceMaxQNameInternEntries,
	)
	if err != nil {
		return resolvedRuntimeOptions{}, fmt.Errorf("instance xml limits: %w", err)
	}
	return resolvedRuntimeOptions{
		maxDFAStates:         o.maxDFAStates,
		maxOccursLimit:       o.maxOccursLimit,
		instanceParseOptions: instanceLimits.options(),
	}, nil
}
