package runtimecompile

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

func mergeAttributes(schema *parser.Schema, attrs []*types.AttributeDecl, groups []types.QName, attrMap map[types.QName]*types.AttributeDecl, mode attrCollectionMode) error {
	for _, attr := range attrs {
		if attr == nil {
			continue
		}
		key := typeops.EffectiveAttributeQName(schema, attr)
		attrMap[key] = attr
	}
	if len(groups) == 0 {
		return nil
	}
	visited := make(map[*types.AttributeGroup]bool)
	return mergeAttributesFromGroups(schema, groups, attrMap, mode, visited)
}

func mergeAttributesFromGroups(schema *parser.Schema, groups []types.QName, attrMap map[types.QName]*types.AttributeDecl, mode attrCollectionMode, visited map[*types.AttributeGroup]bool) error {
	for _, ref := range groups {
		group, ok := schema.AttributeGroups[ref]
		if !ok {
			return fmt.Errorf("attributeGroup %s not found", ref)
		}
		if visited[group] {
			continue
		}
		visited[group] = true
		groupMode := mode
		if mode == attrRestriction {
			groupMode = attrMerge
		}
		attrs := group.Attributes
		for _, attr := range attrs {
			if attr != nil && attr.Use == types.Prohibited {
				// W3C attZ015: ignore prohibited uses from attribute groups.
				filtered := make([]*types.AttributeDecl, 0, len(attrs))
				for _, candidate := range attrs {
					if candidate == nil || candidate.Use == types.Prohibited {
						continue
					}
					filtered = append(filtered, candidate)
				}
				attrs = filtered
				break
			}
		}
		if err := mergeAttributes(schema, attrs, group.AttrGroups, attrMap, groupMode); err != nil {
			return err
		}
	}
	return nil
}
