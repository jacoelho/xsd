package semanticresolve

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/graphcycle"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

// validateNoCyclicSubstitutionGroups checks for cycles in substitution group chains.
func validateNoCyclicSubstitutionGroups(sch *parser.Schema) error {
	for _, startQName := range traversal.SortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[startQName]
		if decl.SubstitutionGroup.IsZero() {
			continue
		}

		err := graphcycle.Detect(graphcycle.Config[types.QName]{
			Starts:  []types.QName{startQName},
			Missing: graphcycle.MissingPolicyIgnore,
			Exists: func(name types.QName) bool {
				return sch.ElementDecls[name] != nil
			},
			Next: func(name types.QName) ([]types.QName, error) {
				decl, exists := sch.ElementDecls[name]
				if !exists || decl.SubstitutionGroup.IsZero() {
					return nil, nil
				}
				return []types.QName{decl.SubstitutionGroup}, nil
			},
		})
		if err != nil {
			var cycleErr graphcycle.CycleError[types.QName]
			if errors.As(err, &cycleErr) {
				return fmt.Errorf("cyclic substitution group detected: element %s is part of a cycle", startQName)
			}
			return err
		}
	}

	return nil
}
