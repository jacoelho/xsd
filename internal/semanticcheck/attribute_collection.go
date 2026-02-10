package semanticcheck

import (
	"slices"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typegraph"
	"github.com/jacoelho/xsd/internal/typeops"
)

// collectAllAttributesForValidation collects all attributes from a complex type.
// This includes attributes from extensions, restrictions, and attribute groups.
func collectAllAttributesForValidation(schema *parser.Schema, ct *model.ComplexType) []*model.AttributeDecl {
	allAttrs := slices.Clone(ct.Attributes())
	allAttrs = append(allAttrs, collectAttributesFromGroups(schema, ct.AttrGroups, nil)...)

	content := ct.Content()
	if content != nil {
		if ext := content.ExtensionDef(); ext != nil {
			allAttrs = append(allAttrs, ext.Attributes...)
			allAttrs = append(allAttrs, collectAttributesFromGroups(schema, ext.AttrGroups, nil)...)
		}
		if restr := content.RestrictionDef(); restr != nil {
			allAttrs = append(allAttrs, restr.Attributes...)
			allAttrs = append(allAttrs, collectAttributesFromGroups(schema, restr.AttrGroups, nil)...)
		}
	}

	return allAttrs
}

func collectEffectiveAttributeUses(schema *parser.Schema, ct *model.ComplexType) map[model.QName]*model.AttributeDecl {
	if ct == nil {
		return nil
	}
	chain := typegraph.CollectComplexTypeChain(schema, ct, typegraph.ComplexTypeChainExplicitBaseOnly)
	attrMap := make(map[model.QName]*model.AttributeDecl)
	for i := len(chain) - 1; i >= 0; i-- {
		mergeAttributesFromTypeForValidation(schema, chain[i], attrMap)
	}
	return attrMap
}

func mergeAttributesFromTypeForValidation(schema *parser.Schema, ct *model.ComplexType, attrMap map[model.QName]*model.AttributeDecl) {
	addAttr := func(attr *model.AttributeDecl) {
		key := typeops.EffectiveAttributeQName(schema, attr)
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
	for _, agRef := range agRefs {
		ag, ok := schema.AttributeGroups[agRef]
		if !ok {
			continue
		}
		mergeAttributesFromGroupForValidation(schema, ag, attrMap)
	}
}

func mergeAttributesFromGroupForValidation(schema *parser.Schema, ag *model.AttributeGroup, attrMap map[model.QName]*model.AttributeDecl) {
	visited := make(map[*model.AttributeGroup]bool)
	queue := []*model.AttributeGroup{ag}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current] {
			continue
		}
		visited[current] = true
		for _, attr := range current.Attributes {
			key := typeops.EffectiveAttributeQName(schema, attr)
			if attr.Use == model.Prohibited && !attr.HasFixed {
				delete(attrMap, key)
				continue
			}
			attrMap[key] = attr
		}
		for _, ref := range current.AttrGroups {
			if refAG, ok := schema.AttributeGroups[ref]; ok {
				queue = append(queue, refAG)
			}
		}
	}
}

// collectAttributesFromGroups collects attributes from attribute group references.
func collectAttributesFromGroups(schema *parser.Schema, agRefs []model.QName, visited map[model.QName]bool) []*model.AttributeDecl {
	if visited == nil {
		visited = make(map[model.QName]bool)
	}
	var result []*model.AttributeDecl
	for _, ref := range agRefs {
		if visited[ref] {
			continue
		}
		visited[ref] = true
		ag, ok := schema.AttributeGroups[ref]
		if !ok {
			continue
		}
		result = append(result, ag.Attributes...)
		result = append(result, collectAttributesFromGroups(schema, ag.AttrGroups, visited)...)
	}
	return result
}
