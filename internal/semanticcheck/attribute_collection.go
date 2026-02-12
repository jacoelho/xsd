package semanticcheck

import (
	"slices"

	"github.com/jacoelho/xsd/internal/attrgroupwalk"
	"github.com/jacoelho/xsd/internal/model"
	parser "github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typechain"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

// collectAllAttributesForValidation collects all attributes from a complex type.
// This includes attributes from extensions, restrictions, and attribute groups.
func collectAllAttributesForValidation(schema *parser.Schema, ct *model.ComplexType) []*model.AttributeDecl {
	allAttrs := slices.Clone(ct.Attributes())
	allAttrs = append(allAttrs, collectAttributesFromGroups(schema, ct.AttrGroups)...)

	content := ct.Content()
	if content != nil {
		if ext := content.ExtensionDef(); ext != nil {
			allAttrs = append(allAttrs, ext.Attributes...)
			allAttrs = append(allAttrs, collectAttributesFromGroups(schema, ext.AttrGroups)...)
		}
		if restr := content.RestrictionDef(); restr != nil {
			allAttrs = append(allAttrs, restr.Attributes...)
			allAttrs = append(allAttrs, collectAttributesFromGroups(schema, restr.AttrGroups)...)
		}
	}

	return allAttrs
}

func collectEffectiveAttributeUses(schema *parser.Schema, ct *model.ComplexType) map[model.QName]*model.AttributeDecl {
	if ct == nil {
		return nil
	}
	chain := typechain.CollectComplexTypeChain(schema, ct, typechain.ComplexTypeChainExplicitBaseOnly)
	attrMap := make(map[model.QName]*model.AttributeDecl)
	for i := len(chain) - 1; i >= 0; i-- {
		mergeAttributesFromTypeForValidation(schema, chain[i], attrMap)
	}
	return attrMap
}

func mergeAttributesFromTypeForValidation(schema *parser.Schema, ct *model.ComplexType, attrMap map[model.QName]*model.AttributeDecl) {
	addAttr := func(attr *model.AttributeDecl) {
		key := typeresolve.EffectiveAttributeQName(schema, attr)
		if attr.Use == model.Prohibited && !attr.HasFixed {
			delete(attrMap, key)
			return
		}
		attrMap[key] = attr
	}

	for _, attr := range ct.Attributes() {
		addAttr(attr)
	}
	mergeAttributesFromGroupsForValidation(schema, ct.AttrGroups, attrMap)

	content := ct.Content()
	if content == nil {
		return
	}
	if ext := content.ExtensionDef(); ext != nil {
		for _, attr := range ext.Attributes {
			addAttr(attr)
		}
		mergeAttributesFromGroupsForValidation(schema, ext.AttrGroups, attrMap)
	}
	if restr := content.RestrictionDef(); restr != nil {
		for _, attr := range restr.Attributes {
			addAttr(attr)
		}
		mergeAttributesFromGroupsForValidation(schema, restr.AttrGroups, attrMap)
	}
}

func mergeAttributesFromGroupsForValidation(schema *parser.Schema, agRefs []model.QName, attrMap map[model.QName]*model.AttributeDecl) {
	ctx := attrgroupwalk.NewContext(schema, attrgroupwalk.Options{
		Missing: attrgroupwalk.MissingIgnore,
		Cycles:  attrgroupwalk.CycleIgnore,
	})
	for _, agRef := range agRefs {
		_ = ctx.Walk([]model.QName{agRef}, func(_ model.QName, current *model.AttributeGroup) error {
			for _, attr := range current.Attributes {
				if attr == nil {
					continue
				}
				key := typeresolve.EffectiveAttributeQName(schema, attr)
				if attr.Use == model.Prohibited && !attr.HasFixed {
					delete(attrMap, key)
					continue
				}
				attrMap[key] = attr
			}
			return nil
		})
	}
}

// collectAttributesFromGroups collects attributes from attribute group references.
func collectAttributesFromGroups(schema *parser.Schema, agRefs []model.QName) []*model.AttributeDecl {
	ctx := attrgroupwalk.NewContext(schema, attrgroupwalk.Options{
		Missing: attrgroupwalk.MissingIgnore,
		Cycles:  attrgroupwalk.CycleIgnore,
	})
	var result []*model.AttributeDecl
	_ = ctx.Walk(agRefs, func(_ model.QName, ag *model.AttributeGroup) error {
		result = append(result, ag.Attributes...)
		return nil
	})
	return result
}
