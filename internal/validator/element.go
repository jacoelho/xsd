package validator

import (
	"strings"

	"github.com/jacoelho/xsd/errors"
	"github.com/jacoelho/xsd/internal/grammar"
	"github.com/jacoelho/xsd/internal/grammar/contentmodel"
	"github.com/jacoelho/xsd/internal/types"
	"github.com/jacoelho/xsd/internal/xml"
)

func (r *validationRun) checkElement(elem xml.NodeID) []errors.Validation {
	return r.checkElementWithProcessContents(elem, types.Strict, errors.ErrElementNotDeclared)
}

// checkElementWithProcessContents validates an element using processContents rules.
// missingCode controls the error code used when strict processing finds no declaration.
func (r *validationRun) checkElementWithProcessContents(elem xml.NodeID, processContents types.ProcessContents, missingCode errors.ErrorCode) []errors.Validation {
	if processContents == types.Skip {
		return nil
	}

	qname := r.resolveElementQName(elem)
	decl := r.findElementDeclaration(qname)

	if decl == nil {
		// check for xsi:type - per XSD spec, if no element declaration exists but
		// xsi:type is specified, we can validate using just the type definition.
		// this is especially common for "schema-less" root elements.
		xsiTypeAttr := r.doc.GetAttributeNS(elem, xml.XSINamespace, "type")
		if xsiTypeAttr != "" {
			xsiType, err := r.resolveXsiTypeOnly(elem, xsiTypeAttr)
			if err == nil && xsiType != nil {
				return r.checkElementWithType(elem, xsiType)
			}
		}

		switch processContents {
		case types.Strict:
			if missingCode == errors.ErrWildcardNotDeclared {
				return []errors.Validation{errors.NewValidationf(missingCode, r.path.String(),
					"Element '%s' is not declared (strict wildcard requires declaration)", r.doc.LocalName(elem))}
			}
			return []errors.Validation{errors.NewValidationf(missingCode, r.path.String(),
				"Cannot find declaration for element '%s'", r.doc.LocalName(elem))}
		case types.Lax:
			return nil
		}
		return nil
	}

	return r.checkElementWithDecl(elem, decl)
}

func (r *validationRun) checkElementWithDecl(elem xml.NodeID, decl *grammar.CompiledElement) []errors.Validation {
	var violations []errors.Validation

	actualQName := r.resolveElementQName(elem)
	if decl != nil && actualQName != decl.QName {
		decl = r.resolveSubstitutionDecl(actualQName, decl)
	}

	if decl.Abstract {
		return []errors.Validation{errors.NewValidationf(errors.ErrElementAbstract, r.path.String(),
			"Element '%s' is abstract and cannot be used directly in instance documents", r.doc.LocalName(elem))}
	}

	nilAttr := r.doc.GetAttributeNS(elem, xml.XSINamespace, "nil")
	hasNilAttr := nilAttr != ""
	isNil := nilAttr == "true" || nilAttr == "1"

	// per XSD spec, xsi:nil can only appear on elements declared as nillable
	// this is true even if the value is "false"
	if hasNilAttr && !decl.Nillable {
		violations = append(violations, errors.NewValidationf(errors.ErrElementNotNillable, r.path.String(),
			"Element '%s' is not nillable but has xsi:nil attribute", r.doc.LocalName(elem)))
		return violations
	}

	if decl.Type == nil {
		if isNil {
			// no type, nillable element with xsi:nil=true - just check for empty content
			textContent := strings.TrimSpace(r.doc.TextContent(elem))
			if textContent != "" || len(r.doc.Children(elem)) > 0 {
				violations = append(violations, errors.NewValidation(errors.ErrNilElementNotEmpty,
					"Element with xsi:nil='true' must be empty", r.path.String()))
			}
		}
		return violations
	}

	effectiveType := decl.Type
	xsiTypeAttr := r.doc.GetAttributeNS(elem, xml.XSINamespace, "type")
	if xsiTypeAttr != "" {
		xsiType, err := r.resolveXsiType(elem, xsiTypeAttr, decl.Type, decl.Block)
		if err != nil {
			violations = append(violations, errors.NewValidation(errors.ErrXsiTypeInvalid, err.Error(), r.path.String()))
			return violations
		}
		if xsiType != nil {
			effectiveType = xsiType
		}
	}

	if effectiveType.Abstract {
		violations = append(violations, errors.NewValidationf(errors.ErrElementTypeAbstract, r.path.String(),
			"Type '%s' is abstract and cannot be used for element '%s'", effectiveType.QName.String(), r.doc.LocalName(elem)))
		return violations
	}

	// when xsi:nil is true, validate that element is empty but still check attributes
	// per XSD spec - the type's attribute declarations still apply
	if isNil {
		// per XSD spec: if element has a fixed value, xsi:nil="true" is invalid
		// because fixed values require element content, but xsi:nil means no content
		if decl.HasFixed {
			violations = append(violations, errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
				"Element '%s' has a fixed value constraint and cannot be nil", r.doc.LocalName(elem)))
			return violations
		}
		textContent := strings.TrimSpace(r.doc.TextContent(elem))
		if textContent != "" || len(r.doc.Children(elem)) > 0 {
			violations = append(violations, errors.NewValidation(errors.ErrNilElementNotEmpty,
				"Element with xsi:nil='true' must be empty", r.path.String()))
		}
		// still validate attributes against the effective type
		violations = append(violations, r.checkAttributes(elem, effectiveType.AllAttributes, effectiveType.AnyAttribute)...)
		return violations
	}

	return r.checkElementContent(elem, effectiveType, decl)
}

// checkElementContent validates element content using aspect-based validation.
// This unified approach handles simple types, complex types with simpleContent,
// and complex types with element content.
func (r *validationRun) checkElementContent(elem xml.NodeID, ct *grammar.CompiledType, decl *grammar.CompiledElement) []errors.Validation {
	if isAnyType(ct) {
		return r.validateAnyTypeContent(elem, decl)
	}

	var violations []errors.Validation

	// always check attributes, even if type has none declared, to reject undeclared attributes
	// when AnyAttribute is nil (empty intersection) or absent.
	violations = append(violations, r.checkAttributes(elem, ct.AllAttributes, ct.AnyAttribute)...)

	textViolations, stop := r.validateTextContent(elem, ct, decl)
	violations = append(violations, textViolations...)
	if stop {
		return violations
	}

	emptyViolations, stop := r.validateEmptyContentModel(elem, ct)
	violations = append(violations, emptyViolations...)
	if stop {
		return violations
	}

	violations = append(violations, r.validateContentModel(elem, ct, decl)...)

	return violations
}

func (r *validationRun) validateAnyTypeContent(elem xml.NodeID, decl *grammar.CompiledElement) []errors.Validation {
	var violations []errors.Validation

	// for anyType, we still need to check fixed values before returning
	// per XSD spec 3.3.4: If there is a fixed {value constraint}, the element
	// information item must have no element information item children.
	if decl != nil && decl.HasFixed {
		if len(r.doc.Children(elem)) > 0 {
			violations = append(violations, errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
				"Element '%s' has a fixed value constraint and cannot have element children", decl.QName.Local))
		} else {
			textContent := r.doc.TextContent(elem)
			// per XSD spec 9.1.1: If element is empty, the fixed value is supplied.
			// only validate non-empty content (anyType has whiteSpace=preserve).
			if textContent != "" && textContent != decl.Fixed {
				violations = append(violations, errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
					"Element has fixed value '%s' but actual value is '%s'", decl.Fixed, textContent))
			}
		}
	}

	violations = append(violations, r.checkAnyTypeContent(elem)...)
	return violations
}

func (r *validationRun) validateTextContent(elem xml.NodeID, ct *grammar.CompiledType, decl *grammar.CompiledElement) ([]errors.Validation, bool) {
	var violations []errors.Validation

	textType := ct.TextType()
	if textType != nil {
		text := r.doc.TextContent(elem)
		hadContent := text != ""
		// if element is empty and has a default or fixed value, use it
		if text == "" && decl != nil {
			if decl.Default != "" {
				text = decl.Default
			} else if decl.HasFixed {
				text = decl.Fixed
			}
		}
		violations = append(violations, r.checkSimpleValue(text, textType, elem)...)
		violations = append(violations, r.collectIDRefs(text, textType)...)

		// also validate additional facets on complex types with simpleContent
		if ct.Kind == grammar.TypeKindComplex && len(ct.Facets) > 0 {
			violations = append(violations, r.checkComplexTypeFacets(text, ct)...)
		}

		// check fixed value constraint only when element had actual content.
		// if element was empty, fixed value was applied above and is valid by definition.
		if decl != nil && decl.HasFixed && hadContent {
			violations = append(violations, r.checkFixedValue(text, decl.Fixed, textType)...)
		}

		// simple type content or complex type with simple content cannot have element children
		// (only text content is allowed)
		// exception: If the type has a content model, it can have element children
		if len(r.doc.Children(elem)) > 0 && !ct.HasContentModel() {
			for _, child := range r.doc.Children(elem) {
				r.path.push(r.doc.LocalName(child))
				violations = append(violations, errors.NewValidationf(errors.ErrTextInElementOnly, r.path.String(),
					"Element '%s' is not allowed in simple content", r.doc.LocalName(child)))
				r.path.pop()
			}
			return violations, true
		}
		return violations, false
	}

	if ct.Kind == grammar.TypeKindComplex && ct.Mixed && !ct.HasContentModel() {
		// mixed content type with no content model (empty mixed content)
		// per XSD spec 3.3.4: If there is a fixed {value constraint}, the element
		// information item must have no element information item children.
		if decl != nil && decl.HasFixed {
			if len(r.doc.Children(elem)) > 0 {
				violations = append(violations, errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
					"Element '%s' has a fixed value constraint and cannot have element children", decl.QName.Local))
			} else {
				textContent := r.doc.TextContent(elem)
				violations = append(violations, r.checkFixedValue(textContent, decl.Fixed, textTypeForFixedValue(decl))...)
			}
		}
		return violations, false
	}

	if ct.Kind == grammar.TypeKindComplex && !ct.AllowsText() {
		// complex type that doesn't allow text (empty content or element-only content)
		// reject non-whitespace DIRECT text content (text nodes that are direct children,
		// not text inside nested child elements)
		if text := r.doc.DirectTextContentBytes(elem); !isWhitespaceOnlyBytes(text) {
			violations = append(violations, errors.NewValidation(errors.ErrTextInElementOnly,
				"Element content cannot have character children (non-whitespace text found)", r.path.String()))
		}
	}

	return violations, false
}

func (r *validationRun) validateEmptyContentModel(elem xml.NodeID, ct *grammar.CompiledType) ([]errors.Validation, bool) {
	if ct.ContentModel == nil || !ct.ContentModel.Empty || ct.ContentModel.Mixed {
		return nil, false
	}

	var violations []errors.Validation
	if children := r.doc.Children(elem); len(children) > 0 {
		for _, child := range children {
			r.path.push(r.doc.LocalName(child))
			violations = append(violations, errors.NewValidationf(errors.ErrUnexpectedElement, r.path.String(),
				"Element '%s' is not allowed. No element declaration found for it in the empty content model.", r.doc.LocalName(child)))
			r.path.pop()
		}
		return violations, true
	}

	return nil, false
}

func (r *validationRun) validateContentModel(elem xml.NodeID, ct *grammar.CompiledType, decl *grammar.CompiledElement) []errors.Validation {
	if !ct.HasContentModel() {
		return nil
	}

	var violations []errors.Validation

	// check element-only constraint (redundant if we already checked above, but
	// this is more specific for element-only content models)
	if !ct.Mixed {
		if directText := r.doc.DirectTextContentBytes(elem); !isWhitespaceOnlyBytes(directText) {
			violations = append(violations, errors.NewValidation(errors.ErrTextInElementOnly,
				"Element-only content cannot have character children (non-whitespace text found)", r.path.String()))
		}
	}

	// per XSD spec 3.3.4: If there is a fixed {value constraint}, the element
	// information item must have no element information item children.
	// this applies even to mixed content types.
	if decl != nil && decl.HasFixed && len(r.doc.Children(elem)) > 0 {
		violations = append(violations, errors.NewValidationf(errors.ErrElementFixedValue, r.path.String(),
			"Element '%s' has a fixed value constraint and cannot have element children", decl.QName.Local))
	} else if ct.Mixed && decl != nil && decl.HasFixed {
		// for mixed content types with fixed value (and no element children),
		// if element is empty, use the fixed value as the default (like simple types)
		textContent := r.doc.TextContent(elem)
		if textContent == "" {
			textContent = decl.Fixed
		}
		violations = append(violations, r.checkFixedValue(textContent, decl.Fixed, textTypeForFixedValue(decl))...)
	}

	matches, contentViolations := r.checkContentModel(elem, ct.ContentModel)
	violations = append(violations, contentViolations...)

	// recurse into children based on matches
	violations = append(violations, r.checkMatchedChildren(elem, ct.ContentModel, matches)...)

	return violations
}

// checkAnyTypeContent handles validation for xs:anyType which allows any content.
func (r *validationRun) checkAnyTypeContent(elem xml.NodeID) []errors.Validation {
	var violations []errors.Validation

	// anyType allows any attributes - use a wildcard that allows all
	anyAttr := &types.AnyAttribute{
		Namespace:       types.NSCAny,
		ProcessContents: types.Lax,
		TargetNamespace: types.NamespaceEmpty,
	}
	violations = append(violations, r.checkAttributes(elem, nil, anyAttr)...)

	// anyType allows any child elements - validate in lax mode
	for _, child := range r.doc.Children(elem) {
		r.path.push(r.doc.LocalName(child))
		violations = append(violations, r.checkElementWithProcessContents(child, types.Lax, errors.ErrWildcardNotDeclared)...)
		r.path.pop()
	}

	return violations
}

// checkMatchedChildren validates child elements based on content model match results.
func (r *validationRun) checkMatchedChildren(elem xml.NodeID, cm *grammar.CompiledContentModel, matches []contentmodel.MatchResult) []errors.Validation {
	var violations []errors.Validation
	children := r.doc.Children(elem)
	var decls []*grammar.CompiledElement
	if cm != nil {
		if mappedDecls, ok := r.matchChildrenInSimpleSequence(children, cm); ok {
			decls = mappedDecls
		}
	}
	var contentModelDecls map[types.QName]*grammar.CompiledElement
	if cm != nil {
		contentModelDecls = cm.ElementIndex
	}

	for i, child := range children {
		r.path.push(r.doc.LocalName(child))

		// if content model validation failed, don't validate unmatched children
		if i >= len(matches) {
			r.path.pop()
			continue
		}

		// check if this child was matched by a wildcard
		match := matches[i]
		if match.IsWildcard {
			switch matches[i].ProcessContents {
			case types.Skip:
				r.path.pop()
				continue
			case types.Lax:
				violations = append(violations, r.checkElementWithProcessContents(child, types.Lax, errors.ErrWildcardNotDeclared)...)
				r.path.pop()
				continue
			case types.Strict:
				violations = append(violations, r.checkElementWithProcessContents(child, types.Strict, errors.ErrWildcardNotDeclared)...)
				r.path.pop()
				continue
			}
		}

		// regular element validation
		if !match.MatchedQName.IsZero() {
			if contentModelDecls != nil {
				if decl := contentModelDecls[match.MatchedQName]; decl != nil {
					violations = append(violations, r.checkElementWithDecl(child, decl)...)
					r.path.pop()
					continue
				}
			}
			if decl := r.schema.Element(match.MatchedQName); decl != nil {
				violations = append(violations, r.checkElementWithDecl(child, decl)...)
				r.path.pop()
				continue
			}
			if decl := r.findBySubstitution(match.MatchedQName); decl != nil {
				violations = append(violations, r.checkElementWithDecl(child, decl)...)
				r.path.pop()
				continue
			}
		}
		if i < len(decls) && decls[i] != nil {
			violations = append(violations, r.checkElementWithDecl(child, decls[i])...)
		} else {
			violations = append(violations, r.checkElement(child)...)
		}
		r.path.pop()
	}

	return violations
}

func (r *validationRun) matchChildrenInSimpleSequence(children []xml.NodeID, cm *grammar.CompiledContentModel) ([]*grammar.CompiledElement, bool) {
	if cm == nil || len(children) == 0 {
		return nil, false
	}
	if cm.AllElements != nil {
		return nil, false
	}

	if !cm.IsSimpleSequence {
		return nil, false
	}

	decls := make([]*grammar.CompiledElement, len(children))
	childIdx := 0
	for _, particle := range cm.SimpleSequence {
		if particle == nil || particle.Element == nil {
			return nil, false
		}
		expectedQName := r.effectiveElementQName(particle.Element)
		min := particle.MinOccurs
		max := particle.MaxOccurs
		count := 0

		for childIdx < len(children) && (max == types.UnboundedOccurs || count < max) {
			childQName := r.resolveElementQName(children[childIdx])
			if childQName == expectedQName || r.isSubstitutableQName(childQName, expectedQName) {
				decls[childIdx] = r.substitutionDeclForQName(expectedQName, childQName, particle.Element)
				childIdx++
				count++
			} else {
				break
			}
		}

		if count < min {
			return nil, false
		}
	}

	if childIdx != len(children) {
		return nil, false
	}

	return decls, true
}
