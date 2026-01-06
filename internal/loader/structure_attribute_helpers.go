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
