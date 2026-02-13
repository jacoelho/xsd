package analysis

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/attrgroupwalk"
	"github.com/jacoelho/xsd/internal/globaldecl"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func detectAttributeGroupCycles(schema *parser.Schema) error {
	ctx := attrgroupwalk.NewContext(schema, attrgroupwalk.Options{
		Missing: attrgroupwalk.MissingError,
		Cycles:  attrgroupwalk.CycleError,
	})

	return globaldecl.ForEachAttributeGroup(schema, func(name types.QName, group *types.AttributeGroup) error {
		if group == nil {
			return fmt.Errorf("missing attributeGroup %s", name)
		}
		if err := ctx.Walk([]types.QName{name}, nil); err != nil {
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
