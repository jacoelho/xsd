package preprocessor

import "github.com/jacoelho/xsd/internal/parser"

// MergeSchema merges a parsed source schema into a parsed target schema.
func MergeSchema(target, source *parser.Schema, kind MergeKind, remap NamespaceRemapMode, insertAt int) error {
	staging := parser.CloneSchemaForMerge(target)
	ctx := newMergeContext(staging, source, kind, remap)
	existingDecls := existingGlobalDecls(staging)
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
	ctx.mergeGlobalDecls(existingDecls, insertAt)
	*target = *staging
	return nil
}
