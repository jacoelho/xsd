package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	"github.com/jacoelho/xsd/internal/types"
)

// validateNoCyclicAttributeGroups detects cycles between attribute group definitions.
func validateNoCyclicAttributeGroups(sch *parser.Schema) error {
	detector := NewCycleDetector[types.QName]()
	var visit func(types.QName) error
	visit = func(qname types.QName) error {
		if detector.IsVisited(qname) {
			return nil
		}
		return detector.WithScope(qname, func() error {
			group, exists := sch.AttributeGroups[qname]
			if !exists {
				return nil
			}
			for _, ref := range group.AttrGroups {
				if _, ok := sch.AttributeGroups[ref]; !ok {
					continue
				}
				if err := visit(ref); err != nil {
					return err
				}
			}
			return nil
		})
	}
	for _, qname := range traversal.SortedQNames(sch.AttributeGroups) {
		if err := visit(qname); err != nil {
			return err
		}
	}
	return nil
}
