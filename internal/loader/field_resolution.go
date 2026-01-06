package loader

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
)

// ErrFieldSelectsComplexContent indicates a field XPath selects an element with
// element-only complex content, which is invalid per XSD spec Section 13.2.
var ErrFieldSelectsComplexContent = errors.New("field selects element with complex content")

// resolveFieldType resolves the type of a field XPath expression
// Returns the type of the attribute or element selected by the field
// The selectorXPath is used to determine the context element (the element selected by the selector)
func resolveFieldType(schema *schema.Schema, field *types.Field, constraintElement *types.ElementDecl, selectorXPath string) (types.Type, error) {
	xpath := normalizeXPathForResolution(field.XPath)

	// Handle union expressions (path1|path2|path3)
	// All paths in a union should have the same type, so resolve using the first path
	if strings.Contains(xpath, "|") {
		parts := strings.SplitSeq(xpath, "|")
		for part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				tempField := &types.Field{XPath: part}
				return resolveFieldType(schema, tempField, constraintElement, selectorXPath)
			}
		}
	}

	// First, resolve the selector to find the selected element type
	// Fields are resolved relative to the element selected by the selector
	selectedElementType, err := resolveSelectorElementType(schema, constraintElement, selectorXPath)
	if err != nil {
		return nil, fmt.Errorf("resolve selector '%s': %w", selectorXPath, err)
	}

	// This allows us to reuse findAttributeType and findElementType
	selectedElementDecl := &types.ElementDecl{
		Type: selectedElementType,
	}

	// Handle nested field paths like "part/@number"
	// Split on "/@" to get element path and attribute name
	if idx := strings.Index(xpath, "/@"); idx > 0 {
		elementPath := xpath[:idx]
		attrName := xpath[idx+2:]
		// Remove axis prefix if present
		if strings.HasPrefix(attrName, "attribute::") {
			attrName = attrName[12:]
		}

		elementType, err := resolveFieldElementType(schema, selectedElementDecl, elementPath)
		if err != nil {
			return nil, fmt.Errorf("resolve field element path '%s': %w", elementPath, err)
		}

		elementDecl := &types.ElementDecl{
			Type: elementType,
		}

		// Find attribute in that element's type
		attrType, err := findAttributeType(schema, elementDecl, attrName)
		if err != nil {
			return nil, fmt.Errorf("resolve attribute field '%s' in element from path '%s': %w", attrName, elementPath, err)
		}
		return attrType, nil
	}

	// Handle "." field xpath - returns the selected element's own type/text content
	if xpath == "." || xpath == "self::*" || strings.HasPrefix(xpath, "self::") {
		// For simple types, return the type directly
		if st, ok := selectedElementType.(*types.SimpleType); ok {
			return st, nil
		}
		if bt, ok := selectedElementType.(*types.BuiltinType); ok {
			return bt, nil
		}
		// For complex types with simple content, return the base type
		if ct, ok := selectedElementType.(*types.ComplexType); ok {
			if _, ok := ct.Content().(*types.SimpleContent); ok {
				baseType := ct.BaseType()
				if baseType != nil {
					return baseType, nil
				}
			}
			// Complex types with mixed content can have text extracted
			if ct.Mixed() {
				return types.GetBuiltin(types.TypeNameString), nil
			}
			// Complex types without simple or mixed content can't be used as field values
			return nil, ErrFieldSelectsComplexContent
		}
		return selectedElementType, nil
	}

	// Handle attribute selection (direct attribute like "@number")
	if strings.HasPrefix(xpath, "@") {
		attrName := xpath[1:]
		// Remove axis prefix if present
		if strings.HasPrefix(attrName, "attribute::") {
			attrName = attrName[12:]
		}

		// Find attribute declaration in selected element's type
		attrType, err := findAttributeType(schema, selectedElementDecl, attrName)
		if err != nil {
			return nil, fmt.Errorf("resolve attribute field '%s': %w", attrName, err)
		}
		return attrType, nil
	}

	elementName, _ := parseXPathPatternForField(xpath)

	// Find element declaration in selected element's type
	elementType, err := findElementType(schema, selectedElementDecl, elementName)
	if err != nil {
		return nil, fmt.Errorf("resolve element field '%s': %w", elementName, err)
	}

	// Verify element has simple type
	if ct, ok := elementType.(*types.ComplexType); ok {
		// Check if it's simple content (has simple type base)
		if _, ok := ct.Content().(*types.SimpleContent); ok {
			baseType := ct.BaseType()
			if baseType != nil {
				return baseType, nil
			}
		}
		return nil, fmt.Errorf("field selects element with complex content: %s", elementName)
	}

	return elementType, nil
}

// resolveSelectorElementType resolves the type of the element selected by the selector XPath
func resolveSelectorElementType(schema *schema.Schema, constraintElement *types.ElementDecl, selectorXPath string) (types.Type, error) {
	selectorXPath = normalizeXPathForResolution(selectorXPath)

	// Handle "." selector - selects the constraint element itself
	if selectorXPath == "." {
		elementType := resolveTypeForValidation(schema, constraintElement.Type)
		if elementType == nil {
			return nil, fmt.Errorf("cannot resolve constraint element type")
		}
		return elementType, nil
	}

	// Handle union expressions (path1|path2|path3) by resolving the first non-empty path.
	if strings.Contains(selectorXPath, "|") {
		parts := strings.SplitSeq(selectorXPath, "|")
		for part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				return resolveSelectorElementType(schema, constraintElement, part)
			}
		}
	}

	// Parse selector XPath to get element name(s)
	// Selectors can be simple like "part" or nested like "orders/order/part"
	elementName, axis := parseXPathPatternForField(selectorXPath)

	// Handle nested paths first (paths with "/")
	if strings.Contains(selectorXPath, "/") {
		return resolveNestedSelectorElementType(schema, constraintElement, selectorXPath)
	}

	// For child axis, search direct children only
	// For descendant axis, search all descendants (which findElementType already does recursively)
	elementType, err := findElementType(schema, constraintElement, elementName)
	if err != nil {
		// If descendant axis and direct search failed, try deeper search
		if axis == "descendant" {
			return findElementTypeDescendant(schema, constraintElement, elementName)
		}
		return nil, fmt.Errorf("resolve selector element '%s': %w", elementName, err)
	}

	return elementType, nil
}

// findElementTypeDescendant searches for an element at any depth in the content model
// This is used for descendant axis selectors
func findElementTypeDescendant(schema *schema.Schema, elementDecl *types.ElementDecl, elementName string) (types.Type, error) {
	// Resolve element's type
	elementType := resolveTypeForValidation(schema, elementDecl.Type)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}

	ct, ok := elementType.(*types.ComplexType)
	if !ok {
		return nil, fmt.Errorf("element does not have complex type")
	}

	// Search content model recursively for element declaration
	return findElementInContentDescendant(schema, ct.Content(), elementName)
}

// findElementInContentDescendant searches for an element at any depth in content
func findElementInContentDescendant(schema *schema.Schema, content types.Content, elementName string) (types.Type, error) {
	switch c := content.(type) {
	case *types.ElementContent:
		if c.Particle != nil {
			return findElementInParticleDescendant(schema, c.Particle, elementName)
		}
	case *types.SimpleContent:
		return nil, fmt.Errorf("element '%s' not found in simple content", elementName)
	case *types.ComplexContent:
		if c.Extension != nil && c.Extension.Particle != nil {
			if typ, err := findElementInParticleDescendant(schema, c.Extension.Particle, elementName); err == nil {
				return typ, nil
			}
		}
		if c.Restriction != nil && c.Restriction.Particle != nil {
			return findElementInParticleDescendant(schema, c.Restriction.Particle, elementName)
		}
	case *types.EmptyContent:
		return nil, fmt.Errorf("element '%s' not found in empty content", elementName)
	}

	return nil, fmt.Errorf("element '%s' not found in content model", elementName)
}

// findElementInParticleDescendant searches for an element at any depth in a particle tree
// This recursively searches through all nested particles
func findElementInParticleDescendant(schema *schema.Schema, particle types.Particle, elementName string) (types.Type, error) {
	// Handle namespace prefix in element name (e.g., "tn:key" -> "key")
	localName := elementName
	if _, after, ok := strings.Cut(elementName, ":"); ok {
		localName = after
	}

	switch p := particle.(type) {
	case *types.ElementDecl:
		// Found an element declaration
		if p.Name.Local == localName {
			resolvedType := resolveTypeForValidation(schema, p.Type)
			if resolvedType == nil {
				return nil, fmt.Errorf("cannot resolve element type for '%s'", elementName)
			}
			return resolvedType, nil
		}
		// Even if name doesn't match, if this element has complex type, search its content
		if p.Type != nil {
			if resolvedType := resolveTypeForValidation(schema, p.Type); resolvedType != nil {
				if ct, ok := resolvedType.(*types.ComplexType); ok {
					if typ, err := findElementInContentDescendant(schema, ct.Content(), elementName); err == nil {
						return typ, nil
					}
				}
			}
		}
	case *types.ModelGroup:
		// Search in model group particles recursively
		for _, childParticle := range p.Particles {
			if typ, err := findElementInParticleDescendant(schema, childParticle, elementName); err == nil {
				return typ, nil
			}
		}
		return nil, fmt.Errorf("element '%s' not found in model group", elementName)
	case *types.AnyElement:
		// Wildcard - cannot determine specific element type
		return nil, fmt.Errorf("cannot resolve element type for wildcard")
	}

	return nil, fmt.Errorf("element '%s' not found in particle", elementName)
}

// resolveNestedSelectorElementType resolves nested selector paths like "orders/order/part" or "./number"
func resolveNestedSelectorElementType(schema *schema.Schema, constraintElement *types.ElementDecl, selectorXPath string) (types.Type, error) {
	parts := strings.Split(selectorXPath, "/")
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty selector path")
	}

	currentElement := constraintElement
	descendantNext := false

	// Navigate through each part of the path
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			descendantNext = true
			continue
		}

		// Handle "." - stay on current element
		if part == "." {
			continue
		}

		if after, ok := strings.CutPrefix(part, "child::"); ok {
			part = after
		} else if strings.HasPrefix(part, "attribute::") || strings.HasPrefix(part, "@") {
			return nil, fmt.Errorf("selector xpath cannot select attributes: %s", selectorXPath)
		} else if strings.Contains(part, "::") {
			return nil, fmt.Errorf("selector xpath uses disallowed axis: %s", selectorXPath)
		}

		var (
			elementType types.Type
			err         error
		)
		if descendantNext {
			elementType, err = findElementTypeDescendant(schema, currentElement, part)
			descendantNext = false
		} else {
			elementType, err = findElementType(schema, currentElement, part)
		}
		if err != nil {
			return nil, fmt.Errorf("resolve selector path '%s' at '%s': %w", selectorXPath, part, err)
		}

		currentElement = &types.ElementDecl{
			Type: elementType,
		}
	}

	if currentElement.Type == nil {
		return nil, fmt.Errorf("could not resolve selector path '%s'", selectorXPath)
	}

	return currentElement.Type, nil
}

// findAttributeType finds the type of an attribute in an element's type
func findAttributeType(schema *schema.Schema, elementDecl *types.ElementDecl, attrName string) (types.Type, error) {
	if idx := strings.Index(attrName, ":"); idx >= 0 {
		attrName = attrName[idx+1:]
	}
	// Resolve element's type
	elementType := resolveTypeForValidation(schema, elementDecl.Type)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}

	ct, ok := elementType.(*types.ComplexType)
	if !ok {
		return nil, fmt.Errorf("element does not have complex type")
	}

	// Search attribute uses
	for _, attrUse := range ct.Attributes() {
		if attrUse.Name.Local == attrName {
			resolvedType := resolveTypeForValidation(schema, attrUse.Type)
			if resolvedType == nil {
				return nil, fmt.Errorf("cannot resolve attribute type for '%s'", attrName)
			}
			return resolvedType, nil
		}
	}

	// Search attribute groups
	for _, attrGroupQName := range ct.AttrGroups {
		// Look up attribute group in schema
		if attrGroup, ok := schema.AttributeGroups[attrGroupQName]; ok {
			for _, attr := range attrGroup.Attributes {
				if attr.Name.Local == attrName {
					resolvedType := resolveTypeForValidation(schema, attr.Type)
					if resolvedType == nil {
						return nil, fmt.Errorf("cannot resolve attribute type for '%s' in attribute group", attrName)
					}
					return resolvedType, nil
				}
			}
			// Recursively search nested attribute groups
			for _, nestedAttrGroupQName := range attrGroup.AttrGroups {
				if nestedAttrGroup, ok := schema.AttributeGroups[nestedAttrGroupQName]; ok {
					for _, attr := range nestedAttrGroup.Attributes {
						if attr.Name.Local == attrName {
							resolvedType := resolveTypeForValidation(schema, attr.Type)
							if resolvedType == nil {
								return nil, fmt.Errorf("cannot resolve attribute type for '%s' in nested attribute group", attrName)
							}
							return resolvedType, nil
						}
					}
				}
			}
		}
	}

	// Note: anyAttribute wildcard cannot be used to resolve specific attribute types.
	// Fields must reference declared attributes, not wildcard attributes (per XSD 1.0 spec).

	return nil, fmt.Errorf("attribute '%s' not found in element type", attrName)
}

// resolveFieldElementType resolves the element type from a field element path
// Handles both simple element names and nested paths like "part" or "orders/order"
func resolveFieldElementType(schema *schema.Schema, elementDecl *types.ElementDecl, elementPath string) (types.Type, error) {
	elementPath = normalizeXPathForResolution(elementPath)

	// Handle nested paths (paths with "/")
	if strings.Contains(elementPath, "/") {
		parts := strings.Split(elementPath, "/")
		currentElement := elementDecl
		descendantNext := false

		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				descendantNext = true
				continue
			}

			if after, ok := strings.CutPrefix(part, "child::"); ok {
				part = after
			} else if strings.HasPrefix(part, "attribute::") || strings.HasPrefix(part, "@") {
				return nil, fmt.Errorf("field xpath cannot select attributes: %s", elementPath)
			} else if strings.Contains(part, "::") {
				return nil, fmt.Errorf("field xpath uses disallowed axis: %s", elementPath)
			}

			var (
				elementType types.Type
				err         error
			)
			if descendantNext {
				elementType, err = findElementTypeDescendant(schema, currentElement, part)
				descendantNext = false
			} else {
				elementType, err = findElementType(schema, currentElement, part)
			}
			if err != nil {
				return nil, fmt.Errorf("resolve field path '%s' at '%s': %w", elementPath, part, err)
			}

			currentElement = &types.ElementDecl{
				Type: elementType,
			}
		}

		if currentElement.Type == nil {
			return nil, fmt.Errorf("could not resolve field path '%s'", elementPath)
		}

		return currentElement.Type, nil
	}

	// Simple element name - use findElementType directly
	if after, ok := strings.CutPrefix(elementPath, "child::"); ok {
		elementPath = after
	} else if strings.Contains(elementPath, "::") {
		return nil, fmt.Errorf("field xpath uses disallowed axis: %s", elementPath)
	}
	return findElementType(schema, elementDecl, elementPath)
}

func normalizeXPathForResolution(xpath string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r':
			return -1
		default:
			return r
		}
	}, xpath)
}

// findElementType finds the type of an element in an element's content model
func findElementType(schema *schema.Schema, elementDecl *types.ElementDecl, elementName string) (types.Type, error) {
	// Resolve element's type
	elementType := resolveTypeForValidation(schema, elementDecl.Type)
	if elementType == nil {
		return nil, fmt.Errorf("cannot resolve element type")
	}

	ct, ok := elementType.(*types.ComplexType)
	if !ok {
		return nil, fmt.Errorf("element does not have complex type")
	}

	// Search content model for element declaration
	// This requires traversing the particle tree
	return findElementInContent(schema, ct.Content(), elementName)
}

// findElementInContent searches for an element in a content model
func findElementInContent(schema *schema.Schema, content types.Content, elementName string) (types.Type, error) {
	// Handle content types that don't have particles
	switch content.(type) {
	case *types.SimpleContent:
		return nil, fmt.Errorf("element '%s' not found in simple content", elementName)
	case *types.EmptyContent:
		return nil, fmt.Errorf("element '%s' not found in empty content", elementName)
	}

	var result types.Type
	var resultErr error

	err := WalkContentParticles(content, func(particle types.Particle) error {
		found, err := findElementInParticle(schema, particle, elementName)
		if err == nil && found != nil {
			result = found
			return nil // Found it, stop searching
		}
		if resultErr == nil {
			resultErr = err
		}
		return nil // Continue to next particle
	})

	if err != nil {
		return nil, err
	}
	if result != nil {
		return result, nil
	}
	if resultErr != nil {
		return nil, resultErr
	}
	return nil, fmt.Errorf("element '%s' not found in content model", elementName)
}

// findElementInParticle searches for an element in a particle tree
// This handles basic cases: direct element declarations and model groups (sequence, choice, all).
// For more complex cases with group references and nested structures, this may need enhancement.
func findElementInParticle(schema *schema.Schema, particle types.Particle, elementName string) (types.Type, error) {
	// Handle namespace prefix in element name (e.g., "tn:key" -> "key")
	localName := elementName
	if _, after, ok := strings.Cut(elementName, ":"); ok {
		localName = after
	}

	switch p := particle.(type) {
	case *types.ElementDecl:
		// Found an element declaration
		if p.Name.Local == localName {
			resolvedType := resolveTypeForValidation(schema, p.Type)
			if resolvedType == nil {
				return nil, fmt.Errorf("cannot resolve element type for '%s'", elementName)
			}
			return resolvedType, nil
		}
	case *types.ModelGroup:
		// Search in model group particles
		for _, childParticle := range p.Particles {
			if typ, err := findElementInParticle(schema, childParticle, elementName); err == nil {
				return typ, nil
			}
		}
		return nil, fmt.Errorf("element '%s' not found in model group", elementName)
	case *types.AnyElement:
		// Wildcard - cannot determine specific element type
		return nil, fmt.Errorf("cannot resolve element type for wildcard")
	}

	return nil, fmt.Errorf("element '%s' not found in particle", elementName)
}

// parseXPathPatternForField extracts element name from XPath pattern for field resolution
func parseXPathPatternForField(xpath string) (elementName string, axis string) {
	// Handle "child::elementName"
	if strings.HasPrefix(xpath, "child::") {
		return xpath[7:], "child"
	}
	// Handle "descendant::elementName"
	if strings.HasPrefix(xpath, "descendant::") {
		return xpath[12:], "descendant"
	}
	// Handle "//elementName" (abbreviated descendant)
	if strings.HasPrefix(xpath, "//") {
		return xpath[2:], "descendant"
	}
	// Default: assume child axis
	return xpath, "child"
}
