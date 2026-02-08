package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typeops"
	"github.com/jacoelho/xsd/internal/types"
)

// validateAttributeUniqueness validates that no two attributes in a complex type
// share the same name and namespace.
func validateAttributeUniqueness(schema *parser.Schema, ct *types.ComplexType) error {
	allAttributes := collectAllAttributesForValidation(schema, ct)

	seen := make(map[types.QName]bool)
	for _, attr := range allAttributes {
		key := typeops.EffectiveAttributeQName(schema, attr)
		if seen[key] {
			return fmt.Errorf("duplicate attribute '%s' in namespace '%s'", attr.Name.Local, attr.Name.Namespace)
		}
		seen[key] = true
	}

	return nil
}

func validateExtensionAttributeUniqueness(schema *parser.Schema, ct *types.ComplexType) error {
	if ct == nil {
		return nil
	}
	content := ct.Content()
	if content == nil {
		return nil
	}
	ext := content.ExtensionDef()
	if ext == nil || ext.Base.IsZero() {
		return nil
	}
	baseCT, ok := lookupComplexType(schema, ext.Base)
	if !ok || baseCT == nil {
		return nil
	}

	baseAttrs := collectEffectiveAttributeUses(schema, baseCT)
	if len(baseAttrs) == 0 {
		return nil
	}

	attrs := append([]*types.AttributeDecl{}, ext.Attributes...)
	attrs = append(attrs, collectAttributesFromGroups(schema, ext.AttrGroups, nil)...)
	for _, attr := range attrs {
		key := typeops.EffectiveAttributeQName(schema, attr)
		if _, exists := baseAttrs[key]; exists {
			return fmt.Errorf("extension attribute '%s' in namespace '%s' duplicates base attribute", attr.Name.Local, attr.Name.Namespace)
		}
	}
	return nil
}

// validateAttributeGroupUniqueness validates that no two attributes in the group
// share the same name and namespace.
func validateAttributeGroupUniqueness(schema *parser.Schema, ag *types.AttributeGroup) error {
	seen := make(map[types.QName]bool)
	for _, attr := range ag.Attributes {
		key := typeops.EffectiveAttributeQName(schema, attr)
		if seen[key] {
			return fmt.Errorf("duplicate attribute '%s' in namespace '%s'", attr.Name.Local, attr.Name.Namespace)
		}
		seen[key] = true
	}
	return nil
}
