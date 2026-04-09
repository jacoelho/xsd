package compiler

import "github.com/jacoelho/xsd/internal/parser"

// Apply merges a parsed source schema into a parsed target schema.
func Apply(target, source *parser.Schema, kind Kind, remap NamespaceMode, insertAt int) error {
	staging := parser.CloneSchemaForMerge(target)
	ctx := newMergeContext(staging, source, kind, remap)
	ctx.mergeImportedNamespaces()
	ctx.mergeImportContexts()
	if err := ctx.mergeElementDecls(); err != nil {
		return err
	}
	if err := ctx.mergeTypeDefs(); err != nil {
		return err
	}
	if err := ctx.mergeAttributeDecls(); err != nil {
		return err
	}
	if err := ctx.mergeAttributeGroups(); err != nil {
		return err
	}
	if err := ctx.mergeGroups(); err != nil {
		return err
	}
	ctx.mergeSubstitutionGroups()
	if err := ctx.mergeNotationDecls(); err != nil {
		return err
	}
	if err := ctx.mergeIDAttributes(); err != nil {
		return err
	}
	ctx.mergeGlobalDecls(insertAt)
	*target = *staging
	return nil
}
