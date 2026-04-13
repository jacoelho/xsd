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

func (o sourceConfig) withDefaults() (resolvedSourceOptions, error) {
	schemaLimits, err := o.parseLimits.resolve("schema")
	if err != nil {
		return resolvedSourceOptions{}, err
	}
	return resolvedSourceOptions{
		allowMissingImportLocations: o.allowMissingImportLocations,
		schemaLimits:                schemaLimits,
	}, nil
}

func (o buildConfig) withDefaults() resolvedBuildOptions {
	return resolvedBuildOptions{
		maxDFAStates:   o.maxDFAStates.resolved(),
		maxOccursLimit: o.maxOccursLimit.resolved(),
	}
}

func (o validateConfig) withDefaults() (resolvedValidateOptions, error) {
	instanceLimits, err := o.parseLimits.resolve("instance")
	if err != nil {
		return resolvedValidateOptions{}, err
	}
	return resolvedValidateOptions{
		instanceLimits:       instanceLimits,
		instanceParseOptions: instanceLimits.options(),
	}, nil
}

func resolveCompileOptions(opts []CompileOption) (resolvedSourceOptions, resolvedBuildOptions, error) {
	var cfg compileConfig
	applyCompileOptions(&cfg, opts)

	source, err := cfg.source.withDefaults()
	if err != nil {
		return resolvedSourceOptions{}, resolvedBuildOptions{}, err
	}
	return source, cfg.build.withDefaults(), nil
}

func resolveBuildOptions(opts []BuildOption) resolvedBuildOptions {
	var cfg buildConfig
	applyBuildOptions(&cfg, opts)
	return cfg.withDefaults()
}

func resolveValidateOptions(opts []ValidateOption) (resolvedValidateOptions, error) {
	var cfg validateConfig
	applyValidateOptions(&cfg, opts)
	return cfg.withDefaults()
}

func defaultResolvedValidateOptions() resolvedValidateOptions {
	opts, err := resolveValidateOptions(nil)
	if err != nil {
		panic(err)
	}
	return opts
}
