package semantics

import (
	"github.com/jacoelho/xsd/internal/analysis"
	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
)

// collectAllAttributesForValidation collects all attributes from a complex type.
// This includes attributes from extensions, restrictions, and attribute groups.
func collectAllAttributesForValidation(schema *parser.Schema, ct *model.ComplexType) []*model.AttributeDecl {
	var allAttrs []*model.AttributeDecl
	_ = walkComplexTypeLocalAttributes(schema, ct, analysis.AttributeGroupWalkOptions{
		Missing: analysis.MissingIgnore,
		Cycles:  analysis.CycleIgnore,
	}, func(attr *model.AttributeDecl, _ bool) error {
		allAttrs = append(allAttrs, attr)
		return nil
	})
	return allAttrs
}

func collectEffectiveAttributeUses(schema *parser.Schema, ct *model.ComplexType) map[model.QName]*model.AttributeDecl {
	if ct == nil {
		return nil
	}
	attrMap := make(map[model.QName]*model.AttributeDecl)
	_ = walkComplexTypeAttributeChain(schema, ct, analysis.AttributeGroupWalkOptions{
		Missing: analysis.MissingIgnore,
		Cycles:  analysis.CycleIgnore,
	}, func(_ *model.ComplexType, attr *model.AttributeDecl, _ bool) error {
		mergeValidationAttribute(schema, attr, attrMap)
		return nil
	})
	return attrMap
}

func mergeAttributesFromTypeForValidation(schema *parser.Schema, ct *model.ComplexType, attrMap map[model.QName]*model.AttributeDecl) {
	_ = walkComplexTypeLocalAttributes(schema, ct, analysis.AttributeGroupWalkOptions{
		Missing: analysis.MissingIgnore,
		Cycles:  analysis.CycleIgnore,
	}, func(attr *model.AttributeDecl, _ bool) error {
		mergeValidationAttribute(schema, attr, attrMap)
		return nil
	})
}

func mergeAttributesFromGroupsForValidation(schema *parser.Schema, agRefs []model.QName, attrMap map[model.QName]*model.AttributeDecl) {
	_ = walkAttributeGroupAttributes(schema, agRefs, analysis.AttributeGroupWalkOptions{
		Missing: analysis.MissingIgnore,
		Cycles:  analysis.CycleIgnore,
	}, func(attr *model.AttributeDecl, _ bool) error {
		mergeValidationAttribute(schema, attr, attrMap)
		return nil
	})
}

func mergeValidationAttribute(schema *parser.Schema, attr *model.AttributeDecl, attrMap map[model.QName]*model.AttributeDecl) {
	if attr == nil {
		return
	}
	key := parser.EffectiveAttributeQName(schema, attr)
	if attr.Use == model.Prohibited && !attr.HasFixed {
		delete(attrMap, key)
		return
	}
	attrMap[key] = attr
}

// collectAttributesFromGroups collects attributes from attribute group references.
func collectAttributesFromGroups(schema *parser.Schema, agRefs []model.QName) []*model.AttributeDecl {
	var result []*model.AttributeDecl
	_ = walkAttributeGroupAttributes(schema, agRefs, analysis.AttributeGroupWalkOptions{
		Missing: analysis.MissingIgnore,
		Cycles:  analysis.CycleIgnore,
	}, func(attr *model.AttributeDecl, _ bool) error {
		result = append(result, attr)
		return nil
	})
	return result
}
