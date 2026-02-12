package runtimeassemble

import "github.com/jacoelho/xsd/internal/runtime"

func (b *schemaBuilder) build() (*runtime.Schema, error) {
	if err := b.initSymbols(); err != nil {
		return nil, err
	}
	if b.err != nil {
		return nil, b.err
	}
	rt, err := b.builder.Build()
	if err != nil {
		return nil, err
	}
	b.rt = rt
	b.rt.RootPolicy = runtime.RootStrict
	b.rt.Validators = b.validators.Validators
	b.rt.Facets = b.validators.Facets
	b.rt.Patterns = b.validators.Patterns
	b.rt.Enums = b.validators.Enums
	b.rt.Values = b.validators.Values
	b.rt.Notations = b.notations
	b.wildcards = make([]runtime.WildcardRule, 1)

	if err := b.initIDs(); err != nil {
		return nil, err
	}
	if err := b.buildTypes(); err != nil {
		return nil, err
	}
	if err := b.buildAncestors(); err != nil {
		return nil, err
	}
	if err := b.buildAttributes(); err != nil {
		return nil, err
	}
	if err := b.buildElements(); err != nil {
		return nil, err
	}
	if err := b.buildModels(); err != nil {
		return nil, err
	}
	if err := b.buildIdentityConstraints(); err != nil {
		return nil, err
	}

	b.rt.Wildcards = b.wildcards
	b.rt.WildcardNS = b.wildcardNS
	b.rt.Paths = b.paths

	b.rt.BuildHash = computeBuildHash(b.rt)

	return b.rt, nil
}
