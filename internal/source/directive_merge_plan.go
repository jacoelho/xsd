package source

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/loadmerge"
	"github.com/jacoelho/xsd/internal/parser"
)

type directiveMergePlan struct {
	kind   loadmerge.Kind
	remap  loadmerge.NamespaceRemapMode
	insert int
}

func (l *SchemaLoader) planIncludeMerge(includingNS string, targetEntry *schemaEntry, target *parser.Schema, includeInfo parser.IncludeInfo, schemaLocation string, source *parser.Schema) (directiveMergePlan, error) {
	if !l.isIncludeNamespaceCompatible(includingNS, source.TargetNamespace) {
		return directiveMergePlan{}, fmt.Errorf("included schema %s has different target namespace: %s != %s", schemaLocation, source.TargetNamespace, includingNS)
	}
	remap := loadmerge.KeepNamespace
	if includingNS != "" && source.TargetNamespace == "" {
		remap = loadmerge.RemapNamespace
	}
	insert, err := includeInsertIndex(targetEntry, includeInfo, len(target.GlobalDecls))
	if err != nil {
		return directiveMergePlan{}, err
	}
	return directiveMergePlan{
		kind:   loadmerge.MergeInclude,
		remap:  remap,
		insert: insert,
	}, nil
}

func (l *SchemaLoader) planImportMerge(schemaLocation, expectedNS string, source *parser.Schema, insertAt int) (directiveMergePlan, error) {
	if expectedNS == "" {
		if source.TargetNamespace != "" {
			return directiveMergePlan{}, fmt.Errorf("imported schema %s namespace mismatch: expected no namespace, got %s", schemaLocation, source.TargetNamespace)
		}
	} else if source.TargetNamespace != expectedNS {
		return directiveMergePlan{}, fmt.Errorf("imported schema %s namespace mismatch: expected %s, got %s", schemaLocation, expectedNS, source.TargetNamespace)
	}
	return directiveMergePlan{
		kind:   loadmerge.MergeImport,
		remap:  loadmerge.KeepNamespace,
		insert: insertAt,
	}, nil
}

func (l *SchemaLoader) applyDirectiveMerge(
	target, source *parser.Schema,
	plan directiveMergePlan,
	directiveLabel string,
	schemaLocation string,
) (int, error) {
	beforeLen := len(target.GlobalDecls)
	if err := l.mergeSchema(target, source, plan.kind, plan.remap, plan.insert); err != nil {
		return 0, fmt.Errorf("merge %s schema %s: %w", directiveLabel, schemaLocation, err)
	}
	return len(target.GlobalDecls) - beforeLen, nil
}
