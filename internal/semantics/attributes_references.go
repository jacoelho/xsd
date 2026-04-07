package semantics

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/value"
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
func validateAttributeReference(sch *parser.Schema, contextQName model.QName, attr *model.AttributeDecl, contextType string) error {
	if !shouldValidateAttributeReference(attr) {
		return nil
	}

	target, err := lookupReferencedAttributeDecl(sch, attr, contextQName, contextType)
	if err != nil {
		return err
	}
	return validateAttributeReferenceValueCompatibility(sch, attr, target, contextQName, contextType)
}

func shouldValidateAttributeReference(attr *model.AttributeDecl) bool {
	// Skip local attribute declarations - they're not references.
	if !attr.IsReference {
		return false
	}
	// Skip built-in XML namespace attributes (xml:base, xml:lang, xml:space).
	return !isBuiltinXMLAttribute(attr)
}

func lookupReferencedAttributeDecl(sch *parser.Schema, attr *model.AttributeDecl, contextQName model.QName, contextType string) (*model.AttributeDecl, error) {
	target, exists := sch.AttributeDecls[attr.Name]
	if !exists {
		return nil, fmt.Errorf("%s %s: attribute reference %s does not exist", contextType, contextQName, attr.Name)
	}
	return target, nil
}

func validateAttributeReferenceValueCompatibility(
	sch *parser.Schema,
	attr *model.AttributeDecl,
	target *model.AttributeDecl,
	contextQName model.QName,
	contextType string,
) error {
	// Per XSD spec "Attribute Use Correct": if the declaration has a fixed value,
	// the referencing use must not provide a default value.
	if attr.HasDefault && target.HasFixed {
		return fmt.Errorf("%s %s: attribute reference '%s' cannot specify a default when declaration is fixed",
			contextType, contextQName, attr.Name)
	}

	// Per XSD spec "Attribute Use Correct": if the reference specifies a fixed value,
	// it must match the referenced attribute's fixed value.
	if !attr.HasFixed || !target.HasFixed {
		return nil
	}
	// per XSD spec "Attribute Use Correct": if the reference specifies a fixed value,
	// it must match the referenced attribute's fixed value.
	match, err := fixedValuesEqual(sch, attr, target)
	if err != nil {
		return fmt.Errorf("%s %s: attribute reference '%s' fixed value comparison failed: %w",
			contextType, contextQName, attr.Name, err)
	}
	if !match {
		return fmt.Errorf("%s %s: attribute reference '%s' fixed value '%s' conflicts with declaration fixed value '%s'",
			contextType, contextQName, attr.Name, attr.Fixed, target.Fixed)
	}
	return nil
}

// isBuiltinXMLAttribute checks if an attribute is a built-in XML namespace attribute.
// XML namespace attributes (xml:base, xml:lang, xml:space) are built-in and always available.
func isBuiltinXMLAttribute(attr *model.AttributeDecl) bool {
	return attr.Name.Namespace == value.XMLNamespace
}

// validateAttributeGroupReference validates that an attribute group reference exists.
// If the reference has the target namespace and is not found, also checks the no-namespace.
// This handles cases where attribute groups from imported schemas with no target namespace
// are referenced without a prefix (resolved to target namespace).
func validateAttributeGroupReference(sch *parser.Schema, agRef, contextQName model.QName) error {
	if attributeGroupReferenceExists(sch, agRef) {
		return nil
	}
	return fmt.Errorf("type %s: attributeGroup reference %s does not exist", contextQName, agRef)
}

func attributeGroupReferenceExists(sch *parser.Schema, agRef model.QName) bool {
	if _, exists := sch.AttributeGroups[agRef]; exists {
		return true
	}
	// If reference has target namespace and not found, also check no-namespace.
	// This handles imported schemas with no target namespace referenced unprefixed.
	if agRef.Namespace != sch.TargetNamespace || sch.TargetNamespace == "" {
		return false
	}

	noNSRef := model.QName{
		Namespace: "",
		Local:     agRef.Local,
	}
	_, exists := sch.AttributeGroups[noNSRef]
	return exists
}
