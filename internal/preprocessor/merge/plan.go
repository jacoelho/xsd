package merge

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// Plan captures the deterministic merge parameters for one include/import.
type Plan struct {
	kind   Kind
	remap  NamespaceMode
	insert int
}

// PlanInclude derives the merge plan for one include directive.
func PlanInclude(includingNS string, targetInserted []int, target *parser.Schema, includeInfo parser.IncludeInfo, schemaLocation string, source *parser.Schema) (Plan, error) {
	if !includeNamespaceCompatible(includingNS, source.TargetNamespace) {
		return Plan{}, fmt.Errorf("included schema %s has different target namespace: %s != %s", schemaLocation, source.TargetNamespace, includingNS)
	}
	remap := KeepNamespace
	if includingNS != "" && source.TargetNamespace == "" {
		remap = RemapNamespace
	}
	insert, err := includeInsertIndex(targetInserted, includeInfo, len(target.GlobalDecls))
	if err != nil {
		return Plan{}, err
	}
	return Plan{
		kind:   Include,
		remap:  remap,
		insert: insert,
	}, nil
}

// PlanImport derives the merge plan for one import directive.
func PlanImport(schemaLocation, expectedNS string, source *parser.Schema, insertAt int) (Plan, error) {
	if expectedNS == "" {
		if source.TargetNamespace != "" {
			return Plan{}, fmt.Errorf("imported schema %s namespace mismatch: expected no namespace, got %s", schemaLocation, source.TargetNamespace)
		}
	} else if source.TargetNamespace != expectedNS {
		return Plan{}, fmt.Errorf("imported schema %s namespace mismatch: expected %s, got %s", schemaLocation, expectedNS, source.TargetNamespace)
	}
	return Plan{
		kind:   Import,
		remap:  KeepNamespace,
		insert: insertAt,
	}, nil
}

// ApplyPlanned applies one previously derived merge plan.
func ApplyPlanned(target, source *parser.Schema, plan Plan, directiveLabel, schemaLocation string) (int, error) {
	beforeLen := len(target.GlobalDecls)
	if err := Apply(target, source, plan.kind, plan.remap, plan.insert); err != nil {
		return 0, fmt.Errorf("merge %s schema %s: %w", directiveLabel, schemaLocation, err)
	}
	return len(target.GlobalDecls) - beforeLen, nil
}

func includeNamespaceCompatible(includingNS, includedNS model.NamespaceURI) bool {
	if includingNS == includedNS {
		return true
	}
	return includingNS != "" && includedNS == ""
}

func includeInsertIndex(inserted []int, include parser.IncludeInfo, currentDecls int) (int, error) {
	if include.IncludeIndex < 0 || include.IncludeIndex >= len(inserted) {
		return 0, fmt.Errorf("include index %d out of range", include.IncludeIndex)
	}
	if include.DeclIndex < 0 {
		return 0, fmt.Errorf("include decl index %d out of range", include.DeclIndex)
	}
	offset := 0
	for i := 0; i < include.IncludeIndex; i++ {
		offset += inserted[i]
	}
	insertAt := include.DeclIndex + offset
	if insertAt > currentDecls {
		return 0, fmt.Errorf("include position %d out of range (decls=%d)", insertAt, currentDecls)
	}
	return insertAt, nil
}

// RecordIncludeInserted updates the running inserted-counts for include ordering.
func RecordIncludeInserted(inserted []int, includeIndex, count int) error {
	if includeIndex < 0 || includeIndex >= len(inserted) {
		return fmt.Errorf("include index %d out of range", includeIndex)
	}
	if count < 0 {
		return fmt.Errorf("include inserted count %d out of range", count)
	}
	inserted[includeIndex] += count
	return nil
}
