package analysis

import (
	"errors"
	"fmt"

	"github.com/jacoelho/xsd/internal/globaldecl"
	"github.com/jacoelho/xsd/internal/graphcycle"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
)

func detectTypeCycles(schema *parser.Schema) error {
	starts := make([]types.QName, 0, len(schema.TypeDefs))
	if err := globaldecl.ForEachType(schema, func(name types.QName, typ types.Type) error {
		if typ == nil {
			return fmt.Errorf("missing global type %s", name)
		}
		starts = append(starts, name)
		return nil
	}); err != nil {
		return err
	}

	err := graphcycle.Detect(graphcycle.Config[types.QName]{
		Starts:  starts,
		Missing: graphcycle.MissingPolicyError,
		Exists: func(name types.QName) bool {
			return schema.TypeDefs[name] != nil
		},
		Next: func(name types.QName) ([]types.QName, error) {
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
			if baseName.IsZero() || baseName.Namespace == types.XSDNamespace {
				return nil, nil
			}
			return []types.QName{baseName}, nil
		},
	})
	if err == nil {
		return nil
	}
	var cycleErr graphcycle.CycleError[types.QName]
	if errors.As(err, &cycleErr) {
		return fmt.Errorf("type cycle detected at %s", cycleErr.Key)
	}
	var missingErr graphcycle.MissingError[types.QName]
	if errors.As(err, &missingErr) {
		return fmt.Errorf("missing global type %s", missingErr.Key)
	}
	return err
}
