package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typegraph"
	"github.com/jacoelho/xsd/internal/types"
)

// validateAnyAttributeDerivation validates anyAttribute constraints in type derivation
// According to XSD 1.0 spec:
// - For extension: anyAttribute must union with base type's anyAttribute (cos-aw-union)
// - For restriction: anyAttribute namespace constraint must be a subset of base type's anyAttribute (cos-aw-subset)
func validateAnyAttributeDerivation(schema *parser.Schema, ct *types.ComplexType) error {
	baseQName := ct.Content().BaseTypeQName()
	if baseQName.IsZero() {
		return nil
	}

	baseCT, ok := typegraph.LookupComplexType(schema, baseQName)
	if !ok {
		return nil
	}

	baseAnyAttr, err := collectAnyAttributeFromType(schema, baseCT)
	if err != nil {
		return err
	}
	derivedAnyAttr, err := collectAnyAttributeFromType(schema, ct)
	if err != nil {
		return err
	}

	if ct.IsExtension() {
		if baseAnyAttr != nil && derivedAnyAttr != nil {
			union := types.UnionAnyAttribute(derivedAnyAttr, baseAnyAttr)
			if union == nil {
				return fmt.Errorf("anyAttribute extension: union of derived and base anyAttribute is not expressible")
			}
		}
	} else if ct.IsRestriction() {
		if baseAnyAttr == nil && derivedAnyAttr != nil {
			return fmt.Errorf("anyAttribute restriction: cannot add anyAttribute when base type has no anyAttribute")
		}
		if derivedAnyAttr != nil && baseAnyAttr != nil {
			if !processContentsStrongerOrEqual(derivedAnyAttr.ProcessContents, baseAnyAttr.ProcessContents) ||
				!namespaceConstraintSubset(
					derivedAnyAttr.Namespace, derivedAnyAttr.NamespaceList, derivedAnyAttr.TargetNamespace,
					baseAnyAttr.Namespace, baseAnyAttr.NamespaceList, baseAnyAttr.TargetNamespace,
				) {
				return fmt.Errorf("anyAttribute restriction: derived anyAttribute is not a valid subset of base anyAttribute")
			}
		}
	}

	return nil
}

// collectAnyAttributeFromType collects anyAttribute from a complex type
// Checks both direct anyAttribute and anyAttribute in extension/restriction
func collectAnyAttributeFromType(schema *parser.Schema, ct *types.ComplexType) (*types.AnyAttribute, error) {
	var anyAttrs []*types.AnyAttribute

	if ct.AnyAttribute() != nil {
		anyAttrs = append(anyAttrs, ct.AnyAttribute())
	}
	anyAttrs = append(anyAttrs, collectAnyAttributeFromGroups(schema, ct.AttrGroups, nil)...)

	content := ct.Content()
	if ext := content.ExtensionDef(); ext != nil {
		if ext.AnyAttribute != nil {
			anyAttrs = append(anyAttrs, ext.AnyAttribute)
		}
		anyAttrs = append(anyAttrs, collectAnyAttributeFromGroups(schema, ext.AttrGroups, nil)...)
	}
	if restr := content.RestrictionDef(); restr != nil {
		if restr.AnyAttribute != nil {
			anyAttrs = append(anyAttrs, restr.AnyAttribute)
		}
		anyAttrs = append(anyAttrs, collectAnyAttributeFromGroups(schema, restr.AttrGroups, nil)...)
	}

	if len(anyAttrs) == 0 {
		return nil, nil
	}

	result := anyAttrs[0]
	for i := 1; i < len(anyAttrs); i++ {
		intersected, expressible, empty := types.IntersectAnyAttributeDetailed(result, anyAttrs[i])
		if !expressible {
			return nil, fmt.Errorf("anyAttribute intersection is not expressible")
		}
		if empty {
			return nil, nil
		}
		result = intersected
	}
	return result, nil
}

// collectAnyAttributeFromGroups collects anyAttribute from attribute groups (recursively)
func collectAnyAttributeFromGroups(schema *parser.Schema, agRefs []types.QName, visited map[types.QName]bool) []*types.AnyAttribute {
	if visited == nil {
		visited = make(map[types.QName]bool)
	}
	var result []*types.AnyAttribute
	for _, ref := range agRefs {
		if visited[ref] {
			continue
		}
		visited[ref] = true
		ag, ok := schema.AttributeGroups[ref]
		if !ok {
			continue
		}
		if ag.AnyAttribute != nil {
			result = append(result, ag.AnyAttribute)
		}
		if len(ag.AttrGroups) > 0 {
			result = append(result, collectAnyAttributeFromGroups(schema, ag.AttrGroups, visited)...)
		}
	}
	return result
}
