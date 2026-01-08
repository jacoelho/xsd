package loader

import (
	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// collectAllAttributesForValidation collects all attributes from a complex type
// This includes attributes from extensions, restrictions, and attribute groups
// Note: We don't recursively collect from base types since they might not be fully resolved
// during schema validation. This checks for duplicates within the same type definition.
func collectAllAttributesForValidation(schema *schema.Schema, ct *types.ComplexType) []*types.AttributeDecl {
	allAttrs := append([]*types.AttributeDecl(nil), ct.Attributes()...)
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

func collectEffectiveAttributeUses(schema *schema.Schema, ct *types.ComplexType) map[types.QName]*types.AttributeDecl {
	if ct == nil {
		return nil
	}
	chain := collectComplexTypeChain(schema, ct)
	attrMap := make(map[types.QName]*types.AttributeDecl)
	for i := len(chain) - 1; i >= 0; i-- {
		mergeAttributesFromTypeForValidation(schema, chain[i], attrMap)
	}
	return attrMap
}

func collectComplexTypeChain(schema *schema.Schema, ct *types.ComplexType) []*types.ComplexType {
	var chain []*types.ComplexType
	visited := make(map[*types.ComplexType]bool)
	for current := ct; current != nil; {
		if visited[current] {
			break
		}
		visited[current] = true
		chain = append(chain, current)
		var next *types.ComplexType
		if baseCT, ok := current.ResolvedBase.(*types.ComplexType); ok {
			next = baseCT
		} else if current.ResolvedBase == nil {
			baseQName := types.QName{}
			if content := current.Content(); content != nil {
				baseQName = content.BaseTypeQName()
			}
			if !baseQName.IsZero() {
				if baseType, ok := schema.TypeDefs[baseQName]; ok {
					if baseCT, ok := baseType.(*types.ComplexType); ok {
						next = baseCT
					}
				}
			}
		}
		if next == nil {
			break
		}
		current = next
	}
	return chain
}

func mergeAttributesFromTypeForValidation(schema *schema.Schema, ct *types.ComplexType, attrMap map[types.QName]*types.AttributeDecl) {
	addAttr := func(attr *types.AttributeDecl) {
		key := effectiveAttributeQNameForValidation(schema, attr)
		if attr.Use == types.Prohibited && !attr.HasFixed {
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

func mergeAttributesFromGroupsForValidation(schema *schema.Schema, agRefs []types.QName, attrMap map[types.QName]*types.AttributeDecl) {
	for _, agRef := range agRefs {
		ag, ok := schema.AttributeGroups[agRef]
		if !ok {
			continue
		}
		mergeAttributesFromGroupForValidation(schema, ag, attrMap)
	}
}

func mergeAttributesFromGroupForValidation(schema *schema.Schema, ag *types.AttributeGroup, attrMap map[types.QName]*types.AttributeDecl) {
	visited := make(map[*types.AttributeGroup]bool)
	queue := []*types.AttributeGroup{ag}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if visited[current] {
			continue
		}
		visited[current] = true
		for _, attr := range current.Attributes {
			key := effectiveAttributeQNameForValidation(schema, attr)
			if attr.Use == types.Prohibited && !attr.HasFixed {
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

// collectAttributesFromGroups collects attributes from attribute group references
func collectAttributesFromGroups(schema *schema.Schema, agRefs []types.QName, visited map[types.QName]bool) []*types.AttributeDecl {
	if visited == nil {
		visited = make(map[types.QName]bool)
	}
	var result []*types.AttributeDecl
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

// effectiveAttributeQNameForValidation returns the effective QName for an attribute
// considering form defaults and namespace qualification
func effectiveAttributeQNameForValidation(sch *schema.Schema, attr *types.AttributeDecl) types.QName {
	if attr.IsReference {
		return attr.Name
	}
	form := attr.Form
	if form == types.FormDefault {
		if sch.AttributeFormDefault == schema.Qualified {
			form = types.FormQualified
		} else {
			form = types.FormUnqualified
		}
	}
	if form == types.FormQualified {
		if attr.SourceNamespace != "" {
			return types.QName{
				Namespace: attr.SourceNamespace,
				Local:     attr.Name.Local,
			}
		}
		return types.QName{
			Namespace: sch.TargetNamespace,
			Local:     attr.Name.Local,
		}
	}
	return types.QName{
		Namespace: "",
		Local:     attr.Name.Local,
	}
}
