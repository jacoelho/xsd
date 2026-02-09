package xsd

import (
	"fmt"

	"github.com/jacoelho/xsd/pkg/xmlstream"
)

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

// LoadOptions configures schema loading and default runtime compilation.
type LoadOptions struct {
	runtime                     RuntimeOptions
	allowMissingImportLocations bool
	schemaMaxDepth              intOption
	schemaMaxAttrs              intOption
	schemaMaxTokenSize          intOption
	schemaMaxQNameInternEntries intOption
}

// RuntimeOptions configures runtime compilation and instance XML limits.
type RuntimeOptions struct {
	maxDFAStates                  uint32Option
	maxOccursLimit                uint32Option
	instanceMaxDepth              intOption
	instanceMaxAttrs              intOption
	instanceMaxTokenSize          intOption
	instanceMaxQNameInternEntries intOption
}

type resolvedLoadOptions struct {
	schemaLimits                xmlParseLimits
	allowMissingImportLocations bool
}

type resolvedRuntimeOptions struct {
	instanceParseOptions []xmlstream.Option
	instanceLimits       xmlParseLimits
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

func (o LoadOptions) withDefaults() (resolvedLoadOptions, resolvedRuntimeOptions, error) {
	schemaLimits, err := resolveXMLParseLimits(
		o.schemaMaxDepth.resolved(),
		o.schemaMaxAttrs.resolved(),
		o.schemaMaxTokenSize.resolved(),
		o.schemaMaxQNameInternEntries.resolved(),
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
		o.instanceMaxDepth.resolved(),
		o.instanceMaxAttrs.resolved(),
		o.instanceMaxTokenSize.resolved(),
		o.instanceMaxQNameInternEntries.resolved(),
	)
	if err != nil {
		return resolvedRuntimeOptions{}, fmt.Errorf("instance xml limits: %w", err)
	}
	return resolvedRuntimeOptions{
		maxDFAStates:         o.maxDFAStates.resolved(),
		maxOccursLimit:       o.maxOccursLimit.resolved(),
		instanceLimits:       instanceLimits,
		instanceParseOptions: instanceLimits.options(),
	}, nil
}
