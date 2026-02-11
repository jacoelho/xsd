package schemaanalysis

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/globaldecl"
	"github.com/jacoelho/xsd/internal/graphcycle"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func detectTypeCycles(schema *parser.Schema) error {
	starts := make([]model.QName, 0, len(schema.TypeDefs))
	if err := globaldecl.ForEachType(schema, func(name model.QName, typ model.Type) error {
		if typ == nil {
			return fmt.Errorf("missing global type %s", name)
		}
		starts = append(starts, name)
		return nil
	}); err != nil {
		return err
	}

	err := graphcycle.Detect(graphcycle.Config[model.QName]{
		Starts:  starts,
		Missing: graphcycle.MissingPolicyError,
		Exists: func(name model.QName) bool {
			return schema.TypeDefs[name] != nil
		},
		Next: func(name model.QName) ([]model.QName, error) {
			typ := schema.TypeDefs[name]
			if typ == nil {
				return nil, nil
			}
			baseType, _, err := baseTypeFor(schema, typ)
			if err != nil {
				return nil, fmt.Errorf("type %s: %w", name, err)
			}
			if baseType == nil {
				return nil, nil
			}
			baseName := baseType.Name()
			if baseName.IsZero() || baseName.Namespace == model.XSDNamespace {
				return nil, nil
			}
			return []model.QName{baseName}, nil
		},
	})
	if err == nil {
		return nil
	}
	var cycleErr graphcycle.CycleError[model.QName]
	if errors.As(err, &cycleErr) {
		return fmt.Errorf("type cycle detected at %s", cycleErr.Key)
	}
	var missingErr graphcycle.MissingError[model.QName]
	if errors.As(err, &missingErr) {
		return fmt.Errorf("missing global type %s", missingErr.Key)
	}
	return err
}
