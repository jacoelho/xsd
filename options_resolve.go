package xsd

import "fmt"

func (o parseLimitOptions) resolve(label string) (xmlParseLimits, error) {
	limits, err := resolveXMLParseLimits(
		o.maxDepth.resolved(),
		o.maxAttrs.resolved(),
		o.maxTokenSize.resolved(),
		o.maxQNameInternEntries.resolved(),
	)
	if err != nil {
		return xmlParseLimits{}, fmt.Errorf("%s xml limits: %w", label, err)
	}
	return limits, nil
}

func (o SourceOptions) withDefaults() (resolvedSourceOptions, error) {
	schemaLimits, err := o.parseLimits.resolve("schema")
	if err != nil {
		return resolvedSourceOptions{}, err
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
	instanceLimits, err := o.parseLimits.resolve("instance")
	if err != nil {
		return resolvedValidateOptions{}, err
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
