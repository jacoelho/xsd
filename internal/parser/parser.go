package parser

import (
	"fmt"
	"io"
	"strings"

	xsdschema "github.com/jacoelho/xsd/internal/schema"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

// ParseError represents a schema parsing error with an error code
type ParseError struct {
	Code    string
	Message string
	Err     error
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error, if any.
func (e *ParseError) Unwrap() error {
	return e.Err
}

// newParseError creates a new ParseError with the schema-parse-error code
func newParseError(msg string, err error) *ParseError {
	return &ParseError{
		Code:    "schema-parse-error",
		Message: msg,
		Err:     err,
	}
}

// ImportInfo represents an import directive from an XSD schema.
// Imports allow referencing components from a different namespace.
type ImportInfo struct {
	Namespace      string
	SchemaLocation string
}

// IncludeInfo represents an include directive from an XSD schema.
// Includes allow referencing components from the same namespace or no namespace.
type IncludeInfo struct {
	SchemaLocation string
}

// getAttr returns an attribute value with whitespace trimmed.
// XSD attribute values should be normalized per XML spec, so we always trim.
func getAttr(elem xml.Element, name string) string {
	return strings.TrimSpace(elem.GetAttribute(name))
}

// ParseResult contains the parsed schema and import/include directives
type ParseResult struct {
	Schema   *xsdschema.Schema
	Imports  []ImportInfo
	Includes []IncludeInfo
}

// Parse parses an XSD schema from a reader
func Parse(r io.Reader) (*xsdschema.Schema, error) {
	result, err := ParseWithImports(r)
	if err != nil {
		return nil, err
	}
	return result.Schema, nil
}

// ParseWithImports parses an XSD schema and returns import/include information
func ParseWithImports(r io.Reader) (*ParseResult, error) {
	doc, err := xml.Parse(r)
	if err != nil {
		return nil, newParseError("parse XML", err)
	}

	root := doc.DocumentElement()
	if root == nil {
		return nil, fmt.Errorf("empty document")
	}

	if root.LocalName() != "schema" || root.NamespaceURI() != xml.XSDNamespace {
		return nil, fmt.Errorf("root element must be xs:schema, got {%s}%s",
			root.NamespaceURI(), root.LocalName())
	}

	if err := validateSchemaAttributeNamespaces(root); err != nil {
		return nil, err
	}

	schema := xsdschema.NewSchema()

	// check if targetNamespace attribute is present and validate it
	// according to XSD 1.0 spec, targetNamespace cannot be an empty string
	// it must either be absent (no namespace) or have a non-empty value
	// also, schema attributes must be unprefixed (not in XSD namespace)
	targetNSAttr := ""
	targetNSFound := false
	for _, attr := range root.Attributes() {
		// schema attributes must be unprefixed (empty namespace)
		// prefixed attributes like xsd:targetNamespace are invalid
		if attr.LocalName() == "targetNamespace" {
			if attr.NamespaceURI() != "" {
				return nil, fmt.Errorf("schema attribute 'targetNamespace' must be unprefixed (found '%s:targetNamespace')", attr.NamespaceURI())
			}
			targetNSAttr = attr.Value()
			targetNSFound = true
			break
		}
	}
	if !targetNSFound {
		// attribute is absent - this is valid (means no target namespace)
		schema.TargetNamespace = types.NamespaceEmpty
	} else {
		// attribute is present - validate it's not empty
		if targetNSAttr == "" {
			return nil, fmt.Errorf("targetNamespace attribute cannot be empty (must be absent or have a non-empty value)")
		}
		schema.TargetNamespace = types.NamespaceURI(targetNSAttr)
	}

	// note: Go's encoding/xml represents xmlns:prefix attributes with NamespaceURI="xmlns"
	// and the local name is the prefix
	for _, attr := range root.Attributes() {
		if attr.LocalName() == "xmlns" && (attr.NamespaceURI() == "" || attr.NamespaceURI() == xml.XMLNSNamespace) {
			// xmlns="namespace" - default namespace (no prefix)
			schema.NamespaceDecls[""] = attr.Value()
		} else if attr.NamespaceURI() == "xmlns" || attr.NamespaceURI() == xml.XMLNSNamespace {
			// xmlns:prefix="namespace" - prefix is the local name
			prefix := attr.LocalName()
			if attr.Value() == "" {
				return nil, fmt.Errorf("namespace prefix %q cannot be bound to empty namespace", prefix)
			}
			schema.NamespaceDecls[prefix] = attr.Value()
		}
	}

	if root.HasAttribute("elementFormDefault") {
		elemForm := root.GetAttribute("elementFormDefault")
		if elemForm == "" {
			return nil, fmt.Errorf("elementFormDefault attribute cannot be empty")
		}
		switch elemForm {
		case "qualified":
			schema.ElementFormDefault = xsdschema.Qualified
		case "unqualified":
			schema.ElementFormDefault = xsdschema.Unqualified
		default:
			return nil, fmt.Errorf("invalid elementFormDefault attribute value '%s': must be 'qualified' or 'unqualified'", elemForm)
		}
	}

	if root.HasAttribute("attributeFormDefault") {
		attrForm := root.GetAttribute("attributeFormDefault")
		if attrForm == "" {
			return nil, fmt.Errorf("attributeFormDefault attribute cannot be empty")
		}
		switch attrForm {
		case "qualified":
			schema.AttributeFormDefault = xsdschema.Qualified
		case "unqualified":
			schema.AttributeFormDefault = xsdschema.Unqualified
		default:
			return nil, fmt.Errorf("invalid attributeFormDefault attribute value '%s': must be 'qualified' or 'unqualified'", attrForm)
		}
	}

	if root.HasAttribute("blockDefault") {
		blockDefaultAttr := root.GetAttribute("blockDefault")
		if blockDefaultAttr != "" {
			block, err := parseDerivationSetWithValidation(blockDefaultAttr, types.DerivationSet(types.DerivationSubstitution|types.DerivationExtension|types.DerivationRestriction))
			if err != nil {
				return nil, fmt.Errorf("invalid blockDefault attribute value '%s': %w", blockDefaultAttr, err)
			}
			schema.BlockDefault = block
		}
	}

	if root.HasAttribute("finalDefault") {
		finalDefaultAttr := root.GetAttribute("finalDefault")
		if finalDefaultAttr != "" {
			final, err := parseDerivationSetWithValidation(finalDefaultAttr, types.DerivationSet(types.DerivationExtension|types.DerivationRestriction|types.DerivationList|types.DerivationUnion))
			if err != nil {
				return nil, fmt.Errorf("invalid finalDefault attribute value '%s': %w", finalDefaultAttr, err)
			}
			schema.FinalDefault = final
		}
	}

	result := &ParseResult{
		Schema:   schema,
		Imports:  []ImportInfo{},
		Includes: []IncludeInfo{},
	}

	for _, child := range root.Children() {
		if child.NamespaceURI() != xml.XSDNamespace {
			continue
		}

		switch child.LocalName() {
		case "annotation":
			// allowed at top-level; nothing to parse.
		case "import":
			if hasIDAttribute(child) {
				idAttr := child.GetAttribute("id")
				if err := validateIDAttribute(idAttr, "import", schema); err != nil {
					return nil, err
				}
			}
			if err := validateOnlyAnnotationChildren(child, "import"); err != nil {
				return nil, err
			}
			importInfo := ImportInfo{
				Namespace:      child.GetAttribute("namespace"),
				SchemaLocation: child.GetAttribute("schemaLocation"),
			}
			result.Imports = append(result.Imports, importInfo)
		case "include":
			if hasIDAttribute(child) {
				idAttr := child.GetAttribute("id")
				if err := validateIDAttribute(idAttr, "include", schema); err != nil {
					return nil, err
				}
			}
			if err := validateOnlyAnnotationChildren(child, "include"); err != nil {
				return nil, err
			}
			includeInfo := IncludeInfo{
				SchemaLocation: child.GetAttribute("schemaLocation"),
			}
			if includeInfo.SchemaLocation == "" {
				return nil, fmt.Errorf("include directive missing schemaLocation")
			}
			result.Includes = append(result.Includes, includeInfo)
		case "element":
			if err := parseTopLevelElement(child, schema); err != nil {
				return nil, fmt.Errorf("parse element: %w", err)
			}
		case "complexType":
			if err := parseComplexType(child, schema); err != nil {
				return nil, fmt.Errorf("parse complexType: %w", err)
			}
		case "simpleType":
			if err := parseSimpleType(child, schema); err != nil {
				return nil, fmt.Errorf("parse simpleType: %w", err)
			}
		case "group":
			if err := parseTopLevelGroup(child, schema); err != nil {
				return nil, fmt.Errorf("parse group: %w", err)
			}
		case "attribute":
			if err := parseTopLevelAttribute(child, schema); err != nil {
				return nil, fmt.Errorf("parse attribute: %w", err)
			}
		case "attributeGroup":
			if err := parseTopLevelAttributeGroup(child, schema); err != nil {
				return nil, fmt.Errorf("parse attributeGroup: %w", err)
			}
		case "notation":
			if err := parseTopLevelNotation(child, schema); err != nil {
				return nil, fmt.Errorf("parse notation: %w", err)
			}
		case "key", "keyref", "unique":
			return nil, fmt.Errorf("identity constraint '%s' is only allowed as a child of element declarations", child.LocalName())
		case "redefine":
			return nil, fmt.Errorf("redefine is not supported")
		default:
			return nil, fmt.Errorf("unexpected top-level element '%s'", child.LocalName())
		}
	}

	return result, nil
}

// parseTopLevelNotation parses a top-level notation declaration
func parseTopLevelNotation(elem xml.Element, schema *xsdschema.Schema) error {
	if err := validateAllowedAttributes(elem, "notation", map[string]bool{
		"name":   true,
		"id":     true,
		"public": true,
		"system": true,
	}); err != nil {
		return err
	}

	if strings.TrimSpace(elem.DirectTextContent()) != "" {
		return fmt.Errorf("notation must not contain character data")
	}

	// notation must have a name attribute
	name := elem.GetAttribute("name")
	if name == "" {
		return fmt.Errorf("notation must have a 'name' attribute")
	}

	if !types.IsValidNCName(name) {
		return fmt.Errorf("notation name '%s' must be a valid NCName", name)
	}

	if hasIDAttribute(elem) {
		idAttr := elem.GetAttribute("id")
		if err := validateIDAttribute(idAttr, "notation", schema); err != nil {
			return err
		}
	}

	// notation must have either public or system attribute
	// per XSD spec, both public and system can be empty strings (they're URIs)
	// the requirement is that at least ONE attribute must be present
	public := elem.GetAttribute("public")
	system := elem.GetAttribute("system")
	hasPublic := elem.HasAttribute("public")
	hasSystem := elem.HasAttribute("system")
	if !hasPublic && !hasSystem {
		return fmt.Errorf("notation must have either 'public' or 'system' attribute")
	}

	// validate annotation constraints: at most one annotation, must be first
	hasAnnotation := false
	for _, child := range elem.Children() {
		if child.NamespaceURI() != xml.XSDNamespace {
			return fmt.Errorf("notation '%s': unexpected child element '%s'", name, child.LocalName())
		}

		switch child.LocalName() {
		case "annotation":
			if hasAnnotation {
				return fmt.Errorf("notation '%s': at most one annotation is allowed", name)
			}
			hasAnnotation = true
		default:
			// notation can only have annotation as child
			return fmt.Errorf("notation '%s': unexpected child element '%s'", name, child.LocalName())
		}
	}

	// top-level notation names are NCNames, not QNames.
	// the component is always in the schema's target namespace.
	notationQName := types.QName{
		Local:     name,
		Namespace: schema.TargetNamespace,
	}

	if _, exists := schema.NotationDecls[notationQName]; exists {
		return fmt.Errorf("duplicate notation declaration %s", notationQName.String())
	}

	notation := &types.NotationDecl{
		Name:            notationQName,
		Public:          public,
		System:          system,
		SourceNamespace: schema.TargetNamespace,
	}

	// store in schema's global notation declarations
	schema.NotationDecls[notationQName] = notation

	return nil
}