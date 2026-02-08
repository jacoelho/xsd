package semanticresolve

import (
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

// validateNoCyclicAttributeGroups detects cycles between attribute group definitions.
func validateNoCyclicAttributeGroups(sch *parser.Schema) error {
	detector := NewCycleDetector[types.QName]()
	for _, qname := range sortedQNames(sch.AttributeGroups) {
		if err := visitAttributeGroup(sch, qname, detector); err != nil {
			return err
		}
	}
	return nil
}

func visitAttributeGroup(sch *parser.Schema, qname types.QName, detector *CycleDetector[types.QName]) error {
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
			if err := visitAttributeGroup(sch, ref, detector); err != nil {
				return err
			}
		}
		return nil
	})
}
