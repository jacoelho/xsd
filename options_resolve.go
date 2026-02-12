package xsd

import "fmt"

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
