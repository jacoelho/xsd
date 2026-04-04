package analysis

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func detectAttributeGroupCycles(schema *parser.Schema) error {
	ctx := NewAttributeGroupContext(schema, AttributeGroupWalkOptions{
		Missing: MissingError,
		Cycles:  CycleError,
	})

	return parser.ForEachGlobalAttributeGroup(&schema.SchemaGraph, func(name model.QName, group *model.AttributeGroup) error {
		if group == nil {
			return fmt.Errorf("missing attributeGroup %s", name)
		}
		if err := ctx.Walk([]model.QName{name}, nil); err != nil {
			var cycleErr AttributeGroupCycleError
			if errors.As(err, &cycleErr) {
				return fmt.Errorf("attributeGroup cycle detected at %s", cycleErr.QName)
			}
			var missingErr AttributeGroupMissingError
			if errors.As(err, &missingErr) {
				return fmt.Errorf("attributeGroup %s ref %s not found", name, missingErr.QName)
			}
			return err
		}
		return nil
	})
}
