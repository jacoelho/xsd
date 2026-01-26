package schemacheck

import (
	"fmt"
	"slices"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	xsdxml "github.com/jacoelho/xsd/internal/xml"
)

// validateAttributeDeclStructure validates structural constraints of an attribute declaration
// Does not validate references (which might be forward references or imports)
func validateAttributeDeclStructure(schemaDef *parser.Schema, qname types.QName, decl *types.AttributeDecl) error {
	// this is a structural constraint that is definitely invalid if violated
	if !isValidNCName(qname.Local) {
		return fmt.Errorf("invalid attribute name '%s': must be a valid NCName", qname.Local)
	}
	if qname.Local == "xmlns" {
		return fmt.Errorf("invalid attribute name '%s': reserved XMLNS name", qname.Local)
	}
	effectiveNamespace := qname.Namespace
	if !decl.IsReference {
		switch decl.Form {
		case types.FormQualified:
			effectiveNamespace = schemaDef.TargetNamespace
		case types.FormUnqualified:
			effectiveNamespace = ""
		default:
			if schemaDef.AttributeFormDefault == parser.Qualified {
				effectiveNamespace = schemaDef.TargetNamespace
			}
		}
	}
	if effectiveNamespace == xsdxml.XSINamespace {
		return fmt.Errorf("invalid attribute name '%s': attributes in the xsi namespace are not allowed", qname.Local)
	}

	// validate inline types (simpleType defined inline in the attribute)
	if decl.Type != nil {
		if st, ok := decl.Type.(*types.SimpleType); ok {
			if err := validateSimpleTypeStructure(schemaDef, st); err != nil {
				return fmt.Errorf("inline simpleType: %w", err)
			}
		}
	}

	// per XSD spec: cannot have both default and fixed on an attribute
	// (au-props-correct constraint 2)
	if decl.HasDefault && decl.HasFixed {
		return fmt.Errorf("attribute cannot have both 'default' and 'fixed' values")
	}

	// per XSD spec: if use="required", default is not allowed
	// (au-props-correct constraint 1)
	if decl.Use == types.Required && decl.HasDefault {
		return fmt.Errorf("attribute with use='required' cannot have a default value")
	}
	// per XSD spec: if use="prohibited", default is not allowed
	// (au-props-correct constraint - prohibited attributes cannot have defaults)
	if decl.Use == types.Prohibited && decl.HasDefault {
		return fmt.Errorf("attribute with use='prohibited' cannot have a default value")
	}
	// validate default value if present (basic validation only - full type checking after resolution)
	if decl.HasDefault {
		if err := validateDefaultOrFixedValueWithContext(schemaDef, decl.Default, decl.Type, decl.DefaultContext); err != nil {
			return fmt.Errorf("invalid default value '%s': %w", decl.Default, err)
		}
	}

	// validate fixed value if present (basic validation only - full type checking after resolution)
	if decl.HasFixed {
		if err := validateDefaultOrFixedValueWithContext(schemaDef, decl.Fixed, decl.Type, decl.FixedContext); err != nil {
			return fmt.Errorf("invalid fixed value '%s': %w", decl.Fixed, err)
		}
	}

	// don't validate type references - they might be forward references or from imports

	return nil
}

// validateAttributeGroupStructure validates structural constraints of an attribute group
func validateAttributeGroupStructure(schema *parser.Schema, qname types.QName, ag *types.AttributeGroup) error {
	// this is a structural constraint that is definitely invalid if violated
	if !isValidNCName(qname.Local) {
		return fmt.Errorf("invalid attributeGroup name '%s': must be a valid NCName", qname.Local)
	}

	// validate attribute declarations - only structural constraints
	for _, attr := range ag.Attributes {
		if err := validateAttributeDeclStructure(schema, attr.Name, attr); err != nil {
			return fmt.Errorf("attribute: %w", err)
		}
	}

	if err := validateAttributeGroupUniqueness(schema, ag); err != nil {
		return err
	}

	// don't validate attribute group references - they might be forward references

	return nil
}

// validateAttributeUniqueness validates that no two attributes in a complex type
// share the same name and namespace.
func validateAttributeUniqueness(schema *parser.Schema, ct *types.ComplexType) error {
	allAttributes := collectAllAttributesForValidation(schema, ct)

	seen := make(map[types.QName]bool)
	for _, attr := range allAttributes {
		key := effectiveAttributeQNameForValidation(schema, attr)
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
		key := effectiveAttributeQNameForValidation(schema, attr)
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
		key := effectiveAttributeQNameForValidation(schema, attr)
		if seen[key] {
			return fmt.Errorf("duplicate attribute '%s' in namespace '%s'", attr.Name.Local, attr.Name.Namespace)
		}
		seen[key] = true
	}
	return nil
}

// collectAllAttributesForValidation collects all attributes from a complex type
// This includes attributes from extensions, restrictions, and attribute groups
// Note: We don't recursively collect from base types since they might not be fully resolved
// during schema schemacheck. This checks for duplicates within the same type definition.
func collectAllAttributesForValidation(schema *parser.Schema, ct *types.ComplexType) []*types.AttributeDecl {
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

func collectEffectiveAttributeUses(schema *parser.Schema, ct *types.ComplexType) map[types.QName]*types.AttributeDecl {
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

func collectComplexTypeChain(schema *parser.Schema, ct *types.ComplexType) []*types.ComplexType {
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
				if baseCT, ok := lookupComplexType(schema, baseQName); ok {
					next = baseCT
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

func mergeAttributesFromTypeForValidation(schema *parser.Schema, ct *types.ComplexType, attrMap map[types.QName]*types.AttributeDecl) {
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

func mergeAttributesFromGroupsForValidation(schema *parser.Schema, agRefs []types.QName, attrMap map[types.QName]*types.AttributeDecl) {
	for _, agRef := range agRefs {
		ag, ok := schema.AttributeGroups[agRef]
		if !ok {
			continue
		}
		mergeAttributesFromGroupForValidation(schema, ag, attrMap)
	}
}

func mergeAttributesFromGroupForValidation(schema *parser.Schema, ag *types.AttributeGroup, attrMap map[types.QName]*types.AttributeDecl) {
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
func collectAttributesFromGroups(schema *parser.Schema, agRefs []types.QName, visited map[types.QName]bool) []*types.AttributeDecl {
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
func effectiveAttributeQNameForValidation(sch *parser.Schema, attr *types.AttributeDecl) types.QName {
	if attr.IsReference {
		return attr.Name
	}
	form := attr.Form
	if form == types.FormDefault {
		if sch.AttributeFormDefault == parser.Qualified {
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
