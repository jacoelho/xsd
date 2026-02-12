package semanticresolve

import (
	"errors"

	"github.com/jacoelho/xsd/internal/attrgroupwalk"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/traversal"
	model "github.com/jacoelho/xsd/internal/types"
)

// validateNoCyclicAttributeGroups detects cycles between attribute group definitions.
func validateNoCyclicAttributeGroups(sch *parser.Schema) error {
	ctx := attrgroupwalk.NewContext(sch, attrgroupwalk.Options{
		Missing: attrgroupwalk.MissingIgnore,
		Cycles:  attrgroupwalk.CycleError,
	})

	for _, qname := range traversal.SortedQNames(sch.AttributeGroups) {
		if err := ctx.Walk([]model.QName{qname}, nil); err != nil {
			var cycleErr attrgroupwalk.AttrGroupCycleError
			if errors.As(err, &cycleErr) {
				return CycleError[model.QName]{Key: cycleErr.QName}
			}
			return err
		}
	}
	return nil
}
