package semanticresolve

import (
	"errors"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/qname"
)

// validateNoCyclicAttributeGroups detects cycles between attribute group definitions.
func validateNoCyclicAttributeGroups(sch *parser.Schema) error {
	ctx := analysis.NewAttributeGroupContext(sch, analysis.AttributeGroupWalkOptions{
		Missing: analysis.MissingIgnore,
		Cycles:  analysis.CycleError,
	})

	for _, qname := range qname.SortedMapKeys(sch.AttributeGroups) {
		if err := ctx.Walk([]model.QName{qname}, nil); err != nil {
			var cycleErr analysis.AttributeGroupCycleError
			if errors.As(err, &cycleErr) {
				return CycleError[model.QName]{Key: cycleErr.QName}
			}
			return err
		}
	}
	return nil
}
