package semantics

import (
	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

func mergeAttributes(schema *parser.Schema, attrs []*model.AttributeDecl, groups []model.QName, attrMap map[model.QName]*model.AttributeDecl) error {
	for _, attr := range attrs {
		if attr == nil {
			continue
		}
		key := parser.EffectiveAttributeQName(schema, attr)
		attrMap[key] = attr
	}
	return mergeAttributesFromGroups(schema, groups, attrMap)
}

func mergeAttributesFromGroups(schema *parser.Schema, groups []model.QName, attrMap map[model.QName]*model.AttributeDecl) error {
	return analysis.WalkAttributeGroups(schema, groups, analysis.MissingError, func(_ model.QName, group *model.AttributeGroup) error {
		for _, attr := range group.Attributes {
			if attr == nil || attr.Use == model.Prohibited {
				// W3C attZ015: ignore prohibited uses from attribute groups.
				continue
			}
			key := parser.EffectiveAttributeQName(schema, attr)
			attrMap[key] = attr
		}
		return nil
	})
}
