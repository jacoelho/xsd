package runtimebuild

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
	b.assembler = runtime.NewAssembler(rt)
	if err := b.assembler.SetRootPolicy(runtime.RootStrict); err != nil {
		return nil, err
	}
	if err := b.assembler.SetValidators(b.artifacts.Validators); err != nil {
		return nil, err
	}
	if err := b.assembler.SetFacets(b.artifacts.Facets); err != nil {
		return nil, err
	}
	if err := b.assembler.SetPatterns(b.artifacts.Patterns); err != nil {
		return nil, err
	}
	if err := b.assembler.SetEnums(b.artifacts.Enums); err != nil {
		return nil, err
	}
	if err := b.assembler.SetValues(b.artifacts.Values); err != nil {
		return nil, err
	}
	if err := b.assembler.SetNotations(b.notations); err != nil {
		return nil, err
	}
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
	if err := b.applyGlobalIndexes(); err != nil {
		return nil, err
	}
	if err := b.buildModels(); err != nil {
		return nil, err
	}
	if err := b.buildIdentityConstraints(); err != nil {
		return nil, err
	}

	if err := b.assembler.SetWildcards(b.wildcards); err != nil {
		return nil, err
	}
	if err := b.assembler.SetWildcardNS(b.wildcardNS); err != nil {
		return nil, err
	}
	if err := b.assembler.SetPaths(b.paths); err != nil {
		return nil, err
	}

	if err := b.assembler.SetBuildHash(computeBuildHash(b.rt)); err != nil {
		return nil, err
	}

	return b.assembler.Seal()
}
