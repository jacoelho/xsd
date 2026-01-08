package validator

import (
	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

type attributeIndex struct {
	values map[types.QName]string
	attrs  []xml.Attr
}

func newAttributeIndex(attrs []xml.Attr) attributeIndex {
	values := make(map[types.QName]string, len(attrs))
	for _, attr := range attrs {
		values[types.QName{
			Namespace: types.NamespaceURI(attr.NamespaceURI()),
			Local:     attr.LocalName(),
		}] = attr.Value()
	}
	return attributeIndex{values: values, attrs: attrs}
}

func (a attributeIndex) Value(ns, local string) (string, bool) {
	value, ok := a.values[types.QName{Namespace: types.NamespaceURI(ns), Local: local}]
	return value, ok
}

func (r *streamRun) checkAttributesStream(attrs attributeIndex, decls []*grammar.CompiledAttribute, anyAttr *types.AnyAttribute, scopeDepth int) []errors.Validation {
	var violations []errors.Validation

	declared := make(map[types.QName]bool, len(decls))
	idCount := 0

	for _, attr := range decls {
		if attr.Use == types.Prohibited && !attr.HasFixed {
			continue
		}
		declared[attr.QName] = true

		if attr.Use == types.Required {
			if _, ok := attrs.Value(attr.QName.Namespace.String(), attr.QName.Local); !ok {
				violations = append(violations, errors.NewValidationf(errors.ErrRequiredAttributeMissing, r.path.String(),
					"Required attribute '%s' is missing", attr.QName.Local))
			}
		}

		if value, ok := attrs.Value(attr.QName.Namespace.String(), attr.QName.Local); ok {
			if attr.Type != nil {
				violations = append(violations, r.checkSimpleValue(value, attr.Type, scopeDepth)...)
				if value != "" {
					violations = append(violations, r.collectIDRefs(value, attr.Type)...)
				}
				if attr.Type.IDTypeName == "ID" {
					idCount++
				}
			}

			if attr.Fixed != "" {
				var typ types.Type
				if attr.Type != nil {
					typ = attr.Type.Original
				}
				if !fixedValueMatches(value, attr.Fixed, typ) {
					violations = append(violations, errors.NewValidationf(errors.ErrAttributeFixedValue, r.path.String(),
						"Attribute '%s' has fixed value '%s', but found '%s'", attr.QName.Local, attr.Fixed, value))
				}
			}
		} else if attr.Use == types.Optional && (attr.HasFixed || attr.Default != "") {
			value := attr.Default
			if attr.HasFixed {
				value = attr.Fixed
			}
			if attr.Type != nil {
				violations = append(violations, r.checkSimpleValue(value, attr.Type, scopeDepth)...)
				violations = append(violations, r.collectIDRefs(value, attr.Type)...)
				if attr.Type.IDTypeName == "ID" {
					idCount++
				}
			}
		}
	}

	for _, xmlAttr := range attrs.attrs {
		if isXMLNSAttribute(xmlAttr) {
			continue
		}

		attrQName := types.QName{
			Namespace: types.NamespaceURI(xmlAttr.NamespaceURI()),
			Local:     xmlAttr.LocalName(),
		}

		if !declared[attrQName] && !isSpecialAttribute(attrQName) {
			if anyAttr == nil || !anyAttr.AllowsQName(attrQName) {
				violations = append(violations, errors.NewValidationf(errors.ErrAttributeNotDeclared, r.path.String(),
					"Attribute '%s' is not declared", attrQName.Local))
			} else {
				violations = append(violations, r.checkWildcardAttributeStream(xmlAttr, anyAttr, scopeDepth)...)
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
		violations = append(violations, errors.NewValidationf(errors.ErrDuplicateID, r.path.String(),
			"Element has multiple ID attributes"))
	}

	return violations
}

func (r *streamRun) checkWildcardAttributeStream(xmlAttr xml.Attr, anyAttr *types.AnyAttribute, scopeDepth int) []errors.Validation {
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
			return []errors.Validation{errors.NewValidationf(errors.ErrWildcardNotDeclared, r.path.String(),
				"Attribute '%s' is not declared (strict wildcard requires declaration)", attrQName.Local)}
		}
		return nil
	}

	return r.checkDeclaredAttributeValueStream(xmlAttr.Value(), attrDecl, scopeDepth)
}

func (r *streamRun) checkDeclaredAttributeValueStream(value string, decl *grammar.CompiledAttribute, scopeDepth int) []errors.Validation {
	var violations []errors.Validation

	if decl.Type != nil {
		violations = append(violations, r.checkSimpleValue(value, decl.Type, scopeDepth)...)
	}

	if decl.HasFixed {
		var typ types.Type
		if decl.Type != nil {
			typ = decl.Type.Original
		}
		if !fixedValueMatches(value, decl.Fixed, typ) {
			violations = append(violations, errors.NewValidationf(errors.ErrAttributeFixedValue, r.path.String(),
				"Attribute '%s' has fixed value '%s', but found '%s'", decl.QName.Local, decl.Fixed, value))
		}
	}

	return violations
}
