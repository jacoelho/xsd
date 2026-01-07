package validator

import (
	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

func (r *validationRun) checkAttributes(elem xml.Element, attrs []*grammar.CompiledAttribute, anyAttr *types.AnyAttribute, path string) []errors.Validation {
	var violations []errors.Validation

	declared := make(map[types.QName]bool)
	idCount := 0

	for _, attr := range attrs {
		if attr.Use == types.Prohibited && !attr.HasFixed {
			continue
		}
		declared[attr.QName] = true

		if attr.Use == types.Required {
			attrValue := elem.GetAttributeNS(attr.QName.Namespace.String(), attr.QName.Local)
			if attrValue == "" {
				violations = append(violations, errors.NewValidationf(errors.ErrRequiredAttributeMissing, path,
					"Required attribute '%s' is missing", attr.QName.Local))
			}
		}

		// validate attribute value only if the attribute is actually present in the XML.
		// an absent optional attribute should not be validated against type facets.
		// note: We validate even empty values (attr="") because facets must validate empty strings.
		if elem.HasAttributeNS(attr.QName.Namespace.String(), attr.QName.Local) {
			value := elem.GetAttributeNS(attr.QName.Namespace.String(), attr.QName.Local)
			if attr.Type != nil {
				violations = append(violations, r.checkSimpleValue(value, attr.Type, path, elem)...)
				if value != "" {
					violations = append(violations, r.collectIDRefs(value, attr.Type, path)...)
				}
				if attr.Type.IDTypeName == "ID" {
					idCount++
				}
			}

			// both values must be normalized according to the type's whitespace facet before comparison
			if attr.Fixed != "" {
				var typ types.Type
				if attr.Type != nil {
					typ = attr.Type.Original
				}
				if !fixedValueMatches(value, attr.Fixed, typ) {
					violations = append(violations, errors.NewValidationf(errors.ErrAttributeFixedValue, path,
						"Attribute '%s' has fixed value '%s', but found '%s'", attr.QName.Local, attr.Fixed, value))
				}
			}
		} else if attr.Use == types.Optional && (attr.HasFixed || attr.Default != "") {
			value := attr.Default
			if attr.HasFixed {
				value = attr.Fixed
			}
			if attr.Type != nil {
				violations = append(violations, r.checkSimpleValue(value, attr.Type, path, elem)...)
				violations = append(violations, r.collectIDRefs(value, attr.Type, path)...)
				if attr.Type.IDTypeName == "ID" {
					idCount++
				}
			}
		}
	}

	for _, xmlAttr := range elem.Attributes() {
		if isXMLNSAttribute(xmlAttr) {
			continue
		}

		attrQName := types.QName{
			Namespace: types.NamespaceURI(xmlAttr.NamespaceURI()),
			Local:     xmlAttr.LocalName(),
		}

		if !declared[attrQName] && !isSpecialAttribute(attrQName) {
			if anyAttr == nil || !anyAttr.AllowsQName(attrQName) {
				violations = append(violations, errors.NewValidationf(errors.ErrAttributeNotDeclared, path,
					"Attribute '%s' is not declared", attrQName.Local))
			} else {
				violations = append(violations, r.checkWildcardAttribute(xmlAttr, anyAttr, path, elem)...)
				if anyAttr.ProcessContents != types.Skip {
					if attrDecl := r.schema.Attribute(attrQName); attrDecl != nil && attrDecl.Type != nil {
						if attrDecl.Type.IDTypeName == "ID" {
							idCount++
						}
					}
				}
			}
		}
	}

	if idCount > 1 {
		violations = append(violations, errors.NewValidationf(errors.ErrDuplicateID, path,
			"Element has multiple ID attributes"))
	}

	return violations
}

// checkWildcardAttribute validates an attribute matched by anyAttribute wildcard.
// elem is the element containing the attribute, needed for NOTATION validation.
func (r *validationRun) checkWildcardAttribute(xmlAttr xml.Attr, anyAttr *types.AnyAttribute, path string, elem xml.Element) []errors.Validation {
	if anyAttr.ProcessContents == types.Skip {
		return nil
	}

	attrQName := types.QName{
		Namespace: types.NamespaceURI(xmlAttr.NamespaceURI()),
		Local:     xmlAttr.LocalName(),
	}

	attrDecl := r.schema.Attribute(attrQName)
	if attrDecl == nil {
		if anyAttr.ProcessContents == types.Strict {
			return []errors.Validation{errors.NewValidationf(errors.ErrWildcardNotDeclared, path,
				"Attribute '%s' is not declared (strict wildcard requires declaration)", attrQName.Local)}
		}
		// lax mode: no error when not found
		return nil
	}

	return r.checkDeclaredAttributeValue(xmlAttr.Value(), attrDecl, path, elem)
}

// checkDeclaredAttributeValue validates an attribute value against its declaration.
// elem is optional and only needed for NOTATION validation.
func (r *validationRun) checkDeclaredAttributeValue(value string, decl *grammar.CompiledAttribute, path string, elem xml.Element) []errors.Validation {
	var violations []errors.Validation

	if decl.Type != nil {
		violations = append(violations, r.checkSimpleValue(value, decl.Type, path, elem)...)
	}

	// check fixed constraint - both values must be normalized per type's whitespace facet
	if decl.HasFixed {
		var typ types.Type
		if decl.Type != nil {
			typ = decl.Type.Original
		}
		if !fixedValueMatches(value, decl.Fixed, typ) {
			violations = append(violations, errors.NewValidationf(errors.ErrAttributeFixedValue, path,
				"Attribute '%s' has fixed value '%s', but found '%s'", decl.QName.Local, decl.Fixed, value))
		}
	}

	return violations
}