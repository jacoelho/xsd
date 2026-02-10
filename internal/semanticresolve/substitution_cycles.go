package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
)

// validateNoCyclicSubstitutionGroups checks for cycles in substitution group chains.
func validateNoCyclicSubstitutionGroups(sch *parser.Schema) error {
	for _, startQName := range traversal.SortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[startQName]
		if decl.SubstitutionGroup.IsZero() {
			continue
		}

		detector := NewCycleDetector[model.QName]()
		var visit func(model.QName) error
		visit = func(qname model.QName) error {
			if detector.IsVisited(qname) {
				return nil
			}
			return detector.WithScope(qname, func() error {
				decl, exists := sch.ElementDecls[qname]
				if !exists {
					return nil
				}
				next := decl.SubstitutionGroup
				if next.IsZero() {
					return nil
				}
				if _, ok := sch.ElementDecls[next]; !ok {
					return nil
				}
				return visit(next)
			})
		}
		if err := visit(startQName); err != nil {
			if IsCycleError(err) {
				return fmt.Errorf("cyclic substitution group detected: element %s is part of a cycle", startQName)
			}
			return err
		}
	}

	return nil
}
