package semanticresolve

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/parser"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xsdxml"
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
func validateAttributeReference(sch *parser.Schema, contextQName types.QName, attr *types.AttributeDecl, contextType string) error {
	// skip local attribute declarations - they're not references.
	if !attr.IsReference {
		return nil
	}

	// skip built-in XML namespace attributes (xml:base, xml:lang, xml:space).
	if isBuiltinXMLAttribute(attr) {
		return nil
	}

	// this is a reference, so it must exist.
	target, exists := sch.AttributeDecls[attr.Name]
	if !exists {
		return fmt.Errorf("%s %s: attribute reference %s does not exist", contextType, contextQName, attr.Name)
	}

	// per XSD spec "Attribute Use Correct": if the declaration has a fixed value,
	// the referencing use must not provide a default value.
	if attr.HasDefault && target.HasFixed {
		return fmt.Errorf("%s %s: attribute reference '%s' cannot specify a default when declaration is fixed",
			contextType, contextQName, attr.Name)
	}

	// per XSD spec "Attribute Use Correct": if the reference specifies a fixed value,
	// it must match the referenced attribute's fixed value.
	if attr.HasFixed && target.HasFixed {
		match, err := fixedValuesEqual(sch, attr, target)
		if err != nil {
			return fmt.Errorf("%s %s: attribute reference '%s' fixed value comparison failed: %w",
				contextType, contextQName, attr.Name, err)
		}
		if !match {
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
func validateAttributeGroupReference(sch *parser.Schema, agRef, contextQName types.QName) error {
	if _, exists := sch.AttributeGroups[agRef]; !exists {
		// if reference has target namespace and not found, also check no-namespace.
		// this handles cases where attribute groups from imported schemas with no
		// target namespace are referenced without a prefix (resolved to target namespace).
		if agRef.Namespace == sch.TargetNamespace && !sch.TargetNamespace.IsEmpty() {
			noNSRef := types.QName{
				Namespace: "",
				Local:     agRef.Local,
			}
			if _, exists := sch.AttributeGroups[noNSRef]; !exists {
				return fmt.Errorf("type %s: attributeGroup reference %s does not exist", contextQName, agRef)
			}
		} else {
			return fmt.Errorf("type %s: attributeGroup reference %s does not exist", contextQName, agRef)
		}
	}
	return nil
}
