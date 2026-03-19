package xsd

import "fmt"

func (o SourceOptions) withDefaults() (resolvedSourceOptions, error) {
	schemaLimits, err := resolveXMLParseLimits(
		o.schemaMaxDepth.resolved(),
		o.schemaMaxAttrs.resolved(),
		o.schemaMaxTokenSize.resolved(),
		o.schemaMaxQNameInternEntries.resolved(),
	)
	if err != nil {
		return resolvedSourceOptions{}, fmt.Errorf("schema xml limits: %w", err)
	}
	return resolvedSourceOptions{
		allowMissingImportLocations: o.allowMissingImportLocations,
		schemaLimits:                schemaLimits,
	}, nil
}

func (o BuildOptions) withDefaults() resolvedBuildOptions {
	return resolvedBuildOptions{
		maxDFAStates:   o.maxDFAStates.resolved(),
		maxOccursLimit: o.maxOccursLimit.resolved(),
	}
}

func (o ValidateOptions) withDefaults() (resolvedValidateOptions, error) {
	instanceLimits, err := resolveXMLParseLimits(
		o.instanceMaxDepth.resolved(),
		o.instanceMaxAttrs.resolved(),
		o.instanceMaxTokenSize.resolved(),
		o.instanceMaxQNameInternEntries.resolved(),
	)
	if err != nil {
		return resolvedValidateOptions{}, fmt.Errorf("instance xml limits: %w", err)
	}
	return resolvedValidateOptions{
		instanceLimits:       instanceLimits,
		instanceParseOptions: instanceLimits.options(),
	}, nil
}

func defaultResolvedValidateOptions() resolvedValidateOptions {
	opts, err := NewValidateOptions().withDefaults()
	if err != nil {
		panic(err)
	}
	return opts
}
