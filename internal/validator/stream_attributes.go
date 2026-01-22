package validator

import (
	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/types"
)

type attributeIndex struct {
	mapValues map[types.QName]string
	attrs     []streamAttr
}

const attributeMapThreshold = 8

type declaredAttrSet struct {
	mapValues map[types.QName]bool
	list      []declaredAttrEntry
}

type declaredAttrEntry struct {
	name       types.QName
	prohibited bool
}

func (s *declaredAttrSet) reset(size int) {
	if s.mapValues != nil {
		clear(s.mapValues)
		s.list = s.list[:0]
		return
	}
	if size > attributeMapThreshold {
		s.mapValues = make(map[types.QName]bool, size)
		s.list = s.list[:0]
		return
	}
	if cap(s.list) < size {
		s.list = make([]declaredAttrEntry, 0, size)
		return
	}
	s.list = s.list[:0]
}

func (s *declaredAttrSet) add(name types.QName, prohibited bool) {
	if s.mapValues != nil {
		s.mapValues[name] = prohibited
		return
	}
	s.list = append(s.list, declaredAttrEntry{name: name, prohibited: prohibited})
}

func (s declaredAttrSet) lookup(name types.QName) (declared, prohibited bool) {
	if s.mapValues != nil {
		value, ok := s.mapValues[name]
		return ok, value
	}
	for _, entry := range s.list {
		if entry.name == name {
			return true, entry.prohibited
		}
	}
	return false, false
}

func newAttributeIndex(attrs []streamAttr) attributeIndex {
	if len(attrs) > attributeMapThreshold {
		values := make(map[types.QName]string, len(attrs))
		for _, attr := range attrs {
			values[types.QName{
				Namespace: types.NamespaceURI(attr.NamespaceURI()),
				Local:     attr.LocalName(),
			}] = attr.Value()
		}
		return attributeIndex{attrs: attrs, mapValues: values}
	}
	return attributeIndex{attrs: attrs}
}

func (a attributeIndex) Value(ns, local string) (string, bool) {
	if a.mapValues != nil {
		value, ok := a.mapValues[types.QName{Namespace: types.NamespaceURI(ns), Local: local}]
		return value, ok
	}
	for _, attr := range a.attrs {
		if attr.NamespaceURI() == ns && attr.LocalName() == local {
			return attr.Value(), true
		}
	}
	return "", false
}

func (r *streamRun) checkAttributesStream(attrs attributeIndex, decls []*grammar.CompiledAttribute, anyAttr *types.AnyAttribute, scopeDepth, line, column int) []errors.Validation {
	var violations []errors.Validation

	declared := &r.declaredAttrs
	declared.reset(len(decls))
	idCount := 0

	for _, attr := range decls {
		if attr.Use == types.Prohibited {
			continue
		}

		if attr.Use == types.Required {
			if _, ok := attrs.Value(attr.QName.Namespace.String(), attr.QName.Local); !ok {
				violations = append(violations, errors.NewValidationf(errors.ErrRequiredAttributeMissing, r.path.String(),
					"Required attribute '%s' is missing", attr.QName.Local))
			}
		}

		if value, ok := attrs.Value(attr.QName.Namespace.String(), attr.QName.Local); ok {
			if attr.Type != nil {
				valueViolations := r.checkSimpleValue(value, attr.Type, scopeDepth)
				violations = append(violations, valueViolations...)
				if value != "" && len(valueViolations) == 0 {
					violations = append(violations, r.collectIDRefs(value, attr.Type, line, column)...)
				}
				if attr.Type.IDTypeName == "ID" {
					idCount++
				}
			}

			if attr.HasFixed {
				violations = append(violations, r.checkAttributeFixedValue(value, attr, scopeDepth)...)
			}
		} else if attr.Use == types.Optional && (attr.HasFixed || attr.HasDefault) {
			value := attr.Default
			var valueContext map[string]string
			if attr.HasFixed {
				value = attr.Fixed
				if attr.Original != nil {
					valueContext = attr.Original.FixedContext
				}
			} else if attr.Original != nil {
				valueContext = attr.Original.DefaultContext
			}
			if attr.Type != nil {
				var valueViolations []errors.Validation
				if valueContext != nil {
					valueViolations = r.checkSimpleValueWithContext(value, attr.Type, valueContext)
				} else {
					valueViolations = r.checkSimpleValue(value, attr.Type, scopeDepth)
				}
				violations = append(violations, valueViolations...)
				if value != "" && len(valueViolations) == 0 {
					violations = append(violations, r.collectIDRefs(value, attr.Type, line, column)...)
				}
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

		isDeclared, isProhibited := declared.lookup(attrQName)
		if isProhibited {
			violations = append(violations, errors.NewValidationf(errors.ErrAttributeProhibited, r.path.String(),
				"Attribute '%s' is prohibited", attrQName.Local))
			continue
		}

		if !isDeclared && !isSpecialAttribute(attrQName) {
			if anyAttr == nil || !anyAttr.AllowsQName(attrQName) {
				violations = append(violations, errors.NewValidationf(errors.ErrAttributeNotDeclared, r.path.String(),
					"Attribute '%s' is not declared", attrQName.Local))
			} else {
				violations = append(violations, r.checkWildcardAttributeStream(xmlAttr, anyAttr, scopeDepth, line, column)...)
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

func (r *streamRun) checkWildcardAttributeStream(xmlAttr streamAttr, anyAttr *types.AnyAttribute, scopeDepth, line, column int) []errors.Validation {
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

	return r.checkDeclaredAttributeValueStream(xmlAttr.Value(), attrDecl, scopeDepth, line, column)
}

func (r *streamRun) checkDeclaredAttributeValueStream(value string, decl *grammar.CompiledAttribute, scopeDepth, line, column int) []errors.Validation {
	var violations []errors.Validation

	if decl.Type != nil {
		valueViolations := r.checkSimpleValue(value, decl.Type, scopeDepth)
		violations = append(violations, valueViolations...)
		if value != "" && len(valueViolations) == 0 {
			violations = append(violations, r.collectIDRefs(value, decl.Type, line, column)...)
		}
	}

	if decl.HasFixed {
		violations = append(violations, r.checkAttributeFixedValue(value, decl, scopeDepth)...)
	}

	return violations
}
