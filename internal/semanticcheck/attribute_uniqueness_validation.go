package semanticcheck

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/typechain"
	"github.com/jacoelho/xsd/internal/typeresolve"
)

// validateAttributeUniqueness validates that no two attributes in a complex type
// share the same name and namespace.
func validateAttributeUniqueness(schema *parser.Schema, ct *model.ComplexType) error {
	allAttributes := collectAllAttributesForValidation(schema, ct)

	seen := make(map[model.QName]bool)
	for _, attr := range allAttributes {
		key := typeresolve.EffectiveAttributeQName(schema, attr)
		if seen[key] {
			return fmt.Errorf("duplicate attribute '%s' in namespace '%s'", attr.Name.Local, attr.Name.Namespace)
		}
		seen[key] = true
	}

	return nil
}

func validateExtensionAttributeUniqueness(schema *parser.Schema, ct *model.ComplexType) error {
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
	baseCT, ok := typechain.LookupComplexType(schema, ext.Base)
	if !ok || baseCT == nil {
		return nil
	}

	baseAttrs := collectEffectiveAttributeUses(schema, baseCT)
	if len(baseAttrs) == 0 {
		return nil
	}

	attrs := append([]*model.AttributeDecl{}, ext.Attributes...)
	attrs = append(attrs, collectAttributesFromGroups(schema, ext.AttrGroups, nil)...)
	for _, attr := range attrs {
		key := typeresolve.EffectiveAttributeQName(schema, attr)
		if _, exists := baseAttrs[key]; exists {
			return fmt.Errorf("extension attribute '%s' in namespace '%s' duplicates base attribute", attr.Name.Local, attr.Name.Namespace)
		}
	}
	return nil
}

// validateAttributeGroupUniqueness validates that no two attributes in the group
// share the same name and namespace.
func validateAttributeGroupUniqueness(schema *parser.Schema, ag *model.AttributeGroup) error {
	seen := make(map[model.QName]bool)
	for _, attr := range ag.Attributes {
		key := typeresolve.EffectiveAttributeQName(schema, attr)
		if seen[key] {
			return fmt.Errorf("duplicate attribute '%s' in namespace '%s'", attr.Name.Local, attr.Name.Namespace)
		}
		seen[key] = true
	}
	return nil
}
