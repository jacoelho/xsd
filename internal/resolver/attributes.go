package resolver

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// validateAttributeReference validates that an attribute reference exists.
// An attribute with Type=nil could be:
// 1. An attribute reference (has ref attribute in XML) - should exist in schema.AttributeDecls
// 2. A local attribute declaration (has name but no type in XML) - won't exist in schema.AttributeDecls, and that's OK
//
// The IsReference field on AttributeDecl distinguishes these cases:
// - IsReference=true: came from ref="..." in XSD, must exist in schema.AttributeDecls
// - IsReference=false: came from name="..." in XSD, local declaration that doesn't need to exist
//
// contextType should be "element" or "type" for error message formatting.
func validateAttributeReference(schema *parser.Schema, contextQName types.QName, attr *types.AttributeDecl, contextType string) error {
	// skip local attribute declarations - they're not references.
	if !attr.IsReference {
		return nil
	}

	// skip built-in XML namespace attributes (xml:base, xml:lang, xml:space).
	if isBuiltinXMLAttribute(attr) {
		return nil
	}

	// this is a reference, so it must exist.
	target, exists := schema.AttributeDecls[attr.Name]
	if !exists {
		return fmt.Errorf("%s %s: attribute reference %s does not exist", contextType, contextQName, attr.Name)
	}

	// per XSD spec "Attribute Use Correct": if the reference specifies a fixed value,
	// it must match the referenced attribute's fixed value.
	if attr.HasFixed && target.HasFixed {
		if attr.Fixed != target.Fixed {
			return fmt.Errorf("%s %s: attribute reference '%s' fixed value '%s' conflicts with declaration fixed value '%s'",
				contextType, contextQName, attr.Name, attr.Fixed, target.Fixed)
		}
	}

	return nil
}

// isBuiltinXMLAttribute checks if an attribute is a built-in XML namespace attribute.
// XML namespace attributes (xml:base, xml:lang, xml:space) are built-in and always available.
func isBuiltinXMLAttribute(attr *types.AttributeDecl) bool {
	return attr.Name.Namespace == xsdxml.XMLNamespace
}

// validateAttributeGroupReference validates that an attribute group reference exists.
// If the reference has the target namespace and is not found, also checks the no-namespace.
// This handles cases where attribute groups from imported schemas with no target namespace
// are referenced without a prefix (resolved to target namespace).
func validateAttributeGroupReference(schema *parser.Schema, agRef, contextQName types.QName) error {
	if _, exists := schema.AttributeGroups[agRef]; !exists {
		// if reference has target namespace and not found, also check no-namespace.
		// this handles cases where attribute groups from imported schemas with no
		// target namespace are referenced without a prefix (resolved to target namespace).
		if agRef.Namespace == schema.TargetNamespace && !schema.TargetNamespace.IsEmpty() {
			noNSRef := types.QName{
				Namespace: "",
				Local:     agRef.Local,
			}
			if _, exists := schema.AttributeGroups[noNSRef]; !exists {
				return fmt.Errorf("type %s: attributeGroup reference %s does not exist", contextQName, agRef)
			}
		} else {
			return fmt.Errorf("type %s: attributeGroup reference %s does not exist", contextQName, agRef)
		}
	}
	return nil
}

// validateNoCyclicAttributeGroups detects cycles between attribute group definitions.
func validateNoCyclicAttributeGroups(schema *parser.Schema) error {
	visiting := make(map[types.QName]bool)
	visited := make(map[types.QName]bool)

	var visit func(types.QName) error
	visit = func(qname types.QName) error {
		if visited[qname] {
			return nil
		}
		if visiting[qname] {
			return fmt.Errorf("circular attribute group definition: %s", qname)
		}
		visiting[qname] = true
		group, exists := schema.AttributeGroups[qname]
		if !exists {
			visiting[qname] = false
			return nil
		}
		for _, ref := range group.AttrGroups {
			if _, ok := schema.AttributeGroups[ref]; !ok {
				continue
			}
			if err := visit(ref); err != nil {
				return err
			}
		}
		visiting[qname] = false
		visited[qname] = true
		return nil
	}

	for qname := range schema.AttributeGroups {
		if err := visit(qname); err != nil {
			return err
		}
	}
	return nil
}

func validateAttributeValueConstraintsForType(schema *parser.Schema, typ types.Type) error {
	ct, ok := typ.(*types.ComplexType)
	if !ok {
		return nil
	}
	validateAttrs := func(attrs []*types.AttributeDecl) error {
		for _, attr := range attrs {
			if err := validateAttributeValueConstraints(schema, attr); err != nil {
				return fmt.Errorf("attribute %s: %w", attr.Name, err)
			}
		}
		return nil
	}
	if err := validateAttrs(ct.Attributes()); err != nil {
		return err
	}
	if ext := ct.Content().ExtensionDef(); ext != nil {
		if err := validateAttrs(ext.Attributes); err != nil {
			return err
		}
	}
	if restr := ct.Content().RestrictionDef(); restr != nil {
		if err := validateAttrs(restr.Attributes); err != nil {
			return err
		}
	}
	return nil
}

func validateAttributeValueConstraints(schema *parser.Schema, decl *types.AttributeDecl) error {
	resolvedType := resolveTypeForFinalValidation(schema, decl.Type)
	if _, ok := resolvedType.(*types.ComplexType); ok {
		return fmt.Errorf("type must be a simple type")
	}
	if isDirectNotationType(resolvedType) {
		return fmt.Errorf("attribute cannot use NOTATION type")
	}
	if decl.Default != "" {
		if err := validateDefaultOrFixedValueWithResolvedType(schema, decl.Default, resolvedType); err != nil {
			return fmt.Errorf("invalid default value '%s': %w", decl.Default, err)
		}
	}
	if decl.HasFixed {
		if err := validateDefaultOrFixedValueWithResolvedType(schema, decl.Fixed, resolvedType); err != nil {
			return fmt.Errorf("invalid fixed value '%s': %w", decl.Fixed, err)
		}
	}
	return nil
}
