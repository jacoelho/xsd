package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
)

// validateNoCyclicAttributeGroups detects cycles between attribute group definitions.
func validateNoCyclicAttributeGroups(sch *parser.Schema) error {
	detector := NewCycleDetector[model.QName]()
	var visit func(model.QName) error
	visit = func(qname model.QName) error {
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
