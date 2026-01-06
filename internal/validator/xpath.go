package validator

import (
	"math"
	"strconv"
	"strings"

	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/parser/lexical"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/validator/xpath"
	"github.com/jacoelho/xsd/internal/xml"
)

// KeyState indicates the status of a key value extraction.
type KeyState int

const (
	// KeyValid means a single valid value was extracted.
	KeyValid KeyState = iota
	// KeyAbsent means no node was selected for a field.
	KeyAbsent
	// KeyMultiple means multiple nodes were selected for a field.
	KeyMultiple
	// KeyInvalid means a non-simple field value selection was encountered.
	KeyInvalid
)

// KeyResult is the extracted key value and its state.
type KeyResult struct {
	Value string
	State KeyState
}

// evaluateSelectorWithNS evaluates a selector XPath with namespace context for prefix resolution.
func (r *validationRun) evaluateSelectorWithNS(root xml.Element, expr string, nsContext map[string]string) []xml.Element {
	evaluator := xpath.New(r.root)
	return evaluator.SelectWithNS(root, expr, nsContext)
}

// evaluateSelector evaluates a simple XPath selector.
func (r *validationRun) evaluateSelector(root xml.Element, expr string) []xml.Element {
	evaluator := xpath.New(r.root)
	return evaluator.Select(root, expr)
}

// extractKeyValueWithNS extracts a key value from an element using field XPath expressions,
// with namespace prefix resolution from the schema context.
func (r *validationRun) extractKeyValueWithNS(elem xml.Element, fields []types.Field, nsContext map[string]string) KeyResult {
	if len(fields) == 0 {
		return KeyResult{State: KeyAbsent}
	}

	// For multiple fields, concatenate values with a separator.
	values := make([]string, 0, len(fields))
	for _, field := range fields {
		rawValue, count := r.evaluateFieldWithCountNS(elem, field.XPath, nsContext)
		if count == 0 {
			return KeyResult{State: KeyAbsent}
		}
		// Per XSD spec, if a field selects more than one node, it's invalid.
		if count > 1 {
			return KeyResult{State: KeyMultiple}
		}
		normalizedValue := r.normalizeKeyValue(rawValue, field, elem, nsContext)
		if normalizedValue.State != KeyValid {
			return normalizedValue
		}
		values = append(values, normalizedValue.Value)
	}

	// Join values with a separator (using a character unlikely to appear in values).
	return KeyResult{Value: strings.Join(values, "\x00"), State: KeyValid}
}

// normalizeKeyValue normalizes a key value according to its type for identity constraint comparison.
// Per XSD spec, values of different types are always considered distinct.
// Values of numeric types (decimal, integer, etc.) are normalized to canonical form.
// String values are compared lexically without normalization.
func (r *validationRun) normalizeKeyValue(value string, field types.Field, elem xml.Element, nsContext map[string]string) KeyResult {
	expr := strings.TrimSpace(field.XPath)
	targetElem, isAttribute, attrNameFromPath, attrPath := r.resolveKeyTarget(elem, expr, nsContext)

	if targetElem != nil && !isAttribute {
		if decl := r.lookupElementDecl(targetElem); decl != nil && decl.Type != nil {
			if !decl.Type.AllowsText() {
				return KeyResult{State: KeyInvalid}
			}
		}
	}

	if value == "" {
		return KeyResult{Value: "", State: KeyValid}
	}

	// For attributes (xpath starts with @), try to look up attribute type from element declaration.
	// This is done early to ensure we have the correct type for normalization.
	var attrQName types.QName
	var attrDecl *grammar.CompiledElement
	attrDeclared := false
	if isAttribute {
		attrName := attrNameFromPath
		if !attrPath {
			attrName = strings.TrimPrefix(expr, "@")
			attrName = strings.TrimPrefix(attrName, "attribute::")
		}
		resolvedQName, resolved := r.resolveAttributeQName(attrName, targetElem, nsContext)
		if !resolved {
			return KeyResult{Value: value, State: KeyValid}
		}
		attrQName = resolvedQName
		if targetElem != nil {
			// Try to find attribute type from element declaration.
			decl := r.lookupElementDecl(targetElem)
			attrDecl = decl
			if decl != nil && decl.Type != nil {
				// Search through all attributes (including inherited ones).
				for _, attr := range decl.Type.AllAttributes {
					if attr.QName.Local == attrQName.Local {
						if attrQName.Namespace != "" && attr.QName.Namespace != attrQName.Namespace {
							continue
						}
						attrDeclared = true
						if attr.Type != nil && attr.Type.Original != nil {
							// Use attribute type for normalization (preferred over field.ResolvedType for consistency).
							return KeyResult{Value: r.normalizeValueByType(value, attr.Type.Original, targetElem), State: KeyValid}
						}
					}
				}
			}
		}
	}

	fieldType, state := r.resolveKeyFieldType(field, targetElem, isAttribute, attrQName, attrDecl, attrDeclared)
	if state != KeyValid {
		return KeyResult{State: state}
	}

	if fieldType != nil {
		return KeyResult{Value: r.normalizeValueByType(value, fieldType, targetElem), State: KeyValid}
	}

	// Attribute normalization - if type unknown, return as-is.
	// Don't fall back to decimal normalization for attributes.
	if isAttribute {
		return KeyResult{Value: value, State: KeyValid}
	}

	// No type information available - return value as-is without normalization.
	// This is the conservative choice: don't guess the type.
	return KeyResult{Value: value, State: KeyValid}
}

func (r *validationRun) resolveKeyTarget(elem xml.Element, expr string, nsContext map[string]string) (xml.Element, bool, string, bool) {
	elementPath, attrNameFromPath, attrPath := splitFieldAttributeXPath(expr)
	isAttribute := strings.HasPrefix(expr, "@") || strings.HasPrefix(expr, "attribute::") || attrPath

	if expr == "." || expr == "self::*" || (strings.HasPrefix(expr, "self::") && len(expr) > 6) {
		return elem, isAttribute, attrNameFromPath, attrPath
	}

	if isAttribute {
		// For attributes, the target element is the element that owns the attribute.
		if attrPath {
			elementPath = normalizeAttributeElementPath(elementPath)
			var selected []xml.Element
			if nsContext != nil {
				selected = r.evaluateSelectorWithNS(elem, elementPath, nsContext)
			} else {
				selected = r.evaluateSelector(elem, elementPath)
			}
			if len(selected) > 0 {
				return selected[0], isAttribute, attrNameFromPath, attrPath
			}
			return nil, isAttribute, attrNameFromPath, attrPath
		}
		return elem, isAttribute, attrNameFromPath, attrPath
	}

	// For child/descendant elements, find the selected element to infer type.
	if nsContext != nil {
		selected := r.evaluateSelectorWithNS(elem, expr, nsContext)
		if len(selected) > 0 {
			return selected[0], isAttribute, attrNameFromPath, attrPath
		}
		return nil, isAttribute, attrNameFromPath, attrPath
	}
	selected := r.evaluateSelector(elem, expr)
	if len(selected) > 0 {
		return selected[0], isAttribute, attrNameFromPath, attrPath
	}
	return nil, isAttribute, attrNameFromPath, attrPath
}

func (r *validationRun) resolveAttributeQName(attrName string, targetElem xml.Element, nsContext map[string]string) (types.QName, bool) {
	attrNamespace := types.NamespaceURI("")
	attrLocal := attrName
	if idx := strings.Index(attrName, ":"); idx > 0 {
		prefix := attrName[:idx]
		attrLocal = attrName[idx+1:]
		if nsContext != nil {
			nsURI, found := nsContext[prefix]
			if !found {
				return types.QName{}, false
			}
			attrNamespace = types.NamespaceURI(nsURI)
		} else if targetElem != nil {
			nsURI := r.lookupNamespaceURI(targetElem, prefix)
			if nsURI == "" {
				return types.QName{}, false
			}
			attrNamespace = types.NamespaceURI(nsURI)
		}
	}
	return types.QName{Namespace: attrNamespace, Local: attrLocal}, true
}

func (r *validationRun) resolveKeyFieldType(field types.Field, targetElem xml.Element, isAttribute bool, attrQName types.QName, attrDecl *grammar.CompiledElement, attrDeclared bool) (types.Type, KeyState) {
	// Try to get type from field's ResolvedType first.
	var fieldType types.Type
	if field.ResolvedType != nil {
		fieldType = field.ResolvedType
	} else if field.Type != nil {
		fieldType = field.Type
	}

	if isAttribute {
		if fieldType == nil && attrDecl != nil && attrDecl.Type != nil && !attrDeclared {
			if attrDecl.Type.AnyAttribute != nil && attrDecl.Type.AnyAttribute.AllowsQName(attrQName) {
				return nil, KeyInvalid
			}
		}
		if fieldType == nil {
			fieldType = types.GetBuiltin(types.TypeName("string"))
		}
		return fieldType, KeyValid
	}

	// Check for xsi:type on the target element.
	// This is important for:
	// 1. anySimpleType where instance specifies actual type
	// 2. Any other case where runtime type differs from schema type
	if targetElem != nil {
		xsiType := targetElem.GetAttributeNS("http://www.w3.org/2001/XMLSchema-instance", "type")
		if xsiType != "" {
			// Parse the xsi:type value to get the type.
			if resolvedXsiType := r.lookupTypeFromXsiType(targetElem, xsiType); resolvedXsiType != nil {
				// Use the xsi:type as the actual type for comparison.
				fieldType = resolvedXsiType
			}
		}
	}

	// If still no type, try to infer from element declaration.
	if fieldType == nil && targetElem != nil {
		if decl := r.lookupElementDecl(targetElem); decl != nil && decl.Type != nil {
			fieldType = decl.Type.Original
		}
	}

	return fieldType, KeyValid
}

// lookupTypeFromXsiType resolves an xsi:type attribute value to a types.Type for identity constraint comparison.
func (r *validationRun) lookupTypeFromXsiType(elem xml.Element, xsiTypeValue string) types.Type {
	qname, err := r.parseQNameValue(elem, xsiTypeValue)
	if err != nil {
		return nil
	}

	if ct := r.schema.Type(qname); ct != nil {
		return ct.Original
	}

	// Check builtin types in XSD namespace.
	if bt := types.GetBuiltinNS(qname.Namespace, qname.Local); bt != nil {
		return bt
	}

	return nil
}

func (r *validationRun) lookupAttributeDefault(elem xml.Element, attrQName types.QName) (string, bool) {
	elemQName := types.QName{
		Namespace: types.NamespaceURI(elem.NamespaceURI()),
		Local:     elem.LocalName(),
	}
	decl := r.schema.Element(elemQName)
	if decl == nil {
		decl = r.schema.LocalElement(elemQName)
	}
	if decl == nil || decl.Type == nil {
		return "", false
	}
	for _, attr := range decl.Type.AllAttributes {
		if attr.QName == attrQName {
			if attr.HasFixed {
				return attr.Fixed, true
			}
			if attr.Default != "" {
				return attr.Default, true
			}
			return "", false
		}
	}
	return "", false
}

func (r *validationRun) lookupElementDecl(elem xml.Element) *grammar.CompiledElement {
	elemQName := types.QName{
		Namespace: types.NamespaceURI(elem.NamespaceURI()),
		Local:     elem.LocalName(),
	}
	decl := r.schema.Element(elemQName)
	if decl == nil {
		decl = r.schema.LocalElement(elemQName)
	}
	return decl
}

// normalizeValueByType normalizes a value according to its XSD type.
// Returns the value prefixed with type identifier for proper comparison.
// Per XSD spec, values of different primitive types are never equal (disjoint value spaces).
func (r *validationRun) normalizeValueByType(value string, fieldType types.Type, elem xml.Element) string {
	var primitiveName string
	if bt, ok := fieldType.(*types.BuiltinType); ok {
		if pt := bt.PrimitiveType(); pt != nil {
			if pbt, ok := pt.(*types.BuiltinType); ok {
				primitiveName = pbt.Name().Local
			} else if pst, ok := pt.(*types.SimpleType); ok {
				primitiveName = pst.QName.Local
			}
		} else {
			primitiveName = bt.Name().Local
		}
	} else if pt := fieldType.PrimitiveType(); pt != nil {
		if st, ok := pt.(*types.SimpleType); ok {
			primitiveName = st.QName.Local
		} else if bt, ok := pt.(*types.BuiltinType); ok {
			primitiveName = bt.Name().Local
		}
	}

	// Use primitive type name to keep derived types comparable within the same value space.
	typePrefix := primitiveName
	if typePrefix == "" && fieldType != nil {
		typePrefix = fieldType.Name().String()
	}

	switch primitiveName {
	case "decimal", "integer", "nonPositiveInteger", "negativeInteger",
		"nonNegativeInteger", "positiveInteger", "long", "int", "short", "byte",
		"unsignedLong", "unsignedInt", "unsignedShort", "unsignedByte":
		rat, err := lexical.ParseDecimal(value)
		if err == nil {
			// Prefix with type to ensure different types compare differently.
			return typePrefix + "\x01" + rat.String()
		}
	case "float", "double":
		trimmed := strings.TrimSpace(value)
		bitSize := 64
		if primitiveName == "float" {
			bitSize = 32
		}
		floatValue, err := strconv.ParseFloat(trimmed, bitSize)
		if err == nil {
			if math.IsNaN(floatValue) {
				return typePrefix + "\x01" + "NaN"
			}
			// Canonicalize to the shortest round-trippable form per precision.
			return typePrefix + "\x01" + strconv.FormatFloat(floatValue, 'g', -1, bitSize)
		}
		return typePrefix + "\x01" + trimmed
	case "QName":
		// Normalize QName values (resolve namespace prefix to URI).
		return typePrefix + "\x01" + r.normalizeQName(value, elem)
	}

	// For string and other types, use lexical value with type prefix.
	// This ensures "3" as string is different from "3" as integer.
	return typePrefix + "\x01" + value
}

// evaluateFieldWithNS evaluates a field XPath expression with namespace prefix resolution.
func (r *validationRun) evaluateFieldWithNS(elem xml.Element, expr string, nsContext map[string]string) string {
	value, _ := r.evaluateFieldWithCountNS(elem, expr, nsContext)
	return value
}

// evaluateFieldWithCountNS evaluates a field XPath expression with namespace prefix resolution,
// returning both the value and the count of matching nodes.
func (r *validationRun) evaluateFieldWithCountNS(elem xml.Element, expr string, nsContext map[string]string) (string, int) {
	expr = strings.TrimSpace(expr)

	// Handle XPath union expressions (path1|path2|path3).
	if strings.Contains(expr, "|") {
		parts := strings.SplitSeq(expr, "|")
		for part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			value, count := r.evaluateFieldWithCountNS(elem, part, nsContext)
			if value != "" || count > 0 {
				return value, count
			}
		}
		return "", 0
	}

	// Handle "." (current element).
	if expr == "." || expr == "" {
		return strings.TrimSpace(elem.TextContent()), 1
	}

	if elementPath, attrName, ok := splitFieldAttributeXPath(expr); ok {
		elementPath = normalizeAttributeElementPath(elementPath)
		selectedElements := r.evaluateSelectorWithNS(elem, elementPath, nsContext)
		if len(selectedElements) == 0 {
			return "", 0
		}
		return r.evaluateAttributeSelection(selectedElements, attrName, nsContext)
	}

	// Handle "@prefix:attributeName" for attributes with namespace prefix.
	if strings.HasPrefix(expr, "@") || strings.HasPrefix(expr, "attribute::") {
		attrName := expr[1:]
		if after, ok := strings.CutPrefix(expr, "attribute::"); ok {
			attrName = after
		}

		// Check if attribute name has a prefix.
		if idx := strings.Index(attrName, ":"); idx > 0 {
			prefix := attrName[:idx]
			localName := attrName[idx+1:]

			if nsURI, ok := nsContext[prefix]; ok {
				if localName == "*" {
					var firstValue string
					count := 0
					for _, attr := range elem.Attributes() {
						if attr.NamespaceURI() == nsURI {
							count++
							if firstValue == "" {
								firstValue = attr.Value()
							}
						}
					}
					if count > 0 {
						return firstValue, count
					}
					return "", 0
				}
				// Look for attribute with matching namespace URI.
				for _, attr := range elem.Attributes() {
					if attr.LocalName() == localName && attr.NamespaceURI() == nsURI {
						return attr.Value(), 1
					}
				}
				if defaultValue, ok := r.lookupAttributeDefault(elem, types.QName{Namespace: types.NamespaceURI(nsURI), Local: localName}); ok {
					return defaultValue, 1
				}
				return "", 0
			}
		}

		if attrName == "*" {
			attrs := elem.Attributes()
			if len(attrs) == 0 {
				return "", 0
			}
			return attrs[0].Value(), len(attrs)
		}

		// Fall back to local name matching.
		value := elem.GetAttribute(attrName)
		if value != "" || elem.HasAttribute(attrName) {
			return value, 1
		}
		if defaultValue, ok := r.lookupAttributeDefault(elem, types.QName{Local: attrName}); ok {
			return defaultValue, 1
		}
		return "", 0
	}

	selectedElements := r.evaluateSelectorWithNS(elem, expr, nsContext)
	if len(selectedElements) == 0 {
		return "", 0
	}
	return strings.TrimSpace(selectedElements[0].TextContent()), len(selectedElements)
}

func splitFieldAttributeXPath(expr string) (string, string, bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "", "", false
	}
	if idx := strings.LastIndex(expr, "/@"); idx != -1 {
		return strings.TrimSpace(expr[:idx]), strings.TrimSpace(expr[idx+2:]), true
	}
	if idx := strings.LastIndex(expr, "/attribute::"); idx != -1 {
		elementPath := strings.TrimSpace(expr[:idx])
		attrName := strings.TrimSpace(expr[idx+1:])
		attrName = strings.TrimPrefix(attrName, "attribute::")
		return elementPath, attrName, true
	}
	return "", "", false
}

func normalizeAttributeElementPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "."
	}
	return path
}

func (r *validationRun) evaluateAttributeSelection(elements []xml.Element, attrName string, nsContext map[string]string) (string, int) {
	attrName = strings.TrimSpace(attrName)
	if attrName == "" {
		return "", 0
	}

	if attrName == "*" {
		var firstValue string
		count := 0
		for _, elem := range elements {
			attrs := elem.Attributes()
			if len(attrs) == 0 {
				continue
			}
			count += len(attrs)
			if firstValue == "" {
				firstValue = attrs[0].Value()
			}
		}
		return firstValue, count
	}

	prefix, local, hasPrefix := xpath.SplitQName(attrName)
	if nsContext == nil || !hasPrefix {
		var firstValue string
		count := 0
		for _, elem := range elements {
			if hasPrefix {
				value := elem.GetAttribute(attrName)
				if value != "" || elem.HasAttribute(attrName) {
					if firstValue == "" {
						firstValue = value
					}
					count++
					continue
				}
			} else {
				value := elem.GetAttribute(local)
				if value != "" || elem.HasAttribute(local) {
					if firstValue == "" {
						firstValue = value
					}
					count++
					continue
				}
				if defaultValue, ok := r.lookupAttributeDefault(elem, types.QName{Local: local}); ok {
					if firstValue == "" {
						firstValue = defaultValue
					}
					count++
				}
			}
		}
		return firstValue, count
	}

	nsURI, ok := nsContext[prefix]
	if !ok {
		return "", 0
	}

	var firstValue string
	count := 0
	for _, elem := range elements {
		for _, attr := range elem.Attributes() {
			if attr.NamespaceURI() != nsURI {
				continue
			}
			if local != "*" && attr.LocalName() != local {
				continue
			}
			if firstValue == "" {
				firstValue = attr.Value()
			}
			count++
		}
		if local != "*" && count == 0 {
			if defaultValue, ok := r.lookupAttributeDefault(elem, types.QName{Namespace: types.NamespaceURI(nsURI), Local: local}); ok {
				if firstValue == "" {
					firstValue = defaultValue
				}
				count++
			}
		}
	}
	return firstValue, count
}

// normalizeQName normalizes a QName value by resolving the namespace prefix to a URI.
// Returns the normalized form "{namespaceURI}local" for comparison.
func (r *validationRun) normalizeQName(value string, elem xml.Element) string {
	evaluator := xpath.New(r.root)
	return evaluator.NormalizeQName(value, elem)
}
