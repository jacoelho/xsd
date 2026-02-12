package parser

import (
	"fmt"

	"github.com/jacoelho/xsd/internal/model"
	"github.com/jacoelho/xsd/internal/xmltree"
)

type schemaAttribute struct {
	namespace string
	local     string
	value     string
}

type schemaNamespaceDecl struct {
	prefix string
	uri    string
}

func applySchemaRootAttributes(schema *Schema, attrs []schemaAttribute, decls []schemaNamespaceDecl) error {
	targetNSAttr := ""
	targetNSFound := false
	for _, attr := range attrs {
		if attr.local != "targetNamespace" {
			continue
		}
		switch attr.namespace {
		case "":
			targetNSAttr = model.ApplyWhiteSpace(attr.value, model.WhiteSpaceCollapse)
			targetNSFound = true
		case xmltree.XSDNamespace:
			return fmt.Errorf("schema attribute 'targetNamespace' must be unprefixed (found '%s:targetNamespace')", attr.namespace)
		}
	}
	if !targetNSFound {
		schema.TargetNamespace = model.NamespaceEmpty
	} else {
		if targetNSAttr == "" {
			return fmt.Errorf("targetNamespace attribute cannot be empty (must be absent or have a non-empty value)")
		}
		schema.TargetNamespace = targetNSAttr
	}

	for _, decl := range decls {
		if decl.prefix == "" {
			schema.NamespaceDecls[""] = decl.uri
			continue
		}
		if decl.uri == "" {
			return fmt.Errorf("namespace prefix %q cannot be bound to empty namespace", decl.prefix)
		}
		schema.NamespaceDecls[decl.prefix] = decl.uri
	}

	if elemForm, ok := unprefixedSchemaAttr(attrs, "elementFormDefault"); ok {
		elemForm = model.ApplyWhiteSpace(elemForm, model.WhiteSpaceCollapse)
		if elemForm == "" {
			return fmt.Errorf("elementFormDefault attribute cannot be empty")
		}
		switch elemForm {
		case "qualified":
			schema.ElementFormDefault = Qualified
		case "unqualified":
			schema.ElementFormDefault = Unqualified
		default:
			return fmt.Errorf("invalid elementFormDefault attribute value '%s': must be 'qualified' or 'unqualified'", elemForm)
		}
	}

	if attrForm, ok := unprefixedSchemaAttr(attrs, "attributeFormDefault"); ok {
		attrForm = model.ApplyWhiteSpace(attrForm, model.WhiteSpaceCollapse)
		if attrForm == "" {
			return fmt.Errorf("attributeFormDefault attribute cannot be empty")
		}
		switch attrForm {
		case "qualified":
			schema.AttributeFormDefault = Qualified
		case "unqualified":
			schema.AttributeFormDefault = Unqualified
		default:
			return fmt.Errorf("invalid attributeFormDefault attribute value '%s': must be 'qualified' or 'unqualified'", attrForm)
		}
	}

	if blockDefault, ok := unprefixedSchemaAttr(attrs, "blockDefault"); ok {
		if model.TrimXMLWhitespace(blockDefault) == "" {
			return fmt.Errorf("blockDefault attribute cannot be empty")
		}
		block, err := parseDerivationSetWithValidation(
			blockDefault,
			model.DerivationSet(model.DerivationSubstitution|model.DerivationExtension|model.DerivationRestriction),
		)
		if err != nil {
			return fmt.Errorf("invalid blockDefault attribute value '%s': %w", blockDefault, err)
		}
		schema.BlockDefault = block
	}

	if finalDefault, ok := unprefixedSchemaAttr(attrs, "finalDefault"); ok {
		if model.TrimXMLWhitespace(finalDefault) == "" {
			return fmt.Errorf("finalDefault attribute cannot be empty")
		}
		final, err := parseDerivationSetWithValidation(
			finalDefault,
			model.DerivationSet(model.DerivationExtension|model.DerivationRestriction|model.DerivationList|model.DerivationUnion),
		)
		if err != nil {
			return fmt.Errorf("invalid finalDefault attribute value '%s': %w", finalDefault, err)
		}
		schema.FinalDefault = final
	}

	return nil
}

func unprefixedSchemaAttr(attrs []schemaAttribute, local string) (string, bool) {
	for _, attr := range attrs {
		if attr.namespace != "" {
			continue
		}
		if attr.local == local {
			return attr.value, true
		}
	}
	return "", false
}
