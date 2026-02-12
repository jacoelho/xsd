package analysis

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/attrgroupwalk"
	"github.com/jacoelho/xsd/internal/globaldecl"
	parser "github.com/jacoelho/xsd/internal/parser"
	model "github.com/jacoelho/xsd/internal/types"
)

func detectAttributeGroupCycles(schema *parser.Schema) error {
	ctx := attrgroupwalk.NewContext(schema, attrgroupwalk.Options{
		Missing: attrgroupwalk.MissingError,
		Cycles:  attrgroupwalk.CycleError,
	})

	return globaldecl.ForEachAttributeGroup(schema, func(name model.QName, group *model.AttributeGroup) error {
		if group == nil {
			return fmt.Errorf("missing attributeGroup %s", name)
		}
		if err := ctx.Walk([]model.QName{name}, nil); err != nil {
			var cycleErr attrgroupwalk.AttrGroupCycleError
			if errors.As(err, &cycleErr) {
				return fmt.Errorf("attributeGroup cycle detected at %s", cycleErr.QName)
			}
			var missingErr attrgroupwalk.AttrGroupMissingError
			if errors.As(err, &missingErr) {
				return fmt.Errorf("attributeGroup %s ref %s not found", name, missingErr.QName)
			}
			return err
		}
		return nil
	})
}
