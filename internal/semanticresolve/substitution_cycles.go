package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateNoCyclicSubstitutionGroups checks for cycles in substitution group chains.
func validateNoCyclicSubstitutionGroups(sch *parser.Schema) error {
	for _, startQName := range sortedQNames(sch.ElementDecls) {
		decl := sch.ElementDecls[startQName]
		if decl.SubstitutionGroup.IsZero() {
			continue
		}

		detector := NewCycleDetector[types.QName]()
		if err := visitSubstitutionGroupChain(sch, startQName, detector); err != nil {
			if IsCycleError(err) {
				return fmt.Errorf("cyclic substitution group detected: element %s is part of a cycle", startQName)
			}
			return err
		}
	}

	return nil
}

func visitSubstitutionGroupChain(sch *parser.Schema, qname types.QName, detector *CycleDetector[types.QName]) error {
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
		return visitSubstitutionGroupChain(sch, next, detector)
	})
}
