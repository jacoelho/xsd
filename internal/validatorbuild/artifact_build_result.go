package validatorbuild

import (
	"maps"

	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
)

func (c *artifactCompiler) result(registry *analysis.Registry) *ValidatorArtifacts {
	out := &ValidatorArtifacts{
		Validators:      c.bundle,
		Facets:          c.facets,
		Patterns:        c.patterns,
		Enums:           c.enums.table(),
		Values:          c.values.table(),
		ComplexTypes:    c.complexTypes,
		TypeValidators:  make(map[analysis.TypeID]runtime.ValidatorID),
		ValidatorByType: make(map[model.Type]runtime.ValidatorID),
		elements:        c.elements,
		attributes:      c.attributes,
		attrUses:        c.attrUses,
	}
	maps.Copy(out.ValidatorByType, c.validatorByType)
	for _, entry := range registry.TypeOrder {
		st, ok := model.AsSimpleType(entry.Type)
		if !ok {
			continue
		}
		key := c.canonicalTypeKey(st)
		if id, ok := c.validatorByType[key]; ok {
			out.TypeValidators[entry.ID] = id
		}
	}
	return out
}
