package analysis

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func detectSubstitutionGroupCycles(schema *parser.Schema) error {
	starts := make([]model.QName, 0, len(schema.ElementDecls))
	if err := parser.ForEachGlobalDecl(&schema.SchemaGraph, parser.GlobalDeclHandlers{
		Element: func(name model.QName, _ *model.ElementDecl) error {
			starts = append(starts, name)
			return nil
		},
	}); err != nil {
		return err
	}

	err := DetectGraphCycle(GraphCycleConfig[model.QName]{
		Starts:  starts,
		Missing: GraphCycleMissingPolicyError,
		Exists: func(name model.QName) bool {
			return schema.ElementDecls[name] != nil
		},
		Next: func(name model.QName) ([]model.QName, error) {
			decl := schema.ElementDecls[name]
			if decl == nil || decl.SubstitutionGroup.IsZero() {
				return nil, nil
			}
			return []model.QName{decl.SubstitutionGroup}, nil
		},
	})
	if err == nil {
		return nil
	}
	var cycleErr GraphCycleError[model.QName]
	if errors.As(err, &cycleErr) {
		return fmt.Errorf("substitution group cycle detected at %s", cycleErr.Key)
	}
	var missingErr GraphMissingError[model.QName]
	if errors.As(err, &missingErr) {
		if missingErr.From.IsZero() {
			return fmt.Errorf("element %s not found", missingErr.Key)
		}
		return fmt.Errorf("element %s substitutionGroup %s not found", missingErr.From, missingErr.Key)
	}
	return err
}
