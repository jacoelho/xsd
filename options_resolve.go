package xsd

import "fmt"

func (c XMLConfig) resolve(label string) (xmlParseLimits, error) {
	limits, err := resolveXMLParseLimits(
		c.MaxDepth,
		c.MaxAttrs,
		c.MaxTokenSize,
		c.MaxNameInternEntries,
	)
	if err != nil {
		return xmlParseLimits{}, fmt.Errorf("%s xml limits: %w", label, err)
	}
	return limits, nil
}

func (c SourceConfig) withDefaults() (resolvedSourceOptions, error) {
	schemaLimits, err := c.XML.resolve("schema")
	if err != nil {
		return resolvedSourceOptions{}, err
	}
	return resolvedSourceOptions{
		allowMissingImportLocations: c.AllowMissingImportLocations,
		schemaLimits:                schemaLimits,
	}, nil
}

func (c BuildConfig) withDefaults() resolvedBuildOptions {
	return resolvedBuildOptions{
		maxDFAStates:   c.MaxDFAStates,
		maxOccursLimit: c.MaxOccursLimit,
	}
}

func (c ValidateConfig) withDefaults() (resolvedValidateOptions, error) {
	instanceLimits, err := c.XML.resolve("instance")
	if err != nil {
		return resolvedValidateOptions{}, err
	}
	return resolvedValidateOptions{
		instanceLimits:       instanceLimits,
		instanceParseOptions: instanceLimits.options(),
	}, nil
}

func (c CompileConfig) withDefaults() (resolvedSourceOptions, resolvedBuildOptions, resolvedValidateOptions, error) {
	source, err := c.Source.withDefaults()
	if err != nil {
		return resolvedSourceOptions{}, resolvedBuildOptions{}, resolvedValidateOptions{}, err
	}
	validate, err := c.Validate.withDefaults()
	if err != nil {
		return resolvedSourceOptions{}, resolvedBuildOptions{}, resolvedValidateOptions{}, err
	}
	return source, c.Build.withDefaults(), validate, nil
}
