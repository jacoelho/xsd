package validatorgen

import (
	"github.com/jacoelho/xsd/internal/attrgroupwalk"
	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

func mergeAttributes(schema *parser.Schema, attrs []*model.AttributeDecl, groups []model.QName, attrMap map[model.QName]*model.AttributeDecl) error {
	for _, attr := range attrs {
		if attr == nil {
			continue
		}
		key := typeresolve.EffectiveAttributeQName(schema, attr)
		attrMap[key] = attr
	}
	return mergeAttributesFromGroups(schema, groups, attrMap)
}

func mergeAttributesFromGroups(schema *parser.Schema, groups []model.QName, attrMap map[model.QName]*model.AttributeDecl) error {
	return attrgroupwalk.Walk(schema, groups, attrgroupwalk.MissingError, func(_ model.QName, group *model.AttributeGroup) error {
		for _, attr := range group.Attributes {
			if attr == nil || attr.Use == model.Prohibited {
				// W3C attZ015: ignore prohibited uses from attribute groups.
				continue
			}
			key := typeresolve.EffectiveAttributeQName(schema, attr)
			attrMap[key] = attr
		}
		return nil
	})
}
