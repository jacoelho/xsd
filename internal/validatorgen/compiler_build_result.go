package validatorgen

import (
	"maps"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/runtime"
	schema "github.com/jacoelho/xsd/internal/schemaanalysis"
)

func (c *compiler) result(registry *schema.Registry) *compiledValidators {
	out := &compiledValidators{
		Validators:      c.bundle,
		Facets:          c.facets,
		Patterns:        c.patterns,
		Enums:           c.enums.table(),
		Values:          c.values.table(),
		TypeValidators:  make(map[schema.TypeID]runtime.ValidatorID),
		ValidatorByType: make(map[model.Type]runtime.ValidatorID),
		elements:        c.elements,
		attributes:      c.attributes,
		attrUses:        c.attrUses,
	}
	if len(c.simpleContent) > 0 {
		out.SimpleContentTypes = make(map[*model.ComplexType]model.Type, len(c.simpleContent))
		maps.Copy(out.SimpleContentTypes, c.simpleContent)
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
