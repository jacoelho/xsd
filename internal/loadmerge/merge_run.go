package loadmerge

import parser "github.com/jacoelho/xsd/internal/parser"

// Merge merges a source schema into a target schema.
// For imports, preserves source namespace.
// For includes, uses chameleon namespace remapping if needed.
func (DefaultMerger) Merge(target, source *parser.Schema, kind Kind, remap NamespaceRemapMode, insertAt int) error {
	staging := CloneSchemaForMerge(target)
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
